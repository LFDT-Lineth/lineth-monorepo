package zkc_r5

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

const (
	entryPointAndBlobsCountKey = "entry_point_and_blobs_count"
	blobsOffsetAndSizeKey      = "blobs_offset_and_size"
	blobsDataKey               = "blobs_data"
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
// guest reads at _in_start, placed at [DefaultINOrigin].
func PrepareInput(guestElfBytes, guestInputData []byte) (map[string][]byte, error) {
	programSections, err := LoadGuestElf(bytes.NewReader(guestElfBytes))
	if err != nil {
		return nil, err
	}
	dataSections, err := NewDataSection(DefaultINOrigin, guestInputData)
	if err != nil {
		return nil, err
	}
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
		if p.Filesz > p.Memsz {
			return nil, fmt.Errorf("loadable segment at %#x has file size larger than memory size", p.Vaddr)
		}
		progEnd := p.Vaddr + p.Memsz
		if progEnd < p.Vaddr {
			return nil, fmt.Errorf("loadable segment address overflow at %#x", p.Vaddr)
		}

		var segBlobs []ElfSection
		for _, s := range ef.Sections {
			if s.Size == 0 || s.Type == elf.SHT_NOBITS || s.Flags&elf.SHF_ALLOC == 0 {
				continue
			}
			sectionEnd := s.Addr + s.Size
			if sectionEnd < s.Addr {
				return nil, fmt.Errorf("section %s address overflow at %#x", s.Name, s.Addr)
			}
			if s.Addr < p.Vaddr || sectionEnd > progEnd {
				continue
			}
			data, err := io.ReadAll(s.Open())
			if err != nil {
				return nil, fmt.Errorf("reading ELF section %s: %w", s.Name, err)
			}
			if uint64(len(data)) != s.Size {
				return nil, fmt.Errorf("short read for section %s: got %d bytes, expected %d", s.Name, len(data), s.Size)
			}
			segBlobs = append(segBlobs, ElfSection{Offset: s.Addr, Data: data})
		}

		sort.Slice(segBlobs, func(i, j int) bool {
			return segBlobs[i].Offset < segBlobs[j].Offset
		})
		result = append(result, segBlobs...)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("guest ELF has no loadable sections")
	}
	return result, nil
}

// NewDataSection splits data into the two memory blobs that linea_zkvm_io expects at
// _in_start: an 8-byte LE length prefix followed by the payload bytes. It does
// not interpret the payload.
func NewDataSection(inOrigin uint64, data []byte) ([]ElfSection, error) {
	payloadOffset := inOrigin + 8
	if payloadOffset < inOrigin {
		return nil, fmt.Errorf("data input offset overflow: in_origin=%#x", inOrigin)
	}
	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(data)))
	memBlobs := []ElfSection{{Offset: inOrigin, Data: prefix}}
	if len(data) > 0 {
		memBlobs = append(memBlobs, ElfSection{Offset: payloadOffset, Data: data})
	}
	return memBlobs, nil
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
		entryPointAndBlobsCountKey: entryAndCount,
		blobsOffsetAndSizeKey:      offsetAndSize,
		blobsDataKey:               data,
	}
}
