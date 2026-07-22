package backend

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"sort"
)

// memoryBlob is an in-memory guest RAM region: a contiguous byte slice mapped
// at a specific address. Named to match the arith team's memoryBlob in
// elf_to_json_gen/main.go. Not related to EIP-4844 blobs.
type memoryBlob struct {
	offset uint64
	data   []byte
}

// BuildZkcInputs constructs the map[string][]byte that [zkcdriver.PreReadInputs]
// expects. It produces the three pub-input keys that RISCV-ZKC.bin's main.zkc
// declares:
//
//   - "entry_point_and_blobs_count"
//   - "blobs_offset_and_size"
//   - "blobs_data"
//
// elfBytes is the raw guest ELF. sszInput is the framed StatelessInput the
// guest reads at _in_start: the 0x0001 schema id followed by the SSZ body
// (the guest's deserializer strips the schema id). BuildZkcInputs prepends
// only the [u64 LE len] prefix; it neither adds nor strips the schema id.
// inOrigin is the guest RAM address where the input is placed
// (use [DefaultINOrigin]).
func BuildZkcInputs(elfBytes, sszInput []byte, inOrigin uint64) (map[string][]byte, error) {
	memBlobs, entry, err := loadELFBlobs(elfBytes)
	if err != nil {
		return nil, err
	}
	memBlobs = append(memBlobs, sszBlobs(inOrigin, sszInput)...)
	return encodeInputs(memBlobs, entry), nil
}

// loadELFBlobs parses elfBytes as a guest ELF and returns the pre-extracted
// memory blobs and entry point. Callers that process many jobs from the same ELF
// should call this once at startup and cache the result on [Core].
func loadELFBlobs(elfBytes []byte) (memBlobs []memoryBlob, entry uint64, err error) {
	ef, err := elf.NewFile(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("parsing guest ELF: %w", err)
	}
	memBlobs, err = elfBlobs(ef)
	if err != nil {
		return nil, 0, err
	}
	return memBlobs, ef.Entry, nil
}

// elfBlobs extracts allocated, file-backed ELF sections as memory blobs.
// SHT_NOBITS sections (.bss, padding) are omitted: guest RAM is zero-
// initialized before memory blob loading, so explicit zeros waste space.
func elfBlobs(ef *elf.File) ([]memoryBlob, error) {
	var result []memoryBlob

	for _, p := range ef.Progs {
		if p.Type != elf.PT_LOAD || p.Memsz == 0 {
			continue
		}
		progEnd := p.Vaddr + p.Memsz

		var segBlobs []memoryBlob
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
			segBlobs = append(segBlobs, memoryBlob{offset: s.Addr, data: data})
		}

		sort.Slice(segBlobs, func(i, j int) bool {
			return segBlobs[i].offset < segBlobs[j].offset
		})
		result = append(result, segBlobs...)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("guest ELF has no loadable sections")
	}
	return result, nil
}

// sszBlobs splits ssz into the two memory blobs that linea_zkvm_io expects at
// _in_start: an 8-byte LE length prefix followed by the payload bytes. It does
// not interpret the payload; callers pass the framed StatelessInput (0x0001
// schema id + SSZ body, see [BuildZkcInputs]). The split matches
// elf_to_json_gen's sszInputBlobs (commit 09fcdb42).
func sszBlobs(inOrigin uint64, ssz []byte) []memoryBlob {
	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(ssz)))
	memBlobs := []memoryBlob{{offset: inOrigin, data: prefix}}
	if len(ssz) > 0 {
		memBlobs = append(memBlobs, memoryBlob{offset: inOrigin + 8, data: ssz})
	}
	return memBlobs
}

// encodeInputs builds the keyed byte map that [zkcdriver.PreReadInputs]
// expects, one entry per pub-input key:
//
//   - "entry_point_and_blobs_count": [8 BE entry point][8 BE blob count]
//   - "blobs_offset_and_size":       per blob, [8 BE offset][8 BE size]
//   - "blobs_data":                  all blob bytes concatenated
//
// The layout is byte-identical to zkc_util.ParseJsonInputFile applied to the
// JSON that the reference elf_to_json_gen tool emits, without the JSON round
// trip; TestEncodeInputs_MatchesReferenceJSON pins the equivalence.
func encodeInputs(memBlobs []memoryBlob, entryPoint uint64) map[string][]byte {
	entryAndCount := binary.BigEndian.AppendUint64(make([]byte, 0, 16), entryPoint)
	entryAndCount = binary.BigEndian.AppendUint64(entryAndCount, uint64(len(memBlobs)))

	var dataLen int
	for _, b := range memBlobs {
		dataLen += len(b.data)
	}
	offsetAndSize := make([]byte, 0, 16*len(memBlobs))
	data := make([]byte, 0, dataLen)
	for _, b := range memBlobs {
		offsetAndSize = binary.BigEndian.AppendUint64(offsetAndSize, b.offset)
		offsetAndSize = binary.BigEndian.AppendUint64(offsetAndSize, uint64(len(b.data)))
		data = append(data, b.data...)
	}

	return map[string][]byte{
		"entry_point_and_blobs_count": entryAndCount,
		"blobs_offset_and_size":       offsetAndSize,
		"blobs_data":                  data,
	}
}
