// Command generate-aggregator-image derives the two-proof aggregator pair
// images from the COMMITTED single proof image, without proving anything:
//
//	testdata/riscv_proof_image.bin           --Decode-->  VerifyInput
//	VerifyInput x2  --EncodeAggregatorPair-->  testdata/riscv_proof_pair_image.bin
//	                                           testdata/riscv_proof_pair_image_test.bin
//
// Deriving from the committed image (rather than re-proving inside
// generate-riscv-system) keeps the pair fixtures byte-consistent with the
// committed system by construction, and keeps this generator fast enough to
// run as a plain build dependency.
//
// The pair holds the same honest proof twice. The prover is deterministic, so
// proving the guest again would yield this identical proof anyway; two
// relocations of the one VerifyInput are the equivalent honest pair. Its
// public-input statement is currently empty (the riscv system registers no
// public inputs yet), so the aggregator's consistency check passes trivially
// here — the adversarial consistency coverage lives in the synthetic
// verify.zig fixtures.
//
// Neither output is committed (see testdata/.gitignore): each is ~2x the
// single image, above Git hosting file-size limits, and fully reproducible.
package main

import (
	"fmt"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	verifierraycodegen "github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

const singleImagePath = "../../testdata/riscv_proof_image.bin"

// pairTestBase is the base address of the Zig-unit-test copy of the pair
// image, mirrored by test/aggregator_image_test.zig's fixture_base. The test
// binary also maps riscv_proof_image_test.zig's fixture at GuestBase in the
// same process, and a pair image based at GuestBase would always overlap it,
// so the test copy lives at its own base, far above GuestBase + the pair size.
const pairTestBase = 0x50000000

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-aggregator-image:", err)
		os.Exit(1)
	}
}

func run() error {
	singleImage, err := os.ReadFile(singleImagePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", singleImagePath, err)
	}
	input, err := proofserialization.Decode(singleImage, proofserialization.GuestBase)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", singleImagePath, err)
	}

	for _, out := range []struct {
		path string
		base uint64
	}{
		{"../../testdata/riscv_proof_pair_image.bin", proofserialization.GuestBase},
		{"../../testdata/riscv_proof_pair_image_test.bin", pairTestBase},
	} {
		pairImage, err := verifierraycodegen.EncodeAggregatorPair(input, input, out.base)
		if err != nil {
			return fmt.Errorf("EncodeAggregatorPair for %s: %w", out.path, err)
		}
		if err := os.WriteFile(out.path, pairImage, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", out.path, err)
		}
		fmt.Println("wrote", out.path)
	}

	return nil
}
