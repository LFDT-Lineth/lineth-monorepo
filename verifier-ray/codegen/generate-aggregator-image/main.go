// Command generate-aggregator-image derives the two-proof aggregator pair
// image from the single proof image written by generate-riscv-system, without
// proving anything:
//
//	testdata/riscv_proof_image.bin  --Decode-->  VerifyInput
//	VerifyInput x2  --EncodeAggregatorPair-->  testdata/riscv_proof_pair_image.bin
//
// Deriving from that image (rather than re-proving inside
// generate-riscv-system) keeps the pair fixture byte-consistent with the
// generated system by construction, and keeps this generator fast enough to
// run as a plain build dependency.
//
// The pair holds the same honest proof twice. The prover is deterministic, so
// proving the guest again would yield this identical proof anyway; two
// relocations of the one VerifyInput are the equivalent honest pair. Its
// public-input statement is the production RISC-V statement; adversarial
// consistency coverage lives in the synthetic verify.zig fixtures.
//
// The output is not committed (see verifier-ray/.gitignore): it is ~2x the single
// image, above Git hosting file-size limits, and fully reproducible.
package main

import (
	"fmt"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	verifierraycodegen "github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

const (
	singleImagePath = "../../testdata/riscv_proof_image.bin"
	pairImagePath   = "../../testdata/riscv_proof_pair_image.bin"
)

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

	pairImage, err := verifierraycodegen.EncodeAggregatorPair(input, input, proofserialization.GuestBase)
	if err != nil {
		return fmt.Errorf("EncodeAggregatorPair: %w", err)
	}
	if err := os.WriteFile(pairImagePath, pairImage, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", pairImagePath, err)
	}
	fmt.Println("wrote", pairImagePath)
	return nil
}
