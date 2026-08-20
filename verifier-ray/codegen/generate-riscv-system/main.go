// Command generate-riscv-system compiles the real RISC-V main.zkc arithmetization,
// proves an honest witness for a real guest ELF, and writes the verifier-facing
// artifacts verifier-ray consumes directly:
//
//   - testdata/generated/riscv_system.zig
//   - testdata/proof_image.bin
//
// Both artifacts come from the same honest proof, so the committed verifier
// system and the committed proof image cannot drift onto different synthetic
// paths.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	verifierraycodegen "github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-riscv-system:", err)
		os.Exit(1)
	}
}

func run() error {
	artifacts, err := verifierraycodegen.BuildHonestRiscvArtifacts()
	if err != nil {
		return err
	}

	// Step 1: render the CompiledSystem (including PCS, via WritePcs) as Zig
	// source into systemBuf.
	var systemBuf bytes.Buffer
	if err := verifierraycodegen.WriteCompiledSystemZig(&systemBuf, 0, artifacts.CompiledSystem, verifierraycodegen.CompiledSystemZigOptions{
		EmitHeader:         true,
		EvalBranchQuota:    2_000_000,
		ProtocolImport:     `@import("verifier_ray").protocol`,
		FieldImport:        `@import("verifier_ray").field.koalabear`,
		VanishingImport:    `@import("verifier_ray").query.vanishing`,
		LogDerivImport:     `@import("verifier_ray").query.logderivativesum`,
		GrandProductImport: `@import("verifier_ray").query.grandproduct`,
		RowLimitImport:     `@import("verifier_ray").query.rowlimit`,
		WritePcs:           true,
		PcsImport:          `@import("verifier_ray").query.pcs`,
		FriImport:          `@import("verifier_ray").query.fri`,
	}); err != nil {
		return fmt.Errorf("WriteCompiledSystemZig: %w", err)
	}

	// Step 2: stitch the sub-verifier systems just written into a single
	// verifier.Systems value, the top-level struct verifier.verify expects.
	fmt.Fprintf(&systemBuf,
		"\nconst verifier = @import(\"verifier_ray\").verifier;\npub const system_0_systems = verifier.Systems{ .public_input = system_0_public_input, .vanishing = system_0, .logderivativesum = system_0_logderiv, .grandproduct = system_0_grandproduct, .rowlimit = system_0_rowlimit, .pcs = pcs_system_0 };\n",
	)

	generatedDir := "../../testdata/generated"
	systemPath := filepath.Join(generatedDir, "riscv_system.zig")
	formatted, err := runZigFmt(systemBuf.Bytes())
	if err != nil {
		return fmt.Errorf("zig fmt %s: %w", systemPath, err)
	}
	if err := os.WriteFile(systemPath, formatted, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", systemPath, err)
	}
	fmt.Println("wrote", systemPath)

	// Step 3: serialize the very same honest proof as the executable/test proof
	// image that verifier-ray mmaps or receives at _in_start.
	image, err := proofserialization.Encode(artifacts.VerifyInput, proofserialization.GuestBase)
	if err != nil {
		return fmt.Errorf("proofserialization.Encode: %w", err)
	}
	imagePath := "../../testdata/proof_image.bin"
	if err := os.WriteFile(imagePath, image, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", imagePath, err)
	}
	fmt.Println("wrote", imagePath)

	return nil
}

// runZigFmt pipes data through `zig fmt` on a temp file and returns the
// formatted result, mirroring testdata/generate/main.go's own runZigFmt so
// generated output stays consistent with the rest of this repo's generated
// Zig files.
func runZigFmt(data []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "verifier-ray-riscv-system-*.zig")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	cmd := os.Getenv("ZIG")
	if cmd == "" {
		cmd = "zig"
	}
	if err := exec.Command(cmd, "fmt", tmp.Name()).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}
