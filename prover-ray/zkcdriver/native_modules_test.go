package zkcdriver_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
	"github.com/LFDT-Lineth/zkc/pkg/util/file"
)

func TestModExp(t *testing.T) {
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
	inputs, _, err := parseTestCase(tc, binf, !testing.Short())
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

// TestSecp256k1Ecrecover drives ecrecover_run.zkc against the exhaustive
// vectors. ecrecover_generic is total (soft-fail sentinel, never aborts), so
// EVERY case — valid recovery, soft-failure (QNR/infinity), or invalid input —
// must trace and constraint-check cleanly: the driver checks each case against
// its expected (pkx, pky, isSuccess) triple and only fails on a mismatch. A
// single representative case additionally runs the full prove/verify pipeline
// (skipped under -short, since an ecrecover proof is expensive).
func TestSecp256k1Ecrecover(t *testing.T) {
	runSecp256k1Cases(t, "testdata/ecrecover_run.zkc", "testdata/ecrecover.accepts")
}

// TestSecp256k1Verify checks ordinary ECDSA verification against the supplied
// plain public key, including Ethereum's low-s transaction rule.
func TestSecp256k1Verify(t *testing.T) {
	runSecp256k1Cases(t, "testdata/secp256k1_verify_run.zkc", "testdata/secp256k1_verify.accepts")
}

func runSecp256k1Cases(t *testing.T, program, acceptCasesPath string) {
	t.Helper()
	binf, err := compileBinaryConstraints(program)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}
	acceptCases, ok := file.ReadInputFileAsLines(acceptCasesPath)
	if !ok {
		t.Fatalf("failed to read accept file %s for test-case %s", acceptCasesPath, program)
	}
	for lineNr, line := range filterCommentsFromZkcInput(acceptCases) {
		t.Run(fmt.Sprintf("case=%d", lineNr), func(t *testing.T) {
			tc := zkcTestCase{ZkcFilePath: program, InputStr: line}
			inputs, _, err := parseTestCase(tc, binf, !testing.Short())
			if err != nil {
				t.Fatalf("expected secp256k1 result to match; tracing/constraint-check failed: %v", err)
			}
			// Prove exactly one representative case per primitive; secp256k1
			// proofs are expensive, so gate behind !-short.
			if lineNr == 0 && !testing.Short() {
				if err := runProveVerify(inputs, binf, proverCompilePipeline); err != nil {
					t.Fatalf("prove/verify failed: %v", err)
				}
			}
		})
	}
}
