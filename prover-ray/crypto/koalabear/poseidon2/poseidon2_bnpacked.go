package poseidon2

import (
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	bnposeidon2 "github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon2"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	gnarkhash "github.com/consensys/gnark/std/hash"
	stdposeidon2 "github.com/consensys/gnark/std/hash/poseidon2"
	"github.com/consensys/gnark/std/rangecheck"
)

// BN-packed Poseidon2 hashing of KoalaBear elements.
//
// Instead of emulating the KoalaBear Poseidon2 permutation inside a BN254
// circuit (see [KoalagnarkMDHasher]), this hasher packs as many KoalaBear
// elements as possible into a single BN254 scalar-field element and hashes the
// packed elements with the *native* BN254 Poseidon2 Merkle-Damgard hasher.
//
// Packing layout: a KoalaBear element is canonically < 2^31, and the BN254
// scalar field has ~254 bits, so [BNPackedNbKoalaPerLimb] = 8 elements fit in
// one BN254 element (8*31 = 248 bits < 253) without any wrap-around. A chunk
// (x_0, ..., x_{k-1}), k <= 8, is packed big-endian as
//
//	sum_i x_i * 2^(31*(k-1-i))
//
// After all chunks are absorbed, the total number of KoalaBear elements is
// absorbed as a final block. This length suffix makes the encoding injective
// (otherwise e.g. [x] and [0, x] would collide).
//
// The digest is a single BN254 element; it is NOT compatible with
// [MDHasher]/[KoalagnarkMDHasher] digests. It exists so that the two
// approaches can be compared.

const (
	// BNPackedBitsPerKoala is the number of bits reserved for each KoalaBear
	// element when packing.
	BNPackedBitsPerKoala = 31
	// BNPackedNbKoalaPerLimb is the number of KoalaBear elements packed in a
	// single BN254 scalar-field element.
	BNPackedNbKoalaPerLimb = 8
)

// ---------------------------------------------------------------------------
// Native
// ---------------------------------------------------------------------------

// BNPackedHasher is the native counterpart of [BNPackedGnarkHasher].
type BNPackedHasher struct {
	buffer []field.Element
}

// NewBNPackedHasher returns a new native BN-packed hasher.
func NewBNPackedHasher() *BNPackedHasher {
	return &BNPackedHasher{}
}

// Reset clears the buffer.
func (h *BNPackedHasher) Reset() { h.buffer = h.buffer[:0] }

// Write appends KoalaBear elements to the buffer.
func (h *BNPackedHasher) Write(data ...field.Element) {
	h.buffer = append(h.buffer, data...)
}

// WriteOctuplet appends octuplets to the buffer.
func (h *BNPackedHasher) WriteOctuplet(data ...field.Octuplet) {
	for i := range data {
		h.buffer = append(h.buffer, data[i][:]...)
	}
}

// Sum returns the BN254 digest of the buffered elements. It does not modify
// the buffer.
func (h *BNPackedHasher) Sum() fr.Element {
	md := bnposeidon2.NewMerkleDamgardHasher()
	for _, limb := range BNPackKoala(h.buffer) {
		b := limb.Bytes()
		_, _ = md.Write(b[:])
	}
	var length fr.Element
	length.SetUint64(uint64(len(h.buffer)))
	lb := length.Bytes()
	_, _ = md.Write(lb[:])

	var res fr.Element
	res.SetBytes(md.Sum(nil))
	return res
}

// BNPackKoala packs KoalaBear elements into BN254 elements, 8 per element
// (the last chunk may be shorter), using the layout described above.
func BNPackKoala(xs []field.Element) []fr.Element {
	res := make([]fr.Element, 0, (len(xs)+BNPackedNbKoalaPerLimb-1)/BNPackedNbKoalaPerLimb)
	acc, tmp := new(big.Int), new(big.Int)
	for start := 0; start < len(xs); start += BNPackedNbKoalaPerLimb {
		end := min(start+BNPackedNbKoalaPerLimb, len(xs))
		acc.SetUint64(0)
		for _, x := range xs[start:end] {
			acc.Lsh(acc, BNPackedBitsPerKoala)
			acc.Add(acc, tmp.SetUint64(x.Uint64()))
		}
		var e fr.Element
		e.SetBigInt(acc)
		res = append(res, e)
	}
	return res
}

// ---------------------------------------------------------------------------
// Circuit
// ---------------------------------------------------------------------------

// BNPackedGnarkHasher hashes emulated KoalaBear elements using BN254-native
// Poseidon2 after packing them 8-per-BN254-element. It must be compiled over
// BN254.
type BNPackedGnarkHasher struct {
	api      frontend.API
	koalaAPI *circuit.KoalaBearAPI
	buffer   []circuit.Element
}

// NewBNPackedGnarkHasher creates a new in-circuit BN-packed hasher.
func NewBNPackedGnarkHasher(api frontend.API) *BNPackedGnarkHasher {
	return &BNPackedGnarkHasher{api: api, koalaAPI: circuit.NewAPI(api)}
}

// Reset clears the buffer.
func (h *BNPackedGnarkHasher) Reset() { h.buffer = h.buffer[:0] }

// Write appends KoalaBear elements to the buffer.
func (h *BNPackedGnarkHasher) Write(data ...circuit.Element) {
	h.buffer = append(h.buffer, data...)
}

// WriteOctuplet appends octuplets to the buffer.
func (h *BNPackedGnarkHasher) WriteOctuplet(data ...circuit.Octuplet) {
	for i := range data {
		h.buffer = append(h.buffer, data[i][:]...)
	}
}

// Sum returns the BN254 digest of the buffered elements. It does not modify
// the buffer.
func (h *BNPackedGnarkHasher) Sum() frontend.Variable {
	md, err := stdposeidon2.New(h.api)
	if err != nil {
		panic(err)
	}
	h.absorb(md)
	return md.Sum()
}

func (h *BNPackedGnarkHasher) absorb(md gnarkhash.FieldHasher) {
	md.Write(h.pack()...)
	md.Write(len(h.buffer))
}

// pack packs the buffered elements into native BN254 variables. Each element
// is first brought to its canonical value in [0, p) by [canonicalKoala]: this
// keeps the encoding unique (x and x+p must not hash differently) and
// guarantees each slot fits in 31 bits.
func (h *BNPackedGnarkHasher) pack() []frontend.Variable {
	rc := rangecheck.New(h.api)
	shift := new(big.Int).Lsh(big.NewInt(1), BNPackedBitsPerKoala)

	res := make([]frontend.Variable, 0, (len(h.buffer)+BNPackedNbKoalaPerLimb-1)/BNPackedNbKoalaPerLimb)
	for start := 0; start < len(h.buffer); start += BNPackedNbKoalaPerLimb {
		end := min(start+BNPackedNbKoalaPerLimb, len(h.buffer))
		var acc frontend.Variable = 0
		for i := start; i < end; i++ {
			x := canonicalKoala(h.api, h.koalaAPI, rc, h.buffer[i])
			acc = h.api.Add(h.api.Mul(acc, shift), x)
		}
		res = append(res, acc)
	}
	return res
}

// KoalaBear modulus p = 2^31 - 2^24 + 1 = 127 * 2^24 + 1.
var (
	koalaModulus = big.NewInt(2130706433)
	koalaLowMask = big.NewInt(1<<24 - 1)
)

func init() {
	solver.RegisterHint(koalaCanonicalSplitHint)
}

// koalaCanonicalSplitHint takes v in [0, 2^31) and returns (q, h, l) such that
// v = q*p + x with q in {0, 1}, x = v mod p, and x = h*2^24 + l with
// h < 2^7 and l < 2^24.
func koalaCanonicalSplitHint(_ *big.Int, in, out []*big.Int) error {
	q, x := new(big.Int), new(big.Int)
	q.DivMod(in[0], koalaModulus, x)
	out[0].Set(q)
	out[1].Rsh(x, 24)
	out[2].And(x, koalaLowMask)
	return nil
}

// canonicalKoala returns the canonical value x in [0, p) of a KoalaBear
// element as a native variable. This is "Option A" of the hashing report,
// following SP1's reduceWithMaxBits.
//
// After the weak Reduce, gnark constrains the single 31-bit limb, so
// v < 2^31 < 2p and the quotient q = floor(v/p) is 0 or 1 (B = 31). We then
// check:
//
//	q * (q - 1) = 0
//	v = q*p + h*2^24 + l
//	h < 2^7, l < 2^24            (so x < 2^31)
//	IsZero(h - 127) * l = 0      (so x < p = 127*2^24 + 1)
//
// If q were 1 while v < p, x = v - p would wrap mod r and could not be
// written as h*2^24 + l with those bounds, so the checks reject it.
func canonicalKoala(api frontend.API, koalaAPI *circuit.KoalaBearAPI, rc frontend.Rangechecker, e circuit.Element) frontend.Variable {
	reduced := koalaAPI.EmulatedField().Reduce(&e.EV)
	v := reduced.Limbs[0]

	out, err := api.Compiler().NewHint(koalaCanonicalSplitHint, 3, v)
	if err != nil {
		panic(err)
	}
	q, hi, lo := out[0], out[1], out[2]

	api.AssertIsBoolean(q)
	rc.Check(hi, 7)
	rc.Check(lo, 24)
	x := api.Add(api.Mul(hi, 1<<24), lo)
	api.AssertIsEqual(v, api.Add(api.Mul(q, koalaModulus), x))
	api.AssertIsEqual(api.Mul(api.IsZero(api.Sub(hi, 127)), lo), 0)
	return x
}
