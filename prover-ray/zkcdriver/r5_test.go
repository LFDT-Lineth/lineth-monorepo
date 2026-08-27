package zkcdriver_test

import (
	"os"
	"testing"

	zkc_r5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

const riscvMainZkcPath = "../../arithmetization/src/main/riscv/main.zkc"

// TestRisc5InstructionCoverageGuest traces zkc_r5.AllInOneGuestELF through
// the real main.zkc interpreter and checks the resulting trace against every
// compiled constraint. This only traces and checks constraints — it does not
// run the (expensive) prove/verify pipeline, which is already covered by
// verifier-ray/codegen/riscv_bootstrap.go.
func TestRisc5InstructionCoverageGuest(t *testing.T) {
	binf, err := compileBinaryConstraints(riscvMainZkcPath)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}

	inputsMap, err := zkc_r5.PrepareInput(zkc_r5.AllInOneGuestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
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
	outputs, err := traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, inputsMap, false)
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
