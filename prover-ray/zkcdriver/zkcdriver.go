package zkcdriver

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/consensys/go-corset/pkg/asm"
	"github.com/consensys/go-corset/pkg/binfile"
	"github.com/consensys/go-corset/pkg/ir"
	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/ir/mir"
	"github.com/consensys/go-corset/pkg/schema/module"
	"github.com/consensys/go-corset/pkg/trace/lt"
	"github.com/consensys/go-corset/pkg/util/collection/typed"
	"github.com/consensys/go-corset/pkg/util/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/sirupsen/logrus"
)

// Settings specifies the parameters for the arithmetization part of the zkEVM.
type Settings struct {
	// IgnoreCompatibilityCheck disables the strong compatibility check.
	// Specifically, it does not require the constraints and the trace file to
	// have both originated from the same commit.  By default, the compatibility
	// check should be enabled.
	IgnoreCompatibilityCheck bool
	// OptimisationLevel determines the optimisation level which go-corset will
	// apply when compiling the zkevm.bin file to AIR constraints.  If in doubt,
	// use mir.DEFAULT_OPTIMISATION_LEVEL.
	OptimisationLevel *mir.OptimisationConfig
}

// ZkCDriver exposes all the methods relevant for the user to interact
// with the arithmetization of the zkEVM. It is a sub-component of the whole
// ZkEvm object as it does not includes the precompiles, the keccaks and the
// signature verification.
type ZkCDriver struct {
	Settings *Settings
	// Binary encoding of the zkevm.bin file, which captures the high-level
	// structure of constraints.  This is primarily useful for assembly
	// functions (as these have big differences between their assembly
	// representation and their constraints representation).
	BinaryFile *binfile.BinaryFile `serde:"omit"`
	// Air schema defines the low-level columns, constraints and computations
	// used to expand a given trace, and to subsequently to check
	// satisfiability.
	AirSchema *air.Schema[koalabear.Element] `serde:"omit"`
	// Maps each column in the raw trace file into one (or more) columns in the
	// expanded trace file.  In particular, columns which are too large for the
	// given field are split into multiple "limbs".
	LimbMapping module.LimbsMap `serde:"omit"`
	// Metadata embedded in the zkevm.bin file, as needed to check
	// compatibility.  Guaranteed non-nil.
	Metadata typed.Map `serde:"omit"`
}

// NewZkCDriver is the function that declares all the columns and the constraints of
// the zkEVM in the input builder object.
func NewZkCDriver(sys *wiop.System, settings Settings, bin io.Reader) *ZkCDriver {

	// Read and parse the binary file
	binf, metadata, errS := ReadZkevmBin(bin)
	if errS != nil {
		panic(errS)
	}

	// Compile binary file into an air.Schema
	schema, mapping := CompileZkevmBin(binf, settings.OptimisationLevel)
	// Translate air.Schema into prover's internal representation
	Define(sys, schema)

	return &ZkCDriver{
		BinaryFile:  binf,
		AirSchema:   schema,
		Settings:    &settings,
		LimbMapping: mapping,
		Metadata:    metadata,
	}
}

// Assign the arithmetization related columns. Namely, it will open the file
// specified in the witness object, call corset and assign the prover runtime
// columns. As part of the assignment process, the original trace is expanded
// according to the given schema.  The expansion process is about filling in
// computed columns with concrete values, such for determining multiplicative
// inverses, etc.
func (a *ZkCDriver) Assign(run *wiop.Runtime, traceFile string) {
	a.AssignWithPreRead(run, PreReadTrace(traceFile))
}

// PreReadResult holds the result of pre-reading a trace file.
type PreReadResult struct {
	RawTrace  lt.TraceFile
	Metadata  typed.Map
	Err       error
	TraceFile string
}

// PreReadTrace reads and parses a trace file, returning the raw trace data.
// This can be called early to overlap I/O with other work.
func PreReadTrace(traceFile string) PreReadResult {
	traceF, err := readTraceFile(traceFile)
	if err != nil {
		return PreReadResult{Err: err, TraceFile: traceFile}
	}
	rawTrace, metadata, err := ReadLtTraces(traceF)
	return PreReadResult{RawTrace: rawTrace, Metadata: metadata, Err: err, TraceFile: traceFile}
}

// AssignWithPreRead assigns arithmetization columns using a pre-read trace.
func (a *ZkCDriver) AssignWithPreRead(run *wiop.Runtime, preRead PreReadResult) {
	assignStart := time.Now()
	var (
		errs     []error
		rawTrace = preRead.RawTrace
		metadata = preRead.Metadata
		errT     = preRead.Err
	)
	logrus.Infof("[bootstrapper] trace available (pre-read): %v", time.Since(assignStart))

	// Extract commit metadata from both files
	zkevmBinCommit, zkevmBinCommitOk := a.Metadata.String("commit")
	traceFileCommit, traceFileCommitOk := metadata.String("commit")

	// Performs a compatibility check by comparing the constraints
	// commit of zkevm.bin with the constraints commit of the trace file.
	// Panics if an incompatibility is detected.
	if !a.Settings.IgnoreCompatibilityCheck {
		var errors []string

		if !zkevmBinCommitOk {
			errors = append(errors, "missing constraints commit metadata in 'zkevm.bin'")
		}

		if !traceFileCommitOk {
			errors = append(errors, "missing constraints commit metadata in '.lt' file")
		}

		// Check commit mismatch
		if zkevmBinCommit != traceFileCommit {
			errors = append(errors, "zkevm.bin incompatible with trace file")
			errors = append(errors, fmt.Sprintf("zkevm.bin: %s", zkevmBinCommit))
			errors = append(errors, fmt.Sprintf("trace file: %s", traceFileCommit))
		}

		// Panic only if there are errors
		if len(errors) > 0 {
			for _, err := range errors {
				logrus.Error(err)
			}
			logrus.Panic("compatibility check failed")
		}
	} else {
		logrus.Info("Skip constraints compatibility check between zkevm.bin and trace file")
		logrus.Infof("zkevm.bin: %s", zkevmBinCommit)
		logrus.Infof("trace file: %s", traceFileCommit)
	}

	if errT != nil {
		logrus.Warnf("error loading the trace fpath=%q err=%v", preRead.TraceFile, errT.Error())
	}

	propStart := time.Now()
	rawTrace, errs = asm.Propagate(a.BinaryFile.Schema, rawTrace)

	if len(errs) > 0 {
		logrus.Warnf("corset propagation gave the following errors: %v", errors.Join(errs...).Error())
	}

	logrus.Infof("[bootstrapper] propagation: %v", time.Since(propStart))

	expStart := time.Now()
	expandedTrace, errs := ir.NewTraceBuilder[koalabear.Element]().
		WithBatchSize(1024).
		WithRegisterMapping(a.LimbMapping).
		Build(a.AirSchema, rawTrace)

	if len(errs) > 0 {
		logrus.Warnf("corset expansion gave the following errors: %v", errors.Join(errs...).Error())
	}
	logrus.Infof("[bootstrapper] expansion: %v", time.Since(expStart))

	copyStart := time.Now()
	AssignFromLtTraces(run, expandedTrace)
	logrus.Infof("[bootstrapper] column assignment: %v", time.Since(copyStart))
	logrus.Infof("[bootstrapper] total Arithmetization.Assign: %v", time.Since(assignStart))
}
