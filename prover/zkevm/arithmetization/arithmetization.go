package arithmetization

import (
	"fmt"

	"github.com/consensys/go-corset/pkg/air"
	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// Settings specifies the parameters for the arithmetization part of the zkEVM.
type Settings struct {
	Limits *config.TracesLimits
	// OptimisationLevel determines the optimisation level which go-corset will
	// apply when compiling the zkevm.bin file to AIR constraints.  If in doubt,
	// use mir.DEFAULT_OPTIMISATION_LEVEL.
	OptimisationLevel *mir.OptimisationConfig
}

// Arithmetization exposes all the methods relevant for the user to interact
// with the arithmetization of the zkEVM. It is a sub-component of the whole
// ZkEvm object as it does not includes the precompiles, the keccaks and the
// signature verification.
type Arithmetization struct {
	Settings *Settings
	Schema   *air.Schema
}

// NewArithmetization is the function that declares all the columns and the constraints of
// the zkEVM in the input builder object.
func NewArithmetization(builder *wizard.Builder, settings Settings) *Arithmetization {
	schema, _, errS := ReadZkevmBin(settings.OptimisationLevel)
	if errS != nil {
		panic(errS)
	}

	Define(builder.CompiledIOP, schema, settings.Limits)

	return &Arithmetization{
		Schema:   schema,
		Settings: &settings,
	}
}

// Assign the arithmetization related columns. Namely, it will open the file
// specified in the witness object, call corset and assign the prover runtime
// columns.
func (a *Arithmetization) Assign(run *wizard.ProverRuntime, traceFile string) {

	traceF := files.MustRead(traceFile)
	trace, errT := ReadLtTraces(traceF, a.Schema)
	if errT != nil {
		fmt.Printf("error loading the trace fpath=%q err=%v", traceFile, errT.Error())
	}

	AssignFromLtTraces(run, a.Schema, trace, a.Settings.Limits)
}
