package zkcdriver

import (
	"errors"
	"io"
	"time"

	"github.com/consensys/go-corset/pkg/util/collection/typed"
	"github.com/consensys/go-corset/pkg/util/field/koalabear"
	"github.com/consensys/go-corset/pkg/zkc/constraints"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/sirupsen/logrus"
)

// BinaryFile represents a given set of constraints generated from a ZkC
// program.  A BinaryFile is typically obtained by decoding an array of bytes,
// though it can also be obtained by compiling a given set of ZkC source files.
// A BinaryFile provides two key pieces of functionality: firstly, it provides
// access to the AIR constraints representing the given ZkC program; secondly,
// it provides a means to generate a trace of that program from a given set of
// inputs.
type BinaryFile = constraints.BinaryFile[koalabear.Element]

// Settings specifies the parameters for the arithmetization (a.k.a. the
// "constraints").
type Settings struct {
}

// ZkCDriver exposes all the methods relevant for the user to interact with the
// constraints being used for proving.  This includes all constraints arising
// from the target ZkC program, but it does not include any native circuits
// (e.g. for precompiles, or accelerators, etc).
type ZkCDriver struct {
	Settings *Settings
	// Binary constraints file generated from a ZkC program.  This includes the
	// raw arithmetic (AIR) constraints generated from the program, as well as
	// functionality for tracing the program from a given set of inputs.
	BinaryFile *BinaryFile `serde:"omit"`
	// Configuration options for tracing (e.g. whether or not to use
	// parallelisation, what batch size to use, etc).  Generally speaking this
	// should be left to the provided defaults.
	TracingConfig constraints.TraceConfig `serde:"omit"`
	// Metadata embedded in the zkevm.bin file, as needed to check
	// compatibility.  Guaranteed non-nil.
	Metadata typed.Map `serde:"omit"`
}

// NewZkCDriver is the function that declares all the columns and the constraints of
// the zkEVM in the input builder object.
func NewZkCDriver(sys *wiop.System, settings Settings, bin io.Reader) *ZkCDriver {

	// Read and parse the binary file
	binf, metadata, errS := ReadConstraintsFile(bin)
	if errS != nil {
		panic(errS)
	}
	// Extract the AIR constraints from the binary file
	schema := binf.AirConstraints()
	// Translate air.Schema into prover's internal representation
	Define(sys, &schema)
	// Construct the driver
	return &ZkCDriver{
		BinaryFile:    binf,
		TracingConfig: constraints.DEFAULT_TRACE_CONFIG,
		Settings:      &settings,
		Metadata:      metadata,
	}
}

// Assign the arithmetization related columns. Namely, it will open the file
// specified in the witness object, call corset and assign the prover runtime
// columns. As part of the assignment process, the original trace is expanded
// according to the given schema.  The expansion process is about filling in
// computed columns with concrete values, such for determining multiplicative
// inverses, etc.
func (a *ZkCDriver) Assign(run *wiop.Runtime, inputsFile string) {
	a.AssignWithPreRead(run, ReadZkcInputs(inputsFile))
}

// PreReadInputs holds the result of pre-reading a trace file.
type PreReadInputs struct {
	// Inputs as required for the zkc program.  Each input corresponds with a
	// (non-static) input memory of the ZkC program.  For the RISC-V
	// interpreter, the inputs will include the full binary of the guest program
	// (hence, some entries could be 100Mb or more).
	Inputs map[string][]byte
	// Errors arising from parsing the input file.
	Err error
	// Name of the input file.
	InputsFile string
}

// ReadZkcInputs reads and parses a zkc inputs file, returning the "pre-read"
// input data. This can be called early to overlap I/O with other work.
func ReadZkcInputs(inputsFile string) PreReadInputs {
	traceF, err := ReadMaybeCompressedFile(inputsFile)
	if err != nil {
		return PreReadInputs{Err: err, InputsFile: inputsFile}
	}
	inputs, err := ReadZkcInputFile(traceF)
	return PreReadInputs{Inputs: inputs, Err: err, InputsFile: inputsFile}
}

// AssignWithPreRead assigns arithmetization columns using a pre-read trace.
func (a *ZkCDriver) AssignWithPreRead(run *wiop.Runtime, preRead PreReadInputs) {
	assignStart := time.Now()
	var (
		errs   []error
		inputs = preRead.Inputs
		errT   = preRead.Err
	)
	logrus.Infof("[bootstrapper] trace available (pre-read): %v", time.Since(assignStart))

	// Extract commit metadata from both files
	binfCommit, binfCommitOk := a.Metadata.String("commit")

	if !binfCommitOk {
		logrus.Warn("missing commit metadata from binary constraints file")
	}
	//
	logrus.Infof("constraints commit: %s", binfCommit)

	if errT != nil {
		logrus.Warnf("error loading the trace fpath=%q err=%v", preRead.InputsFile, errT.Error())
	}
	// Attempt to trace the ZkC program using the provided inputs, generating a
	// fully expanded AIR-compatible trace.
	tracingStart := time.Now()
	expandedTrace, errs := a.BinaryFile.Trace(inputs, a.TracingConfig)

	if len(errs) > 0 {
		logrus.Panicf("tracing failed: %v", errors.Join(errs...).Error())
	}
	logrus.Infof("[bootstrapper] tracing: %v", time.Since(tracingStart))

	copyStart := time.Now()
	AssignFromTrace(run, expandedTrace, a.BinaryFile.AirConstraints())
	logrus.Infof("[bootstrapper] column assignment: %v", time.Since(copyStart))
	logrus.Infof("[bootstrapper] total Arithmetization.Assign: %v", time.Since(assignStart))
}
