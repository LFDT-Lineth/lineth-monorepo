package minimalelf_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

var (
	zkcCfg = vm.DEFAULT_TRACE_CONFIG
)

// TestRisc5InstructionCoverageGuest traces minimalelf.AllInOneElfProgram
// through the real main.zkc interpreter and checks the resulting trace against
// every compiled constraint.
//
// See [minimalelf] package for the definition of AllInOneElfProgram.
func TestRisc5InstructionCoverageGuest(t *testing.T) {
	elf, want := minimalelf.AllInOneElfProgram()
	traceAndCheck(t, elf, want)
}

// TestFibonacciGuest traces minimalelf.FibonacciELF at small N, including the
// N = 0 path that skips the loop, and checks the trace and guest output.
func TestFibonacciGuest(t *testing.T) {
	// fib(94) is the first value that wraps mod 2^64.
	for _, n := range []int{0, 1, 2, 10, 93, 94} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			elf, want := minimalelf.FibonacciELF(n)
			traceAndCheck(t, elf, want)
		})
	}
}

// TestMemoryGuest traces minimalelf.MemoryELF at small N and checks the trace
// and guest output.
func TestMemoryGuest(t *testing.T) {
	for _, n := range []int{1, 2, 64, 1000} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			elf, want := minimalelf.MemoryELF(n)
			traceAndCheck(t, elf, want)
		})
	}
}

// traceAndCheck traces elf through the embedded R5 arithmetization, checks the
// trace against every compiled constraint, and compares guest_output to want.
func traceAndCheck(t *testing.T, elf, want []byte) {
	t.Helper()
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}

	inputsMap, err := predecoding.PrepareInputs(elf, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	// trace program with given input
	outputs, tr, errs := binf.Trace(inputsMap, zkcCfg)
	if len(errs) > 0 {
		t.Fatalf("could not trace the binary file: %v", errors.Join(errs...))
	}
	// check the traces work
	if failures, errs := binf.Check(zkcCfg, tr.Unwrap()); len(failures) > 0 || len(errs) > 0 {
		// failures are constraint failures, whilst errs are internal ZkC problems.
		for _, e := range failures {
			errs = append(errs, errors.New(e.Message()))
		}
		t.Fatalf("constraint check failed: %v", errors.Join(errs...))
	}

	got, ok := outputs["guest_output"]
	if !ok {
		t.Fatalf("expected a %q output, got outputs: %v", "guest_output", outputs)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("guest_output = %x, want %x", got, want)
	}
}

// TestOutputAndLogGuest checks that minimalelf.OutputAndLogELF writes its
// output to guest_output and its log to guest_log, and traces within the
// constraints.
func TestOutputAndLogGuest(t *testing.T) {
	output := bytes.Repeat([]byte{0xab}, 32)
	log := []byte{0x00, 0x03, 0xff, 0x7f, 0x80, 0x01}
	elf := minimalelf.OutputAndLogELF(output, log)

	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}
	inputs, err := predecoding.PrepareInputs(elf, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	outputs, errs := binf.Execute(inputs)
	if len(errs) > 0 {
		t.Fatalf("execution failed: %v", errors.Join(errs...))
	}
	got := outputs["guest_log"]
	if !bytes.Equal(got.Bytes(), log) {
		t.Fatalf("guest_log = %x, want %x", got.Bytes(), log)
	}
	traceAndCheck(t, elf, output)
}

// Both buffers at the size cap. ZkC runs the predecoded program, so this passes
// even if a buffer overlaps .text; TestOutputAndLogText_BuffersPastText checks
// the layout.
func TestOutputAndLogGuest_MaxSize(t *testing.T) {
	output := make([]byte, 2047)
	log := make([]byte, 2047)
	for i := range output {
		output[i] = byte(i)
		log[i] = byte(i * 7)
	}
	elf := minimalelf.OutputAndLogELF(output, log)

	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}
	inputs, err := predecoding.PrepareInputs(elf, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	outputs, errs := binf.Execute(inputs)
	if len(errs) > 0 {
		t.Fatalf("execution failed: %v", errors.Join(errs...))
	}
	if got := outputs["guest_output"]; !bytes.Equal(got.Bytes(), output) {
		t.Fatalf("guest_output is %d bytes, want the %d bytes written", len(got.Bytes()), len(output))
	}
	if got := outputs["guest_log"]; !bytes.Equal(got.Bytes(), log) {
		t.Fatalf("guest_log is %d bytes, want the %d bytes written", len(got.Bytes()), len(log))
	}
}
