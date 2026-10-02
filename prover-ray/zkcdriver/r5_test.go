package zkcdriver_test

import (
	"bytes"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// TestRisc5Arithmetization runs actual R5 tracing+proving on synthetic ELF to ensure
// end-to-end completeness
func TestRisc5Arithmetization(t *testing.T) {
	guestELF, wantOutput := minimalelf.AllInOneElfProgram()
	inputsMap, err := predecoding.PrepareInputs(guestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}
	t.Logf("tracing zkc")
	outputs, err := traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, inputsMap, false)
	if err != nil {
		t.Fatalf("failed to parse test case: %v", err)
	}
	if got := outputs["guest_output"]; !bytes.Equal(got, wantOutput) {
		t.Fatalf("unexpected guest_output: got %x, want %x", got, wantOutput)
	}
	driverInputs := &zkcdriver.PreReadInputs{
		Inputs: inputsMap,
	}
	t.Logf("prover/verify")
	if err := runProveVerify(driverInputs, binf, proverCompilePipeline); err != nil {
		t.Fatalf("failed to run test case: %v", err)
	}
}
