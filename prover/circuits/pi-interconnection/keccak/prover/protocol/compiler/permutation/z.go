package permutation

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

const (
	// packingArity determines how factors can be packed into the numerator and
	// into the numerator of the denominator of a single Z.
	packingArity = 3
)

// ZCtx is the part of the compilation responsible for computing Zs column for
// for each query. For a purpose of efficiency, we try to pack the multiple
// products \prod (\gamma + A) / (\gamma + B) from separate queries into larger
// products. Importantly, this is secure because each permutation query uses a
// different coins. This allows the scheme to fall into the analysis of the
// paper.
//
// Without it, it would allow mixing and matching between the different queries.
type ZCtx struct {
	// Round is the declaration round of the Z polynomials.
	Round int

	// Size is the shared size of all the involved columns.
	Size int

	// NumeratorFactors is the list of expressions of the form (\gamma + A) or
	// (\gamma + lincomb(A, \alpha). The corresponding alpha and gamma can be
	// the same if the factors emanates from the same query.
	NumeratorFactors []*symbolic.Expression

	// DenominatorFactor is as NumeratorFactors but for the `Bs` of the queries
	DenominatorFactors []*symbolic.Expression

	// NumeratorFactorsBoarded are DenominatorFactorsBoarded the boarded
	// expressions and are "precomputed" versions of the expression which allow
	// faster execution.
	NumeratorFactorsBoarded, DenominatorFactorsBoarded []symbolic.ExpressionBoard

	// Zs is the list of the packed Zs
	Zs []ifaces.Column

	// ZOpenings are the opening queries to the end of each Z.
	ZOpenings []query.LocalOpening
}

// NewZCtxFromGrandProduct creates list of zctxs from a single grand-product
// query.
func NewZCtxFromGrandProduct(gp query.GrandProduct) []*ZCtx {

	var (
		cmpInt = func(a, b int) bool {
			return a < b
		}
		sizes = utils.SortedKeysOf(gp.Inputs, cmpInt)
		zctxs = make([]*ZCtx, 0, len(sizes))
	)

	for _, size := range sizes {
		zctx := &ZCtx{
			Size:               size,
			Round:              gp.Round,
			NumeratorFactors:   gp.Inputs[size].Numerators,
			DenominatorFactors: gp.Inputs[size].Denominators,
		}
		zctxs = append(zctxs, zctx)
	}

	return zctxs
}

// compileZs declares the Z polynomials and constraint them. It assumes that the
// current z context is partially filled with their Size, Round, NumeratorFactors
// and DenominatorFactors (not the boarded one).
func (z *ZCtx) Compile(comp *wizard.CompiledIOP) {

	var (
		numZs = utils.Max(
			utils.DivCeil(len(z.NumeratorFactors), packingArity),
			utils.DivCeil(len(z.DenominatorFactors), packingArity),
		)
	)

	z.Zs = make([]ifaces.Column, numZs)
	z.ZOpenings = make([]query.LocalOpening, numZs)
	z.NumeratorFactorsBoarded = make([]symbolic.ExpressionBoard, numZs)
	z.DenominatorFactorsBoarded = make([]symbolic.ExpressionBoard, numZs)

	for i := range z.Zs {

		var (
			packedNum = safeAnySubSlice(z.NumeratorFactors, i*packingArity, (i+1)*packingArity)
			packedDen = safeAnySubSlice(z.DenominatorFactors, i*packingArity, (i+1)*packingArity)

			prodNumerator   = symbolic.NewConstant(1)
			prodDenominator = symbolic.NewConstant(1)
		)

		if len(packedNum) > 0 {
			prodNumerator = symbolic.Mul(packedNum...)
		}

		if len(packedDen) > 0 {
			prodDenominator = symbolic.Mul(packedDen...)
		}

		z.NumeratorFactorsBoarded[i] = prodNumerator.Board()
		z.DenominatorFactorsBoarded[i] = prodDenominator.Board()

		z.Zs[i] = comp.InsertCommit(
			z.Round,
			deriveNameGen[ifaces.ColID](comp.SelfRecursionCount, "Z", z.Round, z.Size, "PART", i),
			z.Size,
		)

		comp.InsertGlobal(
			z.Round,
			deriveNameGen[ifaces.QueryID](comp.SelfRecursionCount, "Z", z.Round, z.Size, "PART", i, "GLOBAL"),
			symbolic.Sub(
				symbolic.Mul(z.Zs[i], prodDenominator),
				symbolic.Mul(column.Shift(z.Zs[i], -1), prodNumerator),
			),
		)

		comp.InsertLocal(
			z.Round,
			deriveNameGen[ifaces.QueryID](comp.SelfRecursionCount, "Z", z.Round, z.Size, "PART", i, "LOCAL_INIT"),
			symbolic.Sub(
				symbolic.Mul(z.Zs[i], prodDenominator),
				prodNumerator,
			),
		)

		z.ZOpenings[i] = comp.InsertLocalOpening(
			z.Round,
			deriveNameGen[ifaces.QueryID](comp.SelfRecursionCount, "Z", z.Round, z.Size, "PART", i, "END_OPENING"),
			column.Shift(z.Zs[i], -1),
		)
	}

}

// attempt to take the subslice of a slice, and truncates or returns an empty
// slice if the parameters are out of bounds.
func safeAnySubSlice[T any](t []T, start, stop int) []any {

	if stop < start {
		panic("invalid argument")
	}

	var tmp []T

	switch {
	case start >= len(t):
		return []any{}
	case stop >= len(t):
		tmp = t[start:]
	default:
		tmp = t[start:stop]
	}

	res := make([]any, len(tmp))
	for i := range res {
		res[i] = tmp[i]
	}

	return res
}
