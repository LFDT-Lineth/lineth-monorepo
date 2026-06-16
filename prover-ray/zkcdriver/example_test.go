package zkcdriver_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/consensys/go-corset/pkg/zkc/constraints"
	zkc_util "github.com/consensys/go-corset/pkg/zkc/util"
	"github.com/consensys/linea-monorepo/prover-ray/utils/files"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/zkcdriver"
)

// zkcTestCase represents a zkc testcase. The user only needs to populate
// BinFilePath and InputStr
type zkcTestCase struct {
	BinFilePath string
	InputStr    string
}

func TestRunZKCExamples(t *testing.T) {

	basicTestCases := []zkcTestCase{
		{
			BinFilePath: "testdata/zkc_01.bin",
			InputStr:    "{\"data\": \"0x0041_0042\" }",
		},
		{

			BinFilePath: "testdata/zkc_01.bin",
			InputStr:    "{\"data\": \"0x0000_0001\" }",
		},
		{

			BinFilePath: "testdata/zkc_02.bin",
			InputStr:    "{\"data\": \"0x0003_0008\" }",
		},
		{

			BinFilePath: "testdata/zkc_02.bin",
			InputStr:    "{\"data\": \"0x000f_8000\" }",
		},
	}

	for i, tc := range basicTestCases {
		t.Run(tc.BinFilePath+strconv.Itoa(i), func(t *testing.T) {

			sys, inputs, err := parseTestCase(tc)
			if err != nil {
				t.Fatalf("could not parse test-case: %s", err)
			}
			if err := runTestCase(sys, *inputs, tc); err != nil {
				t.Fatalf("could not run test-case: %s", err)
			}
		})
	}
}

// parseTestCase creates a system and the corresponding zkc-driver running the
// given zkcTestCase. The function also sanity-checks the inputs of the testcase.
func parseTestCase(scenario zkcTestCase) (
	sys *wiop.System,
	inputs *zkcdriver.PreReadInputs,
	err error,
) {

	// Create a system
	sys = wiop.NewSystemf("zkc-test/%s", scenario.BinFilePath)
	sys.NewRound()

	// Parse the inputs of the test-case
	inputs = &zkcdriver.PreReadInputs{}
	inputs.Inputs, inputs.Err = zkc_util.ParseJsonInputFile(
		[]byte(scenario.InputStr))
	if inputs.Err != nil {
		return nil, nil, inputs.Err
	}

	// This sanity-checks the corset inputs of the test-case
	if err := checkZKCConstraints(scenario.BinFilePath,
		constraints.DEFAULT_TRACE_CONFIG, inputs.Inputs); err != nil {
		return nil, nil, err
	}

	return sys, inputs, nil
}

// runTestCase runs the given zkcTestCase
func runTestCase(
	sys *wiop.System,
	inputs zkcdriver.PreReadInputs,
	scenario zkcTestCase,
) error {

	// Construct the ZkC driver
	driver := zkcdriver.NewZkCDriver(
		sys,
		zkcdriver.Settings{},
		files.MustRead(scenario.BinFilePath))

	proof := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignWithPreRead(rt, inputs)
	})

	if err := sys.Verify(proof); err != nil {
		return fmt.Errorf("error running verifier: %w", err)
	}

	return nil
}

func checkZKCConstraints(
	binFilePath string,
	tracingCfg constraints.TraceConfig,
	input map[string][]byte,
) error {

	binFileReader := files.MustRead(binFilePath)
	binFile, _, errBin := zkcdriver.ReadConstraintsFile(binFileReader)
	if errBin != nil {
		return fmt.Errorf("could not read the binary file: %w", errBin)
	}

	// trace program with given input
	tr, errs := binFile.Trace(input, tracingCfg)
	if len(errs) > 0 {
		return fmt.Errorf("tracing failed: %w", errors.Join(errs...))
	}

	// check the traces work
	if errs := binFile.Check(tr, tracingCfg); len(errs) > 0 {
		var err error
		for _, e := range errs {
			err = errors.Join(err, errors.New(e.Message()))
		}
		return fmt.Errorf("constraint check failed: %w", err)
	}

	return nil
}
