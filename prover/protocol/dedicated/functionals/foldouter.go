package functionals

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

type FoldOuterProverAction struct {
	H           ifaces.Column
	X           ifaces.Accessor
	FoldedName  ifaces.ColID
	InnerDegree int
	OuterDegree int
	FoldedSize  int
}

func (a *FoldOuterProverAction) Run(assi *wizard.ProverRuntime) {
	h := a.H.GetColAssignment(assi)
	x := a.X.GetValExt(assi)

	innerChunks := make([]smartvectors.SmartVector, a.OuterDegree)
	for i := range innerChunks {
		innerChunks[i] = h.SubVector(i*a.InnerDegree, (i+1)*a.InnerDegree)
	}

	foldedVal := smartvectors.LinearCombinationExt(innerChunks, x)
	assi.AssignColumn(a.FoldedName, foldedVal)
}

type FoldOuterVerifierAction struct {
	FoldedEvalAcc ifaces.Accessor
	HEvalAcc      ifaces.Accessor
	FoldedName    ifaces.ColID
}

func (a *FoldOuterVerifierAction) Run(run wizard.Runtime) error {
	if a.FoldedEvalAcc.GetVal(run) != a.HEvalAcc.GetVal(run) {
		return fmt.Errorf("verifier of folding failed %v", a.FoldedName)
	}
	return nil
}

func (a *FoldOuterVerifierAction) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	koalaAPI := koalagnark.NewAPI(api)
	c := a.FoldedEvalAcc.GetFrontendVariableExt(api, run)
	c_ := a.HEvalAcc.GetFrontendVariableExt(api, run)
	koalaAPI.AssertIsEqualExt(c, c_)
}

// Same as fold but folds on the outer-variable rather than the inner variable
func FoldOuter(comp *wizard.CompiledIOP, h ifaces.Column, x ifaces.Accessor, outerDegree int) ifaces.Column {

	round := x.Round()

	foldedSize := h.Size() / outerDegree
	innerDegree := foldedSize
	foldedName := ifaces.ColIDf("FOLDED_OUTER_%v_%v_%v", h.GetColID(), x.Name(), outerDegree)
	folded := comp.InsertCommit(round, foldedName, foldedSize, false)

	if x.Round() <= h.Round() {
		logrus.Debugf("Unsafe, the coin is before the commitment : %v", foldedName)
	}

	comp.RegisterProverAction(round, &FoldOuterProverAction{
		H:           h,
		X:           x,
		FoldedName:  foldedName,
		InnerDegree: innerDegree,
		OuterDegree: outerDegree,
		FoldedSize:  foldedSize,
	})

	innerCoinName := coin.Namef("INNER_COIN_%v", folded.GetColID())
	innerCoin := comp.InsertCoin(round+1, innerCoinName, coin.FieldExt)
	innerCoinAcc := accessors.NewFromCoin(innerCoin)

	foldedEvalAcc := CoeffEval(comp, folded.String(), innerCoin, folded)
	hEvalAcc := EvalCoeffBivariate(comp, folded.String(), h, innerCoinAcc, x, innerDegree, outerDegree)

	verRound := utils.Max(innerCoinAcc.Round(), foldedEvalAcc.Round())

	// Check that the two evaluations yield the same result
	comp.RegisterVerifierAction(verRound, &FoldOuterVerifierAction{
		FoldedEvalAcc: foldedEvalAcc,
		HEvalAcc:      hEvalAcc,
		FoldedName:    foldedName,
	})

	return folded
}
