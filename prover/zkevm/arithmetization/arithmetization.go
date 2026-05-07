package arithmetization

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/consensys/go-corset/pkg/asm"
	"github.com/consensys/go-corset/pkg/binfile"
	"github.com/consensys/go-corset/pkg/corset"
	"github.com/consensys/go-corset/pkg/ir"
	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/ir/mir"
	"github.com/consensys/go-corset/pkg/schema/module"
	"github.com/consensys/go-corset/pkg/schema/register"
	"github.com/consensys/go-corset/pkg/trace/lt"
	"github.com/consensys/go-corset/pkg/util/collection/typed"
	"github.com/consensys/go-corset/pkg/util/field/koalabear"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/sirupsen/logrus"
)

type traceBuilderConfig struct {
	batchSize uint
	checks    bool
	validate  bool
	parallel  bool
}

func getenvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		logrus.Warnf("ignoring invalid boolean for %s: %q", key, value)
		return defaultValue
	}

	return parsed
}

func getenvUint(key string, defaultValue uint) uint {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		logrus.Warnf("ignoring invalid unsigned integer for %s: %q", key, value)
		return defaultValue
	}

	if parsed > math.MaxUint {
		logrus.Warnf("ignoring too-large unsigned integer for %s: %q", key, value)
		return defaultValue
	}

	return uint(parsed)
}

func getTraceBuilderConfig() traceBuilderConfig {
	return traceBuilderConfig{
		batchSize: getenvUint("LIMITLESS_TRACE_EXPANSION_BATCH_SIZE", 1024),
		checks:    getenvBool("LIMITLESS_TRACE_EXPANSION_CHECKS", true),
		validate:  getenvBool("LIMITLESS_TRACE_EXPANSION_VALIDATE", true),
		parallel:  getenvBool("LIMITLESS_TRACE_EXPANSION_PARALLEL", true),
	}
}

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
	// ZkEVMBin contains the zkevm.bin file as a byte array. It is kept in the
	// struct as it is used for serialization.
	ZkEVMBin []byte
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

// NewArithmetization is the function that declares all the columns and the constraints of
// the zkEVM in the input builder object.
func NewArithmetization(builder *wizard.Builder, settings Settings) *Arithmetization {
	// Read and parse the binary file
	binf, metadata, errS := ReadZkevmBin()
	if errS != nil {
		panic(errS)
	}
	// Compile binary file into an air.Schema
	schema, mapping := CompileZkevmBin(binf, settings.OptimisationLevel)
	// Translate air.Schema into prover's internal representation
	Define(builder.CompiledIOP, schema, settings.Limits)
	// Done
	return &Arithmetization{
		BinaryFile:  binf,
		AirSchema:   schema,
		Settings:    &settings,
		LimbMapping: mapping,
		Metadata:    metadata,
		ZkEVMBin:    []byte(zkevmStr),
	}
}

// Assign the arithmetization related columns. Namely, it will open the file
// specified in the witness object, call corset and assign the prover runtime
// columns. As part of the assignment processs, the original trace is expanded
// according to the given schema.  The expansion process is about filling in
// computed columns with concrete values, such for determining multiplicative
// inverses, etc.
func (a *Arithmetization) Assign(run *wizard.ProverRuntime, traceFile string) {
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
	traceF := readTraceFile(traceFile)
	rawTrace, metadata, err := ReadLtTraces(traceF)
	return PreReadResult{RawTrace: rawTrace, Metadata: metadata, Err: err, TraceFile: traceFile}
}

// AssignWithPreRead assigns arithmetization columns using a pre-read trace.
func (a *Arithmetization) AssignWithPreRead(run *wizard.ProverRuntime, preRead PreReadResult) {
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
	if !*a.Settings.IgnoreCompatibilityCheck {
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
		fmt.Printf("error loading the trace fpath=%q err=%v", preRead.TraceFile, errT.Error())
	}
	// Perform trace propagation
	propStart := time.Now()
	rawTrace, errs = asm.Propagate(a.BinaryFile.Schema, rawTrace)
	// error check
	if len(errs) > 0 {
		logrus.Warnf("corset propagation gave the following errors: %v", errors.Join(errs...).Error())
	}
	logrus.Infof("[bootstrapper] propagation: %v", time.Since(propStart))
	// Perform trace expansion
	expStart := time.Now()
	traceBuilderCfg := getTraceBuilderConfig()
	expandedTrace, errs := ir.NewTraceBuilder[koalabear.Element]().
		WithBatchSize(traceBuilderCfg.batchSize).
		WithExpansionChecks(traceBuilderCfg.checks).
		WithParallelism(traceBuilderCfg.parallel).
		WithValidation(traceBuilderCfg.validate).
		WithRegisterMapping(a.LimbMapping).
		Build(a.AirSchema, rawTrace)
	//
	if len(errs) > 0 {
		logrus.Warnf("corset expansion gave the following errors: %v", errors.Join(errs...).Error())
	}
	logrus.Infof("[bootstrapper] expansion: %v", time.Since(expStart))
	// Passed
	copyStart := time.Now()
	AssignFromLtTraces(run, a.AirSchema, expandedTrace, a.Settings.Limits)
	logrus.Infof("[bootstrapper] column assignment: %v", time.Since(copyStart))
	logrus.Infof("[bootstrapper] total Arithmetization.Assign: %v", time.Since(assignStart))
}

// limbColumnsOf returns the wizard columns corresponding to the limbs for the
// tuple (moduleName, regName). The function furthermore ensures that the
// function has the requested number of limbs.
func (a *Arithmetization) limbColumnsOf(comp *wizard.CompiledIOP, mod string, column string, nLimbs int) []ifaces.Column {
	names := a.LimbsOf(mod, column, nLimbs)
	cols := make([]ifaces.Column, len(names))
	for i, name := range names {
		cols[i] = comp.Columns.GetHandle(ifaces.ColID(name))
	}
	return cols
}

// GetLimbsOfUint returns a [limbs.Uint] register corresponding to tuple
// (moduleName, regName). The function furthermore ensures that the register has
// the requested number of limbs.
func GetLimbsOf[S limbs.BitSize, E limbs.Endianness](a *Arithmetization,
	comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[S, E] {

	numLimbs := limbs.NumLimbsOf[S]()
	cols := a.limbColumnsOf(comp, mod, column, numLimbs)
	cols = limbs.ConvertSlice[limbs.LittleEndian, E](cols)
	return limbs.FromSliceUnsafe[S, E](
		ifaces.ColID(fmt.Sprintf("%s.%s", mod, column)),
		cols,
	)
}

// GetLimbsOfU16Be returns a [limbs.Uint] register corresponding to a U16 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU16Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S16, limbs.BigEndian] {
	return GetLimbsOf[limbs.S16, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU16Le returns a [limbs.Uint] register corresponding to a U16 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU16Le(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S16, limbs.LittleEndian] {
	return GetLimbsOf[limbs.S16, limbs.LittleEndian](a, comp, mod, column)
}

// GetLimbsOfU32Be returns a [limbs.Uint] register corresponding to a U32 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU32Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S32, limbs.BigEndian] {
	return GetLimbsOf[limbs.S32, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU32Le returns a [limbs.Uint] register corresponding to a U32 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU32Le(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S32, limbs.LittleEndian] {
	return GetLimbsOf[limbs.S32, limbs.LittleEndian](a, comp, mod, column)
}

// GetLimbsOfU48Be returns a [limbs.Uint] register corresponding to a U48 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU48Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S48, limbs.BigEndian] {
	return GetLimbsOf[limbs.S48, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU64Be returns a [limbs.Uint] register corresponding to a U64 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU64Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S64, limbs.BigEndian] {
	return GetLimbsOf[limbs.S64, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU64Le returns a [limbs.Uint] register corresponding to a U64 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU64Le(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S64, limbs.LittleEndian] {
	return GetLimbsOf[limbs.S64, limbs.LittleEndian](a, comp, mod, column)
}

// GetLimbsOfU128Be returns a [limbs.Uint] register corresponding to a U128 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU128Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S128, limbs.BigEndian] {
	return GetLimbsOf[limbs.S128, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU128Le returns a [limbs.Uint] register corresponding to a U128 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU128Le(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S128, limbs.LittleEndian] {
	return GetLimbsOf[limbs.S128, limbs.LittleEndian](a, comp, mod, column)
}

// GetLimbsOfU160Be returns a [limbs.Uint] register corresponding to a U160 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU160Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S160, limbs.BigEndian] {
	return GetLimbsOf[limbs.S160, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU160Le returns a [limbs.Uint] register corresponding to a U160 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU160Le(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S160, limbs.LittleEndian] {
	return GetLimbsOf[limbs.S160, limbs.LittleEndian](a, comp, mod, column)
}

// GetLimbsOfU256 returns a [limbs.Uint] register corresponding to a U256 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU256Be(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S256, limbs.BigEndian] {
	return GetLimbsOf[limbs.S256, limbs.BigEndian](a, comp, mod, column)
}

// GetLimbsOfU256Le returns a [limbs.Uint] register corresponding to a U256 with the
// specified endianness.
func (a *Arithmetization) GetLimbsOfU256Le(comp *wizard.CompiledIOP, mod string, column string) limbs.Uint[limbs.S256, limbs.LittleEndian] {
	return GetLimbsOf[limbs.S256, limbs.LittleEndian](a, comp, mod, column)
}

// ColumnOf returns the wizard column associated with the given name and
// register. The function will fail with panic if the column is not found or the
// column has more than 1 register.
func (a *Arithmetization) ColumnOf(comp *wizard.CompiledIOP, name string, column string) ifaces.Column {
	cols := a.limbColumnsOf(comp, name, column, 1)
	return cols[0]
}

// MashedColumnOf returns a single columnn representing a 'small' integer. The
// underlying representation of the column in the arithmetization may be
// u16 < x <= u32 and thus be fit on exactly 2 limbs. The underlying limbs are
// combined as L0 + 2**16 L1 and returned directly without any bound check. The
// function asserts that the column has exactly 2 limbs. If the same column is
// requested several times, the function will cache the previous call's result.
func (a *Arithmetization) MashedColumnOf(comp *wizard.CompiledIOP, name string, column string) ifaces.Column {
	// This call asserts that the column has exactly 2 limbs. So we don't need
	// to explicitly redo-it here.
	var (
		res     = a.GetLimbsOfU32Le(comp, name, column).GetLimbs()
		colName = fmt.Sprintf("%s_%s", name, column)
		colExpr = sym.Add(res[0], sym.Mul(1<<16, res[1]))
	)

	if comp.Columns.Exists(ifaces.ColID(colName)) {
		return comp.Columns.GetHandle(ifaces.ColID(colName))
	}

	return expr_handle.ExprHandleWithoutProverAction(comp, colExpr, colName)
}

// LimbsOf returns the fully qualified names of the limbs for a given Corset
// column based on its name.  The limbs are returned in little endian order
// (i.e. the least significant limb is at index 0).  This will attempt to match
// the column name with the appropriate raw (underlying) register name.  For
// example, we might have a column written in the Corset lisp file called "DATA"
// that we want to access.  For whatever reason, this column might be mapped to
// a raw register called e.g. "DATA_xor_HASH". Whilst we could use the raw
// register name directly, it is subject to change as the constraints change.
// Instead, we want to be able to use just "DATA" and for this function to
// figure out the raw name of "DATA_xor_HASH" for us, and then determine the
// appropriate limbs.
func (a *Arithmetization) LimbsOf(mod string, column string, nLimbs int) []string {
	// Identify limbs mapping for ecdata module
	var (
		// modPrefix splits the mod='a.b' -> 'a'
		modPrefix = strings.Split(mod, ".")[0]
		// Extract the limb mapping for the given module
		modMap = a.LimbMapping.ModuleOf(module.NewName(modPrefix, 1))
		// Determine corresponding register id
		reg = a.determineRegisterId(mod, column)
	)
	// Extract limbs for given register (least significant limb comes first)
	limbs := modMap.LimbIds(reg)
	names := make([]string, len(limbs))
	// Sanity check we got the number of limbs we expected
	if len(limbs) != nLimbs {
		panic(
			fmt.Sprintf("incorrect number of limbs for %s.%s (expected %d found %d)", mod, column, nLimbs, len(limbs)))
	}
	//
	for i, lid := range limbs {
		limb := modMap.Limb(lid)
		names[i] = fmt.Sprintf("%s.%s", modMap.Name(), limb.Name())
	}
	//
	return names
}

// Attempt to identify the register identifier for the given register.  This
// will first attempt to find a corresponding source-level register of the same
// name and, if that fails, fall back on the raw register name.  To understand
// this, consider a register DATA written in the Corset source-file which is
// declared within a given perspective.  Suppose that the DATA register is
// coalesced with another register from a different perspective (say HASH).
// Then, the source-level name for the DATA register is just DATA, whilst the
// raw (i.e. underlying name) is DATA_xor_HASH.  Thus, this function attempts to
// first resolve the name DATA to the register id corresponding with the raw
// register DATA_xor_HASH.
func (a *Arithmetization) determineRegisterId(mod string, name string) register.Id {
	var rid register.Id
	// Check whether source-level debug information is available.
	if srcmap, srcmap_ok := binfile.GetAttribute[*corset.SourceMap](a.BinaryFile); srcmap_ok {
		// Split module name based on path
		path := strings.Split(mod, ".")
		// Yes, therefore attempt to find a source-level regsiter with the given name.
		module := determineSourceModule(srcmap.Root, path)
		// Check columns within the module
		for _, col := range module.Columns {
			if col.Name == name {
				// Success
				return col.Register.Register()
			}
		}
	}
	// Failed to find a source-level register of the given name, therefore fall
	// back to just looking up the register based on its raw name.
	modMap := a.LimbMapping.ModuleOf(module.NewName(mod, 1))
	rid, ok := modMap.HasRegister(name)
	// Sanity check we found it
	if !ok {
		// This log an insightful message listing which where the column that
		// existed to make the failure easier to debug.
		modInfos := []string{}
		regs := modMap.Registers()
		for _, r := range regs {
			info := fmt.Sprintf("%s(width=%v, kind=%v)", r.Name(), r.Width(), r.Kind())
			modInfos = append(modInfos, info)
		}

		panic(fmt.Sprintf("unknown register %s.%s, available registers: %v", mod, name, modInfos))
	}
	// Done
	return rid
}

func determineSourceModule(module corset.SourceModule, path []string) corset.SourceModule {
	// Check whether moduled reached
	if len(path) == 0 {
		return module
	}
	// Look for matching child
	for _, submodule := range module.Submodules {
		if submodule.Name == path[0] {
			return determineSourceModule(submodule, path[1:])
		}
	}
	// Should not get here
	panic(fmt.Sprintf("unknown module %v", path))
}
