// Package elfmapping maps a RISC-V ELF and additional data into the sparse
// memory inputs consumed by the R5 arithmetization.
//
// # ELF mapping
//
// Load reads allocated, file-backed sections contained in PT_LOAD segments.
// Each section becomes a Blob at its ELF virtual address. SHT_NOBITS sections,
// such as .bss, are omitted because R5 memory is initialized to zero. Blob data
// is copied, so the returned Program remains valid after the reader is closed.
//
// A Program may be cached and combined with different input data:
//
//	program, err := elfmapping.Load(elfReader)
//	if err != nil {
//		return err
//	}
//	input, err := elfmapping.NewData(
//		elfmapping.DefaultInputOrigin,
//		payload,
//		elfmapping.WithLengthPrefix(),
//	)
//	if err != nil {
//		return err
//	}
//	inputs, err := elfmapping.EncodeInputs(program, input)
//
// PrepareInputs is the convenience API when an ELF is used only once.
//
// # R5 inputs
//
// EncodeInputs returns raw bytes for these public inputs:
//
//   - entry_point_and_blobs_count: two big-endian u64 values;
//   - blobs_offset_and_size: one big-endian address and size pair per blob;
//   - blobs_data: all blob bytes concatenated in address order;
//   - blobs_executable: an optional, MSB-first bit per blob.
//
// The combined blobs are stable-sorted without mutating caller-owned slices.
// Overlapping and overflowing non-empty ranges are rejected. Returned input
// bytes do not alias Blob data.
//
// NewData with WithLengthPrefix implements the length-prefixed ABI used by some Lineth
// guests: an eight-byte little-endian payload length followed by the payload.
// The interpreter itself does not impose this framing. Empty payloads produce
// only the length blob.
//
// WithIncludeExecutable adds the bitmap used by the separate predecoding
// proof. Executable bytes are always present in blobs_data; the option controls
// only the bitmap. WithSectionsWriter writes the legacy diagnostic table of
// indexes, addresses, sizes, executable flags, and section names.
package elfmapping
