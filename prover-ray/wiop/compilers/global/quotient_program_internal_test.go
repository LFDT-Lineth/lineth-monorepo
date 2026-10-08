package global

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestQuotientProgram_MatchesTreeEvaluation checks that the quotient program
// produces exactly the aggregate of the point-by-point tree evaluation, on
// random buckets mixing base and extension tables at wrapping shifts, base and
// extension scalars, every operator (divisions and inversions hitting zero
// included), subtrees shared within and across entries, Horner-form linear
// combinations, cancellations, and sizes below, at and across a block.
func TestQuotientProgram_MatchesTreeEvaluation(t *testing.T) {
	for _, tc := range []struct{ n, ratio int }{
		{4, 1}, {8, 2}, {75, 4}, {qBlockSize, 1}, {2*qBlockSize + 40, 4}, {1024, 2},
	} {
		for seed := range uint64(4) {
			t.Run(fmt.Sprintf("n=%d/ratio=%d/seed=%d", tc.n, tc.ratio, seed), func(t *testing.T) {
				g := newExprGen(rand.New(rand.NewPCG(seed, uint64(tc.n))), tc.n, tc.ratio)
				entries := make([]boundEntry, 12)
				for i := range entries {
					entries[i] = boundEntry{expr: g.entryExpr(), coinPow: g.ext()}
					if g.rng.IntN(2) == 0 {
						entries[i].cancellation = g.baseTable()
					}
				}
				annInv := field.VecPseudoRandBase(g.rng, tc.ratio)

				N := tc.n * tc.ratio
				want := make([]field.Ext, N)
				for i := range entries {
					entries[i].accumulate(want, 0, N, tc.n)
				}
				for j := range want {
					want[j].MulByElement(&want[j], &annInv[j/tc.n])
				}

				got := make([]field.Ext, N)
				compileQuotientProgram(entries, tc.n, tc.ratio).run(got, annInv)
				require.Equal(t, want, got)
			})
		}
	}
}

// TestQuotientProgram_FoldsHornerForms checks that a log-derivative style
// constraint, a product of Horner-form random linear combinations each used
// twice, is lowered without any extension-by-extension product inside the
// linear combinations and with each combination computed once.
func TestQuotientProgram_FoldsHornerForms(t *testing.T) {
	g := newExprGen(rand.New(rand.NewPCG(7, 7)), 64, 1)
	d1, d2 := g.horner(8), g.horner(8)
	// d1·d2 − (d1 + d2): each denominator appears twice.
	expr := op(wiop.ArithmeticOperatorSub,
		op(wiop.ArithmeticOperatorMul, d1, d2),
		op(wiop.ArithmeticOperatorAdd, d1, d2))
	p := compileQuotientProgram([]boundEntry{{expr: expr, coinPow: g.ext()}}, g.n, 1)

	var linear, extMul int
	for _, st := range p.steps {
		if st.entry >= 0 {
			continue
		}
		n := &p.nodes[st.node]
		switch {
		case n.kind == qLinear:
			linear++
		case n.kind == qOp && !n.isBase && n.op == wiop.ArithmeticOperatorMul:
			extMul++
		}
	}
	// d1 and d2 are two linear steps; d1 + d2 folds into a third. The only
	// extension product left is d1·d2.
	require.Equal(t, 3, linear)
	require.Equal(t, 1, extMul)
}

// exprGen draws random bound expressions over a fixed set of coset tables of
// ratio small cosets of n points.
type exprGen struct {
	rng        *rand.Rand
	n, N       int
	baseTables [][]field.Element
	extTables  [][]field.Ext
	pool       []boundExpr // earlier subtrees, reused to create sharing
}

func newExprGen(rng *rand.Rand, n, ratio int) *exprGen {
	g := &exprGen{rng: rng, n: n, N: n * ratio}
	for range 5 {
		g.baseTables = append(g.baseTables, g.baseTable())
	}
	for range 3 {
		ext := field.VecPseudoRandExt(rng, g.N)
		for j := range ext {
			if rng.IntN(10) == 0 {
				ext[j] = field.Ext{}
			}
		}
		g.extTables = append(g.extTables, ext)
	}
	return g
}

// baseTable returns N random base elements, a tenth of them zero.
func (g *exprGen) baseTable() []field.Element {
	v := field.VecPseudoRandBase(g.rng, g.N)
	for j := range v {
		if g.rng.IntN(10) == 0 {
			v[j] = field.Element{}
		}
	}
	return v
}

func (g *exprGen) ext() field.Ext { return field.VecPseudoRandExt(g.rng, 1)[0] }

func (g *exprGen) leaf() boundExpr {
	switch g.rng.IntN(6) {
	case 0, 1:
		return boundExpr{kind: boundVecBase, isBase: true,
			vecBase: g.baseTables[g.rng.IntN(len(g.baseTables))], offset: g.rng.IntN(g.n)}
	case 2:
		return boundExpr{kind: boundVecExt,
			vecExt: g.extTables[g.rng.IntN(len(g.extTables))], offset: g.rng.IntN(g.n)}
	case 3:
		var s field.Element
		if g.rng.IntN(4) != 0 {
			s = field.VecPseudoRandBase(g.rng, 1)[0]
		}
		return boundExpr{kind: boundScalarBase, isBase: true, scalarBase: s}
	default:
		return boundExpr{kind: boundScalarExt, scalarExt: g.ext()}
	}
}

func op(o wiop.ArithmeticOperator, operands ...boundExpr) boundExpr {
	isBase := true
	for i := range operands {
		isBase = isBase && operands[i].isBase
	}
	return boundExpr{kind: boundOp, isBase: isBase, operator: o, operands: operands}
}

var allOperators = []wiop.ArithmeticOperator{
	wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub, wiop.ArithmeticOperatorMul,
	wiop.ArithmeticOperatorDiv, wiop.ArithmeticOperatorDouble, wiop.ArithmeticOperatorSquare,
	wiop.ArithmeticOperatorNegate, wiop.ArithmeticOperatorInverse,
}

func (g *exprGen) expr(depth int) boundExpr {
	if depth == 0 || g.rng.IntN(5) == 0 {
		if len(g.pool) > 0 && g.rng.IntN(3) == 0 {
			return g.pool[g.rng.IntN(len(g.pool))]
		}
		return g.leaf()
	}
	o := allOperators[g.rng.IntN(len(allOperators))]
	var e boundExpr
	switch o {
	case wiop.ArithmeticOperatorAdd, wiop.ArithmeticOperatorSub,
		wiop.ArithmeticOperatorMul, wiop.ArithmeticOperatorDiv:
		e = op(o, g.expr(depth-1), g.expr(depth-1))
	default:
		e = op(o, g.expr(depth-1))
	}
	g.pool = append(g.pool, e)
	return e
}

// horner returns α + β·(c₁ + β·(c₂ + … β·cₖ)) over random base tables, with
// α and β extension scalars: the shape of a random linear combination.
func (g *exprGen) horner(k int) boundExpr {
	beta := boundExpr{kind: boundScalarExt, scalarExt: g.ext()}
	acc := g.leafBase()
	for range k - 1 {
		acc = op(wiop.ArithmeticOperatorAdd, op(wiop.ArithmeticOperatorMul, beta, acc), g.leafBase())
	}
	return op(wiop.ArithmeticOperatorAdd, boundExpr{kind: boundScalarExt, scalarExt: g.ext()}, acc)
}

func (g *exprGen) leafBase() boundExpr {
	return boundExpr{kind: boundVecBase, isBase: true,
		vecBase: g.baseTables[g.rng.IntN(len(g.baseTables))], offset: g.rng.IntN(g.n)}
}

// entryExpr mixes free-form trees with log-derivative style products of
// Horner forms.
func (g *exprGen) entryExpr() boundExpr {
	if g.rng.IntN(3) == 0 {
		d1, d2 := g.horner(1+g.rng.IntN(6)), g.horner(1+g.rng.IntN(6))
		return op(wiop.ArithmeticOperatorSub,
			op(wiop.ArithmeticOperatorMul, g.expr(2), op(wiop.ArithmeticOperatorMul, d1, d2)),
			op(wiop.ArithmeticOperatorAdd, op(wiop.ArithmeticOperatorMul, g.leaf(), d2), d1))
	}
	return g.expr(5)
}
