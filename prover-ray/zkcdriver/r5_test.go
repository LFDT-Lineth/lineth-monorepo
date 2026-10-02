package zkcdriver_test

import (
	"errors"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// This is a benchmark for the RISC-V arithmetization and not a test so that we
// don't crash the CI on every PR.
func BenchmarkRisc5Arithmetization(b *testing.B) {
	inputsMap, err := r5GuestInputs()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			b.Skipf("skipping integration test: R5 fixture not found (%v)", err)
		}
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
