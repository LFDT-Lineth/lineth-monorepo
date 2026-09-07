package zkcr5

import (
	"bytes"
	"io"
	"maps"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
)

// ElfSection is an in-memory guest RAM region: a contiguous byte slice mapped
// at a specific address.
type ElfSection struct {
	Offset uint64
	Data   []byte
	// Executable reports whether this region holds SHF_EXECINSTR bytes. Only
	// executable regions are statically pre-decoded into the `instruction_base`
	// / `decoded` inputs (see [predecoding.Predecode]); data sections supplied
	// by the caller leave it false.
	Executable bool
}

// PrepareInput constructs the map[string][]byte that [zkcdriver.PreReadInputs]
// expects. It produces the pub-input keys that RISCV-ZKC.bin's main.zkc
// declares (see arithmetization/src/main/common/inputs.zkc):
//
//   - "entry_point_and_blobs_count"
//   - "blobs_offset_and_size"
//   - "blobs_data"
//   - "instruction_base"
//   - "decoded"
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
	return EncodeGuestAndMemoryForZkc(programSections, dataSections)
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
	program, err := elfmapping.Load(r)
	if err != nil {
		return GuestProgramSections{}, err
	}
	return GuestProgramSections{
		Sections:   fromBlobs(program.Blobs),
		EntryPoint: program.EntryPoint,
	}, nil
}

// NewDataSection splits data into the two memory blobs that linea_zkvm_io expects at
// _in_start: an 8-byte LE length prefix followed by the payload bytes. It does
// not interpret the payload.
func NewDataSection(inOrigin uint64, data []byte) ([]ElfSection, error) {
	blobs, err := elfmapping.NewData(inOrigin, data, elfmapping.WithLengthPrefix())
	if err != nil {
		return nil, err
	}
	return fromBlobs(blobs), nil
}

// EncodeGuestAndMemoryForZkc builds the keyed byte map that
// [zkcdriver.PreReadInputs] expects, one entry per pub-input key:
//
//   - "entry_point_and_blobs_count": [8 BE entry point][8 BE blob count]
//   - "blobs_offset_and_size":       per blob, [8 BE offset][8 BE size]
//   - "blobs_data":                  all blob bytes concatenated
//   - "instruction_base":            [8 BE lowest executable address]
//   - "decoded":                     bit-packed pre-decoded instruction table
//
// guestSections is the ELF's loadable sections, and memory is any additional
// memory blobs (e.g. the framed StatelessInput).
//
// It sorts the combined sections by offset without mutating either input slice,
// and rejects overlapping or overflowing address ranges.
func EncodeGuestAndMemoryForZkc(guestSections GuestProgramSections, memory []ElfSection) (map[string][]byte, error) {
	program := elfmapping.Program{
		EntryPoint: guestSections.EntryPoint,
		Blobs:      toBlobs(guestSections.Sections),
	}
	inputs, err := elfmapping.EncodeInputs(program, toBlobs(memory))
	if err != nil {
		return nil, err
	}

	// Statically pre-decode the executable region. main.zkc's interpreter
	// dispatches on the unified compute_op from `decoded` instead of decoding
	// each instruction at every step, so a guest program must supply these
	// inputs: omitting them fails tracing with `missing input
	// "instruction_base"`.
	//
	// Callers that encode a data-only layout (no executable section) get
	// neither key: there is no instruction stream to describe, and emitting an
	// empty table would claim a zero-length executable region at address 0.
	if hasExecutable(program.Blobs) {
		decoded, err := predecoding.Predecode(program)
		if err != nil {
			return nil, err
		}
		maps.Copy(inputs, decoded.EncodeInputs())
	}

	return inputs, nil
}

func hasExecutable(blobs []elfmapping.Blob) bool {
	for _, blob := range blobs {
		if blob.Executable {
			return true
		}
	}
	return false
}

// toBlobs adapts prover-ray's ElfSection to the arithmetization's elfmapping.Blob.
func toBlobs(sections []ElfSection) []elfmapping.Blob {
	blobs := make([]elfmapping.Blob, len(sections))
	for i, s := range sections {
		blobs[i] = elfmapping.Blob{Address: s.Offset, Data: s.Data, Executable: s.Executable}
	}
	return blobs
}

// fromBlobs adapts the arithmetization's elfmapping.Blob to prover-ray's ElfSection.
func fromBlobs(blobs []elfmapping.Blob) []ElfSection {
	sections := make([]ElfSection, len(blobs))
	for i, b := range blobs {
		sections[i] = ElfSection{Offset: b.Address, Data: b.Data, Executable: b.Executable}
	}
	return sections
}
