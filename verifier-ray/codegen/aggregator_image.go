package codegen

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// The aggregator pair image is the two-proof analogue of prover-ray's single
// proof image: the byte image behind verifier-ray's `AggregatorInput`,
// relocated for one base address. The guest treats it as untrusted: it reads
// the header as raw integers and validates the header and both proofs
// (`image_relocation.validatePair`) before forming any typed pointer.
//
// Layout (all offsets relative to base):
//
//	[0,  8)              a: absolute address of proof A's VerifyInput
//	[8, 16)              b: absolute address of proof B's VerifyInput
//	[16, 16+lenA)        proof A's image, encoded by proofserialization.Encode
//	                     at base+16 (so A's root lands exactly where `a` points)
//	[alignUp8(16+lenA),
//	 ... +lenB)          proof B's image, encoded at that 8-aligned offset
//
// The header pointers are absolute little-endian addresses, matching the
// single-image philosophy: once validated, the guest uses them with no
// fix-up pass, at the cost of the image being valid only at this base.
//
// Both sub-images are produced by the existing, ABI-agreement-tested
// proofserialization.Encode — this encoder adds only the 16-byte header and
// the alignment padding between them, so it cannot drift from the proof
// layout independently.
const (
	// AggregatorHeaderSize is the size of the two-pointer header, pinned as
	// verifier.AggregatorInput in src/proof_abi.zig.
	AggregatorHeaderSize = 16
	// OffAggregatorA and OffAggregatorB are the header offsets of the two
	// VerifyInput pointers.
	OffAggregatorA = 0
	OffAggregatorB = 8
)

// EncodeAggregatorPair lays a and b out as one aggregator pair image relocated
// for base. See the layout comment above. base carries Encode's own
// constraints (8-aligned, non-zero); Encode re-checks them for each sub-image.
func EncodeAggregatorPair(a, b proofserialization.VerifyInput, base uint64) ([]byte, error) {
	if base > math.MaxUint64-AggregatorHeaderSize {
		return nil, fmt.Errorf("aggregator pair base 0x%x overflows the address space", base)
	}
	imageA, err := proofserialization.Encode(a, base+AggregatorHeaderSize)
	if err != nil {
		return nil, fmt.Errorf("encoding proof A: %w", err)
	}

	offsetB := alignUp8(AggregatorHeaderSize + uint64(len(imageA)))
	if offsetB < AggregatorHeaderSize || base > math.MaxUint64-offsetB {
		return nil, fmt.Errorf("aggregator proof B address overflows the address space")
	}
	imageB, err := proofserialization.Encode(b, base+offsetB)
	if err != nil {
		return nil, fmt.Errorf("encoding proof B: %w", err)
	}

	total := offsetB + uint64(len(imageB))
	if total < offsetB || base > math.MaxUint64-total {
		return nil, fmt.Errorf("aggregator pair extent overflows the address space")
	}
	if total > proofserialization.MaxImageSize {
		return nil, fmt.Errorf("aggregator pair image is %d bytes, exceeds the guest "+
			"input region's %d", total, proofserialization.MaxImageSize)
	}

	out := make([]byte, total)
	binary.LittleEndian.PutUint64(out[OffAggregatorA:], base+AggregatorHeaderSize)
	binary.LittleEndian.PutUint64(out[OffAggregatorB:], base+offsetB)
	copy(out[AggregatorHeaderSize:], imageA)
	copy(out[offsetB:], imageB)
	return out, nil
}

func alignUp8(n uint64) uint64 {
	return (n + 7) &^ 7
}
