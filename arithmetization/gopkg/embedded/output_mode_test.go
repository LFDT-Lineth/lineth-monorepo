package embedded

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

// These tests run a generated guest through the compiled R5 interpreter to
// check the two output modes of write_output (see output_mode in
// src/main/riscv/memory.zkc):
//   - full (1): guest_output holds the whole output, whatever its size;
//   - prefix (0): guest_output holds only the first outputPrefixLen bytes,
//     which must equal the start of the full output, and the trace does not
//     grow with the output size.

const (
	outputModeInput   = "output_mode"
	guestOutputMemory = "guest_output"
	outputModeFull    = 1
	outputModePrefix  = 0
	// outputPrefixLen mirrors OUTPUT_PREFIX_LEN in src/main/common/constants.zkc.
	outputPrefixLen = 32
	// legacyOutputLimit used to represents large outputs
	legacyOutputLimit = 1<<16 - 1
)

// r5BinaryFile is the type CompiledBinaryFile returns.
type r5BinaryFile = constraints.BinaryFile[koalabear.Element]

var (
	compileOnce     sync.Once
	compiledBinFile *r5BinaryFile
	compileErr      error
)

// compiled returns the embedded R5 interpreter, compiling it once per test
// binary.
func compiled(t *testing.T) *r5BinaryFile {
	t.Helper()
	compileOnce.Do(func() {
		compiledBinFile, compileErr = CompiledBinaryFile()
	})
	if compileErr != nil {
		t.Fatalf("compiling embedded R5 interpreter: %v", compileErr)
	}
	return compiledBinFile
}

func TestOutputModeFullAndPrefix(t *testing.T) {
	binf := compiled(t)
	tests := []struct {
		name  string
		calls []uint64
	}{
		{"empty", []uint64{0}},
		{"short", []uint64{8}},
		{"exactly the prefix", []uint64{outputPrefixLen}},
		{"calls straddling the prefix", []uint64{31, 1, 5}},
		{"zero-size calls around the prefix", []uint64{0, 32, 0, 3}},
		{"legacy limit", []uint64{legacyOutputLimit}},
		{"one byte past the legacy limit", []uint64{legacyOutputLimit, 1}},
		{"single call past the legacy limit", []uint64{70_000}},
		{"cumulative calls past the legacy limit", []uint64{40_000, 40_000}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := outputTestData(tc.calls)

			full := executeGuestOutput(t, binf, tc.calls, data, outputModeFull)
			if !bytes.Equal(full, data) {
				t.Fatalf("full mode: guest_output has %d bytes, want the whole %d-byte output (first difference at %d)",
					len(full), len(data), firstDifference(full, data))
			}

			prefix := executeGuestOutput(t, binf, tc.calls, data, outputModePrefix)
			want := full[:min(len(full), outputPrefixLen)]
			if !bytes.Equal(prefix, want) {
				t.Fatalf("prefix mode: guest_output = %x, want the first %d bytes of the full output %x",
					prefix, len(want), want)
			}
		})
	}
}

// TestOutputModeTrace checks that both modes produce traces satisfying the
// constraints, and that in prefix mode the output-related part of the trace is
// the same for a small output and for one past the legacy limit.
func TestOutputModeTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("tracing and constraint checking are slow")
	}
	binf := compiled(t)
	smallCalls := []uint64{20, 50}
	largeCalls := []uint64{20, legacyOutputLimit + 10}
	// Modules whose height depends on how many output bytes are traced.
	outputModules := []string{"guest_output", "write_output", "read_8", "output_cursor"}

	traceFullSmall := traceGuest(t, binf, smallCalls, outputModeFull)
	traceSmall := traceGuest(t, binf, smallCalls, outputModePrefix)
	traceLarge := traceGuest(t, binf, largeCalls, outputModePrefix)

	for _, name := range outputModules {
		small, large := traceSmall[name], traceLarge[name]
		if small != large {
			t.Errorf("prefix mode: module %s has %d rows for a %d-byte output but %d rows for a %d-byte output",
				name, small, sum(smallCalls), large, sum(largeCalls))
		}
	}
	// Sanity check that the comparison above is meaningful: full mode does
	// trace every output byte.
	if full, prefix := traceFullSmall["guest_output"], traceSmall["guest_output"]; full <= prefix {
		t.Errorf("guest_output rows: full mode %d, prefix mode %d; want full > prefix", full, prefix)
	}
}

// executeGuestOutput runs the guest under Execute and returns guest_output.
func executeGuestOutput(t *testing.T, binf *r5BinaryFile,
	calls []uint64, data []byte, mode byte) []byte {
	t.Helper()
	output, errs := binf.Execute(guestInputs(t, calls, data, mode))
	if len(errs) > 0 {
		t.Fatalf("mode %d: Execute failed: %v", mode, errs)
	}
	out, ok := output[guestOutputMemory]
	if !ok && len(data) != 0 {
		t.Fatalf("mode %d: Execute returned no %s", mode, guestOutputMemory)
	}
	return out
}

// traceGuest traces the guest, checks the trace against the constraints, and
// returns the height of every module, summed over shards.
func traceGuest(t *testing.T, binf *r5BinaryFile,
	calls []uint64, mode byte) map[string]uint {
	t.Helper()
	cfg := vm.DEFAULT_TRACE_CONFIG
	inputs := guestInputs(t, calls, outputTestData(calls), mode)
	_, tr, errs := binf.Trace(inputs, cfg)
	if len(errs) > 0 || len(tr) == 0 {
		t.Fatalf("mode %d, calls %v: Trace failed: %v", mode, calls, errs)
	}
	if failures := binf.Check(cfg, tr); len(failures) > 0 {
		t.Fatalf("mode %d, calls %v: constraints do not hold: %v", mode, calls, failures)
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

// guestInputs builds the interpreter inputs for a guest that calls
// write_output once per entry of calls, over consecutive slices of data.
func guestInputs(t *testing.T, calls []uint64, data []byte, mode byte) map[string][]byte {
	t.Helper()
	inputs, err := predecoding.PrepareInputs(outputGuestELF(calls), data)
	if err != nil {
		t.Fatalf("preparing inputs: %v", err)
	}
	inputs[outputModeInput] = []byte{mode}
	return inputs
}

// outputTestData returns the guest input for calls: byte i is i mod 256, so a
// misplaced or dropped byte shows up as a mismatch.
func outputTestData(calls []uint64) []byte {
	data := make([]byte, sum(calls))
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

// outputGuestELF returns a guest that writes its input to the guest output with
// one write_output call per entry of calls, then exits with code 0.
func outputGuestELF(calls []uint64) []byte {
	const (
		t1, t3, a0, a7 = 6, 28, 10, 17
		textAddress    = 0x00800000
		exitSyscall    = 93
	)
	// The input data follows its 8-byte length prefix.
	code := li(t1, elfmapping.DefaultInputOrigin+8)
	for _, size := range calls {
		code = append(code, li(t3, size)...)
		code = append(code, writeOutputInsn(t1, t3), addInsn(t1, t1, t3))
	}
	code = append(code, li(a7, exitSyscall)...)
	code = append(code, li(a0, 0)...)
	code = append(code, 0x00000073) // ecall
	text := make([]byte, 4*len(code))
	for i, insn := range code {
		binary.LittleEndian.PutUint32(text[4*i:], insn)
	}
	return makeELF(textAddress, textAddress, text)
}

// li loads a value below 2^31 into rd with LUI + ADDI.
func li(rd uint32, value uint64) []uint32 {
	if value >= 1<<31 {
		panic(fmt.Sprintf("li: value %#x does not fit LUI + ADDI", value))
	}
	lo := int64(value & 0xfff)
	if lo >= 0x800 {
		lo -= 0x1000
	}
	hi := uint32((int64(value)-lo)>>12) & 0xfffff
	lui := hi<<12 | rd<<7 | 0x37
	addi := uint32(lo&0xfff)<<20 | rd<<15 | rd<<7 | 0x13
	return []uint32{lui, addi}
}

func addInsn(rd, rs1, rs2 uint32) uint32 {
	return rs2<<20 | rs1<<15 | rd<<7 | 0x33
}

// writeOutputInsn encodes write_output(rs1 = offset, rs2 = size): CUSTOM_1
// opcode, funct3 0b010 (see src/main/lib/README.md).
func writeOutputInsn(rs1, rs2 uint32) uint32 {
	return rs2<<20 | rs1<<15 | 0b010<<12 | 0b0101011
}

// makeELF builds a minimal ELF64 RISC-V executable with one PT_LOAD segment
// holding a single .text section.
func makeELF(entryPoint, sectionAddress uint64, sectionData []byte) []byte {
	const (
		elfHeaderSize     = 64
		programHeaderSize = 56
		sectionHeaderSize = 64
	)
	stringTable := []byte("\x00.text\x00.shstrtab\x00")
	textOffset := uint64(elfHeaderSize + programHeaderSize)
	stringTableOffset := textOffset + uint64(len(sectionData))
	sectionHeaderOffset := (stringTableOffset + uint64(len(stringTable)) + 7) &^ 7
	result := make([]byte, sectionHeaderOffset+3*sectionHeaderSize)
	le := binary.LittleEndian

	copy(result, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	le.PutUint16(result[16:18], 2)   // ET_EXEC
	le.PutUint16(result[18:20], 243) // EM_RISCV
	le.PutUint32(result[20:24], 1)
	le.PutUint64(result[24:32], entryPoint)
	le.PutUint64(result[32:40], elfHeaderSize)
	le.PutUint64(result[40:48], sectionHeaderOffset)
	le.PutUint16(result[52:54], elfHeaderSize)
	le.PutUint16(result[54:56], programHeaderSize)
	le.PutUint16(result[56:58], 1)
	le.PutUint16(result[58:60], sectionHeaderSize)
	le.PutUint16(result[60:62], 3)
	le.PutUint16(result[62:64], 2)

	programHeader := result[elfHeaderSize : elfHeaderSize+programHeaderSize]
	le.PutUint32(programHeader[0:4], 1) // PT_LOAD
	le.PutUint32(programHeader[4:8], 5) // PF_R | PF_X
	le.PutUint64(programHeader[8:16], textOffset)
	le.PutUint64(programHeader[16:24], sectionAddress)
	le.PutUint64(programHeader[24:32], sectionAddress)
	le.PutUint64(programHeader[32:40], uint64(len(sectionData)))
	le.PutUint64(programHeader[40:48], uint64(len(sectionData)))
	le.PutUint64(programHeader[48:56], 0x1000)
	copy(result[textOffset:], sectionData)
	copy(result[stringTableOffset:], stringTable)

	textHeader := result[sectionHeaderOffset+sectionHeaderSize:]
	le.PutUint32(textHeader[0:4], 1)  // ".text"
	le.PutUint32(textHeader[4:8], 1)  // SHT_PROGBITS
	le.PutUint64(textHeader[8:16], 6) // SHF_ALLOC | SHF_EXECINSTR
	le.PutUint64(textHeader[16:24], sectionAddress)
	le.PutUint64(textHeader[24:32], textOffset)
	le.PutUint64(textHeader[32:40], uint64(len(sectionData)))
	le.PutUint64(textHeader[48:56], 4)

	stringHeader := result[sectionHeaderOffset+2*sectionHeaderSize:]
	le.PutUint32(stringHeader[0:4], 7) // ".shstrtab"
	le.PutUint32(stringHeader[4:8], 3) // SHT_STRTAB
	le.PutUint64(stringHeader[24:32], stringTableOffset)
	le.PutUint64(stringHeader[32:40], uint64(len(stringTable)))
	le.PutUint64(stringHeader[48:56], 1)
	return result
}

func sum(values []uint64) uint64 {
	var total uint64
	for _, v := range values {
		total += v
	}
	return total
}

func firstDifference(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}
