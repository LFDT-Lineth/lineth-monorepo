package encoding

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// multipliers9 contains precomputed powers of 2 for 9-chunk (30-bit) encoding.
// Used for encoding 9 Koalabear elements into a single BLS12-377 element.
var multipliers9 = [KoalabearChunks]*big.Int{
	big.NewInt(1),                                 // 2^0
	big.NewInt(1 << ChunkBits9),                   // 2^30
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*2), // 2^60
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*3), // 2^90
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*4), // 2^120
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*5), // 2^150
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*6), // 2^180
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*7), // 2^210
	new(big.Int).Lsh(big.NewInt(1), ChunkBits9*8), // 2^240
}

// Encode8WVsToFV encodes 8 Koalabear koalagnark.Var into a single BLS12-377 frontend.Variable.
// Each input is treated as a 31-bit value. This is the circuit equivalent of EncodeKoalabearOctupletToFrElement.
// Uses MulAcc for more efficient constraint generation.
func Encode8WVsToFV(api frontend.API, values [8]koalagnark.Element) frontend.Variable {
	// Start with the first term to avoid unnecessary zero addition
	result := api.Mul(values[7].Native(), multipliers8[0])

	// Use MulAcc for remaining terms: result = result + value * multiplier
	for i := 1; i < 8; i++ {
		value := values[7-i].Native()
		result = api.MulAcc(result, value, multipliers8[i])
	}

	return result
}

// EncodeWVsToFVs encodes a slice of Koalabear koalagnark.Var into BLS12-377 frontend.Variable slices.
// Elements are packed 8 at a time, with left-padding of zeros if needed.
func EncodeWVsToFVs(api frontend.API, values []koalagnark.Element) []frontend.Variable {
	var res []frontend.Variable
	for len(values) != 0 {
		var buf [8]koalagnark.Element
		for i := 0; i < 8; i++ {
			buf[i] = koalagnark.NewElement(0)
		}
		// in this case we left pad by zeroes
		if len(values) < 8 {
			copy(buf[8-len(values):], values)
			values = values[:0]
		} else {
			copy(buf[:], values[:8])
			values = values[8:]
		}
		res = append(res, Encode8WVsToFV(api, buf))
	}

	return res
}

// EncodeFVTo8WVs decodes a BLS12-377 frontend.Variable into 8 Koalabear koalagnark.Var.
// Each output represents a 30-bit limb extracted from the input.
// Note: This extracts 30-bit chunks, which differs from the 31-bit encoding in Encode8WVsToFV.
func EncodeFVTo8WVs(api frontend.API, value frontend.Variable) [8]koalagnark.Element {
	var res [8]koalagnark.Element
	bits := api.ToBinary(value, 256)

	koalaAPI := koalagnark.NewAPI(api)

	for i := 0; i < 8; i++ {
		limbBits := bits[32*i : 32*i+30]
		res[i] = koalaAPI.FromBinary(limbBits...)
	}

	return res
}

// Encode9WVsToFV encodes 9 Koalabear koalagnark.Var into a single BLS12-377 frontend.Variable.
// Each input is treated as a 30-bit value. This is the circuit equivalent of DecodeKoalabearToBLS12Root.
// Used for Merkle root round-trip encoding in the gnark verifier.
// Uses MulAcc for more efficient constraint generation.
func Encode9WVsToFV(api frontend.API, values [KoalabearChunks]koalagnark.Element) frontend.Variable {
	// Start with the first term to avoid unnecessary zero addition
	result := api.Mul(values[KoalabearChunks-1].Native(), multipliers9[0])

	// Use MulAcc for remaining terms: result = result + value * multiplier
	for i := 1; i < KoalabearChunks; i++ {
		value := values[KoalabearChunks-1-i].Native()
		result = api.MulAcc(result, value, multipliers9[i])
	}

	return result
}
