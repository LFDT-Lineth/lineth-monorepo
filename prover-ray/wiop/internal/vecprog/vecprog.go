// Package vecprog evaluates arithmetic expressions over vectors of field
// elements, many expressions and many points at a time.
//
// The expressions are lowered into one straight-line program that runs over
// blocks of points, instead of walking every expression tree once per point
// or materialising every intermediate as a full-length vector. Three rewrites
// make it cheaper than the trees it replaces, all decided once at build time:
//
//   - hash-consing: structurally equal subtrees, within an expression or
//     across the expressions of a program, are computed once per point;
//   - scalar folding: subtrees made only of scalars are evaluated once;
//   - linear forms: an extension-valued subtree that is affine in base-valued
//     subtrees with scalar coefficients, such as the Horner form
//     α + β·(c₁ + β·(c₂ + …)) of a random linear combination, becomes
//     c + Σ coefᵢ·atomᵢ. Each term then costs an extension-by-base product
//     instead of a full extension multiplication.
//
// Only additions, subtractions, negations, doublings and multiplications by a
// scalar are folded or redistributed. These are field identities, so the
// results are exactly those of a point-by-point evaluation of the trees.
// Divisions and inversions (where 0⁻¹ = 0 is not an identity) are kept as
// they are; they are evaluated by one batch inversion per block, which gives
// the same values, zero included.
//
// The points of a program are laid out as ratio blocks ("cosets") of n
// points; a table leaf is read cyclically within a coset at a fixed offset,
// and may hold step·ratio cosets, of which the program's coset k is coset
// k·step. A plain vector of n rows is the case ratio = step = 1.
package vecprog

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"slices"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

// BlockSize is the number of points a worker evaluates per program run.
// Registers hold one block each, so the working set stays in cache.
const BlockSize = 256

// Op is an arithmetic operator.
type Op uint8

// Operators. Add, Sub, Mul and Div are binary, the others unary.
const (
	Add Op = iota
	Sub
	Mul
	Div
	Double
	Square
	Negate
	Inverse
)

// Arity is the number of operands o takes.
func (o Op) Arity() int {
	switch o {
	case Add, Sub, Mul, Div:
		return 2
	}
	return 1
}

// kind discriminates the nodes of a program.
type kind uint8

const (
	kScalar  kind = iota // build-time value, in B0.A0 when isBase
	kVecBase             // base-field table, read at an offset
	kVecExt              // extension-field table, read at an offset
	kOp                  // arithmetic node over other nodes
	kLinear              // s + Σ coefs[i]·atoms[i], atoms base-valued
)

// node is a hash-consed node of a program.
type node struct {
	kind    kind
	isBase  bool
	op      Op
	args    []int           // kOp
	s       field.Ext       // kScalar: the value; kLinear: the constant term
	vecBase []field.Element // kVecBase
	vecExt  []field.Ext     // kVecExt
	offset  int             // kVec*: offset within a coset, in [0, n)
	padded  bool            // kVec*: the table is a padded single coset, see BasePadded
	pad     field.Element   // padded: the value of rows outside the data
	start   int             // padded: the row of the table's first element
	atoms   []int           // kLinear, increasing
	coefs   []field.Ext     // kLinear, aligned with atoms, non-zero
}

// key identifies a node up to structural equality: leaves by table identity
// and offset, scalars by value, inner nodes by their (already interned)
// operands. A linear form's variable-length part enters through a hash, so
// nodes sharing a key are compared in full by [node.equal].
type key struct {
	kind   kind
	isBase bool
	op     Op
	a0, a1 int
	table  uintptr
	offset int
	s      field.Ext
	hash   uint64
}

func (n *node) key() key {
	k := key{kind: n.kind, isBase: n.isBase, s: n.s, a0: -1, a1: -1}
	if n.padded {
		k.a1, k.s, k.hash = n.start, field.Lift(n.pad), 1
	}
	switch n.kind {
	case kVecBase:
		k.table, k.offset = uintptr(unsafe.Pointer(unsafe.SliceData(n.vecBase))), n.offset
		k.a0 = len(n.vecBase)
	case kVecExt:
		k.table, k.offset = uintptr(unsafe.Pointer(unsafe.SliceData(n.vecExt))), n.offset
		k.a0 = len(n.vecExt)
	case kOp:
		k.op, k.a0 = n.op, n.args[0]
		if len(n.args) > 1 {
			k.a1 = n.args[1]
		}
	case kLinear:
		h := fnv.New64a()
		var buf [8]byte
		for i, a := range n.atoms {
			binary.LittleEndian.PutUint64(buf[:], uint64(a))
			h.Write(buf[:])
			for _, limb := range asLimbs(n.coefs[i : i+1]) {
				binary.LittleEndian.PutUint32(buf[:4], limb[0])
				h.Write(buf[:4])
			}
		}
		k.hash = h.Sum64()
	}
	return k
}

// equal reports whether two nodes of the same key are the same linear form;
// other kinds are fully determined by their key.
func (n *node) equal(m *node) bool {
	return n.kind != kLinear || (slices.Equal(n.atoms, m.atoms) && slices.Equal(n.coefs, m.coefs))
}

// deps returns the nodes n reads.
func (n *node) deps() []int {
	if n.kind == kLinear {
		return n.atoms
	}
	return n.args
}

// Builder interns the nodes and sinks of a program under construction.
type Builder struct {
	nodes []node
	index map[key][]int
	sinks []sink
}

// NewBuilder returns an empty builder.
func NewBuilder() *Builder {
	return &Builder{index: make(map[key][]int)}
}

func (b *Builder) intern(n node) int {
	key := n.key()
	for _, id := range b.index[key] {
		if b.nodes[id].equal(&n) {
			return id
		}
	}
	id := len(b.nodes)
	b.nodes = append(b.nodes, n)
	b.index[key] = append(b.index[key], id)
	return id
}

// IsBase reports whether node id is base-field valued.
func (b *Builder) IsBase(id int) bool { return b.nodes[id].isBase }

// Base returns the leaf reading a base-field table at offset, within each
// coset.
func (b *Builder) Base(table []field.Element, offset int) int {
	return b.intern(node{kind: kVecBase, isBase: true, vecBase: table, offset: offset})
}

// BasePadded returns the leaf reading, at offset, a single coset of n rows
// whose rows [start, start+len(data)) hold data and the others hold pad, as a
// padded column assignment is laid out: the padded rows are never written
// down. A start below zero drops the first -start data rows.
func (b *Builder) BasePadded(data []field.Element, pad field.Element, start, offset int) int {
	return b.intern(node{
		kind: kVecBase, isBase: true, vecBase: data, offset: offset,
		padded: true, pad: pad, start: start,
	})
}

// ExtPadded is [Builder.BasePadded] for extension data; the padding value is
// a lifted base element.
func (b *Builder) ExtPadded(data []field.Ext, pad field.Element, start, offset int) int {
	return b.intern(node{kind: kVecExt, vecExt: data, offset: offset, padded: true, pad: pad, start: start})
}

// Ext returns the leaf reading an extension-field table at offset, within
// each coset.
func (b *Builder) Ext(table []field.Ext, offset int) int {
	return b.intern(node{kind: kVecExt, vecExt: table, offset: offset})
}

// Scalar returns the constant v; when isBase, v must be a lifted base element.
func (b *Builder) Scalar(v field.Ext, isBase bool) int {
	return b.intern(node{kind: kScalar, isBase: isBase, s: v})
}

// Op returns o applied to args, folding scalars and linear forms. Nodes of
// a base-valued operation are evaluated in the base field; mixed operations
// lift their base operands at the boundary.
func (b *Builder) Op(o Op, args ...int) int {
	if len(args) != o.Arity() {
		panic(fmt.Sprintf("vecprog: operator %d takes %d operands, got %d", o, o.Arity(), len(args)))
	}
	args = slices.Clone(args)
	allScalar, allBase := true, true
	for _, a := range args {
		allScalar = allScalar && b.nodes[a].kind == kScalar
		allBase = allBase && b.nodes[a].isBase
	}
	if allScalar {
		vals := make([]field.Ext, len(args))
		for i, a := range args {
			vals[i] = b.nodes[a].s
		}
		if allBase {
			return b.Scalar(field.Lift(applyBase(o, liftedBases(vals))), true)
		}
		return b.Scalar(applyExt(o, vals), false)
	}
	if allBase {
		return b.intern(node{kind: kOp, isBase: true, op: o, args: args})
	}

	switch o {
	case Add, Sub:
		if b.linearizable(args[0]) && b.linearizable(args[1]) {
			r := b.linForm(args[1])
			if o == Sub {
				r.scale(extMinusOne())
			}
			return b.linear(b.linForm(args[0]).plus(r))
		}
	case Negate:
		if b.linearizable(args[0]) {
			l := b.linForm(args[0])
			l.scale(extMinusOne())
			return b.linear(l)
		}
	case Double:
		if b.linearizable(args[0]) {
			l := b.linForm(args[0])
			var two field.Ext
			two.SetOne()
			two.Double(&two)
			l.scale(two)
			return b.linear(l)
		}
	case Mul:
		for k := range 2 {
			s, other := args[k], args[1-k]
			if b.nodes[s].kind == kScalar && b.linearizable(other) {
				l := b.linForm(other)
				l.scale(b.nodes[s].s)
				return b.linear(l)
			}
		}
	}
	return b.intern(node{kind: kOp, op: o, args: args})
}

// linearizable reports whether node id can take part in a linear form: as
// its constant (a scalar), as an atom (a base-valued node), or by merging (a
// linear form).
func (b *Builder) linearizable(id int) bool {
	n := &b.nodes[id]
	return n.kind == kScalar || n.isBase || n.kind == kLinear
}

// linForm returns a fresh copy of node id as a linear form; id must be
// linearizable.
func (b *Builder) linForm(id int) linForm {
	n := &b.nodes[id]
	switch n.kind {
	case kScalar:
		return linForm{c: n.s}
	case kLinear:
		return linForm{c: n.s, atoms: slices.Clone(n.atoms), coefs: slices.Clone(n.coefs)}
	}
	var one field.Ext
	one.SetOne()
	return linForm{atoms: []int{id}, coefs: []field.Ext{one}}
}

// linear interns l without its zero terms, as a scalar when none is left.
func (b *Builder) linear(l linForm) int {
	n := node{kind: kLinear, s: l.c}
	for i := range l.atoms {
		if !l.coefs[i].IsZero() {
			n.atoms = append(n.atoms, l.atoms[i])
			n.coefs = append(n.coefs, l.coefs[i])
		}
	}
	if len(n.atoms) == 0 {
		return b.Scalar(l.c, false)
	}
	return b.intern(n)
}

// linForm is c + Σ coefs[i]·atoms[i] during construction, atoms increasing.
type linForm struct {
	c     field.Ext
	atoms []int
	coefs []field.Ext
}

// plus returns l + r, merging the atoms of both.
func (l linForm) plus(r linForm) linForm {
	out := linForm{
		atoms: make([]int, 0, len(l.atoms)+len(r.atoms)),
		coefs: make([]field.Ext, 0, len(l.atoms)+len(r.atoms)),
	}
	out.c.Add(&l.c, &r.c)
	i, j := 0, 0
	for i < len(l.atoms) || j < len(r.atoms) {
		switch {
		case j == len(r.atoms) || (i < len(l.atoms) && l.atoms[i] < r.atoms[j]):
			out.atoms, out.coefs = append(out.atoms, l.atoms[i]), append(out.coefs, l.coefs[i])
			i++
		case i == len(l.atoms) || r.atoms[j] < l.atoms[i]:
			out.atoms, out.coefs = append(out.atoms, r.atoms[j]), append(out.coefs, r.coefs[j])
			j++
		default:
			var sum field.Ext
			sum.Add(&l.coefs[i], &r.coefs[j])
			out.atoms, out.coefs = append(out.atoms, l.atoms[i]), append(out.coefs, sum)
			i++
			j++
		}
	}
	return out
}

// scale multiplies l by s in place.
func (l *linForm) scale(s field.Ext) {
	l.c.Mul(&l.c, &s)
	for i := range l.coefs {
		l.coefs[i].Mul(&l.coefs[i], &s)
	}
}

func extMinusOne() field.Ext {
	var m field.Ext
	m.SetOne()
	m.Neg(&m)
	return m
}

func liftedBases(vals []field.Ext) []field.Element {
	out := make([]field.Element, len(vals))
	for i := range vals {
		out[i] = vals[i].B0.A0
	}
	return out
}

// applyBase applies o to base-field operands.
func applyBase(o Op, a []field.Element) field.Element {
	var res field.Element
	switch o {
	case Add:
		res.Add(&a[0], &a[1])
	case Sub:
		res.Sub(&a[0], &a[1])
	case Mul:
		res.Mul(&a[0], &a[1])
	case Div:
		var inv field.Element
		inv.Inverse(&a[1])
		res.Mul(&a[0], &inv)
	case Double:
		res.Add(&a[0], &a[0])
	case Square:
		res.Square(&a[0])
	case Negate:
		res.Neg(&a[0])
	case Inverse:
		res.Inverse(&a[0])
	default:
		panic(fmt.Sprintf("vecprog: unknown operator %d", o))
	}
	return res
}

// applyExt applies o to extension-field operands.
func applyExt(o Op, a []field.Ext) field.Ext {
	var res field.Ext
	switch o {
	case Add:
		res.Add(&a[0], &a[1])
	case Sub:
		res.Sub(&a[0], &a[1])
	case Mul:
		res.Mul(&a[0], &a[1])
	case Div:
		var inv field.Ext
		inv.Inverse(&a[1])
		res.Mul(&a[0], &inv)
	case Double:
		res.Double(&a[0])
	case Square:
		res.Square(&a[0])
	case Negate:
		res.Neg(&a[0])
	case Inverse:
		res.Inverse(&a[0])
	default:
		panic(fmt.Sprintf("vecprog: unknown operator %d", o))
	}
	return res
}

// sinkKind discriminates how a program consumes a root.
type sinkKind uint8

const (
	sinkAccumulate sinkKind = iota // aggregate += coef · root · cancellation
	sinkStore                      // dst = root
	sinkAddTo                      // dst += root
	sinkCheck                      // onZero(t) where root = 0 and guard ≠ 0
)

type sink struct {
	kind         sinkKind
	root         int
	guard        int // sinkCheck
	cancellation []field.Element
	coef         field.Ext
	dst          []field.Ext
	onZero       func(t int)
}

// Accumulate adds coef · root · cancellation into the aggregate passed to
// [Program.Run]; a nil cancellation stands for 1. The cancellation is laid
// out like the tables.
func (b *Builder) Accumulate(root int, cancellation []field.Element, coef field.Ext) {
	b.sinks = append(b.sinks, sink{kind: sinkAccumulate, root: root, cancellation: cancellation, coef: coef})
}

// Store writes root, in the extension field, into dst (one entry per point).
func (b *Builder) Store(root int, dst []field.Ext) {
	b.sinks = append(b.sinks, sink{kind: sinkStore, root: root, dst: dst})
}

// AddTo adds root, in the extension field, into dst (one entry per point).
// Sinks into the same dst run in their order of declaration.
func (b *Builder) AddTo(root int, dst []field.Ext) {
	b.sinks = append(b.sinks, sink{kind: sinkAddTo, root: root, dst: dst})
}

// Check calls onZero(t) at every point t where root is zero and guard is
// not. onZero is expected to panic; it may be called concurrently.
func (b *Builder) Check(root, guard int, onZero func(t int)) {
	b.sinks = append(b.sinks, sink{kind: sinkCheck, root: root, guard: guard, onZero: onZero})
}

// Program is the lowered form of a builder's sinks.
type Program struct {
	nodes    []node
	steps    []step
	reg      []int // computed node -> register of its kind
	leaves   []int // kVec* nodes read by the program
	sinks    []sink
	nbBase   int
	nbExt    int
	n, ratio int
	stride   int // table entries between the program's consecutive cosets
}

// step computes node, or runs sink when sink >= 0.
type step struct {
	node int
	sink int
}

// Compile lowers the sinks for ratio cosets of n points, the program's coset
// k being coset k·cosetStep of the tables. Each sink's nodes are scheduled
// depth first right before it, and registers are recycled after their last
// use.
func (b *Builder) Compile(n, ratio, cosetStep int) *Program {
	p := &Program{nodes: b.nodes, sinks: b.sinks, n: n, ratio: ratio, stride: cosetStep * n}

	emitted := make([]bool, len(p.nodes))
	var emit func(id int)
	emit = func(id int) {
		if emitted[id] {
			return
		}
		emitted[id] = true
		nd := &p.nodes[id]
		switch nd.kind {
		case kVecBase, kVecExt:
			p.leaves = append(p.leaves, id)
			return
		case kScalar:
			return
		}
		for _, a := range nd.deps() {
			emit(a)
		}
		p.steps = append(p.steps, stepOf(id, -1))
	}
	for i := range p.sinks {
		emit(p.sinks[i].root)
		if p.sinks[i].kind == sinkCheck {
			emit(p.sinks[i].guard)
		}
		p.steps = append(p.steps, stepOf(p.sinks[i].root, i))
	}

	reads := func(st step) []int {
		if st.sink < 0 {
			return p.nodes[st.node].deps()
		}
		if sk := &p.sinks[st.sink]; sk.kind == sinkCheck {
			return []int{sk.root, sk.guard}
		}
		return []int{st.node}
	}
	lastUse := make([]int, len(p.nodes))
	for s, st := range p.steps {
		for _, a := range reads(st) {
			lastUse[a] = s
		}
	}

	p.reg = make([]int, len(p.nodes))
	var freeBase, freeExt []int
	release := func(id int) {
		nd := &p.nodes[id]
		if nd.kind != kOp && nd.kind != kLinear {
			return
		}
		if nd.isBase {
			freeBase = append(freeBase, p.reg[id])
		} else {
			freeExt = append(freeExt, p.reg[id])
		}
	}
	for s, st := range p.steps {
		if st.sink < 0 {
			nd := &p.nodes[st.node]
			switch {
			case nd.isBase && len(freeBase) > 0:
				p.reg[st.node], freeBase = freeBase[len(freeBase)-1], freeBase[:len(freeBase)-1]
			case nd.isBase:
				p.reg[st.node] = p.nbBase
				p.nbBase++
			case len(freeExt) > 0:
				p.reg[st.node], freeExt = freeExt[len(freeExt)-1], freeExt[:len(freeExt)-1]
			default:
				p.reg[st.node] = p.nbExt
				p.nbExt++
			}
		}
		// Operands are released after the destination is taken, so a step
		// never writes into a register it reads.
		rd := reads(st)
		for i, a := range rd {
			if lastUse[a] == s && !slices.Contains(rd[:i], a) {
				release(a)
			}
		}
	}
	return p
}

func stepOf(node, sink int) step { return step{node: node, sink: sink} }

// Run evaluates the program on every point. Accumulate sinks add into
// aggregate (nil when there are none), which is then multiplied by scale[k]
// on coset k when scale is not nil. The blocks are spread over up to
// workers goroutines (all CPUs when workers ≤ 0). A block never straddles
// two cosets.
func (p *Program) Run(aggregate []field.Ext, scale []field.Element, workers int) {
	perCoset := (p.n + BlockSize - 1) / BlockSize
	work := func(first, last int) {
		w := p.newWorker()
		for blk := first; blk < last; blk++ {
			k, i0 := blk/perCoset, (blk%perCoset)*BlockSize
			L := min(BlockSize, p.n-i0)
			start := k*p.n + i0
			var agg []field.Ext
			if aggregate != nil {
				agg = aggregate[start : start+L]
			}
			w.runBlock(agg, k, i0, L)
			if scale != nil {
				out := extensions.VectorE6(agg)
				out.ScalarMulByElement(agg, &scale[k])
			}
		}
	}
	if workers > 0 {
		parallel.Execute(p.ratio*perCoset, work, workers)
	} else {
		parallel.Execute(p.ratio*perCoset, work)
	}
}

// worker holds one worker's registers and leaf views.
type worker struct {
	p        *Program
	baseRegs [][]field.Element
	extRegs  [][]field.Ext
	// Per-block view of each leaf, indexed by node; wrap holds the leaf's copy
	// when its window wraps around the end of its coset.
	viewBase [][]field.Element
	viewExt  [][]field.Ext
	wrapBase [][]field.Element
	wrapExt  [][]field.Ext
	tmpBase  []field.Element
	tmpExt   []field.Ext
	tmpExt2  []field.Ext
	invBase  []field.Element // batch-inversion prefix products
	invExt   []field.Ext
}

func (p *Program) newWorker() *worker {
	w := &worker{
		p:        p,
		baseRegs: make([][]field.Element, p.nbBase),
		extRegs:  make([][]field.Ext, p.nbExt),
		viewBase: make([][]field.Element, len(p.nodes)),
		viewExt:  make([][]field.Ext, len(p.nodes)),
		wrapBase: make([][]field.Element, len(p.nodes)),
		wrapExt:  make([][]field.Ext, len(p.nodes)),
		tmpBase:  make([]field.Element, BlockSize),
		tmpExt:   make([]field.Ext, BlockSize),
		tmpExt2:  make([]field.Ext, BlockSize),
		invBase:  make([]field.Element, BlockSize),
		invExt:   make([]field.Ext, BlockSize),
	}
	baseSlab := make([]field.Element, p.nbBase*BlockSize)
	for i := range w.baseRegs {
		w.baseRegs[i] = baseSlab[i*BlockSize : (i+1)*BlockSize]
	}
	extSlab := make([]field.Ext, p.nbExt*BlockSize)
	for i := range w.extRegs {
		w.extRegs[i] = extSlab[i*BlockSize : (i+1)*BlockSize]
	}
	return w
}

// runBlock evaluates the program over points [i0, i0+L) of coset k and runs
// every sink; agg is the block's slice of the aggregate.
func (w *worker) runBlock(agg []field.Ext, k, i0, L int) {
	p := w.p
	tableStart := k * p.stride // the program's coset k in the tables
	for _, id := range p.leaves {
		nd := &p.nodes[id]
		idx := i0 + nd.offset
		if idx >= p.n {
			idx -= p.n
		}
		switch {
		case nd.padded && nd.kind == kVecBase:
			w.viewBase[id] = paddedView(nd.vecBase, nd.pad, nd.start, p.n, idx, L, &w.wrapBase[id])
		case nd.padded:
			w.viewExt[id] = paddedView(nd.vecExt, field.Lift(nd.pad), nd.start, p.n, idx, L, &w.wrapExt[id])
		case nd.kind == kVecBase:
			w.viewBase[id] = wrappedView(nd.vecBase[tableStart:tableStart+p.n], idx, L, &w.wrapBase[id])
		default:
			w.viewExt[id] = wrappedView(nd.vecExt[tableStart:tableStart+p.n], idx, L, &w.wrapExt[id])
		}
	}
	point := k*p.n + i0 // the block's first point in the program's layout
	for _, st := range p.steps {
		if st.sink >= 0 {
			w.sink(&p.sinks[st.sink], agg, tableStart+i0, point, L)
			continue
		}
		nd := &p.nodes[st.node]
		switch {
		case nd.kind == kLinear:
			w.linear(nd, w.extRegs[p.reg[st.node]][:L])
		case nd.isBase:
			w.baseOp(nd, w.baseRegs[p.reg[st.node]][:L])
		default:
			w.extOp(nd, w.extRegs[p.reg[st.node]][:L])
		}
	}
}

// wrappedView returns table[idx : idx+L] cyclically, copying into *buf only
// when the window wraps around the end of the table (a coset).
func wrappedView[T any](table []T, idx, L int, buf *[]T) []T {
	if idx+L <= len(table) {
		return table[idx : idx+L]
	}
	if *buf == nil {
		*buf = make([]T, BlockSize)
	}
	out := (*buf)[:L]
	k := copy(out, table[idx:])
	copy(out[k:], table)
	return out
}

// paddedView returns rows [idx, idx+L) of a padded coset of n rows, cyclically
// (see [Builder.BasePadded]): a slice of data when the window lies within
// it, otherwise a copy into *buf with the padded rows filled in.
func paddedView[T any](data []T, pad T, start, n, idx, L int, buf *[]T) []T {
	if idx >= start && idx+L <= start+len(data) {
		return data[idx-start : idx-start+L]
	}
	if *buf == nil {
		*buf = make([]T, BlockSize)
	}
	out := (*buf)[:L]
	for o := 0; o < L; {
		p := (idx + o) % n
		length := min(L-o, n-p) // a run of rows that does not wrap
		dLo, dHi := max(p, start), min(p+length, start+len(data))
		if dLo >= dHi {
			fill(out[o:o+length], pad)
		} else {
			fill(out[o:o+dLo-p], pad)
			copy(out[o+dLo-p:o+dHi-p], data[dLo-start:dHi-start])
			fill(out[o+dHi-p:o+length], pad)
		}
		o += length
	}
	return out
}

func fill[T any](v []T, x T) {
	for i := range v {
		v[i] = x
	}
}

// operand is a node's value over the current block: a scalar or a vector of
// its field.
type operand struct {
	scalar bool
	isBase bool
	s      field.Ext
	base   []field.Element
	ext    []field.Ext
}

func (o *operand) baseAt(i int) field.Element {
	if o.scalar {
		return o.s.B0.A0
	}
	return o.base[i]
}

func (o *operand) extAt(i int) field.Ext {
	switch {
	case o.scalar:
		return o.s
	case o.isBase:
		return field.Lift(o.base[i])
	}
	return o.ext[i]
}

func (w *worker) operand(id, L int) operand {
	nd := &w.p.nodes[id]
	switch nd.kind {
	case kScalar:
		return operand{scalar: true, isBase: nd.isBase, s: nd.s}
	case kVecBase:
		return operand{isBase: true, base: w.viewBase[id]}
	case kVecExt:
		return operand{ext: w.viewExt[id]}
	}
	if nd.isBase {
		return operand{isBase: true, base: w.baseRegs[w.p.reg[id]][:L]}
	}
	return operand{ext: w.extRegs[w.p.reg[id]][:L]}
}

// extVector returns o as an extension vector of length L, lifting or
// broadcasting into buf when it is not one already.
func extVector(o *operand, L int, buf []field.Ext) []field.Ext {
	if !o.scalar && !o.isBase {
		return o.ext
	}
	out := buf[:L]
	for i := range out {
		out[i] = o.extAt(i)
	}
	return out
}

func (w *worker) linear(nd *node, dst []field.Ext) {
	for i := range dst {
		dst[i] = nd.s
	}
	acc := extensions.VectorE6(dst)
	for i, a := range nd.atoms {
		atom := w.operand(a, len(dst))
		acc.ScalarMulAccByElement(atom.base, &nd.coefs[i])
	}
}

func (w *worker) baseOp(nd *node, dst []field.Element) {
	L := len(dst)
	a := w.operand(nd.args[0], L)
	var b operand
	if len(nd.args) > 1 {
		b = w.operand(nd.args[1], L)
	}
	out := field.Vector(dst)
	switch {
	case nd.op == Add && !a.scalar && !b.scalar:
		out.Add(a.base, b.base)
		return
	case nd.op == Sub && !a.scalar && !b.scalar:
		out.Sub(a.base, b.base)
		return
	case nd.op == Mul && !a.scalar && !b.scalar:
		out.Mul(a.base, b.base)
		return
	case nd.op == Mul && b.scalar:
		s := b.s.B0.A0
		out.ScalarMul(a.base, &s)
		return
	case nd.op == Mul && a.scalar:
		s := a.s.B0.A0
		out.ScalarMul(b.base, &s)
		return
	case nd.op == Inverse && !a.scalar:
		batchInvertBase(a.base, dst, w.invBase[:L])
		return
	case nd.op == Div && !b.scalar:
		batchInvertBase(b.base, w.tmpBase[:L], w.invBase[:L])
		if a.scalar {
			s := a.s.B0.A0
			out.ScalarMul(w.tmpBase[:L], &s)
		} else {
			out.Mul(a.base, w.tmpBase[:L])
		}
		return
	}
	var args [2]field.Element
	for i := range dst {
		args[0] = a.baseAt(i)
		if len(nd.args) > 1 {
			args[1] = b.baseAt(i)
		}
		dst[i] = applyBase(nd.op, args[:len(nd.args)])
	}
}

func (w *worker) extOp(nd *node, dst []field.Ext) {
	L := len(dst)
	a := w.operand(nd.args[0], L)
	var b operand
	if len(nd.args) > 1 {
		b = w.operand(nd.args[1], L)
	}
	out := extensions.VectorE6(dst)
	vecExt := func(o *operand) bool { return !o.scalar && !o.isBase }
	vecBase := func(o *operand) bool { return !o.scalar && o.isBase }

	switch nd.op {
	case Add, Sub:
		sub := nd.op == Sub
		switch {
		case vecExt(&a) && vecExt(&b):
			limbs := asLimbs(dst)
			if sub {
				limbs.Sub(asLimbs(a.ext), asLimbs(b.ext))
			} else {
				limbs.Add(asLimbs(a.ext), asLimbs(b.ext))
			}
			return
		case vecExt(&a) && vecBase(&b):
			copy(dst, a.ext)
			for i := range dst {
				if sub {
					dst[i].B0.A0.Sub(&dst[i].B0.A0, &b.base[i])
				} else {
					dst[i].B0.A0.Add(&dst[i].B0.A0, &b.base[i])
				}
			}
			return
		}
	case Mul:
		for range 2 {
			switch {
			case vecExt(&a) && vecExt(&b):
				for i := range dst {
					dst[i].Mul(&a.ext[i], &b.ext[i])
				}
				return
			case vecExt(&a) && vecBase(&b):
				out.MulByElement(a.ext, b.base)
				return
			case vecExt(&a) && b.scalar && b.isBase:
				out.ScalarMulByElement(a.ext, &b.s.B0.A0)
				return
			case vecExt(&a) && b.scalar:
				out.ScalarMul(a.ext, &b.s)
				return
			case vecBase(&a) && b.scalar:
				clear(dst)
				out.ScalarMulAccByElement(a.base, &b.s)
				return
			}
			a, b = b, a
		}
	case Inverse:
		batchInvertExt(extVector(&a, L, w.tmpExt), dst, w.invExt[:L])
		return
	case Div:
		inv := w.tmpExt2[:L]
		batchInvertExt(extVector(&b, L, w.tmpExt), inv, w.invExt[:L])
		switch {
		case vecExt(&a):
			for i := range dst {
				dst[i].Mul(&a.ext[i], &inv[i])
			}
		case vecBase(&a):
			out.MulByElement(inv, a.base)
		case a.isBase:
			out.ScalarMulByElement(inv, &a.s.B0.A0)
		default:
			out.ScalarMul(inv, &a.s)
		}
		return
	}
	var args [2]field.Ext
	for i := range dst {
		args[0] = a.extAt(i)
		if len(nd.args) > 1 {
			args[1] = b.extAt(i)
		}
		dst[i] = applyExt(nd.op, args[:len(nd.args)])
	}
}

// batchInvertBase writes 1/a[i] into dst[i], and 0 where a[i] is 0, with one
// field inversion; prefix is scratch of the same length. dst may alias
// neither a nor prefix.
func batchInvertBase(a, dst, prefix []field.Element) {
	var acc field.Element
	acc.SetOne()
	for i := range a {
		prefix[i] = acc
		if !a[i].IsZero() {
			acc.Mul(&acc, &a[i])
		}
	}
	acc.Inverse(&acc)
	for i := len(a) - 1; i >= 0; i-- {
		if a[i].IsZero() {
			dst[i] = field.Element{}
			continue
		}
		dst[i].Mul(&acc, &prefix[i])
		acc.Mul(&acc, &a[i])
	}
}

// batchInvertExt is [batchInvertBase] in the extension field.
func batchInvertExt(a, dst, prefix []field.Ext) {
	var acc field.Ext
	acc.SetOne()
	for i := range a {
		prefix[i] = acc
		if !a[i].IsZero() {
			acc.Mul(&acc, &a[i])
		}
	}
	acc.Inverse(&acc)
	for i := len(a) - 1; i >= 0; i-- {
		if a[i].IsZero() {
			dst[i] = field.Ext{}
			continue
		}
		dst[i].Mul(&acc, &prefix[i])
		acc.Mul(&acc, &a[i])
	}
}

// sink consumes a root over the block: tablePos is the block's first entry in
// the tables' layout, point its first point in the program's layout.
func (w *worker) sink(sk *sink, agg []field.Ext, tablePos, point, L int) {
	root := w.operand(sk.root, L)
	switch sk.kind {
	case sinkStore:
		copy(sk.dst[point:point+L], extVector(&root, L, w.tmpExt))
		return
	case sinkAddTo:
		dst := sk.dst[point : point+L]
		if root.isBase && !root.scalar {
			for i := range dst {
				dst[i].B0.A0.Add(&dst[i].B0.A0, &root.base[i])
			}
			return
		}
		limbs := asLimbs(dst)
		limbs.Add(limbs, asLimbs(extVector(&root, L, w.tmpExt)))
		return
	case sinkCheck:
		guard := w.operand(sk.guard, L)
		for i := range L {
			r, g := root.extAt(i), guard.extAt(i)
			if r.IsZero() && !g.IsZero() {
				sk.onZero(point + i)
			}
		}
		return
	}

	acc := extensions.VectorE6(agg)
	if root.scalar {
		// A constant expression: broadcast it once.
		if root.isBase {
			for i := range L {
				w.tmpBase[i] = root.s.B0.A0
			}
			root = operand{isBase: true, base: w.tmpBase[:L]}
		} else {
			for i := range L {
				w.tmpExt[i] = root.s
			}
			root = operand{ext: w.tmpExt[:L]}
		}
	}
	var cancel []field.Element
	if sk.cancellation != nil {
		cancel = sk.cancellation[tablePos : tablePos+L]
	}
	if root.isBase {
		vals := root.base
		if cancel != nil {
			tmp := field.Vector(w.tmpBase[:L])
			tmp.Mul(vals, cancel)
			vals = tmp
		}
		acc.ScalarMulAccByElement(vals, &sk.coef)
		return
	}
	vals := root.ext
	if cancel != nil {
		tmp := extensions.VectorE6(w.tmpExt[:L])
		tmp.MulByElement(vals, cancel)
		vals = tmp
	}
	acc.ScalarMulAcc(vals, &sk.coef)
}

// asLimbs views extension elements as their contiguous base-field limbs.
func asLimbs(v []field.Ext) field.Vector {
	if len(v) == 0 {
		return nil
	}
	return unsafe.Slice((*field.Element)(unsafe.Pointer(&v[0])), 6*len(v))
}
