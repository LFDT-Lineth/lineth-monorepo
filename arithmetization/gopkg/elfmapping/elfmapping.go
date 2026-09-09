package elfmapping

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
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

type config struct {
	includeExecutable bool
	sectionsWriter    io.Writer
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

// Option configures optional R5 mapping inputs.
type Option func(*config) error

// WithIncludeExecutable includes the public input that marks executable blobs.
func WithIncludeExecutable() Option {
	return func(cfg *config) error {
		cfg.includeExecutable = true
		return nil
	}
}

// WithSectionsWriter writes a diagnostic table describing the encoded blobs.
// The caller owns the writer and is responsible for closing it when needed.
func WithSectionsWriter(writer io.Writer) Option {
	return func(cfg *config) error {
		if writer == nil {
			return fmt.Errorf("sections writer is nil")
		}
		cfg.sectionsWriter = writer
		return nil
	}
}

// PrepareInputs maps a guest ELF and input data using the length-prefixed guest
// convention. Call [Load] and [EncodeInputs] separately for guests expecting
// raw input or when the same ELF is reused across many inputs.
func PrepareInputs(
	elfBytes []byte,
	inputData []byte,
	options ...Option,
) (map[string][]byte, error) {
	program, err := Load(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, err
	}
	inputBlobs, err := NewData(
		DefaultInputOrigin,
		inputData,
		WithLengthPrefix(),
	)
	if err != nil {
		return nil, err
	}
	return EncodeInputs(program, inputBlobs, options...)
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

func extractBlobs(elfFile *elf.File) ([]Blob, error) {
	var blobs []Blob
	for _, programHeader := range elfFile.Progs {
		if programHeader.Type != elf.PT_LOAD || programHeader.Memsz == 0 {
			continue
		}
		if programHeader.Filesz > programHeader.Memsz {
			return nil, fmt.Errorf(
				"loadable segment at %#x has file size larger than memory size",
				programHeader.Vaddr,
			)
		}
		programEnd := programHeader.Vaddr + programHeader.Memsz
		if programEnd < programHeader.Vaddr {
			return nil, fmt.Errorf(
				"loadable segment address overflow at %#x",
				programHeader.Vaddr,
			)
		}

		var programBlobs []Blob
		for _, section := range elfFile.Sections {
			if section.Size == 0 || section.Type == elf.SHT_NOBITS ||
				section.Flags&elf.SHF_ALLOC == 0 {
				continue
			}
			sectionEnd := section.Addr + section.Size
			if sectionEnd < section.Addr {
				return nil, fmt.Errorf(
					"section %s address overflow at %#x",
					section.Name,
					section.Addr,
				)
			}
			if section.Addr < programHeader.Vaddr || sectionEnd > programEnd {
				continue
			}

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
			programBlobs = append(programBlobs, Blob{
				Address:    section.Addr,
				Data:       data,
				Name:       section.Name,
				Executable: section.Flags&elf.SHF_EXECINSTR != 0,
			})
		}

		sort.Slice(programBlobs, func(i, j int) bool {
			return programBlobs[i].Address < programBlobs[j].Address
		})
		blobs = append(blobs, programBlobs...)
	}
	if len(blobs) == 0 {
		return nil, fmt.Errorf("guest ELF has no loadable sections")
	}
	return blobs, nil
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
	cfg := dataConfig{}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("applying data option: nil option")
		}
		if err := option(&cfg); err != nil {
			return nil, fmt.Errorf("applying data option: %w", err)
		}
	}
	if !cfg.lengthPrefix {
		if len(data) == 0 {
			return nil, nil
		}
		return []Blob{{Address: address, Data: data, Name: cfg.name}}, nil
	}
	payloadAddress := address + 8
	if payloadAddress < address {
		return nil, fmt.Errorf("data input offset overflow: in_origin=%#x", address)
	}

	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(data)))
	lengthName, payloadName := "", ""
	if cfg.name != "" {
		lengthName = cfg.name + "_length"
		payloadName = cfg.name + "_payload"
	}
	blobs := []Blob{{Address: address, Data: prefix, Name: lengthName}}
	if len(data) != 0 {
		blobs = append(blobs, Blob{
			Address: payloadAddress,
			Data:    data,
			Name:    payloadName,
		})
	}
	return blobs, nil
}

// EncodeInputs encodes a program and additional memory blobs into raw R5
// public-input bytes. It does not mutate either input and returned byte slices
// do not alias blob data.
func EncodeInputs(
	program Program,
	additional []Blob,
	options ...Option,
) (map[string][]byte, error) {
	cfg := config{}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("applying input option: nil option")
		}
		if err := option(&cfg); err != nil {
			return nil, fmt.Errorf("applying input option: %w", err)
		}
	}

	blobs := make([]Blob, 0, len(program.Blobs)+len(additional))
	blobs = append(blobs, program.Blobs...)
	blobs = append(blobs, additional...)
	sort.SliceStable(blobs, func(i, j int) bool {
		return blobs[i].Address < blobs[j].Address
	})
	if err := validateRanges(blobs); err != nil {
		return nil, err
	}
	if cfg.sectionsWriter != nil {
		if err := writeSections(cfg.sectionsWriter, blobs); err != nil {
			return nil, fmt.Errorf("writing sections: %w", err)
		}
	}

	entryAndCount := binary.BigEndian.AppendUint64(nil, program.EntryPoint)
	entryAndCount = binary.BigEndian.AppendUint64(entryAndCount, uint64(len(blobs)))
	offsetsAndSizes := make([]byte, 0, 16*len(blobs))
	data := make([]byte, 0, blobDataSize(blobs))
	for _, blob := range blobs {
		offsetsAndSizes = binary.BigEndian.AppendUint64(offsetsAndSizes, blob.Address)
		offsetsAndSizes = binary.BigEndian.AppendUint64(
			offsetsAndSizes,
			uint64(len(blob.Data)),
		)
		data = append(data, blob.Data...)
	}

	inputs := map[string][]byte{
		EntryPointAndBlobsCountInput: entryAndCount,
		BlobsOffsetAndSizeInput:      offsetsAndSizes,
		BlobsDataInput:               data,
	}
	if cfg.includeExecutable {
		inputs[BlobsExecutableInput] = encodeExecutableBits(blobs)
	}
	return inputs, nil
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

func validateRanges(blobs []Blob) error {
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
