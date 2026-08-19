package zkcdriver_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// zkcTestCase represents a zkc testcase. The user only needs to populate
// BinFilePath and InputStr
type zkcTestCase struct {
	ZkcFilePath string
	InputStr    string
	compileFn   func(*wiop.System)
}

func TestRunZKCExamples(t *testing.T) {

	basicTestCases := []zkcTestCase{
		{
			ZkcFilePath: "testdata/zkc_01.zkc",
			InputStr:    `{"data": "0x0041_0042"}`,
		},
		{

			ZkcFilePath: "testdata/zkc_01.zkc",
			InputStr:    `{"data": "0x0000_0001"}`,
		},
		{

			ZkcFilePath: "testdata/r5_test.zkc",
			InputStr: `{
				"segments": "0x0002_0202",
				"values": "0x0001_0002_0003_0004_0005_0006_0007_0008",
				"expected": "0x0003_0007_000b_000f",
				"segment_totals": "0x000a_001a",
				"grand_total": "0x0024"
			}`,
		},
		{

			ZkcFilePath: "testdata/r5_test.zkc",
			InputStr: `{
				"segments": "0x0001_0103",
				"values": "0x0002_0003_0004_0005_0006_0007_0008_0009",
				"expected": "0x0005_0009_000d_0011",
				"segment_totals": "0x0005_0027",
				"grand_total": "0x002c"
			}`,
		},
		{
			// A test case which doesn't use memory which would translate to lookup constraints
			// which zkc doesn't yet generate.
			ZkcFilePath: "testdata/no-memory.zkc",
			InputStr:    `{}`,
		},
	}

	for _, tc := range basicTestCases {
		t.Run(tc.ZkcFilePath, func(t *testing.T) {
			binF, err := compileBinaryConstraints(tc.ZkcFilePath)
			if err != nil {
				t.Fatalf("failed to compile binary constraints: %v", err)
			}
			inputs, _, err := parseTestCase(tc, binF)
			if err != nil {
				t.Fatalf("failed to parse test case: %v", err)
			}
			compileFn := proverCompilePipeline
			if tc.compileFn != nil {
				compileFn = tc.compileFn
			}

			if err := runProveVerify(inputs, binF, compileFn); err != nil {
				t.Fatalf("failed to run test case: %v", err)
			}
		})
	}
}
