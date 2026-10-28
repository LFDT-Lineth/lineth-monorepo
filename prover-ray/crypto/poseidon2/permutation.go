// Package poseidon2 implements the Poseidon2 permutation, its compression
// function and a Merkle-Damgård hasher generically over any prime field
// satisfying [generic.Element].
//
// The permutation mirrors gnark-crypto's field/<name>/poseidon2 (S-box x³,
// external matrix circ(2M4, M4, ...), internal matrix 1·1ᵀ + diag, round keys
// derived from a keccak-seeded chain), so for a given field and seed it
// produces bit-identical outputs.
package poseidon2

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/sha3"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/generic"
)

// ErrInvalidWidth is returned when the permuted buffer does not match the width.
var ErrInvalidWidth = errors.New("poseidon2: input length does not match the permutation width")

// sBoxDegree is the degree of the S-box (x ↦ x³), valid for every field where
// gcd(3, p-1) = 1, which holds for KoalaBear and MamaBear.
const sBoxDegree = 3

// Parameters of a Poseidon2 instance over the field T.
type Parameters[T any, PT generic.Element[T]] struct {
	Width           int
	NbFullRounds    int
	NbPartialRounds int
	// RoundKeys[r] holds Width keys for full rounds and a single key for
	// partial rounds.
	RoundKeys [][]T
	// InternalDiag is the diagonal D of the internal matrix 1·1ᵀ + D.
	InternalDiag []T
}

// DefaultSeed returns the seed gnark-crypto uses for the given field name
// (e.g. "koalabear", "mamabear") and parameters.
func DefaultSeed(fieldName string, width, nbFullRounds, nbPartialRounds int) string {
	return fmt.Sprintf("Poseidon2-%s[t=%d,rF=%d,rP=%d,d=%d]", fieldName, width, nbFullRounds, nbPartialRounds, sBoxDegree)
}

// NewParameters derives the round keys from seed. Width must be 16 or 24.
func NewParameters[T any, PT generic.Element[T]](width, nbFullRounds, nbPartialRounds int, seed string) *Parameters[T, PT] {
	p := &Parameters[T, PT]{
		Width:           width,
		NbFullRounds:    nbFullRounds,
		NbPartialRounds: nbPartialRounds,
		InternalDiag:    internalDiag[T, PT](width),
	}

	h := sha3.NewLegacyKeccak256()
	_, _ = h.Write([]byte(seed))
	rnd := h.Sum(nil) // pre hash before use
	h.Reset()
	_, _ = h.Write(rnd)
	next := func(dst *T) {
		rnd = h.Sum(nil)
		PT(dst).SetBytes(rnd)
		h.Reset()
		_, _ = h.Write(rnd)
	}

	rf := nbFullRounds / 2
	p.RoundKeys = make([][]T, nbFullRounds+nbPartialRounds)
	for r := range p.RoundKeys {
		n := width
		if r >= rf && r < rf+nbPartialRounds {
			n = 1
		}
		p.RoundKeys[r] = make([]T, n)
		for j := range p.RoundKeys[r] {
			next(&p.RoundKeys[r][j])
		}
	}
	return p
}

// internalDiag returns the diagonal of the internal matrix, as hardcoded in
// gnark-crypto's matMulInternalInPlace for KoalaBear/BabyBear/MamaBear.
func internalDiag[T any, PT generic.Element[T]](width int) []T {
	// Each entry is sign · num · 2^-shift.
	type coeff struct {
		num   int64
		shift uint
	}
	var cs []coeff
	switch width {
	case 16:
		// [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
		cs = []coeff{{-2, 0}, {1, 0}, {2, 0}, {1, 1}, {3, 0}, {4, 0}, {-1, 1}, {-3, 0}, {-4, 0},
			{1, 8}, {1, 3}, {1, 24}, {-1, 8}, {-1, 3}, {-1, 4}, {-1, 24}}
	case 24:
		// [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/4, 1/8, 1/16, 1/32, 1/64, 1/2^24,
		//  -1/2^8, -1/8, -1/16, -1/32, -1/64, -1/2^7, -1/2^9, -1/2^24]
		cs = []coeff{{-2, 0}, {1, 0}, {2, 0}, {1, 1}, {3, 0}, {4, 0}, {-1, 1}, {-3, 0}, {-4, 0},
			{1, 8}, {1, 2}, {1, 3}, {1, 4}, {1, 5}, {1, 6}, {1, 24},
			{-1, 8}, {-1, 3}, {-1, 4}, {-1, 5}, {-1, 6}, {-1, 7}, {-1, 9}, {-1, 24}}
	default:
		panic("poseidon2: only Width=16,24 are supported")
	}
	diag := make([]T, width)
	for i, c := range cs {
		diag[i] = generic.FromInt64[T, PT](c.num)
		inv := generic.Pow2Inv[T, PT](c.shift)
		PT(&diag[i]).Mul(&diag[i], &inv)
	}
	return diag
}

// Permutation is a Poseidon2 permutation over the field T.
type Permutation[T any, PT generic.Element[T]] struct {
	params *Parameters[T, PT]
}

// NewPermutation returns the permutation described by params.
func NewPermutation[T any, PT generic.Element[T]](params *Parameters[T, PT]) *Permutation[T, PT] {
	return &Permutation[T, PT]{params: params}
}

// Params returns the parameters of the permutation.
func (h *Permutation[T, PT]) Params() *Parameters[T, PT] { return h.params }

// Permute applies the permutation to input in place.
func (h *Permutation[T, PT]) Permute(input []T) error {
	p := h.params
	if len(input) != p.Width {
		return ErrInvalidWidth
	}

	h.matMulExternal(input)

	rf := p.NbFullRounds / 2
	for r := range p.NbFullRounds + p.NbPartialRounds {
		h.addRoundKey(r, input)
		if r >= rf && r < rf+p.NbPartialRounds {
			sBox[T, PT](&input[0])
			h.matMulInternal(input)
			continue
		}
		for j := range input {
			sBox[T, PT](&input[j])
		}
		h.matMulExternal(input)
	}
	return nil
}

func sBox[T any, PT generic.Element[T]](x *T) {
	var sq T
	PT(&sq).Square(x)
	PT(x).Mul(x, &sq)
}

func (h *Permutation[T, PT]) addRoundKey(round int, input []T) {
	for i := range h.params.RoundKeys[round] {
		PT(&input[i]).Add(&input[i], &h.params.RoundKeys[round][i])
	}
}

// matMulM4 multiplies each chunk of 4 by
//
//	(2 3 1 1)
//	(1 2 3 1)
//	(1 1 2 3)
//	(3 1 1 2)
//
// using the same addition chain as gnark-crypto / Plonky3.
func matMulM4[T any, PT generic.Element[T]](s []T) {
	for i := 0; i+4 <= len(s); i += 4 {
		var t01, t23, t0123, t01123, t01233 T
		PT(&t01).Add(&s[i], &s[i+1])
		PT(&t23).Add(&s[i+2], &s[i+3])
		PT(&t0123).Add(&t01, &t23)
		PT(&t01123).Add(&t0123, &s[i+1])
		PT(&t01233).Add(&t0123, &s[i+3])
		// x[1] and x[3] must be overwritten before x[0] and x[2].
		PT(&s[i+3]).Double(&s[i])
		PT(&s[i+3]).Add(&s[i+3], &t01233)
		PT(&s[i+1]).Double(&s[i+2])
		PT(&s[i+1]).Add(&s[i+1], &t01123)
		PT(&s[i]).Add(&t01, &t01123)
		PT(&s[i+2]).Add(&t23, &t01233)
	}
}

// matMulExternal multiplies by circ(2M4, M4, ..., M4).
func (h *Permutation[T, PT]) matMulExternal(input []T) {
	matMulM4[T, PT](input)
	var acc [4]T
	for i := 0; i < len(input); i += 4 {
		for j := range acc {
			PT(&acc[j]).Add(&acc[j], &input[i+j])
		}
	}
	for i := 0; i < len(input); i += 4 {
		for j := range acc {
			PT(&input[i+j]).Add(&input[i+j], &acc[j])
		}
	}
}

// matMulInternal multiplies by 1·1ᵀ + diag.
func (h *Permutation[T, PT]) matMulInternal(input []T) {
	var sum T
	for i := range input {
		PT(&sum).Add(&sum, &input[i])
	}
	var tmp T
	for i := range input {
		PT(&tmp).Mul(&input[i], &h.params.InternalDiag[i])
		PT(&input[i]).Add(&sum, &tmp)
	}
}
