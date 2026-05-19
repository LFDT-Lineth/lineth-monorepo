package zkcdriver_test

import (
	"fmt"
	"testing"

	zkc_util "github.com/consensys/go-corset/pkg/zkc/util"
	"github.com/consensys/linea-monorepo/prover-ray/utils/files"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/linea-monorepo/prover-ray/zkcdriver"
)

func TestZkc_01a(t *testing.T) {
	runTest(t, "zkc_01", "{\"data\": \"0x0000_0001\" }")
}

func TestZkc_01b(t *testing.T) {
	runTest(t, "zkc_01", "{\"data\": \"0x0041_0042\" }")
}

func TestZkc_02a(t *testing.T) {
	runTest(t, "zkc_02", "{\"data\": \"0x0003_0008\" }")
}

func TestZkc_02b(t *testing.T) {
	runTest(t, "zkc_02", "{\"data\": \"0x000f_8000\" }")
}

// nolint
func runTest(t *testing.T, test, input string) {
	sys := wiop.NewSystemf("zkc-test")
	// set an example input
	var (
		testfile = fmt.Sprintf("./testdata/%s.bin", test)
		// construct inputs map
		inputs zkcdriver.PreReadInputs
	)
	// parse example input
	inputs.Inputs, inputs.Err = zkc_util.ParseJsonInputFile([]byte(input))
	// Sanity check
	if inputs.Err != nil {
		t.Errorf("error parsing program inputs (%v)", inputs.Err)
		t.FailNow()
	}
	// initialise round
	sys.NewRound()
	// construct ZkC driver
	driver := zkcdriver.NewZkCDriver(
		sys,
		zkcdriver.Settings{},
		files.MustRead(testfile))
	rt := wiop.NewRuntime(sys)
	driver.AssignWithPreRead(&rt, inputs)

	if err := wioptest.RunAndVerify(&rt); err != nil {
		t.Fatalf("error running verifier: %v", err)
	}
	// FIXME: run the prover to complete the test.  For now, I just used
	// go-corset's internal check to illustrate how this can work (e.g. it might
	// be useful for debugging). Run go-corset constraint check
	runZkcConstraintCheck(t, driver, inputs.Inputs)
}

func runZkcConstraintCheck(t *testing.T, driver *zkcdriver.ZkCDriver, input map[string][]byte) {
	t.Helper()
	// trace program with given input
	tr, errs := driver.BinaryFile.Trace(input, driver.TracingConfig)
	// sanity check
	if len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("%s", err.Error())
		}
		t.FailNow()
	}
	//
	if errs := driver.BinaryFile.Check(tr, driver.TracingConfig); len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("%s", err.Message())
		}
		t.FailNow()
	}
}
