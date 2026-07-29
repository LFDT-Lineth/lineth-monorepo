package backend

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

// memoryBlob is an in-memory guest RAM region: a contiguous byte slice mapped
// at a specific address. Named to match the arith team's memoryBlob in
// elf_to_json_gen/main.go. Not related to EIP-4844 blobs.
type memoryBlob struct {
	offset uint64
	data   []byte
}

// buildZkcInputs constructs the map[string][]byte that [zkcdriver.PreReadInputs]
// expects. It produces the three pub-input keys that RISCV-ZKC.bin's main.zkc
// declares:
//
//   - "entry_point_and_blobs_count"
//   - "blobs_offset_and_size"
//   - "blobs_data"
//
// elfBytes is the raw guest ELF. sszInput is the framed StatelessInput the
// guest reads at _in_start: the 0x0001 schema id followed by the SSZ body
// (the guest's deserializer strips the schema id). buildZkcInputs prepends
// only the [u64 LE len] prefix; it neither adds nor strips the schema id.
// inOrigin is the guest RAM address where the input is placed
// (use [DefaultINOrigin]).
//
// This is a one-shot helper that parses the ELF on every call; today only
// tests use it. The per-job proving path does NOT go through here:
// [Core.buildInputs] reuses the guest ELF parsed once in [New] (c.elf) and
// appends only the per-job SSZ blobs.
func buildZkcInputs(elfBytes, sszInput []byte, inOrigin uint64) (map[string][]byte, error) {
	parsedELF, err := loadELFInputs(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, err
	}
	sszMemBlobs, err := sszBlobs(inOrigin, sszInput)
	if err != nil {
		return nil, err
	}
	memBlobs := append(parsedELF.blobs, sszMemBlobs...)
	return encodeInputs(memBlobs, parsedELF.entry), nil
}

// elfInputs is the ELF's precomputed contribution to the ZkC inputs: its
// loadable sections as memory blobs plus the entry point. Extracted once in
// [New] and reused for every job; only the per-job SSZ blobs differ.
type elfInputs struct {
	blobs []memoryBlob
	entry uint64
}

// loadELFInputs parses the guest ELF read from r and returns its memory blobs
// and entry point. r must stay valid until this returns; the section bytes are
// copied out, so the caller may close it afterward. Callers that process many
// jobs from the same ELF should call this once at startup and cache the result
// on [Core].
func loadELFInputs(r io.ReaderAt) (elfInputs, error) {
	ef, err := elf.NewFile(r)
	if err != nil {
		return elfInputs{}, fmt.Errorf("parsing guest ELF: %w", err)
	}
	blobs, err := elfBlobs(ef)
	if err != nil {
		return elfInputs{}, err
	}
	return elfInputs{blobs: blobs, entry: ef.Entry}, nil
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
		if p.Filesz > p.Memsz {
			return nil, fmt.Errorf("loadable segment at %#x has file size larger than memory size", p.Vaddr)
		}
		progEnd := p.Vaddr + p.Memsz
		if progEnd < p.Vaddr {
			return nil, fmt.Errorf("loadable segment address overflow at %#x", p.Vaddr)
		}

		var segBlobs []memoryBlob
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
// schema id + SSZ body, see [buildZkcInputs]). The split matches
// elf_to_json_gen's sszInputBlobs (commit 09fcdb42).
func sszBlobs(inOrigin uint64, ssz []byte) ([]memoryBlob, error) {
	payloadOffset := inOrigin + 8
	if payloadOffset < inOrigin {
		return nil, fmt.Errorf("SSZ input offset overflow: in_origin=%#x", inOrigin)
	}

	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(ssz)))
	memBlobs := []memoryBlob{{offset: inOrigin, data: prefix}}
	if len(ssz) > 0 {
		memBlobs = append(memBlobs, memoryBlob{offset: payloadOffset, data: ssz})
	}
	return memBlobs, nil
}

// encodeInputs builds the keyed byte map that [zkcdriver.PreReadInputs]
// expects, one entry per pub-input key:
//
//   - "entry_point_and_blobs_count": [8 BE entry point][8 BE blob count]
//   - "blobs_offset_and_size":       per blob, [8 BE offset][8 BE size]
//   - "blobs_data":                  all blob bytes concatenated
//
// The layout is intended to be byte-identical to zkc_util.ParseJsonInputFile
// applied to the JSON that the reference elf_to_json_gen tool emits, without the
// JSON round trip; see TestEncodeInputs_* in blobs_test.go for layout checks.
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
		entryPointAndBlobsCountKey: entryAndCount,
		blobsOffsetAndSizeKey:      offsetAndSize,
		blobsDataKey:               data,
	}
}
