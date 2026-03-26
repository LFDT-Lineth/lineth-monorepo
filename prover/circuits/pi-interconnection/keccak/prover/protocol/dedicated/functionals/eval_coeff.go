package functionals

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

const (
	EVAL_COEFF_POLY                 string = "EVAL_COEFF_COEFF_EVAL"
	EVAL_COEFF_LOCAL_CONSTRAINT_END string = "EVAL_COEFF_LOCAL_CONSTRAINT_END"
	EVAL_COEFF_FIXED_POINT_BEGIN    string = "EVAL_COEFF_FIXED_POINT_BEGIN"
	EVAL_COEFF_GLOBAL               string = "EVAL_COEFF_GLOBAL"
)

type CoeffEvalProverAction struct {
	Name   string
	X      coin.Info
	Pol    ifaces.Column
	Length int
}

func (a *CoeffEvalProverAction) Run(assi *wizard.ProverRuntime) {
	x := assi.GetRandomCoinField(a.X.Name)
	p := a.Pol.GetColAssignment(assi)

	h := make([]field.Element, a.Length)
	h[a.Length-1] = p.Get(a.Length - 1)

	for i := a.Length - 2; i >= 0; i-- {
		pi := p.Get(i)
		h[i].Mul(&h[i+1], &x).Add(&h[i], &pi)
	}

	assi.AssignColumn(ifaces.ColIDf("%v_%v", a.Name, EVAL_COEFF_POLY), smartvectors.NewRegular(h))
	assi.AssignLocalPoint(ifaces.QueryIDf("%v_%v", a.Name, EVAL_COEFF_FIXED_POINT_BEGIN), h[0])
}

// Create a dedicated wizard to perform an evaluation in coefficient basis.
// Returns an accessor for the value of the polynomial. Takes an expression
// as input.
func CoeffEval(comp *wizard.CompiledIOP, name string, x coin.Info, pol ifaces.Column) ifaces.Accessor {

	length := pol.Size()
	maxRound := utils.Max(x.Round, pol.Round())

	hornerPoly := comp.InsertCommit(
		maxRound,
		ifaces.ColIDf("%v_%v", name, EVAL_COEFF_POLY),
		length,
	)

	// (x * h[i+1]) + expr[i] == h[i]
	// This will be cancelled at the border already
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

	comp.RegisterProverAction(maxRound, &CoeffEvalProverAction{
		Name:   name,
		X:      x,
		Pol:    pol,
		Length: length,
	})

	return accessors.NewLocalOpeningAccessor(localOpening, maxRound)
}
