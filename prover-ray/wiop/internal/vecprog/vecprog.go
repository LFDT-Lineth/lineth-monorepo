// Package vecprog evaluates arithmetic expressions over vectors of field
// elements, many expressions and many points at a time.
//
// The expressions are lowered into one straight-line program that runs over
// blocks of points: each operation is applied to a block before the next
// one, and intermediates only live for a block. Three rewrites, all decided
// once at build time, reduce the work per point:
//
//   - hash-consing: structurally equal subtrees, within an expression or
//     across the expressions of a program, are computed once per point;
//   - scalar folding: subtrees made only of scalars are evaluated once;
//   - linear forms: an extension-valued subtree that is affine in base-valued
//     subtrees with scalar coefficients, such as the Horner form
//     α + β·(c₁ + β·(c₂ + …)) of a random linear combination, becomes
//     c + Σ coefᵢ·atomᵢ. Each term then costs one extension-by-base
//     product.
//
// Only additions, subtractions, negations, doublings and multiplications by a
// scalar are folded or redistributed. These are field identities, so the
// results are exactly those of a point-by-point evaluation of the trees.
// Divisions and inversions (where 0⁻¹ = 0 is not an identity) are kept as
// they are. A vector divisor or inverse is evaluated by one batch inversion
// per block, a base-field division by a scalar element by element; both give
// the same values as the trees, zero included.
//
// The points of a program are laid out as ratio blocks ("cosets") of n
// points; a table leaf is read cyclically within a coset at a fixed offset,
// and may hold step·ratio cosets, of which the program's coset k is coset
// k·step. A plain vector of n rows is the case ratio = step = 1.
//
// A program can also be built once and run many times on different data: a
// [Builder.Param] is a scalar and a [Builder.BaseSlot] or [Builder.ExtSlot] a
// table whose value is only given to [Program.RunWith], in [Bindings].
// Subtrees and linear-form coefficients over parameters are kept as scalar
// nodes, simplified only by exact identities (adding zero, multiplying by zero
// or one, ordering the operands of a commutative operation), and evaluated
// once per run, before the points; the program itself is never written after
// [Builder.Compile], so it may be run concurrently with different bindings.
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
	kScalar   kind = iota // build-time value, in B0.A0 when isBase
	kVecBase              // base-field table, read at an offset
	kVecExt               // extension-field table, read at an offset
	kOp                   // arithmetic node over other nodes
	kLinear               // c + Σ coefs[i]·atoms[i], atoms base-valued
	kParam                // scalar given at run time, in Bindings.Params[slot]
	kScalarOp             // arithmetic node over scalars, one of them a kParam
)

// node is a hash-consed node of a program.
type node struct {
	kind    kind
	isBase  bool
	op      Op
	args    []int           // kOp
	s       field.Ext       // kScalar: the value
	vecBase []field.Element // kVecBase
	vecExt  []field.Ext     // kVecExt
	offset  int             // kVec*: offset within a coset, in [0, n)
	padded  bool            // kVec*: the table is a padded single coset, see BasePadded
	pad     field.Element   // padded: the value of rows outside the data
	start   int             // padded: the row of the table's first element
	slotted bool            // kVec*: the table is Bindings.Base/Ext[slot]
	slot    int             // kParam, slotted kVec*: the index in Bindings
	c       coef            // kLinear: the constant term
	atoms   []int           // kLinear, increasing
	coefs   []coef          // kLinear, aligned with atoms, not constant zeros
}

// coef is a scalar of a linear form: the constant v, or the value of the
// scalar node id (a kParam or kScalarOp) when id >= 0.
type coef struct {
	id int
	v  field.Ext
}

func constCoef(v field.Ext) coef { return coef{id: -1, v: v} }

func (c coef) isZero() bool { return c.id < 0 && c.v.IsZero() }

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
	if n.slotted {
		k.a0, k.offset, k.hash = n.slot, n.offset, 2
		return k
	}
	switch n.kind {
	case kParam:
		k.a0 = n.slot
	case kScalarOp:
		k.op, k.a0 = n.op, n.args[0]
		if len(n.args) > 1 {
			k.a1 = n.args[1]
		}
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
		writeCoef := func(c coef) {
			binary.LittleEndian.PutUint64(buf[:], uint64(c.id))
			h.Write(buf[:])
			for _, limb := range asLimbs([]field.Ext{c.v}) {
				binary.LittleEndian.PutUint32(buf[:4], limb[0])
				h.Write(buf[:4])
			}
		}
		writeCoef(n.c)
		for i, a := range n.atoms {
			binary.LittleEndian.PutUint64(buf[:], uint64(a))
			h.Write(buf[:])
			writeCoef(n.coefs[i])
		}
		k.hash = h.Sum64()
	}
	return k
}

// equal reports whether two nodes of the same key are the same linear form;
// other kinds are fully determined by their key.
func (n *node) equal(m *node) bool {
	return n.kind != kLinear || (n.c == m.c && slices.Equal(n.atoms, m.atoms) && slices.Equal(n.coefs, m.coefs))
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

// isScalar reports whether node id has the same value at every point: a
// constant, a parameter, or an operation over those.
func (b *Builder) isScalar(id int) bool {
	k := b.nodes[id].kind
	return k == kScalar || k == kParam || k == kScalarOp
}

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

// Param returns the scalar Bindings.Params[slot] of each run; when isBase,
// the bound value must be a lifted base element.
func (b *Builder) Param(slot int, isBase bool) int {
	return b.intern(node{kind: kParam, isBase: isBase, slot: slot})
}

// BaseSlot returns the leaf reading, at offset within each coset, the
// base-field table Bindings.Base[slot] of each run.
func (b *Builder) BaseSlot(slot, offset int) int {
	return b.intern(node{kind: kVecBase, isBase: true, slotted: true, slot: slot, offset: offset})
}

// ExtSlot returns the leaf reading, at offset within each coset, the
// extension-field table Bindings.Ext[slot] of each run.
func (b *Builder) ExtSlot(slot, offset int) int {
	return b.intern(node{kind: kVecExt, slotted: true, slot: slot, offset: offset})
}

// Op returns o applied to args, folding scalars and linear forms. Nodes of
// a base-valued operation are evaluated in the base field; mixed operations
// lift their base operands at the boundary.
func (b *Builder) Op(o Op, args ...int) int {
	if len(args) != o.Arity() {
		panic(fmt.Sprintf("vecprog: operator %d takes %d operands, got %d", o, o.Arity(), len(args)))
	}
	args = slices.Clone(args)
	allConst, allScalar, allBase := true, true, true
	for _, a := range args {
		allConst = allConst && b.nodes[a].kind == kScalar
		allScalar = allScalar && b.isScalar(a)
		allBase = allBase && b.nodes[a].isBase
	}
	if allConst {
		vals := make([]field.Ext, len(args))
		for i, a := range args {
			vals[i] = b.nodes[a].s
		}
		if allBase {
			return b.Scalar(field.Lift(applyBase(o, liftedBases(vals))), true)
		}
		return b.Scalar(applyExt(o, vals), false)
	}
	if allScalar {
		// Over a parameter: evaluated once per run, before the points. The
		// operands of a commutative operation are ordered, so that equal
		// scalars built in either order share a node.
		if (o == Add || o == Mul) && args[0] > args[1] {
			args[0], args[1] = args[1], args[0]
		}
		return b.intern(node{kind: kScalarOp, isBase: allBase, op: o, args: args})
	}
	if allBase {
		return b.intern(node{kind: kOp, isBase: true, op: o, args: args})
	}

	switch o {
	case Add, Sub:
		if b.linearizable(args[0]) && b.linearizable(args[1]) {
			r := b.linForm(args[1])
			if o == Sub {
				b.scale(&r, constCoef(extMinusOne()))
			}
			return b.linear(b.plus(b.linForm(args[0]), r))
		}
	case Negate:
		if b.linearizable(args[0]) {
			l := b.linForm(args[0])
			b.scale(&l, constCoef(extMinusOne()))
			return b.linear(l)
		}
	case Double:
		if b.linearizable(args[0]) {
			l := b.linForm(args[0])
			var two field.Ext
			two.SetOne()
			two.Double(&two)
			b.scale(&l, constCoef(two))
			return b.linear(l)
		}
	case Mul:
		for k := range 2 {
			s, other := args[k], args[1-k]
			if b.isScalar(s) && b.linearizable(other) {
				l := b.linForm(other)
				b.scale(&l, b.coefOf(s))
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
	return b.isScalar(id) || n.isBase || n.kind == kLinear
}

// linForm returns a fresh copy of node id as a linear form; id must be
// linearizable.
func (b *Builder) linForm(id int) linForm {
	n := &b.nodes[id]
	switch {
	case b.isScalar(id):
		return linForm{c: b.coefOf(id)}
	case n.kind == kLinear:
		return linForm{c: n.c, atoms: slices.Clone(n.atoms), coefs: slices.Clone(n.coefs)}
	}
	var one field.Ext
	one.SetOne()
	return linForm{c: constCoef(field.Ext{}), atoms: []int{id}, coefs: []coef{constCoef(one)}}
}

// linear interns l without its constant zero terms, as a scalar when none is
// left.
func (b *Builder) linear(l linForm) int {
	n := node{kind: kLinear, c: l.c}
	for i := range l.atoms {
		if !l.coefs[i].isZero() {
			n.atoms = append(n.atoms, l.atoms[i])
			n.coefs = append(n.coefs, l.coefs[i])
		}
	}
	if len(n.atoms) == 0 {
		return b.coefNode(l.c)
	}
	return b.intern(n)
}

// coefOf returns scalar node id as a coefficient: its value when it is a
// constant.
func (b *Builder) coefOf(id int) coef {
	if n := &b.nodes[id]; n.kind == kScalar {
		return constCoef(n.s)
	}
	return coef{id: id}
}

// coefNode returns the scalar node of c.
func (b *Builder) coefNode(c coef) int {
	if c.id >= 0 {
		return c.id
	}
	return b.Scalar(c.v, false)
}

// coefOp applies o (Add or Mul) to two coefficients: in the field when both
// are constants, as a scalar node evaluated per run otherwise. Adding zero
// and multiplying by one return the other coefficient, and multiplying by
// zero returns zero, so that linear forms over parameters with equal
// coefficients built in different orders merge, as they do over constants.
func (b *Builder) coefOp(o Op, x, y coef) coef {
	if x.id < 0 && y.id < 0 {
		return constCoef(applyExt(o, []field.Ext{x.v, y.v}))
	}
	var one field.Ext
	one.SetOne()
	switch {
	case o == Add && x.isZero():
		return y
	case o == Add && y.isZero():
		return x
	case o == Mul && (x.isZero() || y.isZero()):
		return constCoef(field.Ext{})
	case o == Mul && x.id < 0 && x.v == one:
		return y
	case o == Mul && y.id < 0 && y.v == one:
		return x
	}
	return b.coefOf(b.Op(o, b.coefNode(x), b.coefNode(y)))
}

// linForm is c + Σ coefs[i]·atoms[i] during construction, atoms increasing.
type linForm struct {
	c     coef
	atoms []int
	coefs []coef
}

// plus returns l + r, merging the atoms of both.
func (b *Builder) plus(l, r linForm) linForm {
	out := linForm{
		c:     b.coefOp(Add, l.c, r.c),
		atoms: make([]int, 0, len(l.atoms)+len(r.atoms)),
		coefs: make([]coef, 0, len(l.atoms)+len(r.atoms)),
	}
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
			out.atoms = append(out.atoms, l.atoms[i])
			out.coefs = append(out.coefs, b.coefOp(Add, l.coefs[i], r.coefs[j]))
			i++
			j++
		}
	}
	return out
}

// scale multiplies l by s in place.
func (b *Builder) scale(l *linForm, s coef) {
	l.c = b.coefOp(Mul, l.c, s)
	for i := range l.coefs {
		l.coefs[i] = b.coefOp(Mul, l.coefs[i], s)
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
	kind  sinkKind
	root  int
	guard int // sinkCheck
	// cancelSlot (sinkAccumulate): the cancellation is Bindings.Base[cancelSlot]; none when < 0.
	cancelSlot int
	coef       coef
	dst        []field.Ext
	onZero     func(t int)
}

// AccumulateBy adds coef · root · cancellation into the aggregate passed to
// [Program.RunWith]. The coefficient is a scalar node, such as a power of a
// [Builder.Param], and the cancellation the table Bindings.Base[cancelSlot] of
// each run, laid out like the tables (none when cancelSlot < 0).
func (b *Builder) AccumulateBy(root, cancelSlot, coef int) {
	if !b.isScalar(coef) {
		panic("vecprog: AccumulateBy: the coefficient must be a scalar node")
	}
	b.sinks = append(b.sinks, sink{
		kind: sinkAccumulate, root: root, coef: b.coefOf(coef),
		cancelSlot: max(cancelSlot, -1),
	})
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
	// accumulates: some sink is an Accumulate. bound: the program reads
	// Bindings (a slotted table or a parameter).
	accumulates, bound bool
	// scalars holds the value of every constant node; perRun lists, in
	// increasing order, the parameter-dependent scalar nodes evaluated by
	// each run on a copy of it.
	scalars []field.Ext
	perRun  []int
}

// collectPerRun lists in perRun, in increasing order, the parameter-dependent
// scalars the steps and sinks read, directly or through other scalars: the
// builder also holds scalars that only served intermediate linear forms.
func (p *Program) collectPerRun() {
	used := make([]bool, len(p.nodes))
	var mark func(id int)
	mark = func(id int) {
		nd := &p.nodes[id]
		if used[id] || (nd.kind != kParam && nd.kind != kScalarOp) {
			return
		}
		used[id] = true
		for _, a := range nd.args {
			mark(a)
		}
	}
	markCoef := func(c coef) {
		if c.id >= 0 {
			mark(c.id)
		}
	}
	for _, st := range p.steps {
		if st.sink >= 0 {
			sk := &p.sinks[st.sink]
			mark(sk.root)
			if sk.kind == sinkCheck {
				mark(sk.guard)
			}
			markCoef(sk.coef)
			continue
		}
		nd := &p.nodes[st.node]
		for _, a := range nd.args {
			mark(a)
		}
		if nd.kind == kLinear {
			markCoef(nd.c)
			for _, c := range nd.coefs {
				markCoef(c)
			}
		}
	}
	for id, u := range used {
		if u {
			p.perRun = append(p.perRun, id)
		}
	}
	p.bound = p.bound || len(p.perRun) > 0
}

// Bindings are the data of one run of a program: the tables of its slotted
// leaves and the values of its parameters, indexed by slot.
type Bindings struct {
	Base   [][]field.Element
	Ext    [][]field.Ext
	Params []field.Ext
}

// Stats counts the nodes of a compiled program.
type Stats struct {
	Leaves  int // table leaves read
	Steps   int // nodes computed at every point
	Scalars int // parameter-dependent scalars computed once per run
}

// Stats returns the node counts of p.
func (p *Program) Stats() Stats {
	st := Stats{Leaves: len(p.leaves), Scalars: len(p.perRun)}
	for _, s := range p.steps {
		if s.sink < 0 {
			st.Steps++
		}
	}
	return st
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
	p.scalars = make([]field.Ext, len(p.nodes))
	for id := range p.nodes {
		if nd := &p.nodes[id]; nd.kind == kScalar {
			p.scalars[id] = nd.s
		}
	}

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
			p.bound = p.bound || nd.slotted
			if nd.padded {
				dataLen := len(nd.vecBase) + len(nd.vecExt)
				if nd.start+dataLen > n {
					panic(fmt.Sprintf("vecprog: padded leaf with %d rows from row %d overflows a coset of %d", dataLen, nd.start, n))
				}
			}
			p.leaves = append(p.leaves, id)
			return
		case kScalar, kParam, kScalarOp:
			return
		}
		for _, a := range nd.deps() {
			emit(a)
		}
		p.steps = append(p.steps, step{node: id, sink: -1})
	}
	for i := range p.sinks {
		p.accumulates = p.accumulates || p.sinks[i].kind == sinkAccumulate
		p.bound = p.bound || (p.sinks[i].kind == sinkAccumulate && p.sinks[i].cancelSlot >= 0)
		emit(p.sinks[i].root)
		if p.sinks[i].kind == sinkCheck {
			emit(p.sinks[i].guard)
		}
		p.steps = append(p.steps, step{node: p.sinks[i].root, sink: i})
	}
	p.collectPerRun()

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

// Run evaluates the program on every point. Accumulate sinks add into
// aggregate (nil when there are none), which is then multiplied by scale[k]
// on coset k when scale is not nil: the whole aggregate is scaled, so it is
// expected to start at zero. The blocks are spread over up to
// workers goroutines (all CPUs when workers ≤ 0). A block never straddles
// two cosets.
func (p *Program) Run(aggregate []field.Ext, scale []field.Element, workers int) {
	p.RunWith(nil, aggregate, scale, workers)
}

// RunWith is [Program.Run] with the slotted tables and parameters of bind
// (nil when the program has neither). The parameter-dependent scalars are
// evaluated once, before the points.
func (p *Program) RunWith(bind *Bindings, aggregate []field.Ext, scale []field.Element, workers int) {
	if p.accumulates && len(aggregate) < p.ratio*p.n {
		panic(fmt.Sprintf("vecprog: Run: aggregate has %d entries, the program accumulates into %d", len(aggregate), p.ratio*p.n))
	}
	if p.bound && bind == nil {
		panic("vecprog: Run: the program reads slotted tables or parameters; use RunWith")
	}
	scalars := p.scalars
	if len(p.perRun) > 0 {
		scalars = slices.Clone(p.scalars)
		for _, id := range p.perRun {
			nd := &p.nodes[id]
			if nd.kind == kParam {
				scalars[id] = bind.Params[nd.slot]
				continue
			}
			var vals [2]field.Ext
			for i, a := range nd.args {
				vals[i] = scalars[a]
			}
			if nd.isBase {
				var bases [2]field.Element
				for i := range nd.args {
					bases[i] = vals[i].B0.A0
				}
				scalars[id] = field.Lift(applyBase(nd.op, bases[:len(nd.args)]))
			} else {
				scalars[id] = applyExt(nd.op, vals[:len(nd.args)])
			}
		}
	}
	perCoset := (p.n + BlockSize - 1) / BlockSize
	work := func(first, last int) {
		w := p.newWorker()
		w.bind, w.scalars = bind, scalars
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
	bind     *Bindings
	scalars  []field.Ext // value of every scalar node in this run
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
		case nd.slotted && nd.kind == kVecBase:
			table := w.bind.Base[nd.slot]
			w.viewBase[id] = wrappedView(table[tableStart:tableStart+p.n], idx, L, &w.wrapBase[id])
		case nd.slotted:
			table := w.bind.Ext[nd.slot]
			w.viewExt[id] = wrappedView(table[tableStart:tableStart+p.n], idx, L, &w.wrapExt[id])
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
	case kScalar, kParam, kScalarOp:
		return operand{scalar: true, isBase: nd.isBase, s: w.scalars[id]}
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
	c := w.coef(nd.c)
	for i := range dst {
		dst[i] = c
	}
	acc := extensions.VectorE6(dst)
	for i, a := range nd.atoms {
		atom := w.operand(a, len(dst))
		coef := w.coef(nd.coefs[i])
		acc.ScalarMulAccByElement(atom.base, &coef)
	}
}

// coef returns the value of c in this run.
func (w *worker) coef(c coef) field.Ext {
	if c.id >= 0 {
		return w.scalars[c.id]
	}
	return c.v
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
				field.VecMulExtExt(dst, a.ext, b.ext)
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
			}
			a, b = b, a
		}
	case Square:
		if vecExt(&a) {
			field.VecMulExtExt(dst, a.ext, a.ext)
			return
		}
	case Inverse:
		batchInvertExt(extVector(&a, L, w.tmpExt), dst, w.invExt[:L])
		return
	case Div:
		inv := w.tmpExt2[:L]
		batchInvertExt(extVector(&b, L, w.tmpExt), inv, w.invExt[:L])
		switch {
		case vecExt(&a):
			field.VecMulExtExt(dst, a.ext, inv)
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

// invLanes is the number of interleaved product chains of [batchInvertExt],
// the width of the vectorised extension product.
const invLanes = 16

// batchInvertExt writes 1/a[i] into dst[i], and 0 where a[i] is 0. It runs
// Montgomery's trick as invLanes interleaved chains (chain l takes elements
// l, l+invLanes, …), so the prefix and suffix products of a block are
// vectorised extension products, and finishes the at most invLanes-1
// trailing elements as one more chain; all the chain products are inverted
// with one field inversion. prefix is scratch of the same length; dst may
// alias neither a nor prefix.
func batchInvertExt(a, dst, prefix []field.Ext) {
	// dst first holds a with its zeros replaced by one, which the chains
	// multiply through unchanged.
	for i := range a {
		if a[i].IsZero() {
			dst[i].SetOne()
		} else {
			dst[i] = a[i]
		}
	}
	nz := dst
	m := len(a) / invLanes * invLanes

	// Forward: prefix[i] is the product of the earlier elements of i's chain;
	// chain products end up in acc[0:invLanes] and, for the tail, acc[invLanes].
	var acc [invLanes + 1]field.Ext
	for l := range acc {
		acc[l].SetOne()
	}
	for j := 0; j < m; j += invLanes {
		copy(prefix[j:j+invLanes], acc[:invLanes])
		field.VecMulExtExt(acc[:invLanes], acc[:invLanes], nz[j:j+invLanes])
	}
	for i := m; i < len(a); i++ {
		prefix[i] = acc[invLanes]
		acc[invLanes].Mul(&acc[invLanes], &nz[i])
	}

	// Invert every chain product at once.
	var accPrefix [invLanes + 1]field.Ext
	var run field.Ext
	run.SetOne()
	for l := range acc {
		accPrefix[l] = run
		run.Mul(&run, &acc[l])
	}
	run.Inverse(&run)
	for l := len(acc) - 1; l >= 0; l-- {
		var inv field.Ext
		inv.Mul(&run, &accPrefix[l])
		run.Mul(&run, &acc[l])
		acc[l] = inv
	}

	// Backward: each element's inverse is its chain's running inverse times
	// its prefix; nz[i] is read before dst[i] overwrites it.
	for i := len(a) - 1; i >= m; i-- {
		var inv field.Ext
		inv.Mul(&acc[invLanes], &prefix[i])
		acc[invLanes].Mul(&acc[invLanes], &nz[i])
		dst[i] = inv
	}
	var inv [invLanes]field.Ext
	for j := m - invLanes; j >= 0; j -= invLanes {
		field.VecMulExtExt(inv[:], acc[:invLanes], prefix[j:j+invLanes])
		field.VecMulExtExt(acc[:invLanes], acc[:invLanes], nz[j:j+invLanes])
		copy(dst[j:j+invLanes], inv[:])
	}

	for i := range a {
		if a[i].IsZero() {
			dst[i] = field.Ext{}
		}
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
	if sk.cancelSlot >= 0 {
		cancel = w.bind.Base[sk.cancelSlot][tablePos : tablePos+L]
	}
	if root.isBase {
		vals := root.base
		if cancel != nil {
			tmp := field.Vector(w.tmpBase[:L])
			tmp.Mul(vals, cancel)
			vals = tmp
		}
		coef := w.coef(sk.coef)
		acc.ScalarMulAccByElement(vals, &coef)
		return
	}
	vals := root.ext
	if cancel != nil {
		tmp := extensions.VectorE6(w.tmpExt[:L])
		tmp.MulByElement(vals, cancel)
		vals = tmp
	}
	coef := w.coef(sk.coef)
	acc.ScalarMulAcc(vals, &coef)
}

// asLimbs views extension elements as their contiguous base-field limbs.
func asLimbs(v []field.Ext) field.Vector {
	if len(v) == 0 {
		return nil
	}
	return unsafe.Slice((*field.Element)(unsafe.Pointer(&v[0])), 6*len(v))
}
