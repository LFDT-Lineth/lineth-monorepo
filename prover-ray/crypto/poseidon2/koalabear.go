package poseidon2

import (
	"unsafe"

	"github.com/consensys/gnark-crypto/field/koalabear"
	kbposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

// Concrete KoalaBear versions of the permutation's hot loops. The generic
// versions call field operations through the generic dictionary, which the
// compiler can neither inline nor keep on the stack; these call
// koalabear.Element directly. Each generic hot loop branches on
// [Permutation.kb] and, when set, reinterprets its operands with asKB.

// KoalaBearPath selects the code a KoalaBear [Permutation] runs. It has no
// effect for other fields.
type KoalaBearPath int

const (
	// KoalaBearScalar runs concrete koalabear.Element loops in each hot loop
	// (the default).
	KoalaBearScalar KoalaBearPath = iota
	// KoalaBearVector is KoalaBearScalar with koalabear.Vector batch
	// operations (AVX-512 when available) for the round keys, the full S-box
	// layer and the internal matrix. M4 and the external matrix stay scalar,
	// as they mix neighbouring elements.
	KoalaBearVector
	// KoalaBearAsm replaces the whole permutation with gnark-crypto's
	// (AVX-512 assembly when available).
	KoalaBearAsm
)

// SetKoalaBearPath selects the KoalaBear code path. It is a no-op for other
// fields and must not be called concurrently with Permute.
func (h *Permutation[T, PT]) SetKoalaBearPath(path KoalaBearPath) {
	if !h.kb {
		return
	}
	h.kbPath = path
	h.kbAsm = nil
	if path == KoalaBearAsm {
		p := h.params
		h.kbAsm = kbposeidon2.NewPermutationWithSeed(p.Width, p.NbFullRounds, p.NbPartialRounds, p.Seed)
	}
}

// isKoalaBear reports whether T is koalabear.Element.
func isKoalaBear[T any]() bool {
	_, ok := any(*new(T)).(koalabear.Element)
	return ok
}

// asKB reinterprets s as a KoalaBear slice. Callers must have checked that T
// is koalabear.Element.
func asKB[T any](s []T) []koalabear.Element {
	return unsafe.Slice((*koalabear.Element)(unsafe.Pointer(unsafe.SliceData(s))), len(s)) // #nosec G103 -- T is koalabear.Element
}

func kbSBox(x *koalabear.Element) {
	var sq koalabear.Element
	sq.Square(x)
	x.Mul(x, &sq)
}

func kbAddRoundKey(input, keys []koalabear.Element) {
	for i := range keys {
		input[i].Add(&input[i], &keys[i])
	}
}

func kbMatMulM4(s []koalabear.Element) {
	for i := 0; i+4 <= len(s); i += 4 {
		var t01, t23, t0123, t01123, t01233 koalabear.Element
		t01.Add(&s[i], &s[i+1])
		t23.Add(&s[i+2], &s[i+3])
		t0123.Add(&t01, &t23)
		t01123.Add(&t0123, &s[i+1])
		t01233.Add(&t0123, &s[i+3])
		// x[1] and x[3] must be overwritten before x[0] and x[2].
		s[i+3].Double(&s[i])
		s[i+3].Add(&s[i+3], &t01233)
		s[i+1].Double(&s[i+2])
		s[i+1].Add(&s[i+1], &t01123)
		s[i].Add(&t01, &t01123)
		s[i+2].Add(&t23, &t01233)
	}
}

func kbMatMulExternal(input []koalabear.Element) {
	kbMatMulM4(input)
	var acc [4]koalabear.Element
	for i := 0; i < len(input); i += 4 {
		for j := range acc {
			acc[j].Add(&acc[j], &input[i+j])
		}
	}
	for i := 0; i < len(input); i += 4 {
		for j := range acc {
			input[i+j].Add(&input[i+j], &acc[j])
		}
	}
}

// kbMatMulInternal is gnark-crypto's matMulInternalInPlace for KoalaBear: the
// diagonal is applied with doublings, halvings and shifts.
func kbMatMulInternal(input []koalabear.Element) {
	var sum koalabear.Element
	sum.Set(&input[0])
	for i := 1; i < len(input); i++ {
		sum.Add(&sum, &input[i])
	}
	var temp koalabear.Element
	input[0].Sub(&sum, temp.Double(&input[0]))
	input[1].Add(&sum, &input[1])
	input[2].Add(&sum, temp.Double(&input[2]))
	temp.Set(&input[3]).Halve()
	input[3].Add(&sum, &temp)
	input[4].Add(&sum, temp.Double(&input[4]).Add(&temp, &input[4]))
	input[5].Add(&sum, temp.Double(&input[5]).Double(&temp))
	temp.Set(&input[6]).Halve()
	input[6].Sub(&sum, &temp)
	input[7].Sub(&sum, temp.Double(&input[7]).Add(&temp, &input[7]))
	input[8].Sub(&sum, temp.Double(&input[8]).Double(&temp))
	switch len(input) {
	case 16:
		// [..., 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
		input[9].Add(&sum, temp.Mul2ExpNegN(&input[9], 8))
		input[10].Add(&sum, temp.Mul2ExpNegN(&input[10], 3))
		input[11].Add(&sum, temp.Mul2ExpNegN(&input[11], 24))
		input[12].Sub(&sum, temp.Mul2ExpNegN(&input[12], 8))
		input[13].Sub(&sum, temp.Mul2ExpNegN(&input[13], 3))
		input[14].Sub(&sum, temp.Mul2ExpNegN(&input[14], 4))
		input[15].Sub(&sum, temp.Mul2ExpNegN(&input[15], 24))
	case 24:
		// [..., 1/2^8, 1/4, 1/8, 1/16, 1/32, 1/64, 1/2^24,
		//  -1/2^8, -1/8, -1/16, -1/32, -1/64, -1/2^7, -1/2^9, -1/2^24]
		input[9].Add(&sum, temp.Mul2ExpNegN(&input[9], 8))
		input[10].Add(&sum, temp.Mul2ExpNegN(&input[10], 2))
		input[11].Add(&sum, temp.Mul2ExpNegN(&input[11], 3))
		input[12].Add(&sum, temp.Mul2ExpNegN(&input[12], 4))
		input[13].Add(&sum, temp.Mul2ExpNegN(&input[13], 5))
		input[14].Add(&sum, temp.Mul2ExpNegN(&input[14], 6))
		input[15].Add(&sum, temp.Mul2ExpNegN(&input[15], 24))
		input[16].Sub(&sum, temp.Mul2ExpNegN(&input[16], 8))
		input[17].Sub(&sum, temp.Mul2ExpNegN(&input[17], 3))
		input[18].Sub(&sum, temp.Mul2ExpNegN(&input[18], 4))
		input[19].Sub(&sum, temp.Mul2ExpNegN(&input[19], 5))
		input[20].Sub(&sum, temp.Mul2ExpNegN(&input[20], 6))
		input[21].Sub(&sum, temp.Mul2ExpNegN(&input[21], 7))
		input[22].Sub(&sum, temp.Mul2ExpNegN(&input[22], 9))
		input[23].Sub(&sum, temp.Mul2ExpNegN(&input[23], 24))
	}
}

// maxWidth bounds the stack scratch buffers of the vector paths.
const maxWidth = 24

func kbSBoxFullVec(input []koalabear.Element) {
	var buf [maxWidth]koalabear.Element
	sq := koalabear.Vector(buf[:len(input)])
	v := koalabear.Vector(input)
	sq.Mul(v, v)
	v.Mul(v, sq)
}

func kbAddRoundKeyVec(input, keys []koalabear.Element) {
	v := koalabear.Vector(input)
	v.Add(v, keys)
}

func kbMatMulInternalVec(input, diag []koalabear.Element) {
	v := koalabear.Vector(input)
	sum := v.Sum()
	var buf [maxWidth]koalabear.Element
	sums := koalabear.Vector(buf[:len(input)])
	for i := range sums {
		sums[i] = sum
	}
	v.Mul(v, diag)
	v.Add(v, sums)
}

// kbCompressAsm writes the compression of state and block into state, calling
// gnark-crypto's permutation directly (see [CompressStaticCall]).
func kbCompressAsm(perm *kbposeidon2.Permutation, state, block []koalabear.Element) {
	var x [CompressionWidth]koalabear.Element
	copy(x[:BlockSize], state)
	copy(x[BlockSize:], block)
	if err := perm.Permutation(x[:]); err != nil {
		panic(err) // can't happen, the width is fixed
	}
	for i := range BlockSize {
		state[i].Add(&block[i], &x[BlockSize+i])
	}
}
