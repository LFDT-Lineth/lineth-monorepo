package functionals

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	EVAL_COEFF_POLY                 string = "EVAL_COEFF_COEFF_EVAL"
	EVAL_COEFF_LOCAL_CONSTRAINT_END string = "EVAL_COEFF_LOCAL_CONSTRAINT_END"
	EVAL_COEFF_FIXED_POINT_BEGIN    string = "EVAL_COEFF_FIXED_POINT_BEGIN"
	EVAL_COEFF_GLOBAL               string = "EVAL_COEFF_GLOBAL"
)

type CoeffEvalProverAction struct {
	Name               string
	X                  ifaces.Accessor
	Pol                ifaces.Column
	Length             int
	ResultLocalOpening query.LocalOpening
	Round              int
}

// Create a dedicated wizard to perform an evaluation in coefficient basis.
// Returns an accessor for the value of the polynomial. Takes an expression
// as input.
func CoeffEval(comp *wizard.CompiledIOP, name string, x coin.Info, pol ifaces.Column) ifaces.Accessor {
	pa, res := CoeffEvalNoRegisterPA(comp, name, accessors.NewFromCoin(x), pol)
	if pa != nil {
		comp.RegisterProverAction(pa.Round, pa)
	}
	return res
}

// CoeffEvalNoRegisterPA returns the same as CoeffEval but does not register
// the prover action and returns it instead.
func CoeffEvalNoRegisterPA(comp *wizard.CompiledIOP, name string, x ifaces.Accessor, pol ifaces.Column) (*CoeffEvalProverAction, ifaces.Accessor) {

	length := pol.Size()
	maxRound := utils.Max(x.Round(), pol.Round())

	// When the length of the input is 1 (this can happen when meeting edge-cases
	// ). Then the general purpose solution does not work due to the shift being
	// incorrect. So instead, we simply make "P" to be a proof element and we
	// return an accessor for it. The cost of making it public is essentially
	// zero.
	if length == 1 {

		// When the column is precomputed, we can just return a constant accessor
		// as the return value will be static anyway.
		if comp.Precomputed.Exists(pol.GetColID()) {
			val := comp.Precomputed.MustGet(pol.GetColID()).Get(0)
			return nil, accessors.NewConstant(val)
		}

		// Else, we promote the column to a proof column if it is not already
		// one.
		status := comp.Columns.Status(pol.GetColID())

		switch status {
		case column.Committed:
			comp.Columns.SetStatus(pol.GetColID(), column.Proof)
		case column.Proof:
			// Do nothing
		default:
			panic("the column is neither committed nor proof; this is sort of an unexpected case and this indicates that one missing case has not been implemented.")
		}

		return nil, accessors.NewFromPublicColumn(pol, 0)
	}

	hornerPoly := comp.InsertCommit(
		maxRound,
		ifaces.ColIDf("%v_%v", name, EVAL_COEFF_POLY),
		length,
		false,
	)

	// (x * h[i+1]) + expr[i] == h[i]
	// This will be cancelled at the border already
	// if length == 1, we skip the shift as the poly is constant
	globalExpr := ifaces.ColumnAsVariable(column.Shift(hornerPoly, 1)).
		Mul(x.AsVariable()).
		Add(ifaces.ColumnAsVariable(pol)).
		Sub(ifaces.ColumnAsVariable(hornerPoly))

	comp.InsertGlobal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, EVAL_COEFF_GLOBAL),
		globalExpr,
	)

	// p[-1] = h[-1]
	comp.InsertLocal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, EVAL_COEFF_LOCAL_CONSTRAINT_END),
		ifaces.ColumnAsVariable(column.Shift(hornerPoly, -1)).
			Sub(ifaces.ColumnAsVariable(column.Shift(pol, -1))),
	)

	// The result is given by
	localOpening := comp.InsertLocalOpening(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, EVAL_COEFF_FIXED_POINT_BEGIN),
		hornerPoly,
	)

	pa := &CoeffEvalProverAction{
		Name:               name,
		X:                  x,
		Pol:                pol,
		Length:             length,
		ResultLocalOpening: localOpening,
		Round:              maxRound,
	}

	return pa, accessors.NewLocalOpeningAccessor(localOpening, maxRound)
}

func (a *CoeffEvalProverAction) Run(assi *wizard.ProverRuntime) {
	x := a.X.GetValExt(assi)
	p := a.Pol.GetColAssignment(assi)

	h := make([]fext.Element, a.Length)
	h[a.Length-1] = p.GetExt(a.Length - 1)

	for i := a.Length - 2; i >= 0; i-- {
		pi := p.GetExt(i)
		h[i].Mul(&h[i+1], &x)
		h[i].Add(&h[i], &pi)
	}

	assi.AssignColumn(ifaces.ColIDf("%v_%v", a.Name, EVAL_COEFF_POLY), smartvectors.NewRegularExt(h))
	assi.AssignLocalPointExt(a.ResultLocalOpening.ID, h[0])
}
