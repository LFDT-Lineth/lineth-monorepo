package zkcdriver_test

import (
	"os"
	"strings"
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

// ecrCase is one line of an ecrecover .accepts/.rejects fixture: the JSON input
// object and the `;;` comment that precedes it (used as the subtest name).
type ecrCase struct {
	label string
	input string
}

// readEcrecoverCases parses a `.accepts`/`.rejects` fixture: each non-blank line
// is either a `;;` comment (which labels the following case) or a single-object
// input JSON. Comment-only header lines are naturally discarded because a later
// `;;` overwrites the label before the next `{...}` line.
func readEcrecoverCases(t *testing.T, path string) []ecrCase {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	var cases []ecrCase
	label := ""
	for _, ln := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, ";;") {
			label = strings.TrimSpace(strings.TrimPrefix(s, ";;"))
			continue
		}
		cases = append(cases, ecrCase{label: label, input: s})
	}
	if len(cases) == 0 {
		t.Fatalf("no cases found in fixture %s", path)
	}
	return cases
}

// TestSecp256k1Ecrecover drives ecrecover_run.zkc against the exhaustive
// accept/reject vectors. Every case is traced and constraint-checked; accept
// cases must pass, reject cases must fail. A single representative accept case
// additionally runs the full prove/verify pipeline (skipped under -short, since
// an ecrecover proof is very expensive).
func TestSecp256k1Ecrecover(t *testing.T) {
	const program = "testdata/ecrecover_run.zkc"
	binf, err := compileBinaryConstraints(program)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}

	t.Run("accepts", func(t *testing.T) {
		for i, c := range readEcrecoverCases(t, "testdata/ecrecover.accepts") {
			t.Run(c.label, func(t *testing.T) {
				tc := zkcTestCase{ZkcFilePath: program, InputStr: c.input}
				inputs, _, err := parseTestCase(tc, binf)
				if err != nil {
					t.Fatalf("expected accept, but tracing/constraint-check failed: %v", err)
				}
				// Prove exactly one representative accept case; ecrecover proofs
				// are ~20+ min, so gate behind !-short.
				if i == 0 && !testing.Short() {
					if err := runProveVerify(inputs, binf, proverCompilePipeline); err != nil {
						t.Fatalf("prove/verify failed: %v", err)
					}
				}
			})
		}
	})

	t.Run("rejects", func(t *testing.T) {
		for _, c := range readEcrecoverCases(t, "testdata/ecrecover.rejects") {
			t.Run(c.label, func(t *testing.T) {
				tc := zkcTestCase{ZkcFilePath: program, InputStr: c.input}
				if _, _, err := parseTestCase(tc, binf); err == nil {
					t.Fatalf("expected reject, but the case was accepted")
				}
			})
		}
	})
}
