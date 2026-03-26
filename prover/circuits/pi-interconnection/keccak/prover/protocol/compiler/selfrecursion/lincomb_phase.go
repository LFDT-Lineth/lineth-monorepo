package selfrecursion

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/reedsolomon"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// Linear combination phase,
//
//   - Prover sends the linear combination that he claims as a commitment Uα
//     But it's already provided during the context creation
//   - Test the RS membership using the dedicated wizard below
//   - Build a column from the alleged openings Ys
//   - Check the consistency of CoeffEval(Ys,α) = Interpolate(Uα,x) where x
//     is the opening point
func (ctx *SelfRecursionCtx) RowLinearCombinationPhase() {

	// The reed-solomon check
	reedsolomon.CheckReedSolomon(
		ctx.Comp,
		ctx.VortexCtx.BlowUpFactor,
		ctx.Columns.Ualpha)

	// Create the verifier column ys
	ctx.defineYs()

	// And do the consistency check
	ctx.consistencyBetweenYsAndUalpha()
}

// Gather the alleged evaluation proven by vortex into a vector
func (ctx *SelfRecursionCtx) defineYs() {
	var (
		rangesNonSis = []ifaces.ColID{}
		rangesSis    = []ifaces.ColID{}
	)
	// Includes the precomputed colIds
	if ctx.VortexCtx.IsNonEmptyPrecomputed() {
		precompColIds := make([]ifaces.ColID, len(ctx.VortexCtx.Items.Precomputeds.PrecomputedColums))
		for i, col := range ctx.VortexCtx.Items.Precomputeds.PrecomputedColums {
			precompColIds[i] = col.GetColID()
		}
		// If SIS is applied to precomputed, we need to add the precomputed
		// columns to the rangesSis, otherwise we add them to rangesNonSis
		if ctx.VortexCtx.IsSISAppliedToPrecomputed() {
			rangesSis = append(rangesSis, precompColIds...)
		} else {
			rangesNonSis = append(rangesNonSis, precompColIds...)
		}
	}
	// Collect the SIS round commitments
	for _, colIDs := range ctx.VortexCtx.CommitmentsByRoundsSIS.GetInner() {
		rangesSis = append(rangesSis, colIDs...)
	}
	// Collect the non-SIS round commitments
	for _, colIDs := range ctx.VortexCtx.CommitmentsByRoundsNonSIS.GetInner() {
		rangesNonSis = append(rangesNonSis, colIDs...)
	}
	// append the ranges
	ranges := append(rangesNonSis, rangesSis...)
	ctx.Columns.Ys = verifiercol.NewFromYs(ctx.Comp, ctx.VortexCtx.Query, ranges)
}

type ConsistencyYsUalphaVerifierAction struct {
	Ctx                *SelfRecursionCtx
	InterpolateUalphaX ifaces.Accessor
}

func (a *ConsistencyYsUalphaVerifierAction) Run(run wizard.Runtime) error {
	ys := a.Ctx.Columns.Ys.GetColAssignment(run)
	alpha := run.GetRandomCoinField(a.Ctx.Coins.Alpha.Name)
	ysAlpha := smartvectors.EvalCoeff(ys, alpha)
	uAlphaX := a.InterpolateUalphaX.GetVal(run)
	if uAlphaX != ysAlpha {
		return fmt.Errorf("ConsistencyBetweenYsAndUalpha did not pass, ysAlphaX=%v uAlphaX=%v", ysAlpha.String(), uAlphaX.String())
	}
	return nil
}

func (a *ConsistencyYsUalphaVerifierAction) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	ys := a.Ctx.Columns.Ys.GetColAssignmentGnark(run)
	alpha := run.GetRandomCoinField(a.Ctx.Coins.Alpha.Name)
	uAlphaX := a.InterpolateUalphaX.GetFrontendVariable(api, run)
	ysAlpha := poly.EvaluateUnivariateGnark(api, ys, alpha)
	api.AssertIsEqual(uAlphaX, ysAlpha)
}

// Registers the consistency check between Ys and Ualpha
func (ctx *SelfRecursionCtx) consistencyBetweenYsAndUalpha() {

	// Defer the interpolation of Ualpha to a dedicated wizard
	ctx.Accessors.InterpolateUalphaX = functionals.Interpolation(
		ctx.Comp,
		ctx.interpolateUAlphaX(),
		accessors.NewUnivariateX(ctx.VortexCtx.Query, ctx.Comp.QueriesParams.Round(ctx.VortexCtx.Query.QueryID)),
		ctx.Columns.Ualpha,
	)

	round := ctx.Accessors.InterpolateUalphaX.Round()

	// And let the verifier check that they should be both equal
	ctx.Comp.RegisterVerifierAction(round, &ConsistencyYsUalphaVerifierAction{
		Ctx:                ctx,
		InterpolateUalphaX: ctx.Accessors.InterpolateUalphaX,
	})
}
