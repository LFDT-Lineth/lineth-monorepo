// Package nonnative compiles every unreduced [wiop.NonNative] query into a
// polynomial identity checked at a random point.
//
// For each query, limbs are treated as coefficients of a polynomial (limb i
// is the coefficient of X^i) and the following Schwartz-Zippel identity is
// asserted at a random challenge X:
//
//	left(X)*right(X) - modulus(X)*quotient(X) - result(X) - carry(X)*(2^b - X) = 0
//
// where b is the query's NbBitsPerLimb and carry is a per-query column
// sequence computed by the compiler from the base-2^b schoolbook
// multiplication carries.
//
// All queries are checked at the *same* random point: the compiler samples a
// single coin, in the earliest round that follows every consumed query's
// InputRound, and reuses it for every identity it produces. A new round is
// only created if none already follows that round.
//
// The resulting [wiop.Vanishing] queries are left for the downstream
// localvanishing/global passes to check; this package does not itself
// assert them against the transcript beyond registering them.
package nonnative

import (
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Compile reduces every unreduced [wiop.NonNative] query in sys to a
// [wiop.Vanishing] polynomial identity, checked at one shared random point.
//
// It is a no-op if sys contains no eligible queries.
func Compile(sys *wiop.System) {
	var queries []*wiop.NonNative
	for _, m := range sys.Modules {
		for _, g := range m.NonNatives {
			if g.IsReduced() {
				continue
			}
			queries = append(queries, g)
		}
	}
	if len(queries) == 0 {
		return
	}

	coinRound := coinRoundFor(sys, queries)
	coin := coinRound.NewCoinField(sys.Context.Childf("nonnative-challenge"))

	for i, q := range queries {
		qCtx := q.Context().Childf("q%d", i)
		carry := newCarryColumns(q, qCtx.Childf("carry"))

		q.Module.NewVanishing(
			qCtx.Childf("identity"),
			identityExpr(q, coin, carry),
		)

		q.InputRound.RegisterAction(&carryProverAction{
			query: q,
			carry: carry,
		})

		q.MarkAsReduced()
	}
}

// coinRoundFor returns the round in which the shared challenge coin is drawn:
// the round immediately following the latest InputRound across queries. A new
// round is created only if none already follows it.
func coinRoundFor(sys *wiop.System, queries []*wiop.NonNative) *wiop.Round {
	maxInput := queries[0].InputRound
	for _, q := range queries[1:] {
		if q.InputRound.ID > maxInput.ID {
			maxInput = q.InputRound
		}
	}
	if next, ok := maxInput.Next(); ok {
		return next
	}
	return sys.NewRound()
}

// newCarryColumns allocates the carry columns for q, one column per limb of
// the schoolbook product Left*Right (equivalently Quotient*Modulus+Result).
func newCarryColumns(q *wiop.NonNative, ctx *wiop.ContextFrame) []*wiop.Column {
	nbCarry := nbMultiplicationResLimbs(len(q.Left), len(q.Right))
	carry := make([]*wiop.Column, nbCarry)
	for i := range carry {
		carry[i] = q.Module.NewColumn(ctx.Childf("limb-%d", i), wiop.VisibilityOracle, q.InputRound)
	}
	return carry
}

// identityExpr builds the symbolic polynomial identity
//
//	left(X)*right(X) - modulus(X)*quotient(X) - result(X) - carry(X)*(2^b - X)
//
// which must vanish, where X is the shared challenge coin.
func identityExpr(q *wiop.NonNative, x *wiop.CoinField, carry []*wiop.Column) wiop.Expression {
	leftEval := polyEval(q.Left, x)
	rightEval := polyEval(q.Right, x)
	modEval := polyEval(q.Modulus, x)
	quoEval := polyEval(q.Quotient, x)
	resEval := polyEval(q.Result, x)
	carryEval := polyEval(carry, x)

	// left(X)*right(X)
	lhs := wiop.Mul(leftEval, rightEval)

	// (2^b - X)
	var twoB field.Element
	twoB.SetUint64(uint64(1) << uint(q.NbBitsPerLimb))
	carryCoef := wiop.Sub(wiop.NewConstantField(twoB), x)

	return wiop.Sub(
		wiop.Sub(
			wiop.Sub(
				lhs,
				wiop.Mul(modEval, quoEval),
			),
			resEval,
		),
		wiop.Mul(carryEval, carryCoef),
	)
}

// polyEval builds the coefficient-basis polynomial Σ_i limb[i] * X^i as a
// symbolic expression, X being the shared challenge coin, using Horner's rule.
func polyEval(ls []*wiop.Column, x *wiop.CoinField) wiop.Expression {
	res := wiop.Expression(ls[len(ls)-1].View())
	for i := len(ls) - 2; i >= 0; i-- {
		res = wiop.Add(wiop.Mul(res, x), ls[i].View())
	}
	return res
}

// carryProverAction fills a query's carry columns from the runtime
// assignments of its operand columns.
type carryProverAction struct {
	query *wiop.NonNative
	carry []*wiop.Column
}

func (a *carryProverAction) Run(rt *wiop.Runtime) {
	q := a.query
	m := q.Module
	nbRows := m.RuntimeSize(rt)

	// getColumnAssignments fetches the runtime assignment of every column in cols.
	getColumnAssignments := func(cols []*wiop.Column) []*wiop.ConcreteVector {
		out := make([]*wiop.ConcreteVector, len(cols))
		for i, c := range cols {
			out[i] = rt.GetColumnAssignment(c)
		}
		return out
	}
	// readRow extracts the row-th element of each column assignment in cols into
	// out, as uint64.
	readRow := func(cols []*wiop.ConcreteVector, row int, out []uint64) {
		for j := range out {
			f := cols[j].ElementAtN(m.Padding, nbRows, row).AsBase()
			out[j] = f.Uint64()
		}
	}

	leftCols := getColumnAssignments(q.Left)
	rightCols := getColumnAssignments(q.Right)
	modCols := getColumnAssignments(q.Modulus)
	resCols := getColumnAssignments(q.Result)
	quoCols := getColumnAssignments(q.Quotient)

	carryCols := make([][]field.Element, len(a.carry))
	for i := range a.carry {
		carryCols[i] = make([]field.Element, nbRows)
	}

	leftRow := make([]uint64, len(q.Left))
	rightRow := make([]uint64, len(q.Right))
	modRow := make([]uint64, len(q.Modulus))
	resRow := make([]uint64, len(q.Result))
	quoRow := make([]uint64, len(q.Quotient))

	for i := range nbRows {
		readRow(leftCols, i, leftRow)
		readRow(rightCols, i, rightRow)
		readRow(modCols, i, modRow)
		readRow(resCols, i, resRow)
		readRow(quoCols, i, quoRow)

		row := carryRow(leftRow, rightRow, quoRow, modRow, resRow, q.NbBitsPerLimb)
		for j := range row {
			carryCols[j][i].Set(&row[j])
		}
	}

	for i := range a.carry {
		rt.AssignColumn(a.carry[i], &wiop.ConcreteVector{
			Plain: field.VecFromBase(carryCols[i]),
		})
	}
}

// carryRow computes the carry values for a single row of the multiplication
// identity, base 2^nbBitsPerLimb.
func carryRow(left, right, quo, mod, res []uint64, nbBitsPerLimb int) []field.Element {
	// XXX(ivokub): this is called for every row. We can optimize it by reusing
	// the pools for lhsProd, rhsProd etc., but then we need to be careful when
	// we add parallelization. Only if it shows in profile
	lhsProd := limbMul(left, right)
	rhsProd := limbMul(quo, mod)
	for j := range res {
		if j < len(rhsProd) {
			rhsProd[j] += res[j]
		}
	}
	nbCarry := max(len(lhsProd), len(rhsProd))
	out := make([]field.Element, nbCarry)
	carry := new(big.Int) // XXX(ivokub): can use uint64 for carry?
	tmp := new(big.Int)
	for j := 0; j < nbCarry; j++ {
		if j < len(lhsProd) {
			// + left*right
			carry.Add(carry, tmp.SetUint64(lhsProd[j]))
		}
		if j < len(rhsProd) {
			// - quo*mod - res
			carry.Sub(carry, tmp.SetUint64(rhsProd[j]))
		}
		// shift carry
		carry.Rsh(carry, uint(nbBitsPerLimb))
		out[j].SetBigInt(carry)
	}
	return out
}

// limbMul computes the schoolbook limb product (values may exceed 2^b; the
// carries are handled by the identity). res has len(lhs)+len(rhs)-1 limbs.
func limbMul(lhs, rhs []uint64) []uint64 {
	n := nbMultiplicationResLimbs(len(lhs), len(rhs))
	res := make([]uint64, n)
	for i := range lhs {
		for j := range rhs {
			res[i+j] += lhs[i] * rhs[j]
		}
	}
	return res
}

func nbMultiplicationResLimbs(lenLeft, lenRight int) int {
	return max(lenLeft+lenRight-1, 0)
}
