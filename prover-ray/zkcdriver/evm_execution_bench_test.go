package zkcdriver_test

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
	zkc_util "github.com/LFDT-Lineth/zkc/pkg/zkc/util"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

const (
	evmExecutionInput     = "testdata/evm_execution_guest.json"
	evmArithmetizationZKC = "../../arithmetization/src/main/riscv/main.zkc"
)

// BenchmarkEvmExecutionTrace measures the time to trace the tiny block
// (constraint evaluation only, no proof generation).
//
// To run:
//
//	go test -bench=BenchmarkEvmExecutionTrace -benchtime=1x -run=^$
//
// The input file is not checked in; generate it with:
//
//	make -C riscv-guests/l2-execution exec   # from the monorepo root
func BenchmarkEvmExecutionTrace(b *testing.B) {
	if files.CheckFilePath(evmExecutionInput) != nil {
		b.Skipf("missing %s; generate with `make -C riscv-guests/l2-execution exec` from the monorepo root", evmExecutionInput)
	}

	binf, err := compileBinaryConstraints(evmArithmetizationZKC)
	if err != nil {
		b.Fatalf("compile arithmetization: %v", err)
	}

	inputBytes, err := os.ReadFile(evmExecutionInput)
	if err != nil {
		b.Fatalf("read input: %v", err)
	}
	rawInputs, rawErr := zkc_util.ParseJsonInputFile(inputBytes)
	if rawErr != nil {
		b.Fatalf("parse JSON inputs: %v", rawErr)
	}
	filteredInputs, _ := vm.FilterInputs(binf.TracingProgram(), rawInputs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, filteredInputs, false); err != nil {
			b.Fatalf("trace: %v", err)
		}
	}
}

// BenchmarkEvmExecutionProveVerify measures the full prove + verify cycle for
// the tiny block.  Tracing is performed once during setup and its cost is
// excluded from the measurement.
//
// To run:
//
//	go test -bench=BenchmarkEvmExecutionProveVerify -benchtime=1x -run=^$
//
// The input file is not checked in; generate it with:
//
//	make -C riscv-guests/l2-execution exec   # from the monorepo root
func BenchmarkEvmExecutionProveVerify(b *testing.B) {
	if files.CheckFilePath(evmExecutionInput) != nil {
		b.Skipf("missing %s; generate with `make -C riscv-guests/l2-execution exec` from the monorepo root", evmExecutionInput)
	}

	binf, err := compileBinaryConstraints(evmArithmetizationZKC)
	if err != nil {
		b.Fatalf("compile arithmetization: %v", err)
	}

	inputBytes, err := os.ReadFile(evmExecutionInput)
	if err != nil {
		b.Fatalf("read input: %v", err)
	}
	tc := zkcTestCase{
		ZkcFilePath: evmArithmetizationZKC,
		InputStr:    string(inputBytes),
	}
	inputs, _, err := parseTestCase(tc, binf, false)
	if err != nil {
		b.Fatalf("trace + constraint check: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := runProveVerify(inputs, binf, proverCompilePipeline); err != nil {
			b.Fatalf("prove/verify: %v", err)
		}
	}
}
