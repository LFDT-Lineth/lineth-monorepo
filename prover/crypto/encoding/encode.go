// Package encoding provides functions for encoding/decoding between
// Koalabear field elements (31-bit) and BLS12-377 field elements (253-bit).
//
// Two encoding schemes are supported:
// - 8-chunk encoding: 8 × 31-bit = 248 bits (for general use)
// - 9-chunk encoding: 9 × 30-bit = 270 bits (for Merkle root round-trips)
package encoding

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Constants for bit widths used in encoding
const (
	// KoalabearChunks is the number of 30-bit chunks for BLS12 root encoding
	KoalabearChunks = 9
	// ChunkBits8 is the bit width for 8-chunk encoding
	ChunkBits8 = 31
	// ChunkBits9 is the bit width for 9-chunk encoding
	ChunkBits9 = 30
	// Mask30Bits is the mask for extracting 30 bits (0x3FFFFFFF)
	Mask30Bits = uint64((1 << 30) - 1)
)

// Package-level multipliers to avoid per-call allocation
var multipliers8 = [8]*big.Int{
	big.NewInt(1),                                 // 2^0
	big.NewInt(1 << ChunkBits8),                   // 2^31
	new(big.Int).Lsh(big.NewInt(1), ChunkBits8*2), // 2^62
	new(big.Int).Lsh(big.NewInt(1), ChunkBits8*3), // 2^93
	new(big.Int).Lsh(big.NewInt(1), ChunkBits8*4), // 2^124
	new(big.Int).Lsh(big.NewInt(1), ChunkBits8*5), // 2^155
	new(big.Int).Lsh(big.NewInt(1), ChunkBits8*6), // 2^186
	new(big.Int).Lsh(big.NewInt(1), ChunkBits8*7), // 2^217
}

// EncodeKoalabearOctupletToFrElement encodes 8 Koalabear field elements into a single BLS12-377 field element.
// Each Koalabear element is treated as a 31-bit value, packed into the 253-bit BLS12-377 field.
// Optimized to reduce big.Int allocations by reusing variables.
func EncodeKoalabearOctupletToFrElement(elements [8]field.Element) fr.Element {
	var res fr.Element
	var bres, bElement, bScaled big.Int

	for i := 0; i < 8; i++ {
		elements[7-i].BigInt(&bElement)

		// Reuse bScaled instead of allocating new big.Int for multiplication result
		bScaled.Mul(&bElement, multipliers8[i])
		bres.Add(&bres, &bScaled)
	}

	res.SetBigInt(&bres)
	return res
}

// EncodeKoalabearsToFrElement encodes a slice of Koalabear field elements into BLS12-377 field elements.
// Elements are packed 8 at a time, with left-padding of zeros if the input length is not a multiple of 8.
// Optimized with pre-allocated result slice to avoid repeated slice growth.
func EncodeKoalabearsToFrElement(elements []field.Element) []fr.Element {
	if len(elements) == 0 {
		return nil
	}
	// Pre-allocate result slice with exact capacity needed
	numResults := (len(elements) + 7) / 8
	res := make([]fr.Element, 0, numResults)

	for len(elements) != 0 {
		var buf [8]field.Element
		// in this case we left pad by zeroes
		if len(elements) < 8 {
			copy(buf[8-len(elements):], elements[:])
			elements = elements[:0]
		} else {
			copy(buf[:], elements[:8])
			elements = elements[8:]
		}
		res = append(res, EncodeKoalabearOctupletToFrElement(buf))
	}
	return res
}

// EncodeFrElementToOctuplet decodes a BLS12-377 field element into 8 Koalabear field elements.
// It is used for randomness generation, not necessary be the inverse of EncodeKoalabearOctupletToFrElement, .
// Note: This extracts 30-bit chunks, which differs from the 31-bit encoding in EncodeKoalabearOctupletToFrElement.
func EncodeFrElementToOctuplet(element fr.Element) field.Octuplet {
	bits := element.Bits()
	var res field.Octuplet
	for i := 0; i < 4; i++ {
		// Each 64-bit word in element gives two 30-bit chunks
		res[2*i].SetUint64(bits[i] & Mask30Bits)
		res[2*i+1].SetUint64((bits[i] >> 32) & Mask30Bits)
	}
	return res
}

// mask30Bits is a pre-allocated big.Int for masking, used to avoid allocations
var mask30BitsBigInt = big.NewInt(int64(Mask30Bits))

// EncodeBLS12RootToKoalabear decomposes a BLS12-377 field element (Merkle root) into 9 Koalabear field elements.
// Each chunk is 30 bits, allowing for lossless round-trip encoding.
func EncodeBLS12RootToKoalabear(encoded fr.Element) [KoalabearChunks]field.Element {
	var elements, res [KoalabearChunks]field.Element

	bytes := encoded.Bytes()
	var value, chunk big.Int
	value.SetBytes(bytes[:])

	// Extract each 30-bit chunk using reusable big.Int
	for i := 0; i < KoalabearChunks; i++ {
		chunk.And(&value, mask30BitsBigInt)
		elements[i].SetBigInt(&chunk)
		value.Rsh(&value, ChunkBits9)
	}

	// Reverse to match expected ordering (little-endian to big-endian)
	for i := 0; i < KoalabearChunks; i++ {
		res[i] = elements[KoalabearChunks-1-i]
	}
	return res
}

// DecodeKoalabearToBLS12Root reconstructs a BLS12-377 field element from 9 Koalabear field elements.
// This is the inverse of EncodeBLS12RootToKoalabear.
func DecodeKoalabearToBLS12Root(elements [KoalabearChunks]field.Element) fr.Element {
	var expectedResult, bElement big.Int

	// Process all chunks in a single loop using reusable big.Int
	for i := 0; i < KoalabearChunks; i++ {
		elements[KoalabearChunks-1-i].BigInt(&bElement)
		bElement.Lsh(&bElement, uint(ChunkBits9*i))
		expectedResult.Or(&expectedResult, &bElement)
	}

	var res types.Bls12377Fr
	expectedBytes := expectedResult.Bytes()
	copy(res[32-len(expectedBytes):], expectedBytes) // left pad with zeroes to 32 bytes

	var resElem fr.Element
	resElem.SetBytes(res[:])
	return resElem
}
