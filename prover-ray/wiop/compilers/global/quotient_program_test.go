package global

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// programFixture is a module with base and extension columns, base and
// extension cells, random coset tables, and a pool of randomly generated
// expressions over them that share subexpressions both by pointer and by
// structure.
type programFixture struct {
	rt             *wiop.Runtime
	cosetEvals     map[wiop.ObjectID][]field.Element
	cosetEvalsExt  map[wiop.ObjectID][]field.Ext
	selectorCosets map[int][]field.Element
	ratio, N       int
	pool           []wiop.Expression
}

func randElem(rng *rand.Rand) field.Element {
	var e field.Element
	e.SetUint64(rng.Uint64())
	return e
}

func randExt(rng *rand.Rand) field.Ext {
	var e field.Ext
	e.B0.A0, e.B0.A1 = randElem(rng), randElem(rng)
	e.B1.A0, e.B1.A1 = randElem(rng), randElem(rng)
	e.B2.A0, e.B2.A1 = randElem(rng), randElem(rng)
	return e
}

func newProgramFixture(t *testing.T, rng *rand.Rand, poolSize int) *programFixture {
	t.Helper()
	const n, ratio = 32, 4
	N := n * ratio

	sys := wiop.NewSystemf("quotient-program")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)

	f := &programFixture{
		rt:             nil,
		cosetEvals:     map[wiop.ObjectID][]field.Element{},
		cosetEvalsExt:  map[wiop.ObjectID][]field.Ext{},
		selectorCosets: map[int][]field.Element{},
		ratio:          ratio,
		N:              N,
	}

	baseCols := make([]*wiop.Column, 4)
	for i := range baseCols {
		baseCols[i] = mod.NewColumn(sys.Context.Childf("b%d", i), r0)
	}
	extCols := make([]*wiop.Column, 2)
	for i := range extCols {
		extCols[i] = mod.NewExtensionColumn(sys.Context.Childf("e%d", i), r0)
	}
	leaves := make([]wiop.Expression, 0, 5*len(baseCols)+2*len(extCols)+3)
	baseCell := r0.NewCell(sys.Context.Childf("cb"), false)
	extCell := r0.NewCell(sys.Context.Childf("ce"), true)

	f.rt = wiop.NewRuntime(sys)
	f.rt.AssignCell(baseCell, field.ElemFromBase(randElem(rng)))
	f.rt.AssignCell(extCell, field.ElemFromExt(randExt(rng)))

	for _, c := range baseCols {
		vals := make([]field.Element, N)
		for j := range vals {
			vals[j] = randElem(rng)
		}
		f.cosetEvals[c.Context.ID] = vals
		for _, shift := range []int{0, 1, -1, 3, n + 2} {
			leaves = append(leaves, c.View().Shift(shift))
		}
	}
	for _, c := range extCols {
		vals := make([]field.Ext, N)
		for j := range vals {
			vals[j] = randExt(rng)
		}
		f.cosetEvalsExt[c.Context.ID] = vals
		leaves = append(leaves, c.View(), c.View().Shift(-2))
	}
	for _, pos := range []int{0, 5, -1} {
		vals := make([]field.Element, N)
		for j := range vals {
			vals[j] = randElem(rng)
		}
		f.selectorCosets[pos] = vals
		leaves = append(leaves, wiop.NewLagrangeSelector(mod, pos))
	}
	scalars := []wiop.Expression{
		wiop.NewConstantField(randElem(rng)),
		wiop.NewConstantField(field.Element{}), // zero: exercises Div/Inverse by zero
		baseCell,
		extCell,
	}

	pick := func() wiop.Expression {
		switch k := rng.IntN(10); {
		case k < 3:
			return leaves[rng.IntN(len(leaves))]
		case k < 4:
			return scalars[rng.IntN(len(scalars))]
		default:
			if len(f.pool) == 0 {
				return leaves[rng.IntN(len(leaves))]
			}
			return f.pool[rng.IntN(len(f.pool))]
		}
	}
	binary := []wiop.ArithmeticOperator{
		wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub, wiop.ArithmeticOperatorMul,
		wiop.ArithmeticOperatorMul, wiop.ArithmeticOperatorDiv,
	}
	unary := []wiop.ArithmeticOperator{
		wiop.ArithmeticOperatorDouble, wiop.ArithmeticOperatorSquare,
		wiop.ArithmeticOperatorNegate, wiop.ArithmeticOperatorInverse,
	}
	for len(f.pool) < poolSize {
		var e wiop.Expression
		switch k := rng.IntN(10); {
		case k < 6:
			e = wiop.NewArithmeticOperation(binary[rng.IntN(len(binary))], pick(), pick())
		case k < 8:
			e = wiop.NewArithmeticOperation(unary[rng.IntN(len(unary))], pick())
		case k < 9 && len(f.pool) > 0:
			// A structural duplicate under a fresh pointer.
			if op, ok := f.pool[rng.IntN(len(f.pool))].(*wiop.ArithmeticOperation); ok {
				e = wiop.NewArithmeticOperation(op.Operator, op.Operands...)
			}
		default:
			// A point-invariant subtree.
			e = wiop.Mul(scalars[rng.IntN(len(scalars))], scalars[rng.IntN(len(scalars))])
		}
		if e != nil {
			f.pool = append(f.pool, e)
		}
	}
	return f
}

// refEval evaluates expr at coset point j entirely in the extension field,
// re-descending into shared nodes: the semantics the program must reproduce.
func (f *programFixture) refEval(expr wiop.Expression, j int) field.Ext {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		idx := ((j+e.ShiftingOffset*f.ratio)%f.N + f.N) % f.N
		if e.Column.IsExtension {
			return f.cosetEvalsExt[e.Column.Context.ID][idx]
		}
		return field.Lift(f.cosetEvals[e.Column.Context.ID][idx])
	case *wiop.LagrangeSelector:
		return field.Lift(f.selectorCosets[e.Position][j])
	case *wiop.Constant:
		return field.Lift(e.Value)
	case *wiop.Cell:
		return f.rt.GetCellValue(e).AsExt()
	case *wiop.ArithmeticOperation:
		x := f.refEval(e.Operands[0], j)
		var res field.Ext
		switch e.Operator {
		case wiop.ArithmeticOperatorAdd:
			y := f.refEval(e.Operands[1], j)
			res.Add(&x, &y)
		case wiop.ArithmeticOperatorSub:
			y := f.refEval(e.Operands[1], j)
			res.Sub(&x, &y)
		case wiop.ArithmeticOperatorMul:
			y := f.refEval(e.Operands[1], j)
			res.Mul(&x, &y)
		case wiop.ArithmeticOperatorDiv:
			y := f.refEval(e.Operands[1], j)
			y.Inverse(&y)
			res.Mul(&x, &y)
		case wiop.ArithmeticOperatorDouble:
			res.Double(&x)
		case wiop.ArithmeticOperatorSquare:
			res.Square(&x)
		case wiop.ArithmeticOperatorNegate:
			res.Neg(&x)
		case wiop.ArithmeticOperatorInverse:
			res.Inverse(&x)
		}
		return res
	}
	panic("unexpected expression")
}

func treeSize(expr wiop.Expression) int {
	op, ok := expr.(*wiop.ArithmeticOperation)
	if !ok {
		return 0
	}
	s := 1
	for _, o := range op.Operands {
		s += treeSize(o)
	}
	return s
}

// TestQuotientProgramMatchesReference checks the program against a direct
// per-point evaluation, for chunkings that exercise partial batches, batch
// boundaries and the wrap-around of shifted leaves.
func TestQuotientProgramMatchesReference(t *testing.T) {
	for seed := range uint64(8) {
		rng := rand.New(rand.NewPCG(seed, 0xD06))
		f := newProgramFixture(t, rng, 200)

		roots := make([]quotientRoot, 0, 40)
		var coinPow field.Ext
		coin := randExt(rng)
		coinPow.SetOne()
		for range 40 {
			r := quotientRoot{expr: f.pool[rng.IntN(len(f.pool))], coinPow: coinPow}
			if rng.IntN(2) == 0 {
				r.cancellation = make([]field.Element, f.N)
				for j := range r.cancellation {
					r.cancellation[j] = randElem(rng)
				}
			}
			roots = append(roots, r)
			coinPow.Mul(&coinPow, &coin)
		}

		want := make([]field.Ext, f.N)
		for j := range f.N {
			for _, r := range roots {
				p := f.refEval(r.expr, j)
				if r.cancellation != nil {
					p.MulByElement(&p, &r.cancellation[j])
				}
				p.Mul(&p, &r.coinPow)
				want[j].Add(&want[j], &p)
			}
		}

		prog := buildQuotientProgram(f.rt, roots, f.cosetEvals, f.cosetEvalsExt, f.selectorCosets, f.ratio, f.N)
		for _, chunk := range []int{f.N, quotientBatchSize, 7, 1} {
			got := make([]field.Ext, f.N)
			for start := 0; start < f.N; start += chunk {
				prog.run(got, start, min(start+chunk, f.N))
			}
			for j := range f.N {
				require.Truef(t, got[j].Equal(&want[j]), "seed %d chunk %d point %d", seed, chunk, j)
			}
		}
	}
}

// TestQuotientProgramSharesNodes checks that the program evaluates each
// distinct node once: shared pointers and structural duplicates collapse, and
// point-invariant subtrees are folded away.
func TestQuotientProgramSharesNodes(t *testing.T) {
	// x·y + x·y written with two distinct x·y pointers, and (c·c)·(x·y) with
	// a third one, where c is a constant.
	sys := wiop.NewSystemf("sharing")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	x := mod.NewColumn(sys.Context.Childf("x"), r0)
	y := mod.NewColumn(sys.Context.Childf("y"), r0)
	rt := wiop.NewRuntime(sys)
	N := 8
	evals := map[wiop.ObjectID][]field.Element{
		x.Context.ID: make([]field.Element, N),
		y.Context.ID: make([]field.Element, N),
	}
	c := wiop.NewConstantField(field.NewElement(3))
	a := wiop.Add(wiop.Mul(x.View(), y.View()), wiop.Mul(x.View(), y.View()))
	b := wiop.Mul(wiop.Mul(c, c), wiop.Mul(x.View(), y.View()))

	prog := buildQuotientProgram(rt, []quotientRoot{{expr: a}, {expr: b}}, evals, nil, nil, 1, N)
	// Distinct vector nodes: x, y, x·y, (x·y)+(x·y), 9·(x·y), against 6
	// compound nodes and 6 leaf reads in the trees; c·c folds to a scalar.
	require.Equal(t, 5, prog.nodes)
	require.Equal(t, 6, treeSize(a)+treeSize(b))
	require.Len(t, prog.instrs, 5+2) // 5 nodes + 2 accumulations
}
