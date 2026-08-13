// Package zkcpipeline provides the reusable, non-test steps for compiling a
// zkc source file into a wiop.System, running the prover compiler pipeline on
// it, and proving/verifying an execution against it.
//
// This logic previously lived only in zkcdriver's internal test package
// (zkcdriver_test.go), which a production binary cannot import (Go excludes
// _test.go files from ordinary builds). Exporting it here lets tooling such as
// the verifier-ray codegen generator and the prove driver program reuse it
// without duplicating the pipeline.
package zkcpipeline

import (
	"bytes"
	"fmt"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

var (
	zkcField = field.KOALABEAR_16
	zkcCfg   = codegen.DEFAULT_CONFIG
)

// CompileBinaryConstraints compiles a zkc source file into a
// constraints.BinaryFile ready for zkcdriver.NewZkCDriver.
func CompileBinaryConstraints(srcPath string) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
	// recover panics. ZKC tends to panic when it fails compiling, so we want to catch those and return them as errors.
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic during zkc compilation: %v", r)
		}
	}()

	srcZkc, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read zkc source file: %w", err)
	}
	src := source.NewSourceFile(srcPath, srcZkc)
	macroProgram, _, errs := compiler.Compile(zkcField, *src)
	if len(errs) > 0 {
		for i := range errs {
			fmt.Printf("zkc compile error: %s\n", errs[i].Error())
		}
		return nil, fmt.Errorf("failed to compile zkc source")
	}
	ir, errs := ast.Compile(macroProgram, zkcCfg)
	if len(errs) > 0 {
		for i := range errs {
			fmt.Printf("zkc compile error: %s\n", errs[i].Error())
		}
		return nil, fmt.Errorf("failed to compile zkc source")
	}
	binfile = constraints.NewBinaryFile[koalabear.Element](nil, nil, zkcField, zkcCfg.GetMaxStaticDepth(), ir)
	return binfile, nil
}

// RunCompilePipeline runs the full wiop compiler pipeline a zkc-derived
// system needs before it can be proved: nonnative, rangecheck,
// lookuptologderivsum, messagebus, grandproduct, logderivativesum,
// localvanishing, global, then pcs.
func RunCompilePipeline(sys *wiop.System) {
	nonnative.Compile(sys)
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	messagebus.Compile(sys)
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	// XXX(ivokub): we have disabled pcs compiler for now as zkc compiler doesn't generate lookup constraints.
	// in that case we would have columns which are not constrained at all and we would get a panic in the
	// pcs compiler due to shifts not defined.
	//
	// replug when zkc start emitting lookup constraints, see https://github.com/LFDT-Lineth/zkc/issues/2013
	//
	// and when replugging, then we should also construct a new wiop.System for verifier to ensure that the
	// verifier doesn't have access to the prover's internal state, so that we would have a more realistic
	// test case. We should also do it in the pipeline test then.
	pcs.Compile(sys)
}

// newSystemAndDriver builds a fresh wiop.System and the ZkCDriver bound to it,
// ready for pipeline to run and a witness to be assigned. Shared by
// RunProveVerify and RunProve so both build the exact same system shape.
func newSystemAndDriver(binFile *constraints.BinaryFile[koalabear.Element], pipeline func(*wiop.System)) (*wiop.System, *zkcdriver.ZkCDriver, error) {
	sys := wiop.NewSystemf("zkc-test")
	sys.NewRound()

	// lets do constraint serialization roundtrip, to ensure that the binary file is still valid after serialization
	compiledConstraints, err := binFile.MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal binary constraints file: %w", err)
	}

	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))
	pipeline(sys)
	return sys, driver, nil
}

// RunProveVerify proves and verifies a given test-case, returning an error if
// the proof fails to verify.
func RunProveVerify(inputs *zkcdriver.PreReadInputs, binFile *constraints.BinaryFile[koalabear.Element], pipeline func(*wiop.System)) (err error) {
	// recover panics. ZKC tends to panic when it fails tracing, so we want to catch those and return them as errors.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during test-case execution: %v", r)
		}
	}()

	sys, driver, err := newSystemAndDriver(binFile, pipeline)
	if err != nil {
		return err
	}

	proof, pub := sys.Prove(
		func(rt *wiop.Runtime) { driver.AssignWithPreRead(rt, inputs) },
		wiop.ProveOptions{CheckUnreducedQueries: true})

	if err := sys.Verify(proof, pub); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	return nil
}

// RunProve is RunProveVerify's non-test-only twin: it proves and verifies
// (failing loudly on a verify failure, since silently emitting a bad proof is
// worse than crashing) and additionally returns the compiled system, proof,
// and public input, so a caller can serialize the proof or extract codegen
// from the system/runtime.
func RunProve(
	inputs *zkcdriver.PreReadInputs,
	binFile *constraints.BinaryFile[koalabear.Element],
	pipeline func(*wiop.System),
) (sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput, err error) {
	sys, proof, pub, _, err = RunProveWithRuntime(inputs, binFile, pipeline)
	return sys, proof, pub, err
}

// RunProveWithRuntime is RunProve's twin that also returns the *wiop.Runtime
// sys.Prove drove internally to produce proof/pub. A caller that needs the
// runtime too (e.g. verifier-ray codegen's CompileToZig, which reads dynamic
// module sizes and LagrangeEval claims off it) cannot get it from sys.Prove
// itself, since Prove builds its own Runtime and returns only (Proof,
// PublicInput) — this captures the same Runtime via a closure over Prove's
// assign hook instead of duplicating Prove's round-driving loop (which
// touches wiop-internal fields this package cannot reach).
func RunProveWithRuntime(
	inputs *zkcdriver.PreReadInputs,
	binFile *constraints.BinaryFile[koalabear.Element],
	pipeline func(*wiop.System),
) (sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput, rt *wiop.Runtime, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during proving: %v", r)
		}
	}()

	sys, driver, err := newSystemAndDriver(binFile, pipeline)
	if err != nil {
		return nil, wiop.Proof{}, nil, nil, err
	}

	proof, pub = sys.Prove(
		func(assignRt *wiop.Runtime) {
			rt = assignRt
			driver.AssignWithPreRead(assignRt, inputs)
		},
		wiop.ProveOptions{CheckUnreducedQueries: true})

	if err := sys.Verify(proof, pub); err != nil {
		return nil, wiop.Proof{}, nil, nil, fmt.Errorf("verification failed: %w", err)
	}
	return sys, proof, pub, rt, nil
}
