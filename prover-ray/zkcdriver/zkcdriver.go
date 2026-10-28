package zkcdriver

import (
	"errors"
	"io"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/zkc/pkg/trace"
	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
	"github.com/LFDT-Lineth/zkc/pkg/util/collection/typed"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/sirupsen/logrus"
)

// BinaryFile represents a given set of constraints generated from a ZkC
// program.  A BinaryFile is typically obtained by decoding an array of bytes,
// though it can also be obtained by compiling a given set of ZkC source files.
// A BinaryFile provides two key pieces of functionality: firstly, it provides
// access to the AIR constraints representing the given ZkC program; secondly,
// it provides a means to generate a trace of that program from a given set of
// inputs.
type BinaryFile = constraints.BinaryFile[koalabear.Element, vm.Uint32]

// LazyTrace represents a partially realised trace whose shards can be
// materalised as required (i.e. lazily).  A lazy trace has been computed from a
// fast-mode execution of the target program, and contains internally a
// checkpoint for each shard.  When a given shard is to be materialised, tracing
// begins from that checkpoint until the shard is fully materialised (i.e.
// traced).  The primary benefit of a lazy trace is that it allows one to
// process shards without requiring that all shards are held in memory at the
// same time.
type LazyTrace = trace.LazyTrace[koalabear.Element]

// Shard represents an atomic unit of a given trace which has been fully
// materialised.
type Shard = trace.Shard[koalabear.Element]

// Settings specifies the parameters for the arithmetization (a.k.a. the
// "constraints").
type Settings struct {
	// TraceRound is the round holding the trace columns, apart from the
	// program ones which are in [ProgramRound]. Zero puts the whole trace in
	// round 0.
	TraceRound int
	// ShardSize is the number of [ShardFunction] invocations (i.e. executed
	// RISC-V instructions) per trace shard. Zero disables sharding: the whole
	// execution is traced eagerly as a single shard.
	ShardSize uint64
}

// ShardFunction is the ZkC function around which the trace is sharded. It is
// invoked once per executed RISC-V instruction.
const ShardFunction = "interpreter"

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
	TracingConfig vm.TraceConfig `serde:"omit"`
	// Metadata embedded in the zkevm.bin file, as needed to check
	// compatibility.  Guaranteed non-nil.
	Metadata typed.Map `serde:"omit"`
	// system is the System the driver defined its columns in, whose dynamic
	// modules' minimum sizes the tracing pads to (see [PadToMinSizes]).
	system *wiop.System
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
	Define(sys, &schema, settings.TraceRound)
	// With sharding, tracing only runs a fast-mode pass recording a checkpoint
	// per shard; each shard is then traced on demand from its checkpoint.
	tracingConfig := vm.DEFAULT_TRACE_CONFIG
	if settings.ShardSize > 0 {
		tracingConfig = tracingConfig.WithSharding(vm.NewShardingStrategy(ShardFunction, settings.ShardSize))
	}
	// Construct the driver
	return &ZkCDriver{
		BinaryFile:    binf,
		TracingConfig: tracingConfig,
		Settings:      &settings,
		Metadata:      metadata,
		system:        sys,
	}
}

// PadToMinSizes returns cfg with zkc padding every module to at least the
// largest [wiop.Module.MinRuntimeSize] of sys's dynamic modules, on top of
// cfg's own padding strategy. Proving rejects a dynamic module below its
// minimum, and only zkc can grow a module with valid rows: it pads with rows
// that satisfy the module's constraints, which constant padding need not.
// zkc's padding strategy sees a module's height, not the module, so every
// module gets the largest minimum.
//
// sys must be compiled: the minimums come from its compiled openings, so a
// call before compilation under-pads (a module the guest does not use stays at
// one row, while compilation may give it a logup column of minimum 4), and
// Prove rejects the trace.
func PadToMinSizes(cfg vm.TraceConfig, sys *wiop.System) vm.TraceConfig {
	minRows := uint(1)
	for _, n := range sys.MinRuntimeSizes() {
		minRows = max(minRows, uint(n))
	}
	if minRows == 1 {
		return cfg
	}
	pad := cfg.PaddingStrategy()
	return cfg.WithPadding(func(height, multiplier uint) uint {
		return max(pad(height, multiplier), minRows*multiplier)
	})
}

// PreReadInputs holds the result of pre-reading a trace file.
type PreReadInputs struct {
	// Inputs as required for the zkc program.  Each input corresponds with a
	// (non-static) input memory of the ZkC program.  For the RISC-V
	// interpreter, the inputs will include the full binary of the guest program
	// (hence, some entries could be 100Mb or more).
	Inputs map[string]bit.Array
	// Errors arising from parsing the input file.
	Err error
	// Name of the input file.
	InputsFile string
}

// PreReadZkcInputs reads and parses a zkc inputs file, returning the "pre-read"
// input data. This can be called early to overlap I/O with other work.
func PreReadZkcInputs(inputsFile string) *PreReadInputs {
	traceF, err := ReadMaybeCompressedFile(inputsFile)
	if err != nil {
		utils.Panic("boom: %v", err)
		return &PreReadInputs{Err: err, InputsFile: inputsFile}
	}
	inputs, err := ReadZkcInputFile(traceF)
	return &PreReadInputs{Inputs: inputs, Err: err, InputsFile: inputsFile}
}

// TraceZkcInputs returns the lazy trace of the pre-read inputs. Shards are only
// traced and expanded when requested (see [LazyTrace.Get]); with sharding
// enabled, this call only runs the fast-mode pass recording one checkpoint per
// shard. The function panics on errors. The
// driver's System must be compiled before tracing: the trace is padded to its
// dynamic modules' minimum sizes (see [PadToMinSizes]).
func (a *ZkCDriver) TraceZkcInputs(preRead *PreReadInputs) trace.LazyTrace[koalabear.Element] {

	assignStart := time.Now()
	var (
		errs []error
		// filter the inputs so that we only pass the inputs that are actually
		// used by the zkc program.
		inputs, _ = vm.FilterInputs(a.BinaryFile.TracingProgram(), preRead.Inputs)
		errT      = preRead.Err
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
	_, expandedTrace, errs := a.BinaryFile.Trace(inputs, PadToMinSizes(a.TracingConfig, a.system))

	if len(errs) > 0 {
		logrus.Panicf("tracing failed: %v", errors.Join(errs...).Error())
	}
	logrus.Infof("[bootstrapper] tracing: %v", time.Since(tracingStart))

	return expandedTrace.Unwrap()
}

// AssignTraceShard assigns arithmetization columns using a pre-read trace.
func (a *ZkCDriver) AssignTraceShard(
	run *wiop.Runtime,
	expandedShard Shard,
	sharedRandomness field.Octuplet,
) {

	copyStart := time.Now()
	AssignFromTraceShard(run, expandedShard, a.BinaryFile.AirConstraints(), sharedRandomness)
	logrus.Infof("[bootstrapper] column assignment: %v", time.Since(copyStart))
}
