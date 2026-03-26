package common

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func DecomposeCol(n []field.Element, base, nb int) [][]field.Element {

	res := make([][]field.Element, nb)
	for i := range n {
		a := DecomposeU64(n[i].Uint64(), base, nb)
		for j := range res {
			res[j] = append(res[j], a[j])
		}
	}
	return res
}

// CleanBaseChi takes as input a slice of uint64 numbers in base BaseChi (11) and clean the representation.
func CleanBaseChi(in []field.Element) (out []field.Element) {
	out = make([]field.Element, len(in))
	for i := 0; i < len(in); i++ {
		// take the second bit
		a := in[i].Uint64() >> 1 & 1
		out[i] = field.NewElement(a)
	}
	return out
}

// it recompose the given columns of field elements n into the given base.
func RecomposeCols(n [][]field.Element, base []field.Element) (res []field.Element) {

	for j := range n[0] {

		var t []field.Element
		for i := range n {
			t = append(t, n[i][j])
		}

		res = append(res, RecomposeRow(t, &base[j]))
	}
	return res
}

// it recompose the given slice of field elements r into the given base.
func RecomposeRow(r []field.Element, base *field.Element) field.Element {
	// .. using the Horner method
	s := field.Zero()
	for i := len(r) - 1; i >= 0; i-- {
		s.Mul(&s, base)
		s.Add(&s, &r[i])
	}
	return s
}

// it decompose the given field element n into the given base.
func DecomposeU64(n uint64, base, nb int) []field.Element {
	// It will essentially be used for chunk to slice decomposition
	var (
		res    = make([]field.Element, 0, nb)
		curr   = n
		base64 = uint64(base)
	)
	for curr > 0 {
		limb := field.NewElement(uint64(curr % base64))
		res = append(res, limb)
		curr /= base64
	}

	if len(res) > nb {
		utils.Panic("expected %v limbs, but got %v", nb, len(res))
	}

	// Complete with zeroes
	for len(res) < nb {
		res = append(res, field.Zero())
	}

	return res
}

func DecomposeAndCleanFr(n field.Element, base, nb int) []field.Element {

	var (
		bits  []field.Element
		limbs = DecomposeU64(n.Uint64(), base, nb) // in base 'base', nb limbs
	)

	switch {
	case base == BaseChi:
		bits = CleanBaseChi(limbs) // bits representation of each limb
	case base == BaseTheta:
		bits = cleanBaseTheta(limbs) // bits representation of each limb
	default:
		utils.Panic("unsupported base %v for cleaning", base)
	}

	return bits
}

func cleanBaseTheta(in []field.Element) (out []field.Element) {
	out = make([]field.Element, 0, len(in))
	for _, element := range in {
		if element.Uint64()%2 == 0 {
			out = append(out, field.Zero())
		} else {
			out = append(out, field.One())
		}
	}
	return out
}

func DecomposeAndCleanCol(n []field.Element, base, nb int) [][]field.Element {

	var (
		res = make([][]field.Element, nb)
	)
	for i := range n {
		dec := DecomposeAndCleanFr(n[i], base, nb)
		for j := range dec {
			res[j] = append(res[j], dec[j])
		}
	}

	return res
}

// decompose decomposes the non-negative integer r into digits (limbs) in the given base.
//
// The function returns a slice of uint64 holding the base-"base" representation of r,
// with the least-significant limb first (little-endian digit order). The returned slice
// has length equal to the number of non-zero limbs produced (which may be 0 when r == 0)
// and a capacity reserved for nb limbs (make([]uint64, 0, nb)).
//
// Parameters:
//   - r: the value to decompose (uint64).
//   - base: the base to use for decomposition (interpreted as uint64 internally).
//   - numLimbs: the maximum number of limbs expected; also used as the initial capacity of the result.
//   - useFullLength: if true, the function will ensure that the returned slice has exactly numLimbs elements.
//
// Behavior and guarantees:
//   - Uses repeated division and modulo to extract limbs: limb = curr % base, curr /= base.
//   - Returns a slice of limbs in little-endian order (least-significant first).
//   - If the decomposition requires more than numLimbs limbs, the function triggers utils.Panic
//     with an explanatory message ("expected %v limbs, but got %v").
//   - res may contain fewer than numLimbs limbs if r is small.
func Decompose(r uint64, base int, numLimbs int, useFullLength bool) (res []uint64) {
	// It will essentially be used for chunk to slice decomposition
	if base < 2 {
		utils.Panic("base must be at least 2, got %v", base)
	}
	res = make([]uint64, 0, numLimbs)
	base64 := uint64(base)
	curr := r
	// Handle the zero case explicitly
	if curr == 0 {
		if useFullLength {
			for i := 0; i < numLimbs; i++ {
				res = append(res, 0)
			}
		} else {
			res = append(res, 0)
		}
		return res
	}
	for curr > 0 {
		limb := curr % base64
		res = append(res, limb)
		curr /= base64
	}

	if len(res) > numLimbs {
		utils.Panic("expected %v limbs, but got %v", numLimbs, len(res))
	}

	if useFullLength {
		// pad with zeros to reach numLimbs
		for len(res) < numLimbs {
			res = append(res, 0)
		}
	}

	return res
}

// Converts a U64 to a given base, the base should be given in field element
// form to save on expensive conversion.
func U64ToBaseX(x uint64, base *field.Element) field.Element {
	res := field.Zero()
	one := field.One()
	resIsZero := true

	for k := 64; k >= 0; k-- {
		// The test allows skipping useless field muls or testing
		// the entire field element.
		if !resIsZero {
			res.Mul(&res, base)
		}

		// Skips the field addition if the bit is zero
		bit := (x >> k) & 1
		if bit > 0 {
			res.Add(&res, &one)
			resIsZero = false
		}
	}

	return res
}

// CleanBaseBlock takes as input a keccak block and converts each byte to clean base
func CleanBaseBlock(block keccak.Block, base *field.Element) (res [NumLanesInBlock][NumSlices]field.Element) {
	// extract the byte of each lane, in little endian
	for i := 0; i < NumLanesInBlock; i++ {
		lanebytes := [NumSlices]uint8{}
		for j := 0; j < NumSlices; j++ {
			lanebytes[j] = uint8((block[i] >> (NumSlices * j)) & 0xff)
		}
		// convert each byte to clean base
		for j := 0; j < NumSlices; j++ {
			res[i][j] = U64ToBaseX(uint64(lanebytes[j]), base)
		}
	}
	return res
}
