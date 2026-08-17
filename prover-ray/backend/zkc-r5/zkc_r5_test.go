package zkcr5_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	zkcr5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/internal/minimal-elf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSection_LengthPrefixAtInOrigin(t *testing.T) {
	data := []byte{0xAA, 0xBB, 0xCC}
	got, err := zkcr5.NewDataSection(zkcr5.DefaultINOrigin, data)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// First memory blob: 8-byte LE length at inOrigin.
	assert.Equal(t, zkcr5.DefaultINOrigin, got[0].Offset, "first memory blob offset must be inOrigin")
	require.Len(t, got[0].Data, 8, "length prefix must be exactly 8 bytes")
	assert.Equal(t, uint64(3), binary.LittleEndian.Uint64(got[0].Data), "length prefix must encode payload length as LE uint64")
}

func TestDataSection_PayloadAtInOriginPlus8(t *testing.T) {
	payload := []byte{0xAA, 0xBB, 0xCC}
	got, err := zkcr5.NewDataSection(zkcr5.DefaultINOrigin, payload)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Second memory blob: the payload bytes at inOrigin+8.
	assert.Equal(t, zkcr5.DefaultINOrigin+8, got[1].Offset, "payload memory blob offset must be inOrigin+8")
	assert.Equal(t, payload, got[1].Data, "payload memory blob must contain the payload bytes verbatim")
}

func TestDataSection_Empty(t *testing.T) {
	// Empty data: only the 8-byte length memory blob, no payload memory blob.
	got, err := zkcr5.NewDataSection(zkcr5.DefaultINOrigin, nil)
	require.NoError(t, err)
	require.Len(t, got, 1, "empty data must produce exactly one memory blob (length prefix only)")
	assert.Equal(t, zkcr5.DefaultINOrigin, got[0].Offset, "length prefix memory blob offset must be inOrigin")
	assert.Equal(t, uint64(0), binary.LittleEndian.Uint64(got[0].Data), "length prefix must be zero for empty data")
}

func TestDataSection_OffsetOverflow(t *testing.T) {
	_, err := zkcr5.NewDataSection(math.MaxUint64-4, []byte{0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data input offset overflow")
	assert.Contains(t, err.Error(), "in_origin=0xfffffffffffffffb")
}

func TestEncodeInputs_EntryPointAndCount(t *testing.T) {
	const entryPoint = 0x11223344
	inputs, err := zkcr5.EncodeGuestAndMemoryForZkc(
		zkcr5.GuestProgramSections{
			EntryPoint: entryPoint,
		},
		[]zkcr5.ElfSection{
			{Offset: 0x1000, Data: []byte{0x01}},
			{Offset: 0x2000, Data: []byte{0x02}},
		},
	)
	require.NoError(t, err)

	got := inputs["entry_point_and_blobs_count"]
	require.Len(t, got, 16, "entry_point_and_blobs_count must be two BE uint64s")
	assert.Equal(t, uint64(entryPoint), binary.BigEndian.Uint64(got[:8]), "first 8 bytes must be the BE entry point")
	assert.Equal(t, uint64(2), binary.BigEndian.Uint64(got[8:]), "last 8 bytes must be the BE blob count")
}

func TestEncodeInputs_OffsetAndSizePairs(t *testing.T) {
	memBlobs := []zkcr5.ElfSection{
		{Offset: 0x00800000, Data: make([]byte, 0x1234)},
		{Offset: 0x2000, Data: []byte{0xBB}},
	}
	inputs, err := zkcr5.EncodeGuestAndMemoryForZkc(zkcr5.GuestProgramSections{}, memBlobs)
	require.NoError(t, err)

	got := inputs["blobs_offset_and_size"]
	require.Len(t, got, 32, "blobs_offset_and_size must be one 16-byte pair per blob")
	assert.Equal(t, uint64(0x2000), binary.BigEndian.Uint64(got[0:8]))
	assert.Equal(t, uint64(1), binary.BigEndian.Uint64(got[8:16]))
	assert.Equal(t, uint64(0x00800000), binary.BigEndian.Uint64(got[16:24]))
	assert.Equal(t, uint64(0x1234), binary.BigEndian.Uint64(got[24:32]))
}

func TestEncodeInputs_DataConcatenated(t *testing.T) {
	memBlobs := []zkcr5.ElfSection{
		{Offset: 0x1000, Data: []byte{0xAA, 0xBB}},
		{Offset: 0x2000, Data: []byte{0xCC}},
	}
	inputs, err := zkcr5.EncodeGuestAndMemoryForZkc(zkcr5.GuestProgramSections{}, memBlobs)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, inputs["blobs_data"], "blobs_data must be all blob bytes concatenated in order")
}

func TestEncodeInputs_SortsCombinedSectionsWithoutMutatingInputs(t *testing.T) {
	sections := make([]zkcr5.ElfSection, 2, 4)
	sections[0] = zkcr5.ElfSection{Offset: 0x3000, Data: []byte{0x03}}
	sections[1] = zkcr5.ElfSection{Offset: 0x1000, Data: []byte{0x01}}
	guestSections := zkcr5.GuestProgramSections{
		Sections: sections,
	}
	memory := []zkcr5.ElfSection{
		{Offset: 0x2000, Data: []byte{0x02}},
		{Offset: 0x4000, Data: []byte{0x04}},
	}

	inputs, err := zkcr5.EncodeGuestAndMemoryForZkc(guestSections, memory)
	require.NoError(t, err)

	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, inputs["blobs_data"])
	assert.Equal(t, uint64(0x3000), guestSections.Sections[0].Offset)
	assert.Equal(t, uint64(0x1000), guestSections.Sections[1].Offset)
	assert.Equal(t, uint64(0x2000), memory[0].Offset)
	assert.Equal(t, uint64(0x4000), memory[1].Offset)
}

func TestEncodeInputs_RejectsInvalidAddressRanges(t *testing.T) {
	tests := []struct {
		name          string
		guestSections zkcr5.GuestProgramSections
		memory        []zkcr5.ElfSection
		wantErr       string
	}{
		{
			name: "overlapping guest and memory sections",
			guestSections: zkcr5.GuestProgramSections{
				Sections: []zkcr5.ElfSection{{Offset: 0x2000, Data: []byte{0x01, 0x02}}},
			},
			memory:  []zkcr5.ElfSection{{Offset: 0x2001, Data: []byte{0x03}}},
			wantErr: "overlaps",
		},
		{
			name:    "overlapping memory sections",
			memory:  []zkcr5.ElfSection{{Offset: 0x2001, Data: []byte{0x01}}, {Offset: 0x2000, Data: []byte{0x02, 0x03}}},
			wantErr: "overlaps",
		},
		{
			name:    "address range overflow",
			memory:  []zkcr5.ElfSection{{Offset: math.MaxUint64, Data: []byte{0x01}}},
			wantErr: "overflows address space",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputs, err := zkcr5.EncodeGuestAndMemoryForZkc(test.guestSections, test.memory)
			require.Error(t, err)
			assert.Nil(t, inputs)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestEncodeInputs_AllowsAdjacentAndEmptySections(t *testing.T) {
	inputs, err := zkcr5.EncodeGuestAndMemoryForZkc(
		zkcr5.GuestProgramSections{
			Sections: []zkcr5.ElfSection{
				{Offset: 0x1000, Data: []byte{0x01, 0x02}},
				{Offset: 0x1002, Data: nil},
			},
		},
		[]zkcr5.ElfSection{{Offset: 0x1002, Data: []byte{0x03}}},
	)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, inputs["blobs_data"])
}

func TestElfBlobs_ExtractsSectionAtCorrectOffset(t *testing.T) {
	parsedELF, err := zkcr5.LoadGuestElf(bytes.NewReader(minimalelf.MinimalElfProgram))
	require.NoError(t, err)
	require.Len(t, parsedELF.Sections, 1, "one loadable section must yield one memory blob")
	assert.Equal(t, uint64(minimalelf.DefaultSectionAddr), parsedELF.Sections[0].Offset, "memory blob offset must match the section's virtual address")
	assert.Equal(t, minimalelf.ValidSectionData, parsedELF.Sections[0].Data, "memory blob data must match the section bytes")
}

func TestElfBlobs_EntryPointPreserved(t *testing.T) {
	parsedELF, err := zkcr5.LoadGuestElf(bytes.NewReader(minimalelf.MinimalElfProgram))
	require.NoError(t, err)
	assert.Equal(t, uint64(minimalelf.DefaultEntryPoint), parsedELF.EntryPoint, "entry point must match ELF e_entry")
}

func TestElfBlobs_InvalidELFReturnsError(t *testing.T) {
	_, err := zkcr5.LoadGuestElf(bytes.NewReader([]byte("not an elf")))
	assert.Error(t, err, "malformed ELF must return an error")
}

func TestBuildZkcInputs_ReturnsThreeKeys(t *testing.T) {
	// These three key names are declared in RISCV-ZKC.bin's main.zkc. A name
	// mismatch causes a silent no-op when the prover loads inputs.
	inputs, err := zkcr5.PrepareInput(minimalelf.MinimalElfProgram, []byte{0x01, 0x02})
	require.NoError(t, err)

	assert.Contains(t, inputs, "entry_point_and_blobs_count")
	assert.Contains(t, inputs, "blobs_offset_and_size")
	assert.Contains(t, inputs, "blobs_data")
}

// TestBuildZkcInputs_NoLoadableSegments verifies that an ELF whose only
// segment is not PT_LOAD is rejected with a clear error rather than producing
// an empty blob set.
func TestBuildZkcInputs_NoLoadableSegments(t *testing.T) {
	elfBytes := minimalelf.Make(minimalelf.DefaultEntryPoint, minimalelf.DefaultSectionAddr, minimalelf.ValidSectionData)
	// The program header starts at byte 64; its first field is p_type
	// (uint32 LE). Rewrite PT_LOAD (1) to PT_NOTE (4).
	binary.LittleEndian.PutUint32(elfBytes[64:68], 4)

	_, err := zkcr5.PrepareInput(elfBytes, []byte{0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no loadable sections")
}

func TestBuildZkcInputs_LoadableSegmentFileSizeExceedsMemSize(t *testing.T) {
	elfBytes := minimalelf.Make(minimalelf.DefaultEntryPoint, minimalelf.DefaultSectionAddr, minimalelf.ValidSectionData)
	// Program header starts at byte 64. p_filesz is at +32, p_memsz at +40.
	binary.LittleEndian.PutUint64(elfBytes[64+32:64+40], 2)
	binary.LittleEndian.PutUint64(elfBytes[64+40:64+48], 1)

	_, err := zkcr5.PrepareInput(elfBytes, []byte{0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file size larger than memory size")
}

func TestBuildZkcInputs_LoadableSegmentAddressOverflow(t *testing.T) {
	elfBytes := minimalelf.Make(minimalelf.DefaultEntryPoint, minimalelf.DefaultSectionAddr, minimalelf.ValidSectionData)
	// Program header starts at byte 64. p_vaddr is at +16, p_memsz at +40.
	binary.LittleEndian.PutUint64(elfBytes[64+16:64+24], math.MaxUint64-1)
	binary.LittleEndian.PutUint64(elfBytes[64+40:64+48], 4)

	_, err := zkcr5.PrepareInput(elfBytes, []byte{0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "segment address overflow")
}

func TestBuildZkcInputs_SectionAddressOverflow(t *testing.T) {
	elfBytes := minimalelf.Make(minimalelf.DefaultEntryPoint, minimalelf.DefaultSectionAddr, minimalelf.ValidSectionData)
	shOff := binary.LittleEndian.Uint64(elfBytes[40:48])
	textSection := shOff + 64 // section 0 is NULL, section 1 is .text
	// Section header field offsets: sh_addr at +16, sh_size at +32.
	binary.LittleEndian.PutUint64(elfBytes[textSection+16:textSection+24], math.MaxUint64-1)
	binary.LittleEndian.PutUint64(elfBytes[textSection+32:textSection+40], 4)

	_, err := zkcr5.PrepareInput(elfBytes, []byte{0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "section .text address overflow")
}

func TestBuildZkcInputs_SectionShortRead(t *testing.T) {
	elfBytes := minimalelf.Make(minimalelf.DefaultEntryPoint, minimalelf.DefaultSectionAddr, minimalelf.ValidSectionData)
	shOff := binary.LittleEndian.Uint64(elfBytes[40:48])
	textSection := shOff + 64 // section 0 is NULL, section 1 is .text
	// Section header field offsets: sh_offset at +24, sh_size at +32.
	binary.LittleEndian.PutUint64(elfBytes[textSection+24:textSection+32], uint64(len(elfBytes)-2))
	binary.LittleEndian.PutUint64(elfBytes[textSection+32:textSection+40], 4)

	_, err := zkcr5.PrepareInput(elfBytes, []byte{0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "short read for section .text")
}
