package codegen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// Asset systems: the prover-ray systems whose verifier assets verifier-ray consumes.
const (
	// AssetSystemDevmode is the devmode circuit of prover-ray/backend/devmode, the system the
	// rollup guest verifies l2-execution proofs against until recursion is wired in.
	AssetSystemDevmode = "devmode"
	// AssetSystemRiscv is the real RISC-V main.zkc arithmetization.
	AssetSystemRiscv = "riscv"
)

// AssetSystems lists the systems VerifierAssets accepts.
var AssetSystems = []string{AssetSystemDevmode, AssetSystemRiscv}

// VerifierAssets are the verifier-facing files of one prover-ray system.
type VerifierAssets struct {
	// SystemZig is the formatted Zig source of the compiled system, importing verifier-ray as
	// `verifier_ray`. It declares system_0_spec and system_0_systems.
	SystemZig []byte
	// ProofImage is an honest proof of the system encoded for proofserialization.GuestBase, or nil
	// when it was not requested.
	ProofImage []byte
}

// BuildVerifierAssets compiles the named system (one of AssetSystems) and renders its verifier
// system. With withProof it also proves an honest statement, verifies it in Go and encodes it as a
// proof image. The system and the image come from the same compilation, so they cannot drift.
//
// The Zig source is formatted with `zig fmt` (the ZIG environment variable, or `zig` on PATH).
func BuildVerifierAssets(system string, withProof bool) (VerifierAssets, error) {
	var compiled CompiledSystem
	var input *proofserialization.VerifyInput
	switch {
	case system == AssetSystemDevmode && !withProof:
		sys, err := devmode.NewSystem()
		if err != nil {
			return VerifierAssets{}, fmt.Errorf("devmode.NewSystem: %w", err)
		}
		if compiled, err = BuildCompiledSystem(sys); err != nil {
			return VerifierAssets{}, fmt.Errorf("BuildCompiledSystem: %w", err)
		}
	case system == AssetSystemDevmode:
		artifacts, err := BuildDevmodeHonestArtifacts()
		if err != nil {
			return VerifierAssets{}, fmt.Errorf("building devmode honest artifacts: %w", err)
		}
		compiled, input = artifacts.CompiledSystem, &artifacts.VerifyInput
	case system == AssetSystemRiscv:
		// The riscv system is only extracted from an honest proof.
		artifacts, err := BuildAllInOneHonestRiscvArtifacts()
		if err != nil {
			return VerifierAssets{}, fmt.Errorf("building riscv honest artifacts: %w", err)
		}
		compiled = artifacts.CompiledSystem
		if withProof {
			input = &artifacts.VerifyInput
		}
	default:
		return VerifierAssets{}, fmt.Errorf("unknown system %q, want one of %v", system, AssetSystems)
	}

	systemZig, err := SystemZig(compiled)
	if err != nil {
		return VerifierAssets{}, err
	}
	assets := VerifierAssets{SystemZig: systemZig}
	if input != nil {
		if assets.ProofImage, err = proofserialization.Encode(*input, proofserialization.GuestBase); err != nil {
			return VerifierAssets{}, fmt.Errorf("encoding proof image: %w", err)
		}
	}
	return assets, nil
}

// SystemZig renders a compiled system as a standalone verifier-ray system source: every
// sub-verifier system, including PCS, and the verifier.Systems value that verifier.verify takes,
// formatted with `zig fmt`.
func SystemZig(compiled CompiledSystem) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteCompiledSystemZig(&buf, 0, compiled, CompiledSystemZigOptions{
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
		return nil, fmt.Errorf("WriteCompiledSystemZig: %w", err)
	}

	buf.WriteString("\nconst verifier = @import(\"verifier_ray\").verifier;\n" +
		"pub const system_0_systems = verifier.Systems{ .public_input = system_0_public_input, " +
		".vanishing = system_0, .logderivativesum = system_0_logderiv, .grandproduct = system_0_grandproduct, " +
		".rowlimit = system_0_rowlimit, .shared_randomness = system_0_shared_randomness, .pcs = pcs_system_0 };\n")

	return FormatZig(buf.Bytes())
}

// FormatZig formats Zig source with `zig fmt --stdin`, running the ZIG environment variable or
// `zig` on PATH.
func FormatZig(src []byte) ([]byte, error) {
	zig := os.Getenv("ZIG")
	if zig == "" {
		zig = "zig"
	}
	var out, stderr bytes.Buffer
	// The toolchain comes from the caller's environment, as `go` and `make` take theirs.
	cmd := exec.Command(zig, "fmt", "--stdin") //nolint:gosec // G204/G702: see above
	cmd.Stdin = bytes.NewReader(src)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zig fmt: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out.Bytes(), nil
}
