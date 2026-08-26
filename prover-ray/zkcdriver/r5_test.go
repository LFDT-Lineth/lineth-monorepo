package zkcdriver_test

import (
	"os"
	"testing"

	zkc_r5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

const riscvMainZkcPath = "../../arithmetization/src/main/riscv/main.zkc"

// TestRisc5InstructionCoverageGuests traces small hand-written guest ELFs
// through the real main.zkc interpreter and checks each resulting trace
// against every compiled constraint. Each guest below exercises interpreter
// paths that ExitZeroGuestELF (only addi/ecall) never touches. These only
// trace and check constraints — they do not run the (expensive) prove/verify
// pipeline, which is already covered by verifier-ray/codegen/riscv_bootstrap.go
// for ExitZeroGuestELF.
func TestRisc5InstructionCoverageGuests(t *testing.T) {
	binf, err := compileBinaryConstraints(riscvMainZkcPath)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}

	tests := []struct {
		name string
		elf  []byte
	}{
		// MemoryRoundTripGuestELF stores and loads a scratch word, exercising
		// the S-type/I-type memory path.
		{"MemoryRoundTrip", zkc_r5.MemoryRoundTripGuestELF},
		// ArithmeticGuestELF exercises LUI, JAL, JALR, an R-type base op, and
		// an M-extension op.
		{"Arithmetic", zkc_r5.ArithmeticGuestELF},
		// BranchesGuestELF exercises all six B-type variants, both taken and
		// not-taken.
		{"Branches", zkc_r5.BranchesGuestELF},
		// LoadStoreWidthsGuestELF exercises SB/LB, SH/LH, and SD/LD, including
		// sign extension.
		{"LoadStoreWidths", zkc_r5.LoadStoreWidthsGuestELF},
		// Poseidon2GuestELF exercises the R_POSEIDON2 precompile.
		{"Poseidon2", zkc_r5.Poseidon2GuestELF},
		// KeccakGuestELF exercises the R_KECCAK precompile.
		{"Keccak", zkc_r5.KeccakGuestELF},
		// ImmediateALUGuestELF exercises SLTI, SLTIU, XORI, ORI, ANDI, SLLI,
		// SRLI, SRAI, ADDIW, SLLIW, SRLIW, and SRAIW.
		{"ImmediateALU", zkc_r5.ImmediateALUGuestELF},
		// WordWidthGuestELF exercises ADDW, SUBW, SLLW, SRLW, and SRAW.
		{"WordWidth", zkc_r5.WordWidthGuestELF},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputsMap, err := zkc_r5.PrepareInput(test.elf, nil)
			if err != nil {
				t.Fatalf("failed to prepare inputs: %v", err)
			}
			if _, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputsMap); err != nil {
				t.Fatalf("failed to trace/check guest: %v", err)
			}
		})
	}
}

// TestRisc5WriteOutputGuest traces zkc_r5.WriteOutputGuestELF, which invokes
// the R_WRITE_OUTPUT precompile to copy the bytes 'A','B','C' into the
// public guest_output buffer, and asserts that the resulting trace output
// carries those bytes verbatim.
func TestRisc5WriteOutputGuest(t *testing.T) {
	inputsMap, err := zkc_r5.PrepareInput(zkc_r5.WriteOutputGuestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}

	binf, err := compileBinaryConstraints(riscvMainZkcPath)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}

	outputs, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputsMap)
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

// TestRisc5ExitOneGuestIsRejected traces zkc_r5.ExitOneGuestELF, which exits
// with a nonzero code, and asserts that main.zkc's process_syscall rejects
// it (via `fail "EXIT CODE = %d\n"`) instead of silently accepting it.
func TestRisc5ExitOneGuestIsRejected(t *testing.T) {
	inputsMap, err := zkc_r5.PrepareInput(zkc_r5.ExitOneGuestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}

	binf, err := compileBinaryConstraints(riscvMainZkcPath)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}

	if _, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputsMap); err == nil {
		t.Fatalf("expected tracing a nonzero-exit-code guest to fail, but it succeeded")
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
	inputsMap, err := zkc_r5.PrepareInput(verifElf, payload)
	if err != nil {
		b.Fatalf("failed to prepare inputs: %v", err)
	}

	b.Logf("compiling zkc source: %s", riscvMainZkcPath)
	binf, err := compileBinaryConstraints(riscvMainZkcPath)
	if err != nil {
		b.Fatalf("failed to compile zkc source: %v", err)
	}
	b.Logf("tracing zkc")
	outputs, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputsMap)
	if err != nil {
		b.Fatalf("failed to parse test case: %v", err)
	}
	if len(outputs) != 0 {
		b.Fatalf("expected no outputs, got: %v", outputs)
	}
	driverInputs := &zkcdriver.PreReadInputs{
		Inputs: inputsMap,
	}
	b.Logf("prover/verify")
	if err := runProveVerify(driverInputs, binf, proverCompilePipeline); err != nil {
		b.Fatalf("failed to run test case: %v", err)
	}
}
