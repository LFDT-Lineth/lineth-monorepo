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
			runZkcCase(t, zkcPath+cases[i])
		})
	}
}

// runZkcCase compiles the program at <zkcPath>.zkc, traces it against
// <zkcPath>.json, and runs the full prove/verify pipeline.
func runZkcCase(t *testing.T, zkcPath string) {
	t.Helper()

	zkcInputPath := zkcPath + ".json"
	zkcInputProgram := zkcPath + ".zkc"
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
}

func TestSecp256k1Add(t *testing.T) {
	runZkcCase(t, "testdata/secp256k1_add_u256")
}

func TestSecp256k1ScalarMul(t *testing.T) {
	runZkcCase(t, "testdata/secp256k1_scalarmul_u256")
}
