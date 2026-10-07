package guestoutput

import (
	"bytes"
	"encoding/binary"
	"maps"
	"testing"

	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

// commitmentLen mirrors OUTPUT_COMMITMENT_LEN in src/main/lib/io/write_output.zkc.
const commitmentLen = 32

func TestExecute(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatal(err)
	}
	// Output sizes written by each write_output call.
	for _, calls := range [][]uint64{{402}, {1<<16 + 10}, {40, 50}} {
		inputs, data := guestInputs(t, calls)
		outputs, guestOutput, errs := Execute(binf, inputs)
		if len(errs) > 0 {
			t.Fatalf("calls %v: Execute failed: %v", calls, errs)
		}
		if !bytes.Equal(guestOutput, data) {
			t.Errorf("calls %v: guest output has %d bytes, want the whole %d-byte output", calls, len(guestOutput), len(data))
		}
		// guest_output holds the first commitmentLen bytes of each call.
		var traced []byte
		for rest, i := data, 0; i < len(calls); rest, i = rest[calls[i]:], i+1 {
			traced = append(traced, rest[:commitmentLen]...)
		}
		if !bytes.Equal(outputs["guest_output"], traced) {
			t.Errorf("calls %v: guest_output = %x, want %x", calls, outputs["guest_output"], traced)
		}
		// Reading the output on the host does not change the execution.
		if plain, _ := binf.Execute(inputs); !maps.EqualFunc(outputs, plain, bytes.Equal) {
			t.Errorf("calls %v: outputs differ from BinaryFile.Execute", calls)
		}
	}
}

// TestTrace checks that the trace satisfies the constraints and does not grow
// with the output size.
func TestTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("tracing and constraint checking are slow")
	}
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatal(err)
	}
	heights := func(calls []uint64) map[string]uint {
		inputs, _ := guestInputs(t, calls)
		_, tr, errs := binf.Trace(inputs, vm.DEFAULT_TRACE_CONFIG)
		if len(errs) > 0 {
			t.Fatalf("calls %v: Trace failed: %v", calls, errs)
		}
		if failures := binf.Check(vm.DEFAULT_TRACE_CONFIG, tr); len(failures) > 0 {
			t.Fatalf("calls %v: constraints do not hold: %v", calls, failures)
		}
		heights := map[string]uint{}
		for _, shard := range tr {
			for it := shard.Modules(); it.HasNext(); {
				module := it.Next()
				heights[module.Name()] += module.Height()
			}
		}
		return heights
	}
	small, large := heights([]uint64{commitmentLen + 1}), heights([]uint64{1<<16 + 10})
	for _, name := range []string{"guest_output", "guest_output_tmp", "write_output", "read_8"} {
		if small[name] != large[name] {
			t.Errorf("module %s has %d rows for a small output but %d rows for a large one", name, small[name], large[name])
		}
	}
}

// guestInputs returns the interpreter inputs for a guest writing its input to
// the guest output, with one write_output call per entry of calls, and that
// input: byte i is i mod 256.
func guestInputs(t *testing.T, calls []uint64) (map[string][]byte, []byte) {
	t.Helper()
	const textAddress = 0x00800000
	var size uint64
	for _, call := range calls {
		size += call
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	// t1 = address of the input, after its 8-byte length prefix.
	code := li(6, elfmapping.DefaultInputOrigin+8)
	for _, call := range calls {
		code = append(code, li(28, call)...)             // t3 = size
		code = append(code, 28<<20|6<<15|0b010<<12|0x2b) // write_output(t1, t3)
		code = append(code, 28<<20|6<<15|6<<7|0x33)      // t1 += t3
	}
	code = append(code, li(17, 93)...) // a7 = exit
	code = append(code, li(10, 0)...)  // a0 = 0
	code = append(code, 0x00000073)    // ecall
	text := make([]byte, 4*len(code))
	for i, insn := range code {
		binary.LittleEndian.PutUint32(text[4*i:], insn)
	}

	program := elfmapping.Program{
		EntryPoint: textAddress,
		Blobs:      []elfmapping.Blob{{Address: textAddress, Data: text, Executable: true}},
	}
	decoded, err := predecoding.Predecode(program)
	if err != nil {
		t.Fatal(err)
	}
	input, err := elfmapping.NewData(elfmapping.DefaultInputOrigin, data, elfmapping.WithLengthPrefix())
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := elfmapping.EncodeInputs(program, input)
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(inputs, decoded.EncodeInputs())
	return inputs, data
}

// li loads a value below 2^31 into register rd with LUI + ADDI.
func li(rd uint32, value uint64) []uint32 {
	lo := int64(value & 0xfff)
	if lo >= 0x800 {
		lo -= 0x1000
	}
	hi := uint32((int64(value)-lo)>>12) & 0xfffff
	return []uint32{hi<<12 | rd<<7 | 0x37, uint32(lo&0xfff)<<20 | rd<<15 | rd<<7 | 0x13}
}
