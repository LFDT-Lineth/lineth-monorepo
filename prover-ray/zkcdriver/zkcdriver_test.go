package zkcdriver_test

import (
	"errors"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/zkcpipeline"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	zkc_util "github.com/LFDT-Lineth/zkc/pkg/zkc/util"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// compileBinaryConstraints, proverCompilePipeline, and runProveVerify are
// thin aliases over zkcpipeline's exported equivalents, kept so this test
// file's call sites don't need to change. The real logic lives in
// zkcdriver/zkcpipeline so non-test code (codegen generators, driver
// programs) can reuse it too.
func compileBinaryConstraints(srcPath string) (*constraints.BinaryFile[koalabear.Element], error) {
	return zkcpipeline.CompileBinaryConstraints(srcPath)
}

func proverCompilePipeline(sys *wiop.System) {
	zkcpipeline.RunCompilePipeline(sys)
}

func runProveVerify(inputs *zkcdriver.PreReadInputs, binFile *constraints.BinaryFile[koalabear.Element], pipeline func(*wiop.System)) error {
	return zkcpipeline.RunProveVerify(inputs, binFile, pipeline)
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
