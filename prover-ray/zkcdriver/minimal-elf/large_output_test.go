package minimalelf

import (
	"errors"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

// TestExecuteLargeGuestOutput checks that the no-trace execution path returns a
// guest output larger than 2^23 bytes, more than a trace module can hold. The
// guest marks the first and last bytes of its output and writes the whole span
// in one write_output call; the bytes in between are never written and are
// expected to read as zero.
func TestExecuteLargeGuestOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a guest writing more than 2^23 output bytes")
	}

	const size = 1<<23 + 1

	a := newAssembler()
	a.auipc(s0, 0)
	a.addi(t1, s0, 1216) // output buffer, clear of .text
	a.lui(t3, size>>12)
	a.addi(t3, t3, size&0xFFF) // t3 = size
	a.addi(t2, zero, 'A')
	a.sb(t2, t1, 0) // first byte
	a.add(t4, t1, t3)
	a.addi(t2, zero, 'Z')
	a.sb(t2, t4, -1) // last byte
	a.writeOutput(t1, t3)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()

	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}
	inputs, err := predecoding.PrepareInputs(Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes()), nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}

	start := time.Now()
	outputs, errs := binf.Execute(inputs)
	if len(errs) > 0 {
		t.Fatalf("execution failed: %v", errors.Join(errs...))
	}
	t.Logf("executed in %v", time.Since(start))

	got := outputs["guest_output"]
	gotBytes := got.Bytes()
	//
	if len(gotBytes) != size {
		t.Fatalf("guest_output has %d bytes, want %d", len(gotBytes), size)
	}
	if gotBytes[0] != 'A' || gotBytes[size-1] != 'Z' {
		t.Fatalf("guest_output first/last bytes = %q/%q, want 'A'/'Z'", gotBytes[0], gotBytes[size-1])
	}
}
