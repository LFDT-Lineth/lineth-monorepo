// Command generate-riscv-system is the standalone codegen driver for the
// recursive single-proof verifier bootstrap: it compiles a zkc source file
// (the small, pub-input-driven zkc_02.zkc fixture for this milestone — see
// the plan's bootstrap-scope decision), runs it through the full prover-ray
// compiler pipeline, proves an honest witness, and writes the resulting
// verifier-ray CompiledSystem to testdata/generated/riscv_system.zig.
//
// Unlike testdata/generate/main.go's fixture generator (which bakes many
// scenarios' proofs as comptime Zig literals for tests), this program emits
// exactly one real production system with no baked proof — main_recursive.zig
// imports its output for spec/systems and decodes a real proof at runtime via
// proof_codec.zig instead.
//
// Also writes a proof.bin fixture (the wire-format encoding of the same
// honest proof) to testdata/generated/riscv_system_proof.bin, so the Zig
// decoder test (test/proof_codec_test.zig) has a known-good fixture to decode
// without needing a live zkc/R5 toolchain.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/zkcpipeline"
	"github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

const zkcSourcePath = "../../../prover-ray/zkcdriver/testdata/zkc_02.zkc"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-riscv-system:", err)
		os.Exit(1)
	}
}

func run() error {
	binF, err := zkcpipeline.CompileBinaryConstraints(zkcSourcePath)
	if err != nil {
		return fmt.Errorf("compiling %s: %w", zkcSourcePath, err)
	}

	// zkc_02.zkc reads two u16 values from its `data` pub input table: n and
	// the expected result of computing v=1, doubled n times (v = 1<<n). Using
	// n=3 here, so data = [n=3, expected=8].
	inputs := &zkcdriver.PreReadInputs{Inputs: map[string][]byte{
		"data": {0x00, 0x03, 0x00, 0x08},
	}}

	sys, proof, pub, rt, err := zkcpipeline.RunProveWithRuntime(inputs, binF, zkcpipeline.RunCompilePipeline)
	if err != nil {
		return fmt.Errorf("proving %s: %w", zkcSourcePath, err)
	}

	routing, err := codegen.BuildCoinRouting(sys)
	if err != nil {
		return fmt.Errorf("BuildCoinRouting: %w", err)
	}

	var systemBuf bytes.Buffer
	opts := codegen.CompileToZigOptions{
		CompiledSystemZigOptions: codegen.CompiledSystemZigOptions{
			EmitHeader:         true,
			ProtocolImport:     `@import("verifier_ray").protocol`,
			FieldImport:        `@import("verifier_ray").field.koalabear`,
			VanishingImport:    `@import("verifier_ray").query.vanishing`,
			LogDerivImport:     `@import("verifier_ray").query.logderivativesum`,
			GrandProductImport: `@import("verifier_ray").query.grandproduct`,
			RowLimitImport:     `@import("verifier_ray").query.rowlimit`,
		},
		Pcs: codegen.PcsZigOptions{
			PcsImport:  `@import("verifier_ray").query.pcs`,
			FriImport:  `@import("verifier_ray").query.fri`,
			EmitHeader: true,
		},
	}
	if err := codegen.CompileToZig(sys, 0, &systemBuf, opts); err != nil {
		return fmt.Errorf("CompileToZig: %w", err)
	}

	outDir := "../../testdata/generated"

	// Write the proof fixture FIRST: riscv_system.zig's embedFile line (below)
	// references it by name, and Go generates that line into systemBuf before
	// this function returns.
	proofPath := filepath.Join(outDir, "riscv_system_proof.bin")
	proofBytes, err := codegen.EncodeProof(sys, rt, routing, proof, pub)
	if err != nil {
		return fmt.Errorf("EncodeProof: %w", err)
	}
	if err := os.WriteFile(proofPath, proofBytes, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", proofPath, err)
	}
	fmt.Println("wrote", proofPath)

	// Expose the proof fixture as a pub const byte slice so Zig tests
	// (test/proof_codec_test.zig) can decode it without needing a live
	// zkc/R5 toolchain — @embedFile only resolves paths within the
	// importing file's own package, so this must be embedded from within
	// riscv_system.zig itself (rooted at testdata/generated/), not from
	// test/.
	fmt.Fprintf(&systemBuf, "\npub const proof_bytes = @embedFile(\"riscv_system_proof.bin\");\n")

	systemPath := filepath.Join(outDir, "riscv_system.zig")
	formatted, err := runZigFmt(systemBuf.Bytes())
	if err != nil {
		return fmt.Errorf("zig fmt %s: %w", systemPath, err)
	}
	if err := os.WriteFile(systemPath, formatted, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", systemPath, err)
	}
	fmt.Println("wrote", systemPath)

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
