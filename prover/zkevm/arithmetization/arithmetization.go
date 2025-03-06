package arithmetization

import (
	"errors"
	"fmt"
	"strings"

	"github.com/consensys/go-corset/pkg/air"
	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/go-corset/pkg/schema"
	"github.com/consensys/go-corset/pkg/util/collection/typed"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

// Settings specifies the parameters for the arithmetization part of the zkEVM.
type Settings struct {
	Limits *config.TracesLimits
	// IgnoreCompatibilityCheck disables the strong compatibility check.
	// Specifically, it does not require the constraints and the trace file to
	// have both originated from the same commit.  By default, the compability
	// check should be enabled.
	IgnoreCompatibilityCheck *bool
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
	// Schema defines the columns, constraints and computations used to expand a
	// given trace, and to subsequently to check satisfiability.
	Schema *air.Schema
	// Metadata embedded in the zkevm.bin file, as needed to check
	// compatibility.  Guaranteed non-nil.
	Metadata typed.Map
}

// NewArithmetization is the function that declares all the columns and the constraints of
// the zkEVM in the input builder object.
func NewArithmetization(builder *wizard.Builder, settings Settings) *Arithmetization {
	schema, metadata, errS := ReadZkevmBin(settings.OptimisationLevel)
	if errS != nil {
		panic(errS)
	}

	Define(builder.CompiledIOP, schema, settings.Limits)

	return &Arithmetization{
		Schema:   schema,
		Settings: &settings,
		Metadata: metadata,
	}
}

// Assign the arithmetization related columns. Namely, it will open the file
// specified in the witness object, call corset and assign the prover runtime
// columns. As part of the assignment processs, the original trace is expanded
// according to the given schema.  The expansion process is about filling in
// computed columns with concrete values, such for determining multiplicative
// inverses, etc.
func (a *Arithmetization) Assign(run *wizard.ProverRuntime, traceFile string) {
	traceF := files.MustRead(traceFile)
	// Parse trace file and extract raw column data.
	rawColumns, metadata, errT := ReadLtTraces(traceF, a.Schema)

	// Performs a compatibility check by comparing the constraints
	// commit of zkevm.bin with the constraints commit of the trace file.
	// Panics if an incompatibility is detected.
	if *a.Settings.IgnoreCompatibilityCheck == false {
		var errors []string

		zkevmBinCommit, ok := a.Metadata.String("commit")
		if !ok {
			errors = append(errors, "missing constraints commit metadata in 'zkevm.bin'")
		}

		traceFileCommit, ok := metadata.String("commit")
		if !ok {
			errors = append(errors, "missing constraints commit metadata in '.lt' file")
		}

		// Check commit mismatch
		if zkevmBinCommit != traceFileCommit {
			errors = append(errors, fmt.Sprintf(
				"zkevm.bin incompatible with trace file (commit %s vs %s)",
				zkevmBinCommit, traceFileCommit,
			))
		}

		// Panic only if there are errors
		if len(errors) > 0 {
			logrus.Panic("compatibility check failed with error message:\n" + strings.Join(errors, "\n"))
		}
	} else {
		logrus.Info("Skip constraints compatibility check between zkevm.bin and trace file")
	}

	if errT != nil {
		fmt.Printf("error loading the trace fpath=%q err=%v", traceFile, errT.Error())
	}
	// Perform trace expansion
	expandedTrace, errs := schema.NewTraceBuilder(a.Schema).Build(rawColumns)
	if len(errs) > 0 {
		logrus.Warnf("corset expansion gave the following errors: %v", errors.Join(errs...).Error())
	}
	// Passed
	AssignFromLtTraces(run, a.Schema, expandedTrace, a.Settings.Limits)
}
