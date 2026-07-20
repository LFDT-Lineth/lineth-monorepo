package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 0x00800000 is the conventional load address for the Lineth guest ELF on RISC-V.
const (
	testEntry   = uint64(0x00800000)
	testSecAddr = uint64(0x00800000)
)

// testSecData is a valid RISC-V auipc x5, 0 instruction encoding.
var testSecData = []byte{0x97, 0x02, 0x00, 0x00}

// makeMinimalELF builds a minimal valid ELF64 RISC-V binary for testing.
// It has one PT_LOAD segment containing exactly one .text section at
// sectionAddr with sectionData bytes, and an entry point of entryPoint.
//
// Layout:
//
//	0        64   ELF header
//	64       56   PT_LOAD program header
//	120      N    .text bytes  (N = len(sectionData))
//	120+N    17   .shstrtab: "\x00.text\x00.shstrtab\x00"
//	(aligned) …   padding to 8-byte boundary
//	X        64   NULL section header
//	X+64     64   .text section header
//	X+128    64   .shstrtab section header
func makeMinimalELF(t *testing.T, entryPoint, sectionAddr uint64, sectionData []byte) []byte {
	t.Helper()

	const (
		ehdrSize = 64
		phdrSize = 56
		shdrSize = 64
	)

	// section string table: index 1 = ".text", index 7 = ".shstrtab"
	shstrtab := []byte("\x00.text\x00.shstrtab\x00") // 17 bytes

	textOff := uint64(ehdrSize + phdrSize) // = 120
	shstrOff := textOff + uint64(len(sectionData))
	rawEnd := shstrOff + uint64(len(shstrtab))
	shOff := (rawEnd + 7) &^ 7 // align to 8 bytes

	buf := new(bytes.Buffer)
	le := binary.LittleEndian

	w := func(v any) {
		if err := binary.Write(buf, le, v); err != nil {
			t.Fatalf("makeMinimalELF: binary.Write: %v", err)
		}
	}

	// ELF header (64 bytes)
	buf.Write([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}) // e_ident (magic + class/data/version/OS/ABI)

	w(uint16(2))        // e_type:      ET_EXEC
	w(uint16(243))      // e_machine:   EM_RISCV
	w(uint32(1))        // e_version:   EV_CURRENT
	w(entryPoint)       // e_entry
	w(uint64(ehdrSize)) // e_phoff:     program headers start right after Ehdr
	w(shOff)            // e_shoff:     section headers after data
	w(uint32(0))        // e_flags
	w(uint16(ehdrSize)) // e_ehsize
	w(uint16(phdrSize)) // e_phentsize
	w(uint16(1))        // e_phnum
	w(uint16(shdrSize)) // e_shentsize
	w(uint16(3))        // e_shnum:     NULL + .text + .shstrtab
	w(uint16(2))        // e_shstrndx:  .shstrtab is section 2

	// PT_LOAD program header (56 bytes)
	w(uint32(1))                // p_type:   PT_LOAD
	w(uint32(5))                // p_flags:  PF_R | PF_X
	w(textOff)                  // p_offset: file offset of .text
	w(sectionAddr)              // p_vaddr
	w(sectionAddr)              // p_paddr
	w(uint64(len(sectionData))) // p_filesz
	w(uint64(len(sectionData))) // p_memsz
	w(uint64(0x1000))           // p_align

	// section data
	buf.Write(sectionData) // .text
	buf.Write(shstrtab)    // .shstrtab

	// padding to shOff
	for uint64(buf.Len()) < shOff {
		buf.WriteByte(0)
	}

	// NULL section header (index 0)
	for i := 0; i < shdrSize; i++ {
		buf.WriteByte(0)
	}

	// .text section header (index 1)
	w(uint32(1))                // sh_name:      ".text" at shstrtab[1]
	w(uint32(1))                // sh_type:      SHT_PROGBITS
	w(uint64(6))                // sh_flags:     SHF_ALLOC | SHF_EXECINSTR
	w(sectionAddr)              // sh_addr
	w(textOff)                  // sh_offset:    file offset
	w(uint64(len(sectionData))) // sh_size
	w(uint32(0))                // sh_link
	w(uint32(0))                // sh_info
	w(uint64(4))                // sh_addralign
	w(uint64(0))                // sh_entsize

	// .shstrtab section header (index 2)
	w(uint32(7))             // sh_name:      ".shstrtab" at shstrtab[7]
	w(uint32(3))             // sh_type:      SHT_STRTAB
	w(uint64(0))             // sh_flags
	w(uint64(0))             // sh_addr
	w(shstrOff)              // sh_offset
	w(uint64(len(shstrtab))) // sh_size
	w(uint32(0))             // sh_link
	w(uint32(0))             // sh_info
	w(uint64(1))             // sh_addralign
	w(uint64(0))             // sh_entsize

	return buf.Bytes()
}

func TestSszBlobs_LengthPrefixAtInOrigin(t *testing.T) {
	ssz := []byte{0xAA, 0xBB, 0xCC}
	got := sszBlobs(DefaultINOrigin, ssz)
	require.Len(t, got, 2)

	// First memory blob: 8-byte LE length at inOrigin.
	assert.Equal(t, DefaultINOrigin, got[0].offset, "first memory blob offset must be inOrigin")
	require.Len(t, got[0].data, 8, "length prefix must be exactly 8 bytes")
	assert.Equal(t, uint64(3), binary.LittleEndian.Uint64(got[0].data), "length prefix must encode payload length as LE uint64")
}

func TestSszBlobs_PayloadAtInOriginPlus8(t *testing.T) {
	ssz := []byte{0xAA, 0xBB, 0xCC}
	got := sszBlobs(DefaultINOrigin, ssz)
	require.Len(t, got, 2)

	// Second memory blob: raw SSZ bytes at inOrigin+8.
	assert.Equal(t, DefaultINOrigin+8, got[1].offset, "payload memory blob offset must be inOrigin+8")
	assert.Equal(t, ssz, got[1].data, "payload memory blob must contain the raw SSZ bytes")
}

func TestSszBlobs_EmptySSZ(t *testing.T) {
	// Empty SSZ: only the 8-byte length memory blob, no payload memory blob.
	got := sszBlobs(DefaultINOrigin, nil)
	require.Len(t, got, 1, "empty SSZ must produce exactly one memory blob (length prefix only)")
	assert.Equal(t, DefaultINOrigin, got[0].offset, "length prefix memory blob offset must be inOrigin")
	assert.Equal(t, uint64(0), binary.LittleEndian.Uint64(got[0].data), "length prefix must be zero for empty SSZ")
}

func TestBuildJSON_ContainsThreeKeys(t *testing.T) {
	j := string(buildJSON([]memoryBlob{{offset: 0x1000, data: []byte{0x01}}}, 0x800000))
	assert.Contains(t, j, `"entry_point_and_blobs_count"`)
	assert.Contains(t, j, `"blobs_offset_and_size"`)
	assert.Contains(t, j, `"blobs_data"`)
}

func TestBuildJSON_EntryPointIs16HexChars(t *testing.T) {
	j := string(buildJSON([]memoryBlob{{offset: 0, data: []byte{0}}}, 0x00800000))
	// entry_point must be exactly 16 hex characters: 0000000000800000
	assert.Contains(t, j, `"0x0000000000800000_`)
}

func TestBuildJSON_BlobCountEncodedWith16HexChars(t *testing.T) {
	memBlobs := []memoryBlob{
		{offset: 0x1000, data: []byte{0x01}},
		{offset: 0x2000, data: []byte{0x02}},
	}
	j := string(buildJSON(memBlobs, 0))
	// count = 2 must appear as 16 hex chars
	assert.Contains(t, j, `_0000000000000002"`)
}

func TestBuildJSON_FourUnderscoresBetweenBlobs(t *testing.T) {
	// Memory blob boundaries in blobs_offset_and_size and blobs_data must be
	// separated by "____" (four underscores). This is the delimiter that
	// zkc_util.ParseJsonInputFile and the reference elf_to_json_gen tool both use.
	memBlobs := []memoryBlob{
		{offset: 0x1000, data: []byte{0xAA}},
		{offset: 0x2000, data: []byte{0xBB}},
	}
	j := string(buildJSON(memBlobs, 0))
	assert.Contains(t, j, "____", "memory blob boundaries must use four underscores as separator")
}

func TestBuildJSON_OffsetSizeSingleUnderscore(t *testing.T) {
	// Within one memory blob's offset_size entry, offset and size are separated by
	// a single underscore, not four.
	memBlobs := []memoryBlob{{offset: 0x00800000, data: []byte{0x01, 0x02}}}
	j := string(buildJSON(memBlobs, 0))
	// Expect "0000000000800000_0000000000000002" (single _ between offset and size)
	assert.Contains(t, j, "0000000000800000_0000000000000002")
}

func TestBuildJSON_BlobDataEncodedAsLowerHex(t *testing.T) {
	// zkc_util.ParseJsonInputFile is case-sensitive: memory blob data must be lowercase hex.
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	j := string(buildJSON([]memoryBlob{{offset: 0, data: data}}, 0))
	assert.Contains(t, j, hex.EncodeToString(data), `memory blob data must be lowercase hex ("deadbeef", not "DEADBEEF")`)
}

func TestBuildJSON_SingleBlobNoFourUnderscores(t *testing.T) {
	// With only one memory blob, no "____" separator should appear.
	j := string(buildJSON([]memoryBlob{{offset: 0x1000, data: []byte{0x01}}}, 0))
	assert.NotContains(t, j, "____")
}

func TestElfBlobs_ExtractsSectionAtCorrectOffset(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	memBlobs, _, err := loadELFBlobs(elfBytes)
	require.NoError(t, err)
	require.Len(t, memBlobs, 1, "one loadable section must yield one memory blob")
	assert.Equal(t, testSecAddr, memBlobs[0].offset, "memory blob offset must match the section's virtual address")
	assert.Equal(t, testSecData, memBlobs[0].data, "memory blob data must match the section bytes")
}

func TestElfBlobs_EntryPointPreserved(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	_, entry, err := loadELFBlobs(elfBytes)
	require.NoError(t, err)
	assert.Equal(t, testEntry, entry, "entry point must match ELF e_entry")
}

func TestElfBlobs_InvalidELFReturnsError(t *testing.T) {
	_, _, err := loadELFBlobs([]byte("not an elf"))
	assert.Error(t, err, "malformed ELF must return an error")
}

func TestBuildZkcInputs_ReturnsThreeKeys(t *testing.T) {
	// These three key names are declared in RISCV-ZKC.bin's main.zkc. A name
	// mismatch causes a silent no-op when the prover loads inputs.
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	inputs, err := BuildZkcInputs(elfBytes, []byte{0x01, 0x02}, DefaultINOrigin)
	require.NoError(t, err)

	assert.Contains(t, inputs, "entry_point_and_blobs_count")
	assert.Contains(t, inputs, "blobs_offset_and_size")
	assert.Contains(t, inputs, "blobs_data")
}

func TestBuildZkcInputs_InvalidELFReturnsError(t *testing.T) {
	_, err := BuildZkcInputs([]byte("not an elf"), []byte{}, DefaultINOrigin)
	assert.Error(t, err, "malformed ELF must return an error")
}

func TestBuildZkcInputs_EmptySSZ(t *testing.T) {
	// Empty SSZ must still succeed and produce the three keys.
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	inputs, err := BuildZkcInputs(elfBytes, nil, DefaultINOrigin)
	require.NoError(t, err)
	assert.Len(t, inputs, 3, "must return exactly three pub-input keys")
}

func TestBuildZkcInputs_DifferentSSZ(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)

	inputs1, err := BuildZkcInputs(elfBytes, []byte{0x01}, DefaultINOrigin)
	require.NoError(t, err)

	inputs2, err := BuildZkcInputs(elfBytes, []byte{0x02}, DefaultINOrigin)
	require.NoError(t, err)

	assert.NotEqual(t, inputs1["blobs_data"], inputs2["blobs_data"], "different SSZ must produce different blobs_data")
	assert.Equal(t, inputs1["entry_point_and_blobs_count"], inputs2["entry_point_and_blobs_count"], "same ELF must produce identical entry_point_and_blobs_count")
}

func TestCore_BuildInputs_UsesPrecomputedELFBlobs(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	precomputed, entry, err := loadELFBlobs(elfBytes)
	require.NoError(t, err)

	c := &Core{
		cfg:      Config{INOrigin: DefaultINOrigin},
		elfBlobs: precomputed,
		elfEntry: entry,
	}

	ssz1 := []byte{0x01, 0x02}
	ssz2 := []byte{0xFF, 0xFE}

	inputs1, err := c.buildInputs(Job{Payload: ssz1})
	require.NoError(t, err)

	inputs2, err := c.buildInputs(Job{Payload: ssz2})
	require.NoError(t, err)

	assert.NotEqual(t, inputs1["blobs_data"], inputs2["blobs_data"], "different SSZ must produce different blobs_data")
	assert.Equal(t, inputs1["entry_point_and_blobs_count"], inputs2["entry_point_and_blobs_count"], "same ELF must produce identical entry_point_and_blobs_count")
}

func TestCore_BuildInputs_MatchesBuildZkcInputs(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	precomputed, entry, err := loadELFBlobs(elfBytes)
	require.NoError(t, err)

	c := &Core{
		cfg:      Config{INOrigin: DefaultINOrigin},
		elfBlobs: precomputed,
		elfEntry: entry,
	}

	ssz := []byte{0xAA, 0xBB}

	// Core.buildInputs (precomputed path) must produce identical output to
	// BuildZkcInputs (parse-every-call path).
	fromCore, err := c.buildInputs(Job{Payload: ssz})
	require.NoError(t, err)

	fromFull, err := BuildZkcInputs(elfBytes, ssz, DefaultINOrigin)
	require.NoError(t, err)

	assert.Equal(t, fromFull, fromCore, "precomputed path must produce identical output to BuildZkcInputs")
}

// elf_to_json_gen (arithmetization/elf_to_json_gen/main.go) is the reference
// tool that produces ZkC pub-input JSON for the same circuit. The two
// implementations must produce the same key structure and hex encoding so that
// zkc_util.ParseJsonInputFile accepts both without special-casing.

func TestBuildJSON_OffsetSizeFormat(t *testing.T) {
	// elf_to_json_gen formats each memory blob as "%016x_%016x" % (offset, size).
	memBlobs := []memoryBlob{{offset: 0x00800000, data: make([]byte, 0x1234)}}
	j := string(buildJSON(memBlobs, testEntry))
	assert.Contains(t, j, "0000000000800000_0000000000001234",
		"blobs_offset_and_size must use 16-hex-char offset and size separated by a single underscore")
}

func TestBuildJSON_EntryFormat(t *testing.T) {
	// elf_to_json_gen formats the first key as "0x{entry16hex}_{count16hex}".
	memBlobs := []memoryBlob{{offset: 0, data: []byte{0}}}
	j := string(buildJSON(memBlobs, 0x00800150))
	assert.Contains(t, j, `"0x0000000000800150_0000000000000001"`,
		"entry_point_and_blobs_count must be 0x{entry}_{count} in 16-hex-char fields")
}

// TestCore_New_Integration is a placeholder for a full Core.New() integration
// test. It requires go tool zkc (PR 3580) to compile the ZKC binary on-the-fly
// and a real guest ELF. Neither is available yet.
func TestCore_New_Integration(t *testing.T) {
	t.Skip("needs go tool zkc (PR 3580 merged) and a real guest ELF to compile the circuit binary")
}
