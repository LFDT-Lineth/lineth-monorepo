package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zkc_util "github.com/LFDT-Lineth/zkc/pkg/zkc/util"
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

func TestEncodeInputs_EntryPointAndCount(t *testing.T) {
	inputs := encodeInputs([]memoryBlob{
		{offset: 0x1000, data: []byte{0x01}},
		{offset: 0x2000, data: []byte{0x02}},
	}, testEntry)

	got := inputs["entry_point_and_blobs_count"]
	require.Len(t, got, 16, "entry_point_and_blobs_count must be two BE uint64s")
	assert.Equal(t, testEntry, binary.BigEndian.Uint64(got[:8]), "first 8 bytes must be the BE entry point")
	assert.Equal(t, uint64(2), binary.BigEndian.Uint64(got[8:]), "last 8 bytes must be the BE blob count")
}

func TestEncodeInputs_OffsetAndSizePairs(t *testing.T) {
	memBlobs := []memoryBlob{
		{offset: 0x00800000, data: make([]byte, 0x1234)},
		{offset: 0x2000, data: []byte{0xBB}},
	}
	inputs := encodeInputs(memBlobs, 0)

	got := inputs["blobs_offset_and_size"]
	require.Len(t, got, 32, "blobs_offset_and_size must be one 16-byte pair per blob")
	assert.Equal(t, uint64(0x00800000), binary.BigEndian.Uint64(got[0:8]))
	assert.Equal(t, uint64(0x1234), binary.BigEndian.Uint64(got[8:16]))
	assert.Equal(t, uint64(0x2000), binary.BigEndian.Uint64(got[16:24]))
	assert.Equal(t, uint64(1), binary.BigEndian.Uint64(got[24:32]))
}

func TestEncodeInputs_DataConcatenated(t *testing.T) {
	memBlobs := []memoryBlob{
		{offset: 0x1000, data: []byte{0xAA, 0xBB}},
		{offset: 0x2000, data: []byte{0xCC}},
	}
	inputs := encodeInputs(memBlobs, 0)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, inputs["blobs_data"], "blobs_data must be all blob bytes concatenated in order")
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

	inputs1 := c.buildInputs(Job{Payload: ssz1})
	inputs2 := c.buildInputs(Job{Payload: ssz2})

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
	fromCore := c.buildInputs(Job{Payload: ssz})

	fromFull, err := BuildZkcInputs(elfBytes, ssz, DefaultINOrigin)
	require.NoError(t, err)

	assert.Equal(t, fromFull, fromCore, "precomputed path must produce identical output to BuildZkcInputs")
}

// referenceJSON mirrors printJson in the reference elf_to_json_gen tool
// (arithmetization/src/test/scripts/elf_to_json_gen/main.go): the JSON hex
// form of the ZkC pub inputs that zkc_util.ParseJsonInputFile decodes. Blob
// boundaries are separated by "____" (four underscores); offset and size
// within one entry use a single "_".
func referenceJSON(memBlobs []memoryBlob, entryPoint uint64) []byte {
	offsetSizeParts := make([]string, len(memBlobs))
	dataParts := make([]string, len(memBlobs))
	for i, b := range memBlobs {
		offsetSizeParts[i] = fmt.Sprintf("%016x_%016x", b.offset, len(b.data))
		dataParts[i] = hex.EncodeToString(b.data)
	}
	return []byte(`{` +
		`"entry_point_and_blobs_count":"0x` + fmt.Sprintf("%016x_%016x", entryPoint, len(memBlobs)) + `",` +
		`"blobs_offset_and_size":"0x` + strings.Join(offsetSizeParts, "____") + `",` +
		`"blobs_data":"0x` + strings.Join(dataParts, "____") + `"` +
		`}`)
}

// TestEncodeInputs_MatchesReferenceJSON pins encodeInputs to the reference
// wire format: building the map directly must be byte-identical to encoding
// the elf_to_json_gen JSON and decoding it with zkc's own parser.
func TestEncodeInputs_MatchesReferenceJSON(t *testing.T) {
	cases := []struct {
		name     string
		memBlobs []memoryBlob
		entry    uint64
	}{
		{"SingleBlob",
			[]memoryBlob{{offset: 0x00800000, data: []byte{0x97, 0x02, 0x00, 0x00}}},
			testEntry},
		{"MultipleBlobs",
			[]memoryBlob{
				{offset: 0x00800000, data: []byte{0xDE, 0xAD}},
				{offset: 0x00900000, data: []byte{0xBE, 0xEF, 0x01}},
				{offset: 0x00A00000, data: make([]byte, 0x123)},
			},
			0x00800150},
		{"ELFPlusSSZ",
			append(
				[]memoryBlob{{offset: testSecAddr, data: testSecData}},
				sszBlobs(DefaultINOrigin, []byte{0x00, 0x01, 0xAA, 0xBB})...),
			testEntry},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := zkc_util.ParseJsonInputFile(referenceJSON(tc.memBlobs, tc.entry))
			require.NoError(t, err)
			assert.Equal(t, want, encodeInputs(tc.memBlobs, tc.entry))
		})
	}
}

// TestBuildZkcInputs_NoLoadableSegments verifies that an ELF whose only
// segment is not PT_LOAD is rejected with a clear error rather than producing
// an empty blob set.
func TestBuildZkcInputs_NoLoadableSegments(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	// The program header starts at byte 64; its first field is p_type
	// (uint32 LE). Rewrite PT_LOAD (1) to PT_NOTE (4).
	binary.LittleEndian.PutUint32(elfBytes[64:68], 4)

	_, err := BuildZkcInputs(elfBytes, []byte{0x01}, DefaultINOrigin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no loadable sections")
}

// zkcTestBin is a small synthetic ZkC circuit checked in for the zkcdriver
// tests; NewZkCDriver accepts it, which is all Core.New needs.
const zkcTestBin = "../zkcdriver/testdata/zkc_01.bin"

// TestNew verifies that New precomputes the ELF blobs and entry point at
// construction and that the resulting Core builds the same inputs as the
// parse-every-call path.
func TestNew(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elfBytes, 0o600))

	c, err := New(Config{CircuitBinPath: zkcTestBin, GuestELFPath: elfPath})
	require.NoError(t, err)

	assert.Len(t, c.elfBlobs, 1, "one loadable section must be precomputed")
	assert.Equal(t, testEntry, c.elfEntry)

	ssz := []byte{0xAA, 0xBB}
	fromCore := c.buildInputs(Job{Payload: ssz})
	fromFull, err := BuildZkcInputs(elfBytes, ssz, DefaultINOrigin)
	require.NoError(t, err)
	assert.Equal(t, fromFull, fromCore)
}

// TestNew_Errors verifies that New reports missing or invalid startup inputs
// with errors naming the offending path.
func TestNew_Errors(t *testing.T) {
	elfBytes := makeMinimalELF(t, testEntry, testSecAddr, testSecData)
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elfBytes, 0o600))

	badELFPath := filepath.Join(t.TempDir(), "bad.elf")
	require.NoError(t, os.WriteFile(badELFPath, []byte("not an elf"), 0o600))

	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"MissingCircuitBin",
			Config{CircuitBinPath: "does/not/exist.bin", GuestELFPath: elfPath},
			"circuit bin"},
		{"MissingGuestELF",
			Config{CircuitBinPath: zkcTestBin, GuestELFPath: "does/not/exist.elf"},
			"guest ELF"},
		{"InvalidGuestELF",
			Config{CircuitBinPath: zkcTestBin, GuestELFPath: badELFPath},
			"ELF"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
