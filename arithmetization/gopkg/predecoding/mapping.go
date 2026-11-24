package predecoding

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
)

const (
	// EntryPointAndBlobsCountInput is the public input containing the ELF entry
	// point followed by the number of sparse memory blobs.
	EntryPointAndBlobsCountInput = "entry_point_and_blobs_count"
	// BlobsOffsetAndSizeInput is the public input containing each blob's address
	// and size.
	BlobsOffsetAndSizeInput = "blobs_offset_and_size"
	// BlobsExecutableInput marks which sparse blobs contain executable bytes.
	BlobsExecutableInput = "blobs_executable"
	// BlobsDataInput is the concatenation of the sparse blob data.
	BlobsDataInput = "blobs_data"

	// DefaultInputOrigin is the _in_start address used by Lineth R5 guests.
	DefaultInputOrigin uint64 = 0x08800000
)

// Blob is a contiguous region mapped into guest memory.
type Blob struct {
	Address uint64
	Data    []byte
	// Name is used only by diagnostic sections output.
	Name       string
	Executable bool
}

// Program is the contribution of an ELF to the R5 inputs. Blob data returned
// by [Load] is owned by Program and remains valid after the reader is closed.
type Program struct {
	EntryPoint uint64
	Blobs      []Blob
}

type dataConfig struct {
	name         string
	lengthPrefix bool
}

// DataOption configures blobs created by [NewData].
type DataOption func(*dataConfig) error

// WithName assigns a descriptive blob name. Length-prefixed data uses the name
// with _length and _payload suffixes. Names are used only by diagnostic
// sections output and do not affect R5 inputs.
func WithName(name string) DataOption {
	return func(cfg *dataConfig) error {
		if name == "" {
			return fmt.Errorf("blob name is empty")
		}
		cfg.name = name
		return nil
	}
}

// WithLengthPrefix stores an eight-byte little-endian data length before the
// payload. Guests that read raw bytes directly must not use this option.
func WithLengthPrefix() DataOption {
	return func(cfg *dataConfig) error {
		cfg.lengthPrefix = true
		return nil
	}
}

// Load parses a guest ELF and copies its allocated, file-backed sections into
// sparse memory blobs. Zero-filled SHT_NOBITS sections are omitted because R5
// memory is initialized to zero.
func Load(r io.ReaderAt) (Program, error) {
	elfFile, err := elf.NewFile(r)
	if err != nil {
		return Program{}, fmt.Errorf("parsing guest ELF: %w", err)
	}

	blobs, err := extractBlobs(elfFile)
	if err != nil {
		return Program{}, err
	}
	return Program{EntryPoint: elfFile.Entry, Blobs: blobs}, nil
}

// loadableSegments returns the PT_LOAD segments with non-zero memory size.
func loadableSegments(elfFile *elf.File) []*elf.Prog {
	var segments []*elf.Prog
	for _, programHeader := range elfFile.Progs {
		if programHeader.Type == elf.PT_LOAD && programHeader.Memsz != 0 {
			segments = append(segments, programHeader)
		}
	}
	return segments
}

// segmentEnd validates a loadable segment and returns the end of its memory
// range. It rejects a file size larger than the memory size, and a memory range
// that overflows the address space.
func segmentEnd(programHeader *elf.Prog) (uint64, error) {
	if programHeader.Filesz > programHeader.Memsz {
		return 0, fmt.Errorf(
			"loadable segment at %#x has file size larger than memory size",
			programHeader.Vaddr,
		)
	}
	end := programHeader.Vaddr + programHeader.Memsz
	if end < programHeader.Vaddr {
		return 0, fmt.Errorf(
			"loadable segment address overflow at %#x",
			programHeader.Vaddr,
		)
	}
	return end, nil
}

// sectionEnd returns the end of the address range of a section. It rejects a
// range that overflows the address space.
func sectionEnd(section *elf.Section) (uint64, error) {
	end := section.Addr + section.Size
	if end < section.Addr {
		return 0, fmt.Errorf(
			"section %s address overflow at %#x",
			section.Name,
			section.Addr,
		)
	}
	return end, nil
}

// isWithin reports whether the range [start, end) lies fully inside the range
// [outerStart, outerEnd). A section outside the current segment is left to the
// segment that contains it.
func isWithin(start, end, outerStart, outerEnd uint64) bool {
	return start >= outerStart && end <= outerEnd
}

// isLoadableSection reports whether a section puts bytes into memory. Empty
// sections are skipped, and so are SHT_NOBITS sections such as .bss (R5 memory
// starts zeroed) and sections without SHF_ALLOC such as debug info and the
// symbol table.
func isLoadableSection(section *elf.Section) bool {
	return section.Size != 0 &&
		section.Type != elf.SHT_NOBITS &&
		section.Flags&elf.SHF_ALLOC != 0
}

func extractBlobs(elfFile *elf.File) ([]Blob, error) {
	var blobs []Blob
	for _, programHeader := range loadableSegments(elfFile) {
		programEnd, err := segmentEnd(programHeader)
		if err != nil {
			return nil, err
		}

		var programBlobs []Blob
		for _, section := range elfFile.Sections {
			if !isLoadableSection(section) {
				continue
			}
			end, err := sectionEnd(section)
			if err != nil {
				return nil, err
			}
			if !isWithin(section.Addr, end, programHeader.Vaddr, programEnd) {
				continue
			}

			data, err := readSection(section)
			if err != nil {
				return nil, err
			}
			programBlobs = append(programBlobs, buildBlob(section, data))
		}

		sortBlobsByAddress(programBlobs)
		blobs = append(blobs, programBlobs...)
	}
	if len(blobs) == 0 {
		return nil, fmt.Errorf("guest ELF has no loadable sections")
	}
	return blobs, nil
}

// sortBlobsByAddress orders blobs by ascending address, in place.
func sortBlobsByAddress(blobs []Blob) {
	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].Address < blobs[j].Address
	})
}

// readSection copies the bytes of a section out of the ELF. It fails when the
// read fails or returns fewer bytes than the section size.
func readSection(section *elf.Section) ([]byte, error) {
	data, err := io.ReadAll(section.Open())
	if err != nil {
		return nil, fmt.Errorf("reading ELF section %s: %w", section.Name, err)
	}
	if uint64(len(data)) != section.Size {
		return nil, fmt.Errorf(
			"short read for section %s: got %d bytes, expected %d",
			section.Name,
			len(data),
			section.Size,
		)
	}
	return data, nil
}

// buildBlob builds the blob of a section from its virtual address, its bytes
// and its name. The blob is executable when the section has SHF_EXECINSTR.
func buildBlob(section *elf.Section, data []byte) Blob {
	return Blob{
		Address:    section.Addr,
		Data:       data,
		Name:       section.Name,
		Executable: section.Flags&elf.SHF_EXECINSTR != 0,
	}
}

// NewData maps data at the given address. WithLengthPrefix stores an eight-byte
// little-endian length at that address and the payload eight bytes later. The
// R5 interpreter itself does not require this framing. The payload is not
// copied until [EncodeInputs] constructs the final input map.
func NewData(
	address uint64,
	data []byte,
	options ...DataOption,
) ([]Blob, error) {
	cfg, err := applyDataOptions(options)
	if err != nil {
		return nil, err
	}
	if !cfg.lengthPrefix {
		return rawBlobs(address, data, cfg.name), nil
	}
	return lengthPrefixedBlobs(address, data, cfg.name)
}

// applyDataOptions builds the data config from the options. A nil option and a
// failing option are errors.
func applyDataOptions(options []DataOption) (dataConfig, error) {
	cfg := dataConfig{}
	for _, option := range options {
		if option == nil {
			return dataConfig{}, fmt.Errorf("applying data option: nil option")
		}
		if err := option(&cfg); err != nil {
			return dataConfig{}, fmt.Errorf("applying data option: %w", err)
		}
	}
	return cfg, nil
}

// rawBlobs maps data as one blob at address, or no blob when data is empty.
func rawBlobs(address uint64, data []byte, name string) []Blob {
	if len(data) == 0 {
		return nil
	}
	return []Blob{{Address: address, Data: data, Name: name}}
}

// lengthPrefixedBlobs maps an eight-byte little-endian length at address and
// the payload eight bytes later. An empty payload gives only the length blob.
// A name gets the suffixes _length and _payload. It rejects an address whose
// payload address overflows.
func lengthPrefixedBlobs(address uint64, data []byte, name string) ([]Blob, error) {
	payloadAddr, err := payloadAddress(address)
	if err != nil {
		return nil, err
	}

	blobs := []Blob{newBlob(address, lengthPrefix(len(data)), name, "_length")}
	if len(data) != 0 {
		blobs = append(blobs, newBlob(payloadAddr, data, name, "_payload"))
	}
	return blobs, nil
}

// newBlob builds a non-executable blob at address, named after name and suffix.
// The blob stays unnamed when name is empty.
func newBlob(address uint64, data []byte, name, suffix string) Blob {
	return Blob{Address: address, Data: data, Name: suffixedName(name, suffix)}
}

// lengthPrefixSize is the size in bytes of the length prefix.
const lengthPrefixSize = 8

// payloadAddress returns the address of the payload, eight bytes after the
// length prefix at address. It fails when that address overflows.
func payloadAddress(address uint64) (uint64, error) {
	payloadAddr := address + lengthPrefixSize
	if payloadAddr < address {
		return 0, fmt.Errorf("data input offset overflow: in_origin=%#x", address)
	}
	return payloadAddr, nil
}

// lengthPrefix encodes a payload length as an eight-byte little-endian prefix.
func lengthPrefix(length int) []byte {
	prefix := make([]byte, lengthPrefixSize)
	binary.LittleEndian.PutUint64(prefix, uint64(length))
	return prefix
}

// suffixedName appends suffix to name, and keeps an empty name empty.
func suffixedName(name, suffix string) string {
	if name == "" {
		return ""
	}
	return name + suffix
}

// EncodeInputs encodes a program and additional memory blobs into raw R5
// public-input bytes. It does not mutate either input and returned byte slices
// do not alias blob data. The options are the mapping options
// [WithIncludeExecutable] and [WithSectionsWriter].
func EncodeInputs(
	program Program,
	additional []Blob,
	options ...Option,
) (map[string]bit.Array, error) {
	cfg, err := applyInputOptions(options)
	if err != nil {
		return nil, err
	}
	return encodeInputs(program, additional, cfg)
}

// applyInputOptions builds the encoding config from the options. A nil option
// and a failing option are errors.
func applyInputOptions(options []Option) (config, error) {
	cfg := config{}
	for _, option := range options {
		if option == nil {
			return config{}, fmt.Errorf("applying input option: nil option")
		}
		if err := option(&cfg); err != nil {
			return config{}, fmt.Errorf("applying input option: %w", err)
		}
	}
	return cfg, nil
}

func encodeInputs(program Program, additional []Blob, cfg config) (map[string]bit.Array, error) {
	blobs := mergeAndSortBlobs(program.Blobs, additional)
	if err := validateBlobsRanges(blobs); err != nil {
		return nil, err
	}
	if err := writeSectionsIfRequested(cfg.sectionsWriter, blobs); err != nil {
		return nil, err
	}

	entryPoint, addresses, dataSize, err := u32Layout(program.EntryPoint, blobs)
	if err != nil {
		return nil, err
	}

	entryAndCount := encodeEntryAndCount(entryPoint, len(blobs))
	offsetsAndSizes, data := encodeBlobTables(addresses, blobs, dataSize)

	inputs := buildInputs(entryAndCount, offsetsAndSizes, data)
	if cfg.includeExecutable {
		inputs[BlobsExecutableInput] = BitArrayFromBytes(encodeExecutableBits(blobs)...)
	}
	return inputs, nil
}

// encodeEntryAndCount encodes the entry point and the blob count as two
// big-endian u32 values.
func encodeEntryAndCount(entryPoint uint32, count int) []byte {
	entryAndCount := binary.BigEndian.AppendUint32(nil, entryPoint)
	return binary.BigEndian.AppendUint32(entryAndCount, uint32(count))
}

// encodeBlobTables encodes one big-endian address and size per blob, and the
// blob bytes concatenated in the same order.
func encodeBlobTables(addresses []uint32, blobs []Blob, dataSize int) ([]byte, []byte) {
	offsetsAndSizes := make([]byte, 0, 8*len(blobs))
	data := make([]byte, 0, dataSize)
	for i, blob := range blobs {
		offsetsAndSizes = binary.BigEndian.AppendUint32(offsetsAndSizes, addresses[i])
		offsetsAndSizes = binary.BigEndian.AppendUint32(
			offsetsAndSizes,
			uint32(len(blob.Data)),
		)
		data = append(data, blob.Data...)
	}
	return offsetsAndSizes, data
}

// buildInputs wraps the entry header, the blob offsets and sizes, and the blob
// data in bit arrays, keyed by their input names.
func buildInputs(entryAndCount, offsetsAndSizes, data []byte) map[string]bit.Array {
	return map[string]bit.Array{
		EntryPointAndBlobsCountInput: BitArrayFromBytes(entryAndCount...),
		BlobsOffsetAndSizeInput:      BitArrayFromBytes(offsetsAndSizes...),
		BlobsDataInput:               BitArrayFromBytes(data...),
	}
}

// mergeAndSortBlobs returns a new slice with the blobs of first followed by the
// blobs of second, ordered by ascending address. The sort is stable, so blobs
// with the same address keep their order. Neither input is modified.
func mergeAndSortBlobs(first, second []Blob) []Blob {
	blobs := make([]Blob, 0, len(first)+len(second))
	blobs = append(blobs, first...)
	blobs = append(blobs, second...)
	sort.SliceStable(blobs, func(i, j int) bool {
		return blobs[i].Address < blobs[j].Address
	})
	return blobs
}

// writeSectionsIfRequested writes the sections table when a writer is set. A
// nil writer writes nothing.
func writeSectionsIfRequested(writer io.Writer, blobs []Blob) error {
	if writer == nil {
		return nil
	}
	if err := writeSections(writer, blobs); err != nil {
		return fmt.Errorf("writing sections: %w", err)
	}
	return nil
}

func writeSections(writer io.Writer, blobs []Blob) error {
	if _, err := fmt.Fprintln(
		writer,
		"index, offset,             size,               exec, name",
	); err != nil {
		return err
	}
	for i, blob := range blobs {
		executable := "no"
		if blob.Executable {
			executable = "yes"
		}
		if _, err := fmt.Fprintf(
			writer,
			"%-5d, 0x%016x, 0x%016x, %-3s, %s\n",
			i,
			blob.Address,
			len(blob.Data),
			executable,
			blob.Name,
		); err != nil {
			return err
		}
	}
	return nil
}

// u32Layout returns the entry point, the blob addresses and the total blob data
// size as u32 values, which is what the arithmetization reads. It fails when the
// address space check fails, or when the blob count or the data size does not
// fit in a u32.
func u32Layout(entryPoint uint64, blobs []Blob) (uint32, []uint32, int, error) {
	entry, addresses, err := addressSpace32(entryPoint, blobs)
	if err != nil {
		return 0, nil, 0, err
	}
	// The blob count, blob sizes and blobs_data indexes are u32 in the
	// arithmetization.
	dataSize := blobDataSize(blobs)
	if uint64(len(blobs)) > math.MaxUint32 || uint64(dataSize) > math.MaxUint32 {
		return 0, nil, 0, fmt.Errorf(
			"%d memory blobs with %d bytes exceed the u32 input limit",
			len(blobs),
			dataSize,
		)
	}
	return entry, addresses, dataSize, nil
}

// addressSpace32 returns the entry point and blob addresses as u32. It rejects
// an entry point or blob address above the 32-bit guest address space of the
// arithmetization, and a non-empty blob that runs past it.
func addressSpace32(entryPoint uint64, blobs []Blob) (uint32, []uint32, error) {
	if entryPoint > math.MaxUint32 {
		return 0, nil, fmt.Errorf("entry point %#x is outside the 32-bit address space", entryPoint)
	}
	addresses := make([]uint32, len(blobs))
	for i, blob := range blobs {
		if blob.Address > math.MaxUint32 || uint64(len(blob.Data)) > math.MaxUint32+1-blob.Address {
			return 0, nil, fmt.Errorf(
				"memory blob at %#x with %d bytes is outside the 32-bit address space",
				blob.Address,
				len(blob.Data),
			)
		}
		addresses[i] = uint32(blob.Address)
	}
	return uint32(entryPoint), addresses, nil
}

func blobDataSize(blobs []Blob) int {
	var size int
	for _, blob := range blobs {
		size += len(blob.Data)
	}
	return size
}

func encodeExecutableBits(blobs []Blob) []byte {
	bits := make([]byte, (len(blobs)+7)/8)
	for i, blob := range blobs {
		if blob.Executable {
			bits[i/8] |= 1 << (7 - uint(i%8))
		}
	}
	return bits
}

func validateBlobsRanges(blobs []Blob) error {
	var previousAddress, previousEnd uint64
	hasPrevious := false
	for _, blob := range blobs {
		if len(blob.Data) == 0 {
			continue
		}
		end := blob.Address + uint64(len(blob.Data))
		if end < blob.Address {
			return fmt.Errorf(
				"memory blob at %#x with %d bytes overflows address space",
				blob.Address,
				len(blob.Data),
			)
		}
		if hasPrevious && blob.Address < previousEnd {
			return fmt.Errorf(
				"memory blob at %#x overlaps blob at %#x ending at %#x",
				blob.Address,
				previousAddress,
				previousEnd,
			)
		}
		previousAddress, previousEnd, hasPrevious = blob.Address, end, true
	}
	return nil
}

// BitArrayFromBytes constructs a bit array directly from a slice of zero or
// more bytes, such that every bit of every byte is included in the final array.
func BitArrayFromBytes(bytes ...byte) bit.Array {
	return bit.NewArray(bytes, uint(len(bytes)*8), bit.BigEndian)
}
