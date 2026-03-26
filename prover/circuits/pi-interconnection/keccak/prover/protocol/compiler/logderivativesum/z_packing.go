package logderivativesum

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

const (
	packingArity = 3
)

// zCtx is an internal compilation structure responsible for grouping the
// "Sigma" columns together so that we can trade-off commitment complexity with
// global constraint degree.
//
// All the grouped "Sigma" columns are of the same size and same round for the
// same context.
// M is computing the appearance of the rows of T in the rows of S.

// M:       tableCtx.M,
// S:       checkTable,
// T:       lookupTable,
// SFilter: includedFilters,

type ZCtx struct {
	Round, Size      int
	SigmaNumerator   []*sym.Expression // T -> -M, S -> +Filter
	SigmaDenominator []*sym.Expression // S or T -> ({S,T} + X)

	ZNumeratorBoarded, ZDenominatorBoarded []sym.ExpressionBoard

	Zs []ifaces.Column
	// ZOpenings are the opening queries to the end of each Z.
	ZOpenings []query.LocalOpening
	Name      string
}

// check permutation and see how/where Compile is called (see how to constracut z there)
// when constructing z, check if z is T or S
// and change T -> -M, S -> +Filter
// S or T -> ({S,T} + X)
// Compile should be called inside CompileGrandSum
func (z *ZCtx) Compile(comp *wizard.CompiledIOP) {

	var (
		numZs = utils.DivCeil(
			len(z.SigmaDenominator),
			packingArity,
		)
	)

	z.Zs = make([]ifaces.Column, numZs)
	z.ZOpenings = make([]query.LocalOpening, numZs)
	z.ZNumeratorBoarded = make([]sym.ExpressionBoard, numZs)
	z.ZDenominatorBoarded = make([]sym.ExpressionBoard, numZs)

	for i := range z.Zs {

		var (
			packedNum = safeAnySubSlice(z.SigmaNumerator, i*packingArity, (i+1)*packingArity)
			packedDen = safeAnySubSlice(z.SigmaDenominator, i*packingArity, (i+1)*packingArity)

			zNumerator   = sym.NewConstant(0)
			zDenominator = sym.Mul(packedDen...)
		)

		for j := range packedNum {
			term := packedNum[j]
			for k := range packedDen {
				if k != j {
					term = sym.Mul(term, packedDen[k])
				}
			}
			zNumerator = sym.Add(zNumerator, term)
		}

		z.ZNumeratorBoarded[i] = zNumerator.Board()
		z.ZDenominatorBoarded[i] = zDenominator.Board()

		z.Zs[i] = comp.InsertCommit(
			z.Round,
			DeriveName[ifaces.ColID]("Z", comp.SelfRecursionCount, z.Round, z.Size, i),
			z.Size,
		)

		// initial condition
		comp.InsertLocal(
			z.Round,
			DeriveName[ifaces.QueryID]("Z_CONSISTENCY_START", comp.SelfRecursionCount, z.Round, z.Size, i),
			sym.Sub(
				zNumerator,
				sym.Mul(
					z.Zs[i],
					zDenominator,
				),
			),
		)

		// This is the consistency check ensuring that Zs[i] is well-computed vs
		// Zs[i-1]. The check is skipped if the size is 1 because this is not
		// needed in that case and because it creates an edge-case where Zs and
		// Zs << 1 are the same column and cancel out in the expression.
		//
		// In theory, we could also simplify the whole compilation process for that
		// situation by merging the initial and the final checks into one, but that
		// would add complexity and not improve much.
		if z.Size > 1 {
			comp.InsertGlobal(
				z.Round,
				DeriveName[ifaces.QueryID]("Z_CONSISTENCY", comp.SelfRecursionCount, z.Round, z.Size, i),
				sym.Sub(
					zNumerator,
					sym.Mul(
						sym.Sub(z.Zs[i], column.Shift(z.Zs[i], -1)),
						zDenominator,
					),
				),
			)
		}

		// local opening of the final value of the Z polynomial
		z.ZOpenings[i] = comp.InsertLocalOpening(
			z.Round,
			DeriveName[ifaces.QueryID]("Z_FINAL", comp.SelfRecursionCount, z.Round, z.Size, i),
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
