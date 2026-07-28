package zkcdriver_test

import (
	"bytes"
	"errors"
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
	zkc_util "github.com/LFDT-Lineth/zkc/pkg/zkc/util"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

var (
	zkcField = field.KOALABEAR_16
	zkcCfg   = codegen.DEFAULT_CONFIG
)

func compileBinaryConstraints(srcPath string) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
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

// parseTestCase creates a system and the corresponding zkc-driver running the
// given zkcTestCase. The function also sanity-checks the inputs of the testcase.
func parseTestCase(scenario zkcTestCase, binF *constraints.BinaryFile[koalabear.Element]) (
	inputs *zkcdriver.PreReadInputs,
	outputs map[string][]byte,
	err error,
) {

	// Parse the inputs of the test-case
	inputs = &zkcdriver.PreReadInputs{}
	inputs.Inputs, inputs.Err = zkc_util.ParseJsonInputFile([]byte(scenario.InputStr))
	if inputs.Err != nil {
		return nil, nil, fmt.Errorf("could not parse test-case inputs: %w", inputs.Err)
	}
	// the input file also has outputs what the zkc program produces. Lets
	// filter them out for tracing purposes, so that we can sanity-check the
	// inputs of the test-case.
	filteredInputs := vm.FilterInputs(binF.Program(), inputs.Inputs)

	// This sanity-checks the corset inputs of the test-case
	outputs, err = traceZkc(binF, constraints.DEFAULT_TRACE_CONFIG, filteredInputs)
	if err != nil {
		return nil, nil, fmt.Errorf("constraint check failed: %w", err)
	}

	return inputs, outputs, nil
}

func traceZkc(
	binFile *constraints.BinaryFile[koalabear.Element],
	tracingCfg constraints.TraceConfig,
	input map[string][]byte,
) (outputs map[string][]byte, err error) {
	// recover panics. ZKC tends to panic when it fails tracing, so we want to catch those and return them as errors.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during constraint check: %v", r)
		}
	}()

	// trace program with given input
	outputs, _, tr, errs := binFile.Trace(input, tracingCfg)
	if len(errs) > 0 {
		return nil, fmt.Errorf("could not trace the binary file: %w", errors.Join(errs...))
	}

	// check the traces work
	if errsSchema := binFile.Check(tr, tracingCfg); len(errsSchema) > 0 {
		errs := make([]error, len(errsSchema))
		for i, e := range errsSchema {
			errs[i] = errors.New(e.Message())
		}
		return nil, fmt.Errorf("constraint check failed: %w", errors.Join(errs...))
	}
	return outputs, nil
}

func proverCompilePipeline(sys *wiop.System) {
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

// runProveVerify proves and verifies a given test-case, returning an error if the proof fails to verify.
func runProveVerify(inputs *zkcdriver.PreReadInputs, binFile *constraints.BinaryFile[koalabear.Element], proverCompilePipeline func(*wiop.System)) (err error) {
	// recover panics. ZKC tends to panic when it fails tracing, so we want to catch those and return them as errors.
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic during test-case execution: %v", r)
		}
	}()
	// Create a system
	sys := wiop.NewSystemf("zkc-test")
	sys.NewRound()

	// lets do constraint serialization roundtrip, to ensure that the binary file is still valid after serialization
	compiledConstraints, err := binFile.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal binary constraints file: %w", err)
	}

	// Construct the ZkC driver
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))

	// Run the prover compile pipeline, which will compile the system and prepare it for proof generation
	proverCompilePipeline(sys)

	// Run the ZkC driver to produce a proof and public inputs
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignWithPreRead(rt, inputs)
	})

	// Verify the proof and public inputs
	if err := sys.Verify(proof, pub); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	return nil
}
