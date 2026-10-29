package minimalelf

const (
	// sameCodeDataOffset is the offset of .rodata from DefaultSectionAddr,
	// past .text.
	sameCodeDataOffset = 64
	// sameCodeEntryB is the offset of the second entry stub in .text.
	sameCodeEntryB = 12
	// sameCodeOutputLen is the number of .rodata bytes written to guest_output.
	sameCodeOutputLen = 32
)

// EntryPointPairELFs returns two ELFs with the same .text and the same .rodata
// that differ only by their entry point, together with their guest outputs.
// The first writes the first half of .rodata to guest_output and the second
// the second half. See [sameCodeELF].
func EntryPointPairELFs() (elfs, outputs [2][]byte) {
	data := countingData('A', 2*sameCodeOutputLen)
	for i, entry := range []uint64{DefaultEntryPoint, DefaultEntryPoint + sameCodeEntryB} {
		elfs[i] = sameCodeELF(entry, data)
		outputs[i] = data[i*sameCodeOutputLen : (i+1)*sameCodeOutputLen]
	}
	return elfs, outputs
}

// RodataPairELFs returns two ELFs with the same .text and the same entry
// point that differ only by their .rodata, together with their guest outputs:
// each writes the first half of its .rodata to guest_output. See
// [sameCodeELF].
func RodataPairELFs() (elfs, outputs [2][]byte) {
	for i, first := range []byte{'A', '0'} {
		data := countingData(first, 2*sameCodeOutputLen)
		elfs[i] = sameCodeELF(DefaultEntryPoint, data)
		outputs[i] = data[:sameCodeOutputLen]
	}
	return elfs, outputs
}

// sameCodeELF returns an ELF whose .text, at DefaultSectionAddr, is the same
// whatever entry and data are, so that every such ELF has the same
// instruction_base and decoded table. data is mapped as .rodata at
// DefaultSectionAddr+sameCodeDataOffset. The code writes sameCodeOutputLen
// bytes of .rodata to guest_output and exits 0:
//
//   - entered at DefaultEntryPoint, the bytes at .rodata+0;
//   - entered at DefaultEntryPoint+sameCodeEntryB, the bytes at
//     .rodata+sameCodeOutputLen.
func sameCodeELF(entry uint64, data []byte) []byte {
	a := newAssembler()
	a.auipc(t1, 0) // entry A
	a.addi(t1, t1, sameCodeDataOffset)
	a.beq(zero, zero, "write")
	a.auipc(t1, 0) // entry B, at sameCodeEntryB
	a.addi(t1, t1, sameCodeDataOffset-sameCodeEntryB+sameCodeOutputLen)
	a.label("write")
	a.addi(t3, zero, sameCodeOutputLen)
	a.writeOutput(t1, t3)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()
	return makeELF(entry, DefaultSectionAddr, a.bytes(), sameCodeDataOffset, data)
}

// countingData returns n bytes counting up from first.
func countingData(first byte, n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = first + byte(i)
	}
	return data
}
