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
	"unsafe"

	"github.com/consensys/gnark-crypto/field/koalabear"
	kbposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
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
	// Seed the round keys were derived from.
	Seed string
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
		Seed:            seed,
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
	// kb is set when T is koalabear.Element; the hot loops then take the
	// concrete paths of koalabear.go selected by kbPath.
	kb     bool
	kbPath KoalaBearPath
	// kbAsm is gnark-crypto's permutation, set when kbPath is KoalaBearAsm.
	kbAsm *kbposeidon2.Permutation
}

// NewPermutation returns the permutation described by params.
func NewPermutation[T any, PT generic.Element[T]](params *Parameters[T, PT]) *Permutation[T, PT] {
	return &Permutation[T, PT]{params: params, kb: isKoalaBear[T]()}
}

// Params returns the parameters of the permutation.
func (h *Permutation[T, PT]) Params() *Parameters[T, PT] { return h.params }

// Permute applies the permutation to input in place.
func (h *Permutation[T, PT]) Permute(input []T) error {
	p := h.params
	if len(input) != p.Width {
		return ErrInvalidWidth
	}
	if h.kbAsm != nil {
		return h.kbAsm.Permutation(asKB(input))
	}

	h.matMulExternal(input)

	rf := p.NbFullRounds / 2
	for r := range p.NbFullRounds + p.NbPartialRounds {
		h.addRoundKey(r, input)
		if r >= rf && r < rf+p.NbPartialRounds {
			h.sBox(&input[0])
			h.matMulInternal(input)
			continue
		}
		h.sBoxFull(input)
		h.matMulExternal(input)
	}
	return nil
}

func (h *Permutation[T, PT]) sBox(x *T) {
	if h.kb {
		kbSBox((*koalabear.Element)(unsafe.Pointer(x))) // #nosec G103 -- T is koalabear.Element
		return
	}
	var sq T
	PT(&sq).Square(x)
	PT(x).Mul(x, &sq)
}

// sBoxFull applies the S-box to every element of input.
func (h *Permutation[T, PT]) sBoxFull(input []T) {
	if h.kb && h.kbPath == KoalaBearVector {
		kbSBoxFullVec(asKB(input))
		return
	}
	for j := range input {
		h.sBox(&input[j])
	}
}

func (h *Permutation[T, PT]) addRoundKey(round int, input []T) {
	if h.kb {
		keys := asKB(h.params.RoundKeys[round])
		if h.kbPath == KoalaBearVector && len(keys) > 1 {
			kbAddRoundKeyVec(asKB(input), keys)
			return
		}
		kbAddRoundKey(asKB(input), keys)
		return
	}
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
	if h.kb {
		kbMatMulExternal(asKB(input))
		return
	}
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
	if h.kb {
		if h.kbPath == KoalaBearVector {
			kbMatMulInternalVec(asKB(input), asKB(h.params.InternalDiag))
			return
		}
		kbMatMulInternal(asKB(input))
		return
	}
	var sum T
	for i := range input {
		PT(&sum).Add(&sum, &input[i])
	}
	// Multiply by the diagonal with doublings, halvings and shifts instead of
	// multiplications, as gnark-crypto's matMulInternalInPlace does. The
	// entries are listed in internalDiag.
	var t T
	PT(&input[0]).Sub(&sum, PT(&t).Double(&input[0]))
	PT(&input[1]).Add(&sum, &input[1])
	PT(&input[2]).Add(&sum, PT(&t).Double(&input[2]))
	PT(&t).Set(&input[3])
	PT(&t).Halve()
	PT(&input[3]).Add(&sum, &t)
	PT(&input[4]).Add(&sum, PT(&t).Add(PT(&t).Double(&input[4]), &input[4]))
	PT(&input[5]).Add(&sum, PT(&t).Double(PT(&t).Double(&input[5])))
	PT(&t).Set(&input[6])
	PT(&t).Halve()
	PT(&input[6]).Sub(&sum, &t)
	PT(&input[7]).Sub(&sum, PT(&t).Add(PT(&t).Double(&input[7]), &input[7]))
	PT(&input[8]).Sub(&sum, PT(&t).Double(PT(&t).Double(&input[8])))
	pos, neg := diagShifts16Pos, diagShifts16Neg
	if len(input) == 24 {
		pos, neg = diagShifts24Pos, diagShifts24Neg
	}
	i := 9
	for _, n := range pos {
		PT(&input[i]).Add(&sum, PT(&t).Mul2ExpNegN(&input[i], n))
		i++
	}
	for _, n := range neg {
		PT(&input[i]).Sub(&sum, PT(&t).Mul2ExpNegN(&input[i], n))
		i++
	}
}

// Shifts n of the diagonal entries ±2⁻ⁿ from index 9 on: first the positive
// entries, then the negative ones (see internalDiag).
var (
	diagShifts16Pos = []uint32{8, 3, 24}
	diagShifts16Neg = []uint32{8, 3, 4, 24}
	diagShifts24Pos = []uint32{8, 2, 3, 4, 5, 6, 24}
	diagShifts24Neg = []uint32{8, 3, 4, 5, 6, 7, 9, 24}
)
