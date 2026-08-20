package zkcdriver_test

import (
	"os"
	"testing"

	zkc_r5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

// This is a benchmark for the RISC-V arithmetization and not a test so that we
// don't crash the CI on every PR.
func BenchmarkRisc5Arithmetization(b *testing.B) {
	const zkcPath = "../../arithmetization/src/main/riscv/main.zkc"

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

	b.Logf("compiling zkc source: %s", zkcPath)
	binf, err := compileBinaryConstraints(zkcPath)
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

func TestRunRisc5MainWithMinimalExitGuest(t *testing.T) {
	const zkcPath = "../../arithmetization/src/main/riscv/main.zkc"

	inputsMap, err := zkc_r5.PrepareInput(zkc_r5.ExitZeroGuestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}

	binf, err := compileBinaryConstraints(zkcPath)
	if err != nil {
		t.Fatalf("failed to compile zkc source: %v", err)
	}

	outputs, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputsMap)
	if err != nil {
		t.Fatalf("failed to trace main.zkc with minimal exit guest: %v", err)
	}
	if len(outputs) > 1 {
		t.Fatalf("expected no outputs, got: %v", outputs)
	}
	if guestOutput, ok := outputs["guest_output"]; ok && len(guestOutput) != 0 {
		t.Fatalf("expected empty guest_output, got: %x", guestOutput)
	}

	driverInputs := &zkcdriver.PreReadInputs{Inputs: inputsMap}
	if err := runProveVerify(driverInputs, binf, proverCompilePipeline); err != nil {
		t.Fatalf("failed to prove/verify main.zkc with minimal exit guest: %v", err)
	}
}
