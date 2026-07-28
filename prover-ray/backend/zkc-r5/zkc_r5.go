package zkc_r5

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

// ElfSection is an in-memory guest RAM region: a contiguous byte slice mapped
// at a specific address.
type ElfSection struct {
	Offset uint64
	Data   []byte
}

// PrepareInput constructs the map[string][]byte that [zkcdriver.PreReadInputs]
// expects. It produces the three pub-input keys that RISCV-ZKC.bin's main.zkc
// declares:
//
//   - "entry_point_and_blobs_count"
//   - "blobs_offset_and_size"
//   - "blobs_data"
//
// guestElfBytes is the raw guest ELF. guestInputData is the input data the
// guest reads at _in_start. guestInputDataOrigin is the guest RAM address where
// the input is placed (use [DefaultINOrigin]).
func PrepareInput(guestElfBytes, guestInputData []byte, guestInputDataOrigin uint64) (map[string][]byte, error) {
	programSections, err := LoadGuestElf(bytes.NewReader(guestElfBytes))
	if err != nil {
		return nil, err
	}
	dataSections := NewDataSection(guestInputDataOrigin, guestInputData)
	return EncodeGuestAndMemoryForZkc(programSections, dataSections), nil
}

// GuestProgramSections is the ELF's precomputed contribution to the ZkC inputs: its
// loadable sections as memory blobs plus the entry point.
type GuestProgramSections struct {
	Sections   []ElfSection
	EntryPoint uint64
}

// LoadGuestElf parses the guest ELF read from r and returns its memory blobs
// and entry point. r must stay valid until this returns; the section bytes are
// copied out, so the caller may close it afterward. Callers that process many
// jobs from the same ELF should call this once at startup and cache the result
// on [Core].
func LoadGuestElf(r io.ReaderAt) (GuestProgramSections, error) {
	ef, err := elf.NewFile(r)
	if err != nil {
		return GuestProgramSections{}, fmt.Errorf("parsing guest ELF: %w", err)
	}
	blobs, err := extractElfSections(ef)
	if err != nil {
		return GuestProgramSections{}, err
	}
	return GuestProgramSections{Sections: blobs, EntryPoint: ef.Entry}, nil
}

// extractElfSections extracts allocated, file-backed ELF sections as memory
// blobs. SHT_NOBITS sections (.bss, padding) are omitted: guest RAM is zero-
// initialized before memory blob loading, so explicit zeros waste space.
func extractElfSections(ef *elf.File) ([]ElfSection, error) {
	var result []ElfSection

	for _, p := range ef.Progs {
		if p.Type != elf.PT_LOAD || p.Memsz == 0 {
			continue
		}
		progEnd := p.Vaddr + p.Memsz

		var segmentSections []ElfSection
		for _, s := range ef.Sections {
			if s.Size == 0 || s.Type == elf.SHT_NOBITS || s.Flags&elf.SHF_ALLOC == 0 {
				continue
			}
			sectionEnd := s.Addr + s.Size
			if s.Addr < p.Vaddr || sectionEnd > progEnd {
				continue
			}
			data, err := s.Data()
			if err != nil {
				return nil, fmt.Errorf("reading ELF section %s: %w", s.Name, err)
			}
			segmentSections = append(segmentSections, ElfSection{Offset: s.Addr, Data: data})
		}

		sort.Slice(segmentSections, func(i, j int) bool {
			return segmentSections[i].Offset < segmentSections[j].Offset
		})
		result = append(result, segmentSections...)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("guest ELF has no loadable sections")
	}
	return result, nil
}

// NewDataSection splits data into the two memory blobs that linea_zkvm_io expects at
// _in_start: an 8-byte LE length prefix followed by the payload bytes. It does
// not interpret the payload.
func NewDataSection(inOrigin uint64, data []byte) []ElfSection {
	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(data)))
	memBlobs := []ElfSection{{Offset: inOrigin, Data: prefix}}
	if len(data) > 0 {
		memBlobs = append(memBlobs, ElfSection{Offset: inOrigin + 8, Data: data})
	}
	return memBlobs
}

// EncodeGuestAndMemoryForZkc builds the keyed byte map that
// [zkcdriver.PreReadInputs] expects, one entry per pub-input key:
//
//   - "entry_point_and_blobs_count": [8 BE entry point][8 BE blob count]
//   - "blobs_offset_and_size":       per blob, [8 BE offset][8 BE size]
//   - "blobs_data":                  all blob bytes concatenated
//
// guestSections is the ELF's loadable sections, and memory is any additional
// memory blobs (e.g. the framed StatelessInput).
//
// The caller must ensure that the guestSections and memory slices are sorted by
// offset and that the blobs do not overlap. It is ensured by calling
// [PrepareInput] directly on the guest ELF and raw input data.
func EncodeGuestAndMemoryForZkc(guestSections GuestProgramSections, memory []ElfSection) map[string][]byte {
	entryAndCount := binary.BigEndian.AppendUint64(make([]byte, 0, 16), guestSections.EntryPoint)
	entryAndCount = binary.BigEndian.AppendUint64(entryAndCount, uint64(len(guestSections.Sections)+len(memory)))

	var dataLen int
	allSections := append(guestSections.Sections, memory...)
	for _, b := range allSections {
		dataLen += len(b.Data)
	}
	offsetAndSize := make([]byte, 0, 16*len(allSections))
	data := make([]byte, 0, dataLen)
	for _, b := range allSections {
		offsetAndSize = binary.BigEndian.AppendUint64(offsetAndSize, b.Offset)
		offsetAndSize = binary.BigEndian.AppendUint64(offsetAndSize, uint64(len(b.Data)))
		data = append(data, b.Data...)
	}

	return map[string][]byte{
		"entry_point_and_blobs_count": entryAndCount,
		"blobs_offset_and_size":       offsetAndSize,
		"blobs_data":                  data,
	}
}
