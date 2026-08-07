package fri

import (
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gutils "github.com/consensys/gnark-crypto/utils"
)

const (
	// bitRevLogTileSize is the COBRA tile parameter: the scratch buffer holds
	// tileSize^2 elements, 1 MiB for 4-byte elements (within one core's L2).
	// The constants are tuned for 4-byte elements; bitReverseCopy also uses
	// them for 24-byte extension elements, where they remain a large win over
	// the naive permutation even though the tile overflows L2.
	bitRevLogTileSize = uint64(9)
	// bitRevCobraMinLog2 is the naive/COBRA crossover measured on a 96-core
	// EPYC 9R45 with all cores bit-reversing concurrently (the Encode regime):
	// naive wins through 2^20, COBRA wins 1.4x at 2^21 and 2.1x at 2^22.
	bitRevCobraMinLog2 = 21
)

// bitReverse applies the bit-reversal permutation to v; len(v) must be a
// power of two.
//
// gnark-crypto's utils.BitReverse falls back to a naive swap loop whenever
// sizeof(T) < 8, so 4-byte KoalaBear columns always take the cache-hostile
// path: the naive loop accesses v by power-of-two strides and thrashes every
// cache level once a column exceeds L2 (profiles attributed >40% of a large
// commit's CPU to it). This is the COBRA algorithm (Carter and Gatlin, 1998)
// specialized for 4-byte elements, same structure as gnark-crypto's
// bitReverseCobraInPlace; small inputs stay on the naive path where they are
// faster.
func bitReverse(v []field.Element) {
	if len(v) < 1<<bitRevCobraMinLog2 {
		gutils.BitReverse(v) // naive for 4-byte elements
		return
	}
	bitReverseCobraInPlace(v)
}

func bitReverseCobraInPlace(v []field.Element) {
	const (
		logTileSize = bitRevLogTileSize
		tileSize    = uint64(1) << logTileSize
	)
	logN := uint64(bits.Len64(uint64(len(v))) - 1)
	if uint64(len(v)) != 1<<logN {
		panic("len(v) must be a power of 2")
	}
	var (
		logBLen = logN - 2*logTileSize
		bLen    = uint64(1) << logBLen
		bShift  = logBLen + logTileSize
	)

	t := make([]field.Element, tileSize*tileSize)

	for b := range bLen {

		for a := range tileSize {
			aRev := (bits.Reverse64(a) >> (64 - logTileSize)) << logTileSize
			for c := range tileSize {
				idx := (a << bShift) | (b << logTileSize) | c
				t[aRev|c] = v[idx]
			}
		}

		bRev := (bits.Reverse64(b) >> (64 - logBLen)) << logTileSize

		for c := range tileSize {
			cRev := ((bits.Reverse64(c) >> (64 - logTileSize)) << bShift) | bRev
			for aRev := range tileSize {
				a := bits.Reverse64(aRev) >> (64 - logTileSize)
				idx := (a << bShift) | (b << logTileSize) | c
				idxRev := cRev | aRev
				if idx < idxRev {
					tIdx := (aRev << logTileSize) | c
					v[idxRev], t[tIdx] = t[tIdx], v[idxRev]
				}
			}
		}

		for a := range tileSize {
			aRev := bits.Reverse64(a) >> (64 - logTileSize)
			for c := range tileSize {
				cRev := (bits.Reverse64(c) >> (64 - logTileSize)) << bShift
				idx := (a << bShift) | (b << logTileSize) | c
				idxRev := cRev | bRev | aRev
				if idx < idxRev {
					tIdx := (aRev << logTileSize) | c
					v[idx], t[tIdx] = t[tIdx], v[idx]
				}
			}
		}
	}
}

// bitReverseCopy writes dst[bitrev(j)] = src[j]; dst and src must not alias,
// and their common length must be a power of two. Above the COBRA threshold
// it uses the same cache-oblivious tiling as bitReverse, in a cheaper
// out-of-place form (one pass, no swap-back).
func bitReverseCopy[T any](dst, src []T) {
	n := uint64(len(src))
	if len(dst) != len(src) || bits.OnesCount64(n) != 1 {
		panic("fri: bitReverseCopy: length mismatch or not a power of 2")
	}

	if len(src) < 1<<bitRevCobraMinLog2 {
		nn := uint64(64 - bits.TrailingZeros64(n))
		for i := range n {
			dst[bits.Reverse64(i)>>nn] = src[i]
		}
		return
	}

	const (
		logTileSize = bitRevLogTileSize
		tileSize    = uint64(1) << logTileSize
	)
	var (
		logN    = uint64(bits.Len64(n) - 1)
		logBLen = logN - 2*logTileSize
		bShift  = logBLen + logTileSize
	)

	t := make([]T, tileSize*tileSize)
	for b := range uint64(1) << logBLen {
		for a := range tileSize {
			aRev := (bits.Reverse64(a) >> (64 - logTileSize)) << logTileSize
			for c := range tileSize {
				t[aRev|c] = src[(a<<bShift)|(b<<logTileSize)|c]
			}
		}

		bRev := (bits.Reverse64(b) >> (64 - logBLen)) << logTileSize

		for c := range tileSize {
			base := ((bits.Reverse64(c) >> (64 - logTileSize)) << bShift) | bRev
			for aRev := range tileSize {
				dst[base|aRev] = t[(aRev<<logTileSize)|c]
			}
		}
	}
}
