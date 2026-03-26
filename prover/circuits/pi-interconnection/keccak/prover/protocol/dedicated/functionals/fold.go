package functionals

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
)

type FoldProverAction struct {
	H           ifaces.Column
	X           ifaces.Accessor
	FoldedName  ifaces.ColID
	InnerDegree int
	FoldedSize  int
}

func (a *FoldProverAction) Run(assi *wizard.ProverRuntime) {
	h := a.H.GetColAssignment(assi)
	x := a.X.GetVal(assi)

	foldedVal := make([]field.Element, a.FoldedSize)
	for i := range foldedVal {
		subH := h.SubVector(i*a.InnerDegree, (i+1)*a.InnerDegree)
		foldedVal[i] = smartvectors.EvalCoeff(subH, x)
	}

	assi.AssignColumn(a.FoldedName, smartvectors.NewRegular(foldedVal))
}

type FoldVerifierAction struct {
	FoldedEvalAcc ifaces.Accessor
	HEvalAcc      ifaces.Accessor
	FoldedName    ifaces.ColID
}

func (a *FoldVerifierAction) Run(run wizard.Runtime) error {
	if a.FoldedEvalAcc.GetVal(run) != a.HEvalAcc.GetVal(run) {
		return fmt.Errorf("verifier of folding failed %v", a.FoldedName)
	}
	return nil
}

func (a *FoldVerifierAction) RunGnark(api frontend.API, wvc wizard.GnarkRuntime) {
	c := a.FoldedEvalAcc.GetFrontendVariable(api, wvc)
	c_ := a.HEvalAcc.GetFrontendVariable(api, wvc)
	api.AssertIsEqual(c, c_)
}

func Fold(comp *wizard.CompiledIOP, h ifaces.Column, x ifaces.Accessor, innerDegree int) ifaces.Column {

	round := x.Round()

	foldedSize := h.Size() / innerDegree
	foldedName := ifaces.ColIDf("FOLDED_%v_%v_%v", h.GetColID(), x.Name(), innerDegree)
	folded := comp.InsertCommit(round, foldedName, foldedSize)

	if x.Round() <= h.Round() {
		logrus.Debugf("Unsafe, the coin is before the commitment : %v", foldedName)
	}

	comp.RegisterProverAction(round, &FoldProverAction{
		H:           h,
		X:           x,
		FoldedName:  foldedName,
		InnerDegree: innerDegree,
		FoldedSize:  foldedSize,
	})

	outerCoinName := coin.Namef("OUTER_COIN_%v", folded.GetColID())
	outerCoin := comp.InsertCoin(round+1, outerCoinName, coin.Field)
	outerCoinAcc := accessors.NewFromCoin(outerCoin)

	foldedEvalAcc := CoeffEval(comp, folded.String(), outerCoin, folded)
	hEvalAcc := EvalCoeffBivariate(comp, folded.String(), h, x, outerCoinAcc, innerDegree, folded.Size())

	verRound := utils.Max(outerCoinAcc.Round(), foldedEvalAcc.Round())

	// Check that the two evaluations yield the same result
	comp.RegisterVerifierAction(verRound, &FoldVerifierAction{
		FoldedEvalAcc: foldedEvalAcc,
		HEvalAcc:      hEvalAcc,
		FoldedName:    foldedName,
	})

	return folded
}
