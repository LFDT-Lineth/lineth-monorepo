package zkcdriver_test

import (
	"testing"
)

// zkcTestCase represents a zkc testcase. The user only needs to populate
// BinFilePath and InputStr
type zkcTestCase struct {
	ZkcFilePath string
	InputStr    string
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

			ZkcFilePath: "testdata/zkc_02.zkc",
			InputStr:    `{"data": "0x0003_0008"}`,
		},
		{

			ZkcFilePath: "testdata/zkc_02.zkc",
			InputStr:    `{"data": "0x000f_8000"}`,
		},
		{
			// A test case which doesn't use memory which would translate to lookup constraints
			// which zkc doesn't yet generate. This is the only test case which should work even
			// with pcs compiler added to the pipeline.
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
			if err := runProveVerify(inputs, binF, proverCompilePipeline); err != nil {
				t.Fatalf("failed to run test case: %v", err)
			}
		})
	}
}
