package zkcdriver_test

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
)

func TestNative(t *testing.T) {
	const (
		zkcPath = "testdata/modexp_"
	)
	cases := []string{"u64", "u128", "u256"}
	for i := range cases {
		t.Run(cases[i], func(t *testing.T) {
			zkcInputPath := zkcPath + cases[i] + ".json"
			zkcInputProgram := zkcPath + cases[i] + ".zkc"
			binf, err := compileBinaryConstraints(zkcInputProgram)
			if err != nil {
				t.Fatalf("failed to compile zkc source: %v", err)
			}
			if files.CheckFilePath(zkcInputPath) != nil {
				t.Fatalf("zkc input file %s does not exist", zkcInputPath)
			}
			inputBytes, err := os.ReadFile(zkcInputPath)
			if err != nil {
				t.Fatalf("failed to read zkc input file: %v", err)
			}
			tc := zkcTestCase{
				ZkcFilePath: zkcInputPath,
				InputStr:    string(inputBytes),
			}
			inputs, _, err := parseTestCase(tc, binf)
			if err != nil {
				t.Fatalf("failed to parse test case: %v", err)
			}
			if err := runProveVerify(inputs, binf, proverCompilePipeline); err != nil {
				t.Fatalf("failed to run test case: %v", err)
			}
		})
	}
}
