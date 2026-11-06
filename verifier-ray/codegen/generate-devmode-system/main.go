// Command generate-devmode-system compiles the minimal devmode arithmetization
// circuit from prover-ray/backend/devmode, proves one deterministic 40-cell
// statement, and emits:
//   - testdata/generated/devmode_system.zig: the verifier's compiled system
//   - testdata/devmode_proof_image.bin: the single proof image relocated for GuestBase
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	verifierraycodegen "github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

const (
	generatedDevmodeZigPath   = "../../testdata/generated/devmode_system.zig"
	generatedDevmodeProofPath = "../../testdata/devmode_proof_image.bin"
)

func main() {
	zigOnly := flag.Bool("zig-only", false, "write only devmode_system.zig (skip proving and proof image emission)")
	flag.Parse()

	if err := run(*zigOnly); err != nil {
		fmt.Fprintln(os.Stderr, "generate-devmode-system:", err)
		os.Exit(1)
	}
}

func run(zigOnly bool) error {
	if zigOnly {
		sys, err := devmode.NewSystem()
		if err != nil {
			return fmt.Errorf("devmode.NewSystem: %w", err)
		}
		compiled, err := verifierraycodegen.BuildCompiledSystem(sys)
		if err != nil {
			return fmt.Errorf("BuildCompiledSystem: %w", err)
		}
		return writeCompiledZig(compiled)
	}

	artifacts, err := verifierraycodegen.BuildDevmodeHonestArtifacts()
	if err != nil {
		return fmt.Errorf("building devmode honest artifacts: %w", err)
	}
	if err := writeCompiledZig(artifacts.CompiledSystem); err != nil {
		return err
	}

	proofImage, err := proofserialization.Encode(artifacts.VerifyInput, proofserialization.GuestBase)
	if err != nil {
		return fmt.Errorf("encoding proof image: %w", err)
	}
	if err := os.WriteFile(generatedDevmodeProofPath, proofImage, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", generatedDevmodeProofPath, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", generatedDevmodeProofPath, len(proofImage))
	return nil
}

func writeCompiledZig(compiled verifierraycodegen.CompiledSystem) error {
	var systemBuf bytes.Buffer
	if err := verifierraycodegen.WriteCompiledSystemZig(&systemBuf, 0, compiled, verifierraycodegen.CompiledSystemZigOptions{
		EmitHeader:             true,
		EvalBranchQuota:        2_000_000,
		ProtocolImport:         `@import("verifier_ray").protocol`,
		FieldImport:            `@import("verifier_ray").field.koalabear`,
		VanishingImport:        `@import("verifier_ray").query.vanishing`,
		LogDerivImport:         `@import("verifier_ray").query.logderivativesum`,
		GrandProductImport:     `@import("verifier_ray").query.grandproduct`,
		RowLimitImport:         `@import("verifier_ray").query.rowlimit`,
		SharedRandomnessImport: `@import("verifier_ray").query.shared_randomness`,
		WritePcs:               true,
		PcsImport:              `@import("verifier_ray").query.pcs`,
		FriImport:              `@import("verifier_ray").query.fri`,
	}); err != nil {
		return fmt.Errorf("WriteCompiledSystemZig: %w", err)
	}

	fmt.Fprintf(&systemBuf,
		"\nconst verifier = @import(\"verifier_ray\").verifier;\npub const system_0_systems = verifier.Systems{ .public_input = system_0_public_input, .vanishing = system_0, .logderivativesum = system_0_logderiv, .grandproduct = system_0_grandproduct, .rowlimit = system_0_rowlimit, .shared_randomness = system_0_shared_randomness, .pcs = pcs_system_0 };\n",
	)

	formatted, err := runZigFmt(systemBuf.Bytes())
	if err != nil {
		return fmt.Errorf("zig fmt %s: %w", generatedDevmodeZigPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(generatedDevmodeZigPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(generatedDevmodeZigPath, formatted, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", generatedDevmodeZigPath, err)
	}
	fmt.Println("wrote", generatedDevmodeZigPath)
	return nil
}

func runZigFmt(data []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "verifier-ray-devmode-system-*.zig")
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
