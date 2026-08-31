package codegen

import (
	"encoding/binary"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// The aggregator pair image is the two-proof analogue of prover-ray's single
// proof image: a byte image the aggregator guest casts directly to
// verifier-ray's `*const AggregatorInput`, relocated for one base address.
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
// single-image philosophy: the guest dereferences with no arithmetic and no
// fix-up pass, at the cost of the image being valid only at this base.
//
// Both sub-images are produced by the existing, ABI-agreement-tested
// proofserialization.Encode — this encoder adds only the 16-byte header and
// the alignment padding between them, so it cannot drift from the proof
// layout independently.
const aggregatorHeaderSize = 16

// EncodeAggregatorPair lays a and b out as one aggregator pair image relocated
// for base. See the layout comment above. base carries Encode's own
// constraints (8-aligned, non-zero); Encode re-checks them for each sub-image.
func EncodeAggregatorPair(a, b proofserialization.VerifyInput, base uint64) ([]byte, error) {
	imageA, err := proofserialization.Encode(a, base+aggregatorHeaderSize)
	if err != nil {
		return nil, fmt.Errorf("encoding proof A: %w", err)
	}

	offsetB := alignUp8(aggregatorHeaderSize + uint64(len(imageA)))
	imageB, err := proofserialization.Encode(b, base+offsetB)
	if err != nil {
		return nil, fmt.Errorf("encoding proof B: %w", err)
	}

	total := offsetB + uint64(len(imageB))
	if total > proofserialization.MaxImageSize {
		return nil, fmt.Errorf("aggregator pair image is %d bytes, exceeds the guest "+
			"input region's %d", total, proofserialization.MaxImageSize)
	}

	out := make([]byte, total)
	binary.LittleEndian.PutUint64(out[0:8], base+aggregatorHeaderSize)
	binary.LittleEndian.PutUint64(out[8:16], base+offsetB)
	copy(out[aggregatorHeaderSize:], imageA)
	copy(out[offsetB:], imageB)
	return out, nil
}

func alignUp8(n uint64) uint64 {
	return (n + 7) &^ 7
}
