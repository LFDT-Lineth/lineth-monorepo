package reedsolomon

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
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
	domain := fft.NewDomain(uint64(witness.Len()), fft.WithCache())

	if a.H.IsBase() {
		coeffs := make([]field.Element, witness.Len())
		witness.WriteInSlice(coeffs)
		domain.FFTInverse(coeffs, fft.DIF, fft.WithNbTasks(2))
		utils.BitReverse(coeffs)
		// Take only the first `CodeDim` coefficients
		assi.AssignColumn(a.Coeff.GetColID(), smartvectors.NewRegular(coeffs[:a.CodeDim]))
		return
	}

	coeffs := make([]fext.Element, witness.Len())
	witness.WriteInSliceExt(coeffs)
	domain.FFTInverseExt(coeffs, fft.DIF, fft.WithNbTasks(2))
	utils.BitReverse(coeffs)
	// Take only the first `CodeDim` coefficients
	assi.AssignColumn(a.Coeff.GetColID(), smartvectors.NewRegularExt(coeffs[:a.CodeDim]))
}

type ReedSolomonVerifierAction struct {
	CoeffCheck ifaces.Accessor
	EvalCheck  ifaces.Accessor
	HColID     ifaces.ColID
}

func (a *ReedSolomonVerifierAction) Run(run wizard.Runtime) error {
	y := a.CoeffCheck.GetValExt(run)
	y_ := a.EvalCheck.GetValExt(run)

	if y != y_ {
		return fmt.Errorf("reed-solomon check failed - %v is not a codeword", a.HColID)
	}
	return nil
}

func (a *ReedSolomonVerifierAction) RunGnark(api frontend.API, wvc wizard.GnarkRuntime) {
	koalaAPI := koalagnark.NewAPI(api)
	y := a.CoeffCheck.GetFrontendVariableExt(api, wvc)
	y_ := a.EvalCheck.GetFrontendVariableExt(api, wvc)
	koalaAPI.AssertIsEqualExt(y, y_)
}

// Is code-member
func CheckReedSolomon(comp *wizard.CompiledIOP, rate int, h ifaces.Column) {

	round := h.Round()
	codeDim := h.Size() / rate
	coeff := comp.InsertCommit(
		round,
		ifaces.ColIDf("%v_%v", REED_SOLOMON_COEFF, h.GetColID()),
		codeDim,
		h.IsBase(),
	)

	beta := comp.InsertCoin(
		round+1,
		coin.Namef("%v_%v", REED_SOLOMON_BETA, h.GetColID()),
		coin.FieldExt,
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
