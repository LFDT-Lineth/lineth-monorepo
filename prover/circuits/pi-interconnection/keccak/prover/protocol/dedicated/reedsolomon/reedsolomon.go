package reedsolomon

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

const (
	REED_SOLOMON_COEFF       string = "REED_SOLOMON_COEFF"
	REED_SOLOMON_EVAL_CHECK  string = "REED_SOLOMON_EVAL_CHECK"
	REED_SOLOMON_COEFF_CHECK string = "REED_SOLOMON_COEFF_CHECK"
	REED_SOLOMON_BETA        string = "REED_SOLOMON_BETA"
)

type ReedSolomonProverAction struct {
	H       ifaces.Column
	Coeff   ifaces.Column
	CodeDim int
}

func (a *ReedSolomonProverAction) Run(assi *wizard.ProverRuntime) {
	witness := a.H.GetColAssignment(assi)
	coeffs := smartvectors.FFTInverse(witness, fft.DIF, true, 0, 0, nil).SubVector(0, a.CodeDim)
	assi.AssignColumn(a.Coeff.GetColID(), coeffs)
}

type ReedSolomonVerifierAction struct {
	CoeffCheck ifaces.Accessor
	EvalCheck  ifaces.Accessor
	HColID     ifaces.ColID
}

func (a *ReedSolomonVerifierAction) Run(run wizard.Runtime) error {
	y := a.CoeffCheck.GetVal(run)
	y_ := a.EvalCheck.GetVal(run)
	if y != y_ {
		return fmt.Errorf("reed-solomon check failed - %v is not a codeword", a.HColID)
	}
	return nil
}

func (a *ReedSolomonVerifierAction) RunGnark(api frontend.API, wvc wizard.GnarkRuntime) {
	y := a.CoeffCheck.GetFrontendVariable(api, wvc)
	y_ := a.EvalCheck.GetFrontendVariable(api, wvc)
	api.AssertIsEqual(y, y_)
}

// Is code-member
func CheckReedSolomon(comp *wizard.CompiledIOP, rate int, h ifaces.Column) {

	round := h.Round()
	codeDim := h.Size() / rate
	coeff := comp.InsertCommit(
		round,
		ifaces.ColIDf("%v_%v", REED_SOLOMON_COEFF, h.GetColID()),
		codeDim,
	)

	beta := comp.InsertCoin(
		round+1,
		coin.Namef("%v_%v", REED_SOLOMON_BETA, h.GetColID()),
		coin.Field,
	)

	// Inserts the prover before calling the sub-wizard so that it is executed
	// before the sub-prover's wizards.
	comp.RegisterProverAction(round, &ReedSolomonProverAction{
		H:       h,
		Coeff:   coeff,
		CodeDim: codeDim,
	})

	coeffCheck := functionals.CoeffEval(
		comp,
		fmt.Sprintf("%v_%v", REED_SOLOMON_COEFF_CHECK, h.GetColID()),
		beta,
		coeff,
	)

	evalCheck := functionals.Interpolation(
		comp,
		fmt.Sprintf("%v_%v", REED_SOLOMON_EVAL_CHECK, h.GetColID()),
		accessors.NewFromCoin(beta),
		h,
	)

	comp.RegisterVerifierAction(round+1, &ReedSolomonVerifierAction{
		CoeffCheck: coeffCheck,
		EvalCheck:  evalCheck,
		HColID:     h.GetColID(),
	})

}
