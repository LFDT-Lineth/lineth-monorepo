// Command generate-riscv-guest-proofs proves every honest guest ELF in
// verifierraycodegen.HonestRiscvGuests against the real RISC-V main.zkc
// arithmetization, and writes each one's serialized proof image to a scratch
// directory (default ../../testdata/scratch/riscv-guest-proofs, override with
// -out) as riscv_proof_image_<guest>.bin.
//
// Unlike codegen/generate-riscv-system, these images are NOT committed:
// testdata/README.md asks fixtures there to stay small and deterministic, and
// 10 honest proofs at ~52MB apiece would be ~525MB of binary fixtures for no
// benefit over the one guest (ExitZeroGuestELF) codegen/generate-riscv-system
// already commits. This command exists so test/riscv_guest_proofs_test.zig can
// verify every remaining guest through the real Zig verifier.verify() path —
// run it (or `make generate-riscv-guest-proofs`) before that test, in a
// gitignored directory, rather than committing its output.
//
// All images verify against the same testdata/generated/riscv_system.zig:
// main.zkc's circuit doesn't depend on the guest.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	verifierraycodegen "github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

func main() {
	outDir := flag.String("out", "../../testdata/scratch/riscv-guest-proofs", "directory to write riscv_proof_image_<guest>.bin files into")
	flag.Parse()

	if err := run(*outDir); err != nil {
		fmt.Fprintln(os.Stderr, "generate-riscv-guest-proofs:", err)
		os.Exit(1)
	}
}

func run(outDir string) error {
	all, err := verifierraycodegen.BuildAllHonestRiscvArtifacts()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", outDir, err)
	}

	manifestPath := filepath.Join(outDir, "manifest.txt")
	manifest, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", manifestPath, err)
	}
	defer manifest.Close()

	for _, result := range all {
		image, err := proofserialization.Encode(result.Artifacts.VerifyInput, proofserialization.GuestBase)
		if err != nil {
			return fmt.Errorf("guest %q: proofserialization.Encode: %w", result.Guest.Name, err)
		}
		imageName := fmt.Sprintf("riscv_proof_image_%s.bin", result.Guest.Name)
		imagePath := filepath.Join(outDir, imageName)
		if err := os.WriteFile(imagePath, image, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", imagePath, err)
		}
		if _, err := fmt.Fprintln(manifest, result.Guest.Name); err != nil {
			return fmt.Errorf("writing %s: %w", manifestPath, err)
		}
		fmt.Println("wrote", imagePath)
	}

	return nil
}
