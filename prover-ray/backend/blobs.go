package backend

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	zkc_util "github.com/consensys/go-corset/pkg/zkc/util"
)

// blob is an in-memory guest RAM region: a contiguous byte slice mapped at a
// specific address.
type blob struct {
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
// elfBytes is the raw guest ELF. sszInput is the raw SSZ StatelessInput (not
// yet framed — BuildZkcInputs adds the [u64 LE len] prefix). inOrigin is the
// guest RAM address where the SSZ input is placed (use [DefaultINOrigin]).
func BuildZkcInputs(elfBytes, sszInput []byte, inOrigin uint64) (map[string][]byte, error) {
	ef, err := elf.NewFile(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing guest ELF: %w", err)
	}

	blobs := elfBlobs(ef)
	blobs = append(blobs, sszBlobs(inOrigin, sszInput)...)
	return encodeInputs(blobs, ef.Entry)
}

// elfBlobs extracts allocated, file-backed ELF sections as RAM blobs.
// SHT_NOBITS sections (.bss, padding) are omitted: guest RAM is zero-
// initialized before blob loading, so explicit zeros waste space.
func elfBlobs(ef *elf.File) []blob {
	var result []blob

	for _, p := range ef.Progs {
		if p.Type != elf.PT_LOAD || p.Memsz == 0 {
			continue
		}
		progEnd := p.Vaddr + p.Memsz

		var segBlobs []blob
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
				panic(fmt.Sprintf("reading ELF section %s: %v", s.Name, err))
			}
			segBlobs = append(segBlobs, blob{offset: s.Addr, data: data})
		}

		sort.Slice(segBlobs, func(i, j int) bool {
			return segBlobs[i].offset < segBlobs[j].offset
		})
		result = append(result, segBlobs...)
	}

	if len(result) == 0 {
		panic("guest ELF has no loadable sections")
	}
	return result
}

// sszBlobs splits ssz into the two blobs that linea_zkvm_io expects at
// _in_start: an 8-byte LE length prefix followed by the raw SSZ payload.
// The split matches elf_to_json_gen's sszInputBlobs (commit 09fcdb42).
func sszBlobs(inOrigin uint64, ssz []byte) []blob {
	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(ssz)))
	blobs := []blob{{offset: inOrigin, data: prefix}}
	if len(ssz) > 0 {
		blobs = append(blobs, blob{offset: inOrigin + 8, data: ssz})
	}
	return blobs
}

// encodeInputs encodes blobs into the JSON hex format that
// zkc_util.ParseJsonInputFile decodes. The format is identical to what
// elf_to_json_gen/main.go produces, ensuring compatibility with the
// existing zkcdriver input reader.
func encodeInputs(blobs []blob, entryPoint uint64) (map[string][]byte, error) {
	offsetSizeParts := make([]string, len(blobs))
	dataParts := make([]string, len(blobs))
	for i, b := range blobs {
		offsetSizeParts[i] = fmt.Sprintf("%016x_%016x", b.offset, len(b.data))
		dataParts[i] = hex.EncodeToString(b.data)
	}

	// Blob boundaries are separated by "____" (four underscores); individual
	// fields within one entry use a single "_" as a visual separator.
	json := []byte(`{` +
		`"entry_point_and_blobs_count":"0x` + fmt.Sprintf("%016x_%016x", entryPoint, len(blobs)) + `",` +
		`"blobs_offset_and_size":"0x` + strings.Join(offsetSizeParts, "____") + `",` +
		`"blobs_data":"0x` + strings.Join(dataParts, "____") + `"` +
		`}`)

	return zkc_util.ParseJsonInputFile(json)
}
