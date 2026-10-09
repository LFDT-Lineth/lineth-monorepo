package global

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

// quotientBatchSize is the number of coset points a worker evaluates per pass
// over a [quotientProgram]. Each node is computed for the whole batch with the
// vectorised field kernels, so the per-node dispatch cost is paid once per
// batch rather than once per point. It bounds the per-worker scratch to
// (nBase·4 + nExt·24)·quotientBatchSize bytes.
const quotientBatchSize = 64

// quotientMinPointsPerWorker caps the number of workers for small coset
// domains: below this many points per worker, goroutine and program-dispatch
// overhead dominates the arithmetic, and every extra worker needs its own
// scratch.
const quotientMinPointsPerWorker = 32

// quotientProgram is the per-bucket evaluation plan of the aggregated
// quotient numerator
//
//	aggregate[j] += Σ_i coin^i · P_i(coset_j) · C_i(coset_j)
//
// The vanishing expressions of the bucket are flattened into a single
// topologically ordered instruction list in which every distinct node appears
// exactly once: shared subexpressions (same pointer, or same operator over the
// same operands) are bound and evaluated once per point, however many
// vanishings use them. Subtrees that do not depend on the coset point (coins,
// cells, constants) are folded into scalars at build time.
//
// Vector-valued nodes live in registers ("slots") of quotientBatchSize
// elements, reused once their last reader has executed, so the scratch
// footprint follows the live set rather than the node count.
//
// A program is immutable once built: workers only read it, and write their own
// scratch and their own disjoint range of aggregate.
type quotientProgram struct {
	instrs      []progInstr
	scalarsBase []field.Element // scalarsBase[0] is zero, the absent operand
	scalarsExt  []field.Ext
	loads       []progLoad
	accs        []progAcc
	// nBase and nExt are the number of base- and extension-field slots.
	nBase, nExt int
	// nodes is the number of distinct vector-valued nodes, for reporting.
	nodes int
}

// progOperandKind discriminates where a [progRef] reads its value from.
type progOperandKind uint8

const (
	operandVecBase    progOperandKind = iota // base-field slot
	operandVecExt                            // extension-field slot
	operandScalarBase                        // point-invariant base value
	operandScalarExt                         // point-invariant extension value
)

// progRef is an instruction operand or destination: a slot index for the
// vector kinds, an index into the program's scalar tables otherwise.
type progRef struct {
	kind progOperandKind
	idx  int32
}

func (r progRef) isBase() bool {
	return r.kind == operandVecBase || r.kind == operandScalarBase
}

func (r progRef) isVec() bool {
	return r.kind == operandVecBase || r.kind == operandVecExt
}

// progInstrKind discriminates the instructions of a [quotientProgram].
type progInstrKind uint8

const (
	// instrLoadBase / instrLoadExt copy the coset evaluations of a leaf
	// (column view or Lagrange selector) at the batch points into dst. aux
	// indexes the program's loads.
	instrLoadBase progInstrKind = iota
	instrLoadExt
	// instrOp writes operator(a, b) into dst.
	instrOp
	// instrAccumulate adds coinPow · a · cancellation into aggregate. aux
	// indexes the program's accs.
	instrAccumulate
)

type progInstr struct {
	kind      progInstrKind
	operator  wiop.ArithmeticOperator
	dst, a, b progRef
	aux       int32
}

// progLoad is a leaf's length-N coset table and its shift, folded into an
// offset into the table.
type progLoad struct {
	vecBase []field.Element
	vecExt  []field.Ext
	offset  int
}

// progAcc is the per-vanishing data of an accumulation.
type progAcc struct {
	cancellation []field.Element // nil when the constraint has no cancelled positions
	coinPow      field.Ext
}

// quotientRoot is one vanishing of a bucket, ready to be compiled into a
// [quotientProgram].
type quotientRoot struct {
	expr         wiop.Expression
	cancellation []field.Element
	coinPow      field.Ext
}

// ---------------------------------------------------------------------------
// Building
// ---------------------------------------------------------------------------

// progNodeKey identifies a vector-valued node structurally, so that two
// distinct pointers denoting the same value bind to the same node. Leaves are
// keyed by what they read; compounds by their operator and operand node ids.
type progNodeKey struct {
	kind     uint8 // 0 column view, 1 selector, 2 compound
	operator wiop.ArithmeticOperator
	col      wiop.ObjectID
	shift    int
	a, b     int32
}

const (
	keyColumnView uint8 = iota
	keySelector
	keyCompound
)

// progNode is a node of the program. A scalar node's ref is final; a vector
// node's ref receives its slot during allocation.
type progNode struct {
	ref     progRef
	lastUse int32 // index of the last instruction reading the node
}

type programBuilder struct {
	rt             *wiop.Runtime
	cosetEvals     map[wiop.ObjectID][]field.Element
	cosetEvalsExt  map[wiop.ObjectID][]field.Ext
	selectorCosets map[int][]field.Element
	ratio, N       int

	prog  *quotientProgram
	nodes []progNode
	// operandNodes[i] holds the (dst, a, b) node ids of instruction i, -1
	// when absent. allocate turns them into the instruction's refs.
	operandNodes [][3]int32
	byPtr        map[wiop.Expression]int32
	byKey        map[progNodeKey]int32
}

// buildQuotientProgram compiles the vanishings of one bucket against the
// bucket's coset tables. It must run before the parallel evaluation, which
// only reads the result.
func buildQuotientProgram(
	rt *wiop.Runtime,
	roots []quotientRoot,
	cosetEvals map[wiop.ObjectID][]field.Element,
	cosetEvalsExt map[wiop.ObjectID][]field.Ext,
	selectorCosets map[int][]field.Element,
	ratio, N int,
) *quotientProgram {
	b := &programBuilder{
		rt:             rt,
		cosetEvals:     cosetEvals,
		cosetEvalsExt:  cosetEvalsExt,
		selectorCosets: selectorCosets,
		ratio:          ratio,
		N:              N,
		prog: &quotientProgram{
			scalarsBase: []field.Element{{}},
			accs:        make([]progAcc, 0, len(roots)),
		},
		byPtr: make(map[wiop.Expression]int32),
		byKey: make(map[progNodeKey]int32),
	}
	for _, r := range roots {
		node := b.bind(r.expr)
		b.prog.accs = append(b.prog.accs, progAcc{cancellation: r.cancellation, coinPow: r.coinPow})
		b.emit(progInstr{kind: instrAccumulate, aux: int32(len(b.prog.accs) - 1)}, -1, node, -1)
	}
	b.allocate()
	return b.prog
}

// emit appends an instruction and records it as the latest reader of its
// operands.
func (b *programBuilder) emit(in progInstr, dst, a, c int32) int32 {
	idx := int32(len(b.prog.instrs))
	b.prog.instrs = append(b.prog.instrs, in)
	b.operandNodes = append(b.operandNodes, [3]int32{dst, a, c})
	for _, n := range [2]int32{a, c} {
		if n >= 0 {
			b.nodes[n].lastUse = idx
		}
	}
	return idx
}

func (b *programBuilder) newScalarBase(v field.Element) int32 {
	b.prog.scalarsBase = append(b.prog.scalarsBase, v)
	b.nodes = append(b.nodes, progNode{ref: progRef{kind: operandScalarBase, idx: int32(len(b.prog.scalarsBase) - 1)}})
	return int32(len(b.nodes) - 1)
}

func (b *programBuilder) newScalarExt(v field.Ext) int32 {
	b.prog.scalarsExt = append(b.prog.scalarsExt, v)
	b.nodes = append(b.nodes, progNode{ref: progRef{kind: operandScalarExt, idx: int32(len(b.prog.scalarsExt) - 1)}})
	return int32(len(b.nodes) - 1)
}

// newVector registers a vector node, keyed by key, produced by in, and emits
// that instruction.
func (b *programBuilder) newVector(key progNodeKey, isBase bool, in progInstr, a, c int32) int32 {
	kind := operandVecExt
	if isBase {
		kind = operandVecBase
	}
	b.nodes = append(b.nodes, progNode{ref: progRef{kind: kind}})
	id := int32(len(b.nodes) - 1)
	b.byKey[key] = id
	b.nodes[id].lastUse = b.emit(in, id, a, c)
	return id
}

func (b *programBuilder) newLoad(key progNodeKey, isBase bool, load progLoad) int32 {
	b.prog.loads = append(b.prog.loads, load)
	kind := instrLoadExt
	if isBase {
		kind = instrLoadBase
	}
	return b.newVector(key, isBase, progInstr{kind: kind, aux: int32(len(b.prog.loads) - 1)}, -1, -1)
}

// bind returns the node id of expr, binding it and its operands on first
// encounter. The pointer memo makes the walk linear in distinct nodes; the
// structural key additionally merges equal nodes held by distinct pointers.
func (b *programBuilder) bind(expr wiop.Expression) int32 {
	if id, ok := b.byPtr[expr]; ok {
		return id
	}
	id := b.bindUncached(expr)
	b.byPtr[expr] = id
	return id
}

func (b *programBuilder) bindUncached(expr wiop.Expression) int32 {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		offset := ((e.ShiftingOffset*b.ratio)%b.N + b.N) % b.N
		key := progNodeKey{kind: keyColumnView, col: e.Column.Context.ID, shift: offset}
		if id, ok := b.byKey[key]; ok {
			return id
		}
		if e.Column.IsExtension {
			return b.newLoad(key, false, progLoad{vecExt: b.cosetEvalsExt[e.Column.Context.ID], offset: offset})
		}
		return b.newLoad(key, true, progLoad{vecBase: b.cosetEvals[e.Column.Context.ID], offset: offset})
	case *wiop.LagrangeSelector:
		// Selectors are base-field and unshifted.
		key := progNodeKey{kind: keySelector, shift: e.Position}
		if id, ok := b.byKey[key]; ok {
			return id
		}
		return b.newLoad(key, true, progLoad{vecBase: b.selectorCosets[e.Position]})
	case *wiop.Constant:
		return b.newScalarBase(e.Value)
	case *wiop.Cell:
		v := b.rt.GetCellValue(e)
		if e.IsExtension() {
			return b.newScalarExt(v.AsExt())
		}
		if !v.IsBase() {
			panic(fmt.Sprintf(
				"wiop/compilers: cell %q declared as base but holds an extension-field value",
				e.Context.Path(),
			))
		}
		return b.newScalarBase(v.AsBase())
	case *wiop.CoinField:
		return b.newScalarExt(b.rt.GetCoinValue(e).AsExt())
	case *wiop.ArithmeticOperation:
		ops := [2]int32{-1, -1}
		for i, op := range e.Operands {
			ops[i] = b.bind(op)
		}
		isBase, allScalar := true, true
		for _, n := range ops {
			if n < 0 {
				continue
			}
			isBase = isBase && b.nodes[n].ref.isBase()
			allScalar = allScalar && !b.nodes[n].ref.isVec()
		}
		if allScalar {
			// Point-invariant: evaluate once now rather than at every point.
			return b.foldScalar(e.Operator, isBase, ops)
		}
		key := progNodeKey{kind: keyCompound, operator: e.Operator, a: ops[0], b: ops[1]}
		if id, ok := b.byKey[key]; ok {
			return id
		}
		return b.newVector(key, isBase, progInstr{kind: instrOp, operator: e.Operator}, ops[0], ops[1])
	default:
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in quotient program", expr))
	}
}

// foldScalar evaluates a point-invariant operation over scalar nodes. It uses
// the same arithmetic as the per-point kernels so folding never changes a
// value.
func (b *programBuilder) foldScalar(op wiop.ArithmeticOperator, isBase bool, ops [2]int32) int32 {
	x := b.nodes[ops[0]].ref
	y := progRef{kind: operandScalarBase} // absent operand: scalarsBase[0] = 0
	if ops[1] >= 0 {
		y = b.nodes[ops[1]].ref
	}
	p := b.prog
	if isBase {
		var res field.Element
		applyBase(op, &res, &p.scalarsBase[x.idx], &p.scalarsBase[y.idx])
		return b.newScalarBase(res)
	}
	xe, ye := p.scalarExt(x), p.scalarExt(y)
	var xb, yb field.Element
	if x.isBase() {
		xb = p.scalarsBase[x.idx]
	}
	if y.isBase() {
		yb = p.scalarsBase[y.idx]
	}
	var res field.Ext
	applyExt(op, &res, &xe, &ye, &xb, &yb, x.isBase(), y.isBase())
	return b.newScalarExt(res)
}

// allocate assigns slots to vector nodes by linear scan over the instruction
// list and resolves every operand to its final ref. An instruction's
// destination is allocated before its dying operands are released, so a
// kernel never writes over one of its own inputs.
func (b *programBuilder) allocate() {
	p := b.prog
	var freeBase, freeExt []int32
	alloc := func(kind progOperandKind) int32 {
		free, n := &freeExt, &p.nExt
		if kind == operandVecBase {
			free, n = &freeBase, &p.nBase
		}
		if k := len(*free); k > 0 {
			s := (*free)[k-1]
			*free = (*free)[:k-1]
			return s
		}
		*n++
		return int32(*n - 1)
	}
	release := func(n int32, idx int32) {
		node := &b.nodes[n]
		if !node.ref.isVec() || node.lastUse != idx {
			return
		}
		if node.ref.kind == operandVecBase {
			freeBase = append(freeBase, node.ref.idx)
		} else {
			freeExt = append(freeExt, node.ref.idx)
		}
	}

	for i := range p.instrs {
		in := &p.instrs[i]
		nodes := b.operandNodes[i]
		in.b = progRef{kind: operandScalarBase} // absent operand: scalarsBase[0] = 0
		if nodes[1] >= 0 {
			in.a = b.nodes[nodes[1]].ref
		}
		if nodes[2] >= 0 {
			in.b = b.nodes[nodes[2]].ref
		}
		if nodes[0] >= 0 {
			ref := &b.nodes[nodes[0]].ref
			ref.idx = alloc(ref.kind)
			in.dst = *ref
			p.nodes++
		}
		// Release operands read for the last time here. The same node may
		// appear as both operands (e.g. x·x); release it only once.
		if nodes[1] >= 0 {
			release(nodes[1], int32(i))
		}
		if nodes[2] >= 0 && nodes[2] != nodes[1] {
			release(nodes[2], int32(i))
		}
	}
}

func (p *quotientProgram) scalarExt(r progRef) field.Ext {
	if r.kind == operandScalarExt {
		return p.scalarsExt[r.idx]
	}
	return field.Lift(p.scalarsBase[r.idx])
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

// quotientScratch is one worker's register file.
type quotientScratch struct {
	base   []field.Element
	ext    []field.Ext
	tmpB   []field.Element
	tmpE   []field.Ext
	batchB int
}

var quotientScratchPool = sync.Pool{New: func() any { return new(quotientScratch) }}

func (s *quotientScratch) reset(nBase, nExt, bs int) {
	if need := nBase * bs; cap(s.base) < need {
		s.base = make([]field.Element, need)
	} else {
		s.base = s.base[:need]
	}
	if need := nExt * bs; cap(s.ext) < need {
		s.ext = make([]field.Ext, need)
	} else {
		s.ext = s.ext[:need]
	}
	if cap(s.tmpB) < bs {
		s.tmpB = make([]field.Element, bs)
		s.tmpE = make([]field.Ext, bs)
	}
	s.batchB = bs
}

func (s *quotientScratch) baseSlot(slot int32, m int) field.Vector {
	off := int(slot) * s.batchB
	return s.base[off : off+m]
}

func (s *quotientScratch) extSlot(slot int32, m int) extensions.VectorE6 {
	off := int(slot) * s.batchB
	return s.ext[off : off+m]
}

// workers returns the number of workers to split N coset points across.
func (p *quotientProgram) workers(N, maxWorkers int) int {
	return max(1, min(maxWorkers, N/quotientMinPointsPerWorker))
}

// run accumulates the program's contribution into aggregate[start:end].
func (p *quotientProgram) run(aggregate []field.Ext, start, end int) {
	bs := min(quotientBatchSize, end-start)
	if bs <= 0 {
		return
	}
	s := quotientScratchPool.Get().(*quotientScratch)
	defer quotientScratchPool.Put(s)
	s.reset(p.nBase, p.nExt, bs)

	for j0 := start; j0 < end; j0 += bs {
		m := min(bs, end-j0)
		agg := aggregate[j0 : j0+m]
		for i := range p.instrs {
			in := &p.instrs[i]
			switch in.kind {
			case instrLoadBase:
				l := &p.loads[in.aux]
				loadShifted(s.baseSlot(in.dst.idx, m), l.vecBase, j0+l.offset)
			case instrLoadExt:
				l := &p.loads[in.aux]
				loadShifted(s.extSlot(in.dst.idx, m), l.vecExt, j0+l.offset)
			case instrOp:
				if in.dst.kind == operandVecBase {
					p.execBase(s, in, m)
				} else {
					p.execExt(s, in, m)
				}
			case instrAccumulate:
				p.execAccumulate(s, in, agg, j0, m)
			}
		}
	}
}

// loadShifted copies src[(from+t) mod len(src)] into dst[t]. from is in
// [0, 2·len(src)), so at most one wrap-around occurs.
func loadShifted[T any](dst, src []T, from int) {
	n := len(src)
	if from >= n {
		from -= n
	}
	k := copy(dst, src[from:])
	copy(dst[k:], src)
}

// execBase evaluates an operation whose result is base-field: both operands
// are base, at least one of them a slot.
func (p *quotientProgram) execBase(s *quotientScratch, in *progInstr, m int) {
	dst := s.baseSlot(in.dst.idx, m)
	a, b := in.a, in.b
	aVec, bVec := a.kind == operandVecBase, b.kind == operandVecBase
	switch in.operator {
	case wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub, wiop.ArithmeticOperatorMul:
		switch {
		case aVec && bVec:
			av, bv := s.baseSlot(a.idx, m), s.baseSlot(b.idx, m)
			switch in.operator {
			case wiop.ArithmeticOperatorAdd:
				dst.Add(av, bv)
			case wiop.ArithmeticOperatorSub:
				dst.Sub(av, bv)
			default:
				dst.Mul(av, bv)
			}
			return
		case in.operator == wiop.ArithmeticOperatorMul && aVec:
			dst.ScalarMul(s.baseSlot(a.idx, m), &p.scalarsBase[b.idx])
			return
		case in.operator == wiop.ArithmeticOperatorMul && bVec:
			dst.ScalarMul(s.baseSlot(b.idx, m), &p.scalarsBase[a.idx])
			return
		}
	case wiop.ArithmeticOperatorDouble:
		av := s.baseSlot(a.idx, m)
		dst.Add(av, av)
		return
	case wiop.ArithmeticOperatorSquare:
		av := s.baseSlot(a.idx, m)
		dst.Mul(av, av)
		return
	}
	for t := range m {
		x, y := p.baseAt(s, a, t), p.baseAt(s, b, t)
		applyBase(in.operator, &dst[t], &x, &y)
	}
}

// execExt evaluates an operation whose result is extension-field.
func (p *quotientProgram) execExt(s *quotientScratch, in *progInstr, m int) {
	dst := s.extSlot(in.dst.idx, m)
	a, b := in.a, in.b
	switch in.operator {
	case wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub:
		p.execExtAddSub(s, in.operator == wiop.ArithmeticOperatorSub, dst, a, b, m)
		return
	case wiop.ArithmeticOperatorMul:
		if p.execExtMul(s, dst, a, b, m) {
			return
		}
	}
	aBase, bBase := a.isBase(), b.isBase()
	var xb, yb field.Element
	for t := range m {
		x, y := p.extAt(s, a, t), p.extAt(s, b, t)
		if aBase {
			xb = p.baseAt(s, a, t)
		}
		if bBase {
			yb = p.baseAt(s, b, t)
		}
		applyExt(in.operator, &dst[t], &x, &y, &xb, &yb, aBase, bBase)
	}
}

// execExtAddSub computes dst = a ± b when the result is extension-field. Two
// extension vectors use the base kernel over the six coordinates of every
// element. A base operand only touches the B0.A0 coordinate, where [field.Lift]
// places it: dst is set from the extension operand (negated for ext on the
// right of a Sub) and the base operand is added or subtracted there.
func (p *quotientProgram) execExtAddSub(
	s *quotientScratch, sub bool, dst extensions.VectorE6, a, b progRef, m int,
) {
	switch {
	case a.kind == operandVecExt && b.kind == operandVecExt:
		dv, av, bv := extAsBase(dst), extAsBase(s.extSlot(a.idx, m)), extAsBase(s.extSlot(b.idx, m))
		if sub {
			dv.Sub(av, bv)
		} else {
			dv.Add(av, bv)
		}
		return
	case !a.isBase() && !b.isBase():
		// One extension vector, one extension scalar.
		for t := range m {
			x, y := p.extAt(s, a, t), p.extAt(s, b, t)
			if sub {
				dst[t].Sub(&x, &y)
			} else {
				dst[t].Add(&x, &y)
			}
		}
		return
	}

	// Exactly one operand is base.
	extOp, baseOp, negExt, subBase := a, b, false, sub
	if a.isBase() {
		extOp, baseOp, negExt, subBase = b, a, sub, false
	}
	for t := range m {
		e := p.extAt(s, extOp, t)
		if negExt {
			dst[t].Neg(&e)
		} else {
			dst[t] = e
		}
		v := p.baseAt(s, baseOp, t)
		if subBase {
			dst[t].B0.A0.Sub(&dst[t].B0.A0, &v)
		} else {
			dst[t].B0.A0.Add(&dst[t].B0.A0, &v)
		}
	}
}

// execExtMul computes dst = a · b when the result is extension-field, for the
// operand combinations with a dedicated kernel or loop. It reports false for
// the others.
func (p *quotientProgram) execExtMul(s *quotientScratch, dst extensions.VectorE6, a, b progRef, m int) bool {
	// Order the operands so that a vector, if any, comes first.
	if !a.isVec() {
		a, b = b, a
	}
	switch {
	case a.kind == operandVecExt && b.kind == operandVecExt:
		av, bv := s.extSlot(a.idx, m), s.extSlot(b.idx, m)
		for t := range m {
			dst[t].Mul(&av[t], &bv[t])
		}
	case a.kind == operandVecExt && b.kind == operandVecBase:
		dst.MulByElement(s.extSlot(a.idx, m), s.baseSlot(b.idx, m))
	case a.kind == operandVecBase && b.kind == operandVecExt:
		dst.MulByElement(s.extSlot(b.idx, m), s.baseSlot(a.idx, m))
	case a.kind == operandVecExt && b.kind == operandScalarExt:
		dst.ScalarMul(s.extSlot(a.idx, m), &p.scalarsExt[b.idx])
	case a.kind == operandVecExt && b.kind == operandScalarBase:
		dst.ScalarMulByElement(s.extSlot(a.idx, m), &p.scalarsBase[b.idx])
	case a.kind == operandVecBase && b.kind == operandScalarExt:
		av, c := s.baseSlot(a.idx, m), &p.scalarsExt[b.idx]
		for t := range m {
			dst[t].MulByElement(c, &av[t])
		}
	default:
		return false
	}
	return true
}

// execAccumulate adds coinPow · P · C into agg for the batch.
func (p *quotientProgram) execAccumulate(s *quotientScratch, in *progInstr, agg extensions.VectorE6, j0, m int) {
	acc := &p.accs[in.aux]
	a := in.a
	var canc field.Vector
	if acc.cancellation != nil {
		canc = acc.cancellation[j0 : j0+m]
	}
	switch a.kind {
	case operandVecBase:
		v := s.baseSlot(a.idx, m)
		if canc != nil {
			tmp := field.Vector(s.tmpB[:m])
			tmp.Mul(v, canc)
			v = tmp
		}
		agg.ScalarMulAccByElement(v, &acc.coinPow)
	case operandVecExt:
		v := s.extSlot(a.idx, m)
		if canc != nil {
			tmp := extensions.VectorE6(s.tmpE[:m])
			tmp.MulByElement(v, canc)
			v = tmp
		}
		agg.ScalarMulAcc(v, &acc.coinPow)
	case operandScalarBase:
		// A point-invariant constraint: only its cancellation varies.
		for t := range m {
			v := p.scalarsBase[a.idx]
			if canc != nil {
				v.Mul(&v, &canc[t])
			}
			var term field.Ext
			term.MulByElement(&acc.coinPow, &v)
			agg[t].Add(&agg[t], &term)
		}
	case operandScalarExt:
		var c field.Ext
		c.Mul(&p.scalarsExt[a.idx], &acc.coinPow)
		for t := range m {
			term := c
			if canc != nil {
				term.MulByElement(&term, &canc[t])
			}
			agg[t].Add(&agg[t], &term)
		}
	}
}

// extAsBase views an E6 vector as the base vector of its coordinates.
func extAsBase(v extensions.VectorE6) field.Vector {
	if len(v) == 0 {
		return nil
	}
	return unsafe.Slice((*field.Element)(unsafe.Pointer(&v[0])), 6*len(v))
}

func (p *quotientProgram) baseAt(s *quotientScratch, r progRef, t int) field.Element {
	switch r.kind {
	case operandVecBase:
		return s.base[int(r.idx)*s.batchB+t]
	case operandScalarBase:
		return p.scalarsBase[r.idx]
	}
	panic("wiop/compilers: quotient program read an extension operand as base")
}

func (p *quotientProgram) extAt(s *quotientScratch, r progRef, t int) field.Ext {
	switch r.kind {
	case operandVecExt:
		return s.ext[int(r.idx)*s.batchB+t]
	case operandScalarExt:
		return p.scalarsExt[r.idx]
	case operandVecBase:
		return field.Lift(s.base[int(r.idx)*s.batchB+t])
	default:
		return field.Lift(p.scalarsBase[r.idx])
	}
}

// applyBase computes res = op(a, b) in the base field. Division and inversion
// map zero to zero, as the field's Inverse does.
func applyBase(op wiop.ArithmeticOperator, res, a, b *field.Element) {
	switch op {
	case wiop.ArithmeticOperatorAdd:
		res.Add(a, b)
	case wiop.ArithmeticOperatorSub:
		res.Sub(a, b)
	case wiop.ArithmeticOperatorMul:
		res.Mul(a, b)
	case wiop.ArithmeticOperatorDiv:
		var inv field.Element
		inv.Inverse(b)
		res.Mul(a, &inv)
	case wiop.ArithmeticOperatorDouble:
		res.Add(a, a)
	case wiop.ArithmeticOperatorSquare:
		res.Square(a)
	case wiop.ArithmeticOperatorNegate:
		res.Neg(a)
	case wiop.ArithmeticOperatorInverse:
		res.Inverse(a)
	default:
		panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", op))
	}
}

// applyExt computes res = op(x, y) in the extension field. xBase/yBase flag
// operands that are base-field, whose base values are in xb/yb: a Mul with a
// base operand folds it in via MulByElement instead of a full extension
// multiplication. Division and inversion map zero to zero.
func applyExt(op wiop.ArithmeticOperator, res, x, y *field.Ext, xb, yb *field.Element, xBase, yBase bool) {
	switch op {
	case wiop.ArithmeticOperatorAdd:
		res.Add(x, y)
	case wiop.ArithmeticOperatorSub:
		res.Sub(x, y)
	case wiop.ArithmeticOperatorMul:
		switch {
		case xBase:
			res.MulByElement(y, xb)
		case yBase:
			res.MulByElement(x, yb)
		default:
			res.Mul(x, y)
		}
	case wiop.ArithmeticOperatorDiv:
		var inv field.Ext
		inv.Inverse(y)
		res.Mul(x, &inv)
	case wiop.ArithmeticOperatorDouble:
		res.Double(x)
	case wiop.ArithmeticOperatorSquare:
		res.Square(x)
	case wiop.ArithmeticOperatorNegate:
		res.Neg(x)
	case wiop.ArithmeticOperatorInverse:
		res.Inverse(x)
	default:
		panic(fmt.Sprintf("wiop/compilers: unknown ArithmeticOperator %v", op))
	}
}
