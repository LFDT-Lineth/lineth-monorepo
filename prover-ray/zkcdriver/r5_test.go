package zkcdriver_test

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/internal/minimal-elf"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// TestRisc5InstructionCoverageGuest traces minimalelf.AllInOneElfProgram
// through the real main.zkc interpreter and checks the resulting trace against
// every compiled constraint. The single guest exercises the full RV64I +
// M-extension + custom-precompile surface (memory round-trip, LUI/JAL/JALR,
// ADD/MUL, all six branch variants, sub-word loads/stores with sign extension,
// the Poseidon2/Keccak/write-output precompiles, every immediate ALU op, and
// RV64 word-width ops) in one trace, and asserts the write-output bytes. This
// only traces and checks constraints — it does not run the (expensive)
// prove/verify pipeline, which is already covered by
// verifier-ray/codegen/riscv_bootstrap.go.
func TestRisc5InstructionCoverageGuest(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}

	inputsMap, err := predecoding.PrepareInputs(minimalelf.AllInOneElfProgram, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	outputs, err := traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, inputsMap, true)
	if err != nil {
		t.Fatalf("failed to trace/check guest: %v", err)
	}
	got, ok := outputs["guest_output"]
	if !ok {
		t.Fatalf("expected a %q output, got outputs: %v", "guest_output", outputs)
	}
	want := []byte{'A', 'B', 'C'}
	if string(got) != string(want) {
		t.Fatalf("guest_output = %q, want %q", got, want)
	}
}

// This is a benchmark for the RISC-V arithmetization and not a test so that we
// don't crash the CI on every PR.
func BenchmarkRisc5Arithmetization(b *testing.B) {
	verifPath := "../../verifier-ray/zig-out/bin/verifier-ray"
	verifElf, err := os.ReadFile(verifPath)
	if err != nil {
		b.Skipf("skipping integration test: verifier ELF not found at %s (%v)", verifPath, err)
	}
	payload := []byte("foobar")
	inputsMap, err := predecoding.PrepareInputs(verifElf, payload)
	if err != nil {
		b.Fatalf("failed to prepare inputs: %v", err)
	}
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		b.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}
	b.Logf("tracing zkc")
	outputs, err := traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, inputsMap, false)
	if err != nil {
		b.Fatalf("failed to parse test case: %v", err)
	}
	for name, output := range outputs {
		if len(output) != 0 {
			b.Fatalf("expected empty %s output, got: %x", name, output)
		}
	}
	driverInputs := &zkcdriver.PreReadInputs{
		Inputs: inputsMap,
	}
	b.Logf("prover/verify")
	if err := runProveVerify(driverInputs, binf, proverCompilePipeline); err != nil {
		b.Fatalf("failed to run test case: %v", err)
	}
}
