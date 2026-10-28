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

func kbMatMulInternal(input, diag []koalabear.Element) {
	var sum koalabear.Element
	for i := range input {
		sum.Add(&sum, &input[i])
	}
	var tmp koalabear.Element
	for i := range input {
		tmp.Mul(&input[i], &diag[i])
		input[i].Add(&sum, &tmp)
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
