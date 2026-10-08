package global

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"slices"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

// Quotient program.
//
// The bound vanishing expressions of a bucket are lowered into one straight-line
// program that the quotient pass runs over blocks of coset points, instead of
// walking every expression tree once per point. Three rewrites make it cheaper
// than the trees it replaces, all decided once per bucket at bind time:
//
//   - hash-consing: structurally equal subtrees (a log-derivative denominator
//     appears in several products of its constraint, and constraints share
//     them) are computed once per point;
//   - scalar folding: subtrees made only of constants, cells and coins are
//     evaluated once;
//   - linear forms: an extension-valued subtree that is affine in base-valued
//     subtrees with scalar coefficients, such as the Horner form
//     α + β·(c₁ + β·(c₂ + …)) of a random linear combination, becomes
//     c + Σ coefᵢ·atomᵢ. Each term then costs an extension-by-base product
//     instead of a full extension multiplication.
//
// Only additions, subtractions, negations, doublings and multiplications by a
// scalar are folded or redistributed. These are field identities, so the
// aggregate is exactly the one of a point-by-point evaluation of the trees.
// Divisions and inversions (where 0⁻¹ = 0 is not an identity) are kept as
// they are.

// qBlockSize is the number of coset points a worker evaluates per program run.
// Registers hold one block each, so the working set of a bucket stays in cache.
const qBlockSize = 256

// qKind discriminates the nodes of a quotient program.
type qKind uint8

const (
	qScalar  qKind = iota // bind-time value, in B0.A0 when isBase
	qVecBase              // base-field coset table, read at a shift
	qVecExt               // extension-field coset table, read at a shift
	qOp                   // arithmetic node over other nodes
	qLinear               // s + Σ coefs[i]·atoms[i], atoms base-valued
)

// qNode is a hash-consed node of a quotient program.
type qNode struct {
	kind    qKind
	isBase  bool
	op      wiop.ArithmeticOperator
	args    []int           // qOp
	s       field.Ext       // qScalar: the value; qLinear: the constant term
	vecBase []field.Element // qVecBase
	vecExt  []field.Ext     // qVecExt
	offset  int             // qVec*: shift within a small coset, in [0, n)
	atoms   []int           // qLinear, increasing
	coefs   []field.Ext     // qLinear, aligned with atoms, non-zero
}

// qKey identifies a node up to structural equality: leaves by table identity
// and shift, scalars by value, inner nodes by their (already interned)
// operands. A linear form's variable-length part enters through a hash, so
// nodes sharing a key are compared in full by [qNode.equal].
type qKey struct {
	kind   qKind
	isBase bool
	op     wiop.ArithmeticOperator
	a0, a1 int
	table  uintptr
	offset int
	s      field.Ext
	hash   uint64
}

func (n *qNode) key() qKey {
	k := qKey{kind: n.kind, isBase: n.isBase, s: n.s, a0: -1, a1: -1}
	switch n.kind {
	case qVecBase:
		k.table, k.offset = uintptr(unsafe.Pointer(unsafe.SliceData(n.vecBase))), n.offset
	case qVecExt:
		k.table, k.offset = uintptr(unsafe.Pointer(unsafe.SliceData(n.vecExt))), n.offset
	case qOp:
		k.op, k.a0 = n.op, n.args[0]
		if len(n.args) > 1 {
			k.a1 = n.args[1]
		}
	case qLinear:
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
func (n *qNode) equal(m *qNode) bool {
	return n.kind != qLinear || (slices.Equal(n.atoms, m.atoms) && slices.Equal(n.coefs, m.coefs))
}

// qBuilder interns the nodes of a program under construction.
type qBuilder struct {
	nodes []qNode
	index map[qKey][]int
}

func (b *qBuilder) intern(n qNode) int {
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

func (b *qBuilder) scalar(v field.Ext, isBase bool) int {
	return b.intern(qNode{kind: qScalar, isBase: isBase, s: v})
}

// build lowers a bound expression and returns its node.
func (b *qBuilder) build(e *boundExpr) int {
	switch e.kind {
	case boundVecBase:
		return b.intern(qNode{kind: qVecBase, isBase: true, vecBase: e.vecBase, offset: e.offset})
	case boundVecExt:
		return b.intern(qNode{kind: qVecExt, vecExt: e.vecExt, offset: e.offset})
	case boundScalarBase:
		return b.scalar(field.Lift(e.scalarBase), true)
	case boundScalarExt:
		return b.scalar(e.scalarExt, false)
	}
	args := make([]int, arity(e.operator))
	for i := range args {
		args[i] = b.build(&e.operands[i])
	}
	return b.op(e.operator, args)
}

// arity is the number of operands o reads.
func arity(o wiop.ArithmeticOperator) int {
	switch o {
	case wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub,
		wiop.ArithmeticOperatorMul, wiop.ArithmeticOperatorDiv:
		return 2
	}
	return 1
}

// op interns operator(args), folding scalars and linear forms.
func (b *qBuilder) op(o wiop.ArithmeticOperator, args []int) int {
	allScalar, allBase := true, true
	for _, a := range args {
		allScalar = allScalar && b.nodes[a].kind == qScalar
		allBase = allBase && b.nodes[a].isBase
	}
	if allScalar {
		vals := make([]field.Ext, len(args))
		for i, a := range args {
			vals[i] = b.nodes[a].s
		}
		if allBase {
			return b.scalar(field.Lift(applyBase(o, liftedBases(vals))), true)
		}
		return b.scalar(applyExt(o, vals), false)
	}
	if allBase {
		return b.intern(qNode{kind: qOp, isBase: true, op: o, args: args})
	}

	switch o {
	case wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub:
		if b.linearizable(args[0]) && b.linearizable(args[1]) {
			r := b.linForm(args[1])
			if o == wiop.ArithmeticOperatorSub {
				r.scale(extMinusOne())
			}
			return b.linear(b.linForm(args[0]).plus(r))
		}
	case wiop.ArithmeticOperatorNegate:
		if b.linearizable(args[0]) {
			l := b.linForm(args[0])
			l.scale(extMinusOne())
			return b.linear(l)
		}
	case wiop.ArithmeticOperatorDouble:
		if b.linearizable(args[0]) {
			l := b.linForm(args[0])
			var two field.Ext
			two.SetOne()
			two.Double(&two)
			l.scale(two)
			return b.linear(l)
		}
	case wiop.ArithmeticOperatorMul:
		for k := range 2 {
			s, other := args[k], args[1-k]
			if b.nodes[s].kind == qScalar && b.linearizable(other) {
				l := b.linForm(other)
				l.scale(b.nodes[s].s)
				return b.linear(l)
			}
		}
	}
	return b.intern(qNode{kind: qOp, op: o, args: args})
}

// linearizable reports whether node id can take part in a linear form: as
// its constant (a scalar), as an atom (a base-valued node), or by merging (a
// linear form).
func (b *qBuilder) linearizable(id int) bool {
	n := &b.nodes[id]
	return n.kind == qScalar || n.isBase || n.kind == qLinear
}

// linForm returns a fresh copy of node id as a linear form; id must be
// linearizable.
func (b *qBuilder) linForm(id int) qLinForm {
	n := &b.nodes[id]
	switch n.kind {
	case qScalar:
		return qLinForm{c: n.s}
	case qLinear:
		return qLinForm{c: n.s, atoms: slices.Clone(n.atoms), coefs: slices.Clone(n.coefs)}
	}
	var one field.Ext
	one.SetOne()
	return qLinForm{atoms: []int{id}, coefs: []field.Ext{one}}
}

// linear interns l without its zero terms, as a scalar when none is left.
func (b *qBuilder) linear(l qLinForm) int {
	n := qNode{kind: qLinear, s: l.c}
	for i := range l.atoms {
		if !l.coefs[i].IsZero() {
			n.atoms = append(n.atoms, l.atoms[i])
			n.coefs = append(n.coefs, l.coefs[i])
		}
	}
	if len(n.atoms) == 0 {
		return b.scalar(l.c, false)
	}
	return b.intern(n)
}

// qLinForm is c + Σ coefs[i]·atoms[i] during construction, atoms increasing.
type qLinForm struct {
	c     field.Ext
	atoms []int
	coefs []field.Ext
}

// plus returns l + r, merging the atoms of both.
func (l qLinForm) plus(r qLinForm) qLinForm {
	out := qLinForm{
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
func (l *qLinForm) scale(s field.Ext) {
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
func applyBase(o wiop.ArithmeticOperator, a []field.Element) field.Element {
	var res field.Element
	switch o {
	case wiop.ArithmeticOperatorAdd:
		res.Add(&a[0], &a[1])
	case wiop.ArithmeticOperatorSub:
		res.Sub(&a[0], &a[1])
	case wiop.ArithmeticOperatorMul:
		res.Mul(&a[0], &a[1])
	case wiop.ArithmeticOperatorDiv:
		var inv field.Element
		inv.Inverse(&a[1])
		res.Mul(&a[0], &inv)
	case wiop.ArithmeticOperatorDouble:
		res.Add(&a[0], &a[0])
	case wiop.ArithmeticOperatorSquare:
		res.Square(&a[0])
	case wiop.ArithmeticOperatorNegate:
		res.Neg(&a[0])
	case wiop.ArithmeticOperatorInverse:
		res.Inverse(&a[0])
	default:
		panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", o))
	}
	return res
}

// applyExt applies o to extension-field operands.
func applyExt(o wiop.ArithmeticOperator, a []field.Ext) field.Ext {
	var res field.Ext
	switch o {
	case wiop.ArithmeticOperatorAdd:
		res.Add(&a[0], &a[1])
	case wiop.ArithmeticOperatorSub:
		res.Sub(&a[0], &a[1])
	case wiop.ArithmeticOperatorMul:
		res.Mul(&a[0], &a[1])
	case wiop.ArithmeticOperatorDiv:
		var inv field.Ext
		inv.Inverse(&a[1])
		res.Mul(&a[0], &inv)
	case wiop.ArithmeticOperatorDouble:
		res.Double(&a[0])
	case wiop.ArithmeticOperatorSquare:
		res.Square(&a[0])
	case wiop.ArithmeticOperatorNegate:
		res.Neg(&a[0])
	case wiop.ArithmeticOperatorInverse:
		res.Inverse(&a[0])
	default:
		panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", o))
	}
	return res
}

// quotientProgram is the lowered form of a bucket's bound entries. Its
// aggregate is laid out coset-major (see [cosetShifts]): ratio small cosets
// of n points each. The coset tables it reads (columns, selectors,
// cancellations) are the module's, over step·ratio cosets, of which the
// bucket's coset k is coset k·step.
type quotientProgram struct {
	nodes    []qNode
	steps    []qStep
	reg      []int // computed node -> register of its kind
	leaves   []int // qVec* nodes read by the program
	entries  []qEntry
	nbBase   int
	nbExt    int
	n, ratio int
	step     int
}

// qStep computes node, or accumulates entry when entry >= 0.
type qStep struct {
	node  int
	entry int
}

type qEntry struct {
	root         int
	cancellation []field.Element
	coinPow      field.Ext
}

// compileQuotientProgram lowers the bound entries of a bucket of ratio small
// cosets of n points, whose coset k is coset k·step of its tables. Each
// entry's nodes are scheduled depth first right before its accumulation, and
// registers are recycled after their last use.
func compileQuotientProgram(bound []boundEntry, n, ratio, step int) *quotientProgram {
	b := &qBuilder{index: make(map[qKey][]int)}
	p := &quotientProgram{n: n, ratio: ratio, step: step, entries: make([]qEntry, len(bound))}
	for i := range bound {
		p.entries[i] = qEntry{
			root:         b.build(&bound[i].expr),
			cancellation: bound[i].cancellation,
			coinPow:      bound[i].coinPow,
		}
	}
	p.nodes = b.nodes

	emitted := make([]bool, len(p.nodes))
	var emit func(id int)
	emit = func(id int) {
		if emitted[id] {
			return
		}
		emitted[id] = true
		n := &p.nodes[id]
		switch n.kind {
		case qVecBase, qVecExt:
			p.leaves = append(p.leaves, id)
			return
		case qScalar:
			return
		}
		for _, a := range n.deps() {
			emit(a)
		}
		p.steps = append(p.steps, qStep{node: id, entry: -1})
	}
	for i := range p.entries {
		emit(p.entries[i].root)
		p.steps = append(p.steps, qStep{node: p.entries[i].root, entry: i})
	}

	lastUse := make([]int, len(p.nodes))
	for s, st := range p.steps {
		if st.entry >= 0 {
			lastUse[st.node] = s
			continue
		}
		for _, a := range p.nodes[st.node].deps() {
			lastUse[a] = s
		}
	}

	p.reg = make([]int, len(p.nodes))
	var freeBase, freeExt []int
	release := func(id int) {
		n := &p.nodes[id]
		if n.kind != qOp && n.kind != qLinear {
			return
		}
		if n.isBase {
			freeBase = append(freeBase, p.reg[id])
		} else {
			freeExt = append(freeExt, p.reg[id])
		}
	}
	for s, st := range p.steps {
		if st.entry >= 0 {
			if lastUse[st.node] == s {
				release(st.node)
			}
			continue
		}
		n := &p.nodes[st.node]
		switch {
		case n.isBase && len(freeBase) > 0:
			p.reg[st.node], freeBase = freeBase[len(freeBase)-1], freeBase[:len(freeBase)-1]
		case n.isBase:
			p.reg[st.node] = p.nbBase
			p.nbBase++
		case len(freeExt) > 0:
			p.reg[st.node], freeExt = freeExt[len(freeExt)-1], freeExt[:len(freeExt)-1]
		default:
			p.reg[st.node] = p.nbExt
			p.nbExt++
		}
		// Operands are released after the destination is taken, so a step
		// never writes into a register it reads.
		deps := n.deps()
		for i, a := range deps {
			if lastUse[a] == s && !slices.Contains(deps[:i], a) {
				release(a)
			}
		}
	}
	return p
}

// deps returns the nodes n reads.
func (n *qNode) deps() []int {
	if n.kind == qLinear {
		return n.atoms
	}
	return n.args
}

// run adds Σ coinPowᵢ·Pᵢ·Cᵢ into aggregate[t] for every coset point t and
// multiplies the result by annInv[k] on small coset k, block by block in
// parallel. A block never straddles two cosets.
func (p *quotientProgram) run(aggregate []field.Ext, annInv []field.Element) {
	perCoset := (p.n + qBlockSize - 1) / qBlockSize
	parallel.Execute(p.ratio*perCoset, func(first, last int) {
		w := p.newWorker()
		for blk := first; blk < last; blk++ {
			k, i0 := blk/perCoset, (blk%perCoset)*qBlockSize
			start := k*p.n + i0
			agg := aggregate[start : start+min(qBlockSize, p.n-i0)]
			w.runBlock(agg, k, i0)
			out := extensions.VectorE6(agg)
			out.ScalarMulByElement(agg, &annInv[k])
		}
	})
}

// qWorker holds one worker's registers and leaf views.
type qWorker struct {
	p        *quotientProgram
	baseRegs [][]field.Element
	extRegs  [][]field.Ext
	// Per-block view of each leaf, indexed by node; wrap holds the leaf's copy
	// when its shifted window wraps around the end of the table.
	viewBase [][]field.Element
	viewExt  [][]field.Ext
	wrapBase [][]field.Element
	wrapExt  [][]field.Ext
	tmpBase  []field.Element
	tmpExt   []field.Ext
}

func (p *quotientProgram) newWorker() *qWorker {
	w := &qWorker{
		p:        p,
		baseRegs: make([][]field.Element, p.nbBase),
		extRegs:  make([][]field.Ext, p.nbExt),
		viewBase: make([][]field.Element, len(p.nodes)),
		viewExt:  make([][]field.Ext, len(p.nodes)),
		wrapBase: make([][]field.Element, len(p.nodes)),
		wrapExt:  make([][]field.Ext, len(p.nodes)),
		tmpBase:  make([]field.Element, qBlockSize),
		tmpExt:   make([]field.Ext, qBlockSize),
	}
	baseSlab := make([]field.Element, p.nbBase*qBlockSize)
	for i := range w.baseRegs {
		w.baseRegs[i] = baseSlab[i*qBlockSize : (i+1)*qBlockSize]
	}
	extSlab := make([]field.Ext, p.nbExt*qBlockSize)
	for i := range w.extRegs {
		w.extRegs[i] = extSlab[i*qBlockSize : (i+1)*qBlockSize]
	}
	return w
}

// runBlock evaluates the program over points [i0, i0+len(agg)) of small coset
// k and accumulates every entry into agg.
func (w *qWorker) runBlock(agg []field.Ext, k, i0 int) {
	p := w.p
	L := len(agg)
	cosetStart := k * p.step * p.n // the bucket's coset k in the tables
	for _, id := range p.leaves {
		n := &p.nodes[id]
		idx := i0 + n.offset
		if idx >= p.n {
			idx -= p.n
		}
		if n.kind == qVecBase {
			w.viewBase[id] = wrappedView(n.vecBase[cosetStart:cosetStart+p.n], idx, L, &w.wrapBase[id])
		} else {
			w.viewExt[id] = wrappedView(n.vecExt[cosetStart:cosetStart+p.n], idx, L, &w.wrapExt[id])
		}
	}
	for _, st := range p.steps {
		if st.entry >= 0 {
			w.accumulate(agg, cosetStart+i0, &p.entries[st.entry])
			continue
		}
		n := &p.nodes[st.node]
		switch {
		case n.kind == qLinear:
			w.linear(n, w.extRegs[p.reg[st.node]][:L])
		case n.isBase:
			w.baseOp(n, w.baseRegs[p.reg[st.node]][:L])
		default:
			w.extOp(n, w.extRegs[p.reg[st.node]][:L])
		}
	}
}

// wrappedView returns table[idx : idx+L] cyclically, copying into *buf only
// when the window wraps around the end of the table (a small coset).
func wrappedView[T any](table []T, idx, L int, buf *[]T) []T {
	if idx+L <= len(table) {
		return table[idx : idx+L]
	}
	if *buf == nil {
		*buf = make([]T, qBlockSize)
	}
	out := (*buf)[:L]
	k := copy(out, table[idx:])
	copy(out[k:], table)
	return out
}

// qOperand is a node's value over the current block: a scalar or a vector of
// its field.
type qOperand struct {
	scalar bool
	isBase bool
	s      field.Ext
	base   []field.Element
	ext    []field.Ext
}

func (o *qOperand) baseAt(i int) field.Element {
	if o.scalar {
		return o.s.B0.A0
	}
	return o.base[i]
}

func (o *qOperand) extAt(i int) field.Ext {
	switch {
	case o.scalar:
		return o.s
	case o.isBase:
		return field.Lift(o.base[i])
	}
	return o.ext[i]
}

func (w *qWorker) operand(id, L int) qOperand {
	n := &w.p.nodes[id]
	switch n.kind {
	case qScalar:
		return qOperand{scalar: true, isBase: n.isBase, s: n.s}
	case qVecBase:
		return qOperand{isBase: true, base: w.viewBase[id]}
	case qVecExt:
		return qOperand{ext: w.viewExt[id]}
	}
	if n.isBase {
		return qOperand{isBase: true, base: w.baseRegs[w.p.reg[id]][:L]}
	}
	return qOperand{ext: w.extRegs[w.p.reg[id]][:L]}
}

func (w *qWorker) linear(n *qNode, dst []field.Ext) {
	for i := range dst {
		dst[i] = n.s
	}
	acc := extensions.VectorE6(dst)
	for i, a := range n.atoms {
		atom := w.operand(a, len(dst))
		acc.ScalarMulAccByElement(atom.base, &n.coefs[i])
	}
}

func (w *qWorker) baseOp(n *qNode, dst []field.Element) {
	L := len(dst)
	a := w.operand(n.args[0], L)
	var b qOperand
	if len(n.args) > 1 {
		b = w.operand(n.args[1], L)
	}
	out := field.Vector(dst)
	switch {
	case n.op == wiop.ArithmeticOperatorAdd && !a.scalar && !b.scalar:
		out.Add(a.base, b.base)
		return
	case n.op == wiop.ArithmeticOperatorSub && !a.scalar && !b.scalar:
		out.Sub(a.base, b.base)
		return
	case n.op == wiop.ArithmeticOperatorMul && !a.scalar && !b.scalar:
		out.Mul(a.base, b.base)
		return
	case n.op == wiop.ArithmeticOperatorMul && b.scalar:
		s := b.s.B0.A0
		out.ScalarMul(a.base, &s)
		return
	case n.op == wiop.ArithmeticOperatorMul && a.scalar:
		s := a.s.B0.A0
		out.ScalarMul(b.base, &s)
		return
	}
	var args [2]field.Element
	for i := range dst {
		args[0] = a.baseAt(i)
		if len(n.args) > 1 {
			args[1] = b.baseAt(i)
		}
		dst[i] = applyBase(n.op, args[:len(n.args)])
	}
}

func (w *qWorker) extOp(n *qNode, dst []field.Ext) {
	L := len(dst)
	a := w.operand(n.args[0], L)
	var b qOperand
	if len(n.args) > 1 {
		b = w.operand(n.args[1], L)
	}
	out := extensions.VectorE6(dst)
	vecExt := func(o *qOperand) bool { return !o.scalar && !o.isBase }
	vecBase := func(o *qOperand) bool { return !o.scalar && o.isBase }

	switch n.op {
	case wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub:
		sub := n.op == wiop.ArithmeticOperatorSub
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
	case wiop.ArithmeticOperatorMul:
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
	}
	var args [2]field.Ext
	for i := range dst {
		args[0] = a.extAt(i)
		if len(n.args) > 1 {
			args[1] = b.extAt(i)
		}
		dst[i] = applyExt(n.op, args[:len(n.args)])
	}
}

// accumulate adds coinPow · P · C over the block into agg.
func (w *qWorker) accumulate(agg []field.Ext, start int, e *qEntry) {
	L := len(agg)
	acc := extensions.VectorE6(agg)
	root := w.operand(e.root, L)
	if root.scalar {
		// A constant expression: broadcast it once.
		if root.isBase {
			for i := range L {
				w.tmpBase[i] = root.s.B0.A0
			}
			root = qOperand{isBase: true, base: w.tmpBase[:L]}
		} else {
			for i := range L {
				w.tmpExt[i] = root.s
			}
			root = qOperand{ext: w.tmpExt[:L]}
		}
	}
	var cancel []field.Element
	if e.cancellation != nil {
		cancel = e.cancellation[start : start+L]
	}
	if root.isBase {
		vals := root.base
		if cancel != nil {
			tmp := field.Vector(w.tmpBase[:L])
			tmp.Mul(vals, cancel)
			vals = tmp
		}
		acc.ScalarMulAccByElement(vals, &e.coinPow)
		return
	}
	vals := root.ext
	if cancel != nil {
		tmp := extensions.VectorE6(w.tmpExt[:L])
		tmp.MulByElement(vals, cancel)
		vals = tmp
	}
	acc.ScalarMulAcc(vals, &e.coinPow)
}

// asLimbs views extension elements as their contiguous base-field limbs.
func asLimbs(v []field.Ext) field.Vector {
	if len(v) == 0 {
		return nil
	}
	return unsafe.Slice((*field.Element)(unsafe.Pointer(&v[0])), 6*len(v))
}
