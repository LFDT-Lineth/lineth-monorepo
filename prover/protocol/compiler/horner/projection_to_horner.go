package horner

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// ProjectionContext is a compilation artefact generated during the execution of
// the [InsertProjection] and which is used to instantiate the Horner query.
type ProjectionContext struct {
	// Query is the Horner query generated during the compilation of the projection
	// queries.
	Query query.Horner
}

// AssignHornerQuery is a [wizard.ProverAction] that assigns the Horner query from
// the [projectionContext] to the [wizard.ProverRuntime]. The final value is zero
// and the N0 values are zero. The function additionally sanity-checks the values
// of the Horner query.
type AssignHornerQuery struct {
	ProjectionContext
}

// CheckHornerQuery result is a [wizard.VerifierAction] that can be used to check
// the value of a Horner query.
type CheckHornerQuery struct {
	ProjectionContext
	skipped bool `serde:"omit"`
}

// ProjectionToHorner is a compilation step that compiles [query.Projection] queries
// into [query.Horner] queries.
func ProjectionToHorner(comp *wizard.CompiledIOP) {

	round := 0
	parts := []query.HornerPart{}
	ctx := ProjectionContext{}

	for _, qName := range comp.QueriesNoParams.AllUnignoredKeys() {

		// Filter out non projection queries
		projection, ok := comp.QueriesNoParams.Data(qName).(query.Projection)
		if !ok {
			continue
		}

		comp.QueriesNoParams.MarkAsIgnored(qName)

		var (
			qRound     = projection.Round
			widthA     = len(projection.Inp.ColumnsA)
			widthB     = len(projection.Inp.ColumnsB)
			numCols    = len(projection.Inp.ColumnsA[0])
			as         = make([]*sym.Expression, widthA)
			bs         = make([]*sym.Expression, widthB)
			selectorsA = make([]ifaces.Column, widthA)
			selectorsB = make([]ifaces.Column, widthB)
			gamma      coin.Info
			alpha      = comp.InsertCoin(qRound+1, coin.Name(qName+"_COIN_ALPHA"), coin.FieldExt)
		)

		round = max(round, qRound+1)

		if numCols > 1 {
			gamma = comp.InsertCoin(qRound+1, coin.Name(qName+"_COIN_GAMMA"), coin.FieldExt)
		}

		for i := 0; i < widthA; i++ {

			as[widthA-i-1] = sym.NewVariable(projection.Inp.ColumnsA[i][0])
			if numCols > 1 {
				as[widthA-i-1] = wizardutils.RandLinCombColSymbolic(gamma, projection.Inp.ColumnsA[i])
			}

			// The reversal in the assignment is required due to the order
			// in which the [Horner] query iterates over the coefficient in
			// the multi-ary settings.
			selectorsA[widthA-i-1] = projection.Inp.FiltersA[i]
		}

		for i := 0; i < widthB; i++ {

			bs[widthB-i-1] = sym.NewVariable(projection.Inp.ColumnsB[i][0])
			if numCols > 1 {
				bs[widthB-i-1] = wizardutils.RandLinCombColSymbolic(gamma, projection.Inp.ColumnsB[i])
			}

			// The reversal in the assignment is required due to the order
			// in which the [Horner] query iterates over the coefficient in
			// the multi-ary settings.
			selectorsB[widthB-i-1] = projection.Inp.FiltersB[i]
		}

		parts = append(
			parts,
			query.HornerPart{
				Name:         string(qName) + "_A",
				Coefficients: as,
				Selectors:    selectorsA,
				X:            accessors.NewFromCoin(alpha),
			},
			query.HornerPart{
				Name:         string(qName) + "_B",
				SignNegative: true,
				Coefficients: bs,
				Selectors:    selectorsB,
				X:            accessors.NewFromCoin(alpha),
			},
		)
	}

	if len(parts) == 0 {
		return
	}

	ctx.Query = comp.InsertHornerQuery(round, ifaces.QueryIDf("PROJECTION_TO_HORNER_%v", comp.SelfRecursionCount), parts)
	comp.RegisterProverAction(round, AssignHornerQuery{ctx})
	comp.RegisterVerifierAction(round, &CheckHornerQuery{ProjectionContext: ctx})
}

func (a AssignHornerQuery) Run(run *wizard.ProverRuntime) {

	params := query.HornerParams{}

	for range a.Query.Parts {
		params.Parts = append(params.Parts, query.HornerParamsPart{
			N0: 0,
		})
	}

	params.SetResult(run, a.Query)

	if !params.FinalResult.IsZero() {
		utils.Panic("expected final result to be zero, but computed %v", params.FinalResult.String())
	}

	run.AssignHornerParams(a.Query.ID, params)
}

func (c *CheckHornerQuery) Run(run wizard.Runtime) error {

	params := run.GetHornerParams(c.Query.ID)

	if !params.FinalResult.IsZero() {
		return fmt.Errorf("expected final result to be zero")
	}

	for _, p := range params.Parts {
		if p.N0 != 0 {
			return fmt.Errorf("expected N0 to be zero")
		}
	}

	return nil
}

func (c *CheckHornerQuery) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	params := run.GetHornerParams(c.Query.ID)
	koalaAPI := koalagnark.NewAPI(api)

	zero := koalagnark.NewExt(fext.Zero())
	koalaAPI.AssertIsEqualExt(params.FinalResult, zero)

	for _, p := range params.Parts {
		api.AssertIsEqual(p.N0, 0)
	}
}

func (c *CheckHornerQuery) Skip() {
	c.skipped = true
}

func (c *CheckHornerQuery) IsSkipped() bool {
	return c.skipped
}
