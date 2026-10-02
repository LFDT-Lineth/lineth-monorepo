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

// This is a benchmark for the RISC-V arithmetization and not a test so that we
// don't crash the CI on every PR.
func BenchmarkRisc5Arithmetization(b *testing.B) {
	guestELF, wantOutput := minimalelf.AllInOneElfProgram()
	inputsMap, err := predecoding.PrepareInputs(guestELF, nil)
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
	if got := outputs["guest_output"]; !bytes.Equal(got, wantOutput) {
		b.Fatalf("unexpected guest_output: got %x, want %x", got, wantOutput)
	}
	driverInputs := &zkcdriver.PreReadInputs{
		Inputs: inputsMap,
	}
	b.Logf("prover/verify")
	if err := runProveVerify(driverInputs, binf, proverCompilePipeline); err != nil {
		b.Fatalf("failed to run test case: %v", err)
	}
}
