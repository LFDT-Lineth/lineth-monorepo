package predecoding_test

// Experiment: does the pre-decoded instruction table — the basis of the guest
// Program ID — distinguish two guests that differ only in .rodata?
//
// Requirement 2 of the Program ID design is that the
// identifier "must uniquely define a specific guest program — no two different
// programs can share the same identifier". Predecode consumes only the blobs
// flagged Executable, and elfmapping sets that flag per *section*
// (SHF_EXECINSTR), so an allocated-but-not-executable .rodata section is
// dropped before decoding. If so, flipping a constant in .rodata leaves the
// decoded table byte-identical and Requirement 2 does not hold for a Program ID
// derived from the decoded table alone.
//
// This test asserts the *observed* behaviour so the answer is recorded rather
// than argued. If pre-decoding is later changed to cover initialized data, or a
// separate initial-memory commitment is added to the Program ID, this test
// fails and should be updated to assert the new, stronger property.

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

const (
	expEntryPoint  = 0x00800000
	expTextAddr    = 0x00800000
	expRodataAddr  = 0x00800010 // past .text, same PT_LOAD segment
	expSegmentSize = 0x20
)

// expText is `auipc x5, 0` followed by `addi x0, x0, 0`: two valid RV64I words,
// identical in both variants under test.
var expText = []byte{
	0x97, 0x02, 0x00, 0x00, // auipc x5, 0
	0x13, 0x00, 0x00, 0x00, // addi x0, x0, 0  (canonical nop)
}

// makeELFWithRodata builds a minimal ET_EXEC RISC-V ELF64 with one PT_LOAD
// segment containing two allocated sections: .text (SHF_ALLOC|SHF_EXECINSTR)
// and .rodata (SHF_ALLOC only). Only rodata differs between variants.
func makeELFWithRodata(t *testing.T, text, rodata []byte) []byte {
	t.Helper()

	const (
		ehdrSize = 64
		phdrSize = 56
		shdrSize = 64
		numShdr  = 4 // NULL + .text + .rodata + .shstrtab
	)

	// offsets into shstrtab
	shstrtab := []byte("\x00.text\x00.rodata\x00.shstrtab\x00")
	nameText := uint32(1)
	nameRodata := uint32(7)
	nameShstr := uint32(15)

	textOff := uint64(ehdrSize + phdrSize)
	rodataOff := textOff + uint64(len(text))
	shstrOff := rodataOff + uint64(len(rodata))
	shOff := (shstrOff + uint64(len(shstrtab)) + 7) &^ 7

	buf := new(bytes.Buffer)
	le := binary.LittleEndian
	w := func(v any) {
		if err := binary.Write(buf, le, v); err != nil {
			t.Fatalf("writing ELF: %v", err)
		}
	}

	buf.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	w(uint16(2))   // e_type:     ET_EXEC
	w(uint16(243)) // e_machine:  EM_RISCV
	w(uint32(1))   // e_version
	w(uint64(expEntryPoint))
	w(uint64(ehdrSize)) // e_phoff
	w(shOff)            // e_shoff
	w(uint32(0))        // e_flags
	w(uint16(ehdrSize)) // e_ehsize
	w(uint16(phdrSize)) // e_phentsize
	w(uint16(1))        // e_phnum
	w(uint16(shdrSize)) // e_shentsize
	w(uint16(numShdr))  // e_shnum
	w(uint16(3))        // e_shstrndx

	// PT_LOAD spanning .text and .rodata
	w(uint32(1))              // p_type:  PT_LOAD
	w(uint32(5))              // p_flags: PF_R | PF_X
	w(textOff)                // p_offset
	w(uint64(expTextAddr))    // p_vaddr
	w(uint64(expTextAddr))    // p_paddr
	w(uint64(expSegmentSize)) // p_filesz
	w(uint64(expSegmentSize)) // p_memsz
	w(uint64(0x1000))         // p_align

	buf.Write(text)
	buf.Write(rodata)
	buf.Write(shstrtab)
	for uint64(buf.Len()) < shOff {
		buf.WriteByte(0)
	}

	// index 0: NULL
	for range shdrSize {
		buf.WriteByte(0)
	}

	// index 1: .text — allocated AND executable
	w(nameText)
	w(uint32(1)) // SHT_PROGBITS
	w(uint64(6)) // SHF_ALLOC | SHF_EXECINSTR
	w(uint64(expTextAddr))
	w(textOff)
	w(uint64(len(text)))
	w(uint32(0))
	w(uint32(0))
	w(uint64(4))
	w(uint64(0))

	// index 2: .rodata — allocated, NOT executable
	w(nameRodata)
	w(uint32(1)) // SHT_PROGBITS
	w(uint64(2)) // SHF_ALLOC
	w(uint64(expRodataAddr))
	w(rodataOff)
	w(uint64(len(rodata)))
	w(uint32(0))
	w(uint32(0))
	w(uint64(1))
	w(uint64(0))

	// index 3: .shstrtab
	w(nameShstr)
	w(uint32(3)) // SHT_STRTAB
	w(uint64(0))
	w(uint64(0))
	w(shstrOff)
	w(uint64(len(shstrtab)))
	w(uint32(0))
	w(uint32(0))
	w(uint64(1))
	w(uint64(0))

	return buf.Bytes()
}

// TestRodataIsInvisibleToPredecode is the experiment proper: two guests whose
// .rodata differs by a single byte.
func TestRodataIsInvisibleToPredecode(t *testing.T) {
	rodataA := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x00, 0x00, 0x00, 0x00}
	rodataB := []byte{0xAB, 0xBB, 0xCC, 0xDD, 0x00, 0x00, 0x00, 0x00} // one byte flipped

	elfA := makeELFWithRodata(t, expText, rodataA)
	elfB := makeELFWithRodata(t, expText, rodataB)

	if bytes.Equal(elfA, elfB) {
		t.Fatal("test setup is broken: the two ELF files are identical")
	}

	progA, err := elfmapping.Load(bytes.NewReader(elfA))
	if err != nil {
		t.Fatalf("loading ELF A: %v", err)
	}
	progB, err := elfmapping.Load(bytes.NewReader(elfB))
	if err != nil {
		t.Fatalf("loading ELF B: %v", err)
	}

	// Precondition: elfmapping must actually see .rodata as a non-executable
	// blob. If it does not, the rest of this test proves nothing.
	var sawExec, sawNonExec bool
	for _, blob := range progA.Blobs {
		t.Logf("blob %-10s addr=%#x size=%d executable=%v",
			blob.Name, blob.Address, len(blob.Data), blob.Executable)
		if blob.Executable {
			sawExec = true
		} else {
			sawNonExec = true
		}
	}
	if !sawExec || !sawNonExec {
		t.Fatalf("test setup is broken: want both an executable and a non-executable blob, got exec=%v nonExec=%v",
			sawExec, sawNonExec)
	}

	decodedA, err := predecoding.Predecode(progA)
	if err != nil {
		t.Fatalf("predecoding A: %v", err)
	}
	decodedB, err := predecoding.Predecode(progB)
	if err != nil {
		t.Fatalf("predecoding B: %v", err)
	}

	// (1) The decoded table is the basis of the Program ID.
	sameDecoded := bytes.Equal(decodedA.Decoded, decodedB.Decoded)
	sameBase := decodedA.InstructionBase == decodedB.InstructionBase
	t.Logf("decoded tables equal: %v (len %d vs %d); instruction_base equal: %v",
		sameDecoded, len(decodedA.Decoded), len(decodedB.Decoded), sameBase)

	// (2) But the differing bytes DO reach the circuit, via blobs_data.
	inputsA, err := elfmapping.EncodeInputs(progA, nil)
	if err != nil {
		t.Fatalf("encoding inputs A: %v", err)
	}
	inputsB, err := elfmapping.EncodeInputs(progB, nil)
	if err != nil {
		t.Fatalf("encoding inputs B: %v", err)
	}
	blobsDataDiffers := !bytes.Equal(
		inputsA[elfmapping.BlobsDataInput],
		inputsB[elfmapping.BlobsDataInput],
	)
	t.Logf("blobs_data differs between the two guests: %v", blobsDataDiffers)

	if !blobsDataDiffers {
		t.Error("blobs_data is identical: the .rodata difference does not reach the circuit at all, " +
			"so this test cannot speak to Requirement 2")
	}

	if sameDecoded && sameBase {
		t.Errorf("REQUIREMENT 2 NOT MET: two guests differing in .rodata produce an identical " +
			"decoded table and instruction_base, so a Program ID derived from the decoded program " +
			"alone cannot distinguish them — while blobs_data (and therefore the proven execution) " +
			"does differ. Closing this needs either a separate initial-memory commitment or an " +
			"explicit decision to accept the gap.")
	}
}
