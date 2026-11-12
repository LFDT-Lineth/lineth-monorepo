package poseidon2

import (
	"encoding/binary"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	bnposeidon2 "github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon2"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	gnarkhash "github.com/consensys/gnark/std/hash"
	stdperm "github.com/consensys/gnark/std/permutation/poseidon2"
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
// The length n (number of KoalaBear elements) is bound into the digest, as
// otherwise e.g. [x] and [0, x] would collide. How is selected by a
// [BNPackedDomain]: n as the initial chaining value (free, leaves only) or n
// absorbed as a final block (one extra permutation).
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

// BNPackedDomain selects how the input length n (number of KoalaBear
// elements, not packed elements) is bound into a BN-packed Merkle-Damgard
// digest. It is explicit so that a transcript mode can be added later.
type BNPackedDomain int

const (
	// BNPackedLengthInIV starts the Merkle-Damgard chain at n instead of 0
	// and absorbs no length block. This is free (n is a compile-time constant
	// in the circuit) and saves one permutation per hash compared with
	// [BNPackedLengthSuffix].
	//
	// ONLY valid for one-shot hashes whose length is known before absorbing
	// starts, i.e. Merkle leaves. It does NOT work for a Fiat-Shamir
	// transcript, which has no total length up front.
	//
	// Empty inputs are rejected. A leaf can only be confused with a Merkle
	// node ([BNNodeCompress], same compression function) if some node's left
	// child digest equals a small integer n, i.e. by finding a preimage.
	BNPackedLengthInIV BNPackedDomain = iota
	// BNPackedLengthSuffix starts at 0 and absorbs n as a final block, after
	// the data. Costs one extra permutation; works for any length known at
	// the end of hashing.
	BNPackedLengthSuffix
)

// BNPackedHasher is the native counterpart of [BNPackedGnarkHasher].
type BNPackedHasher struct {
	domain BNPackedDomain
	buffer []field.Element
}

// NewBNPackedHasher returns a new native BN-packed hasher.
func NewBNPackedHasher(domain BNPackedDomain) *BNPackedHasher {
	checkBNPackedDomain(domain)
	return &BNPackedHasher{domain: domain}
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

// bnPackedPerm is the BN254 Poseidon2 permutation with gnark-crypto's default
// parameters (width 2), the same one used by gnark's std/hash/poseidon2.
var bnPackedPerm = bnposeidon2.NewDefaultPermutation()

// Sum returns the BN254 digest of the buffered elements. It does not modify
// the buffer.
//
// The Merkle-Damgard chain works on fr.Element directly, with the same
// compression as gnark-crypto's (state, block) -> P(state, block)[1] + block,
// so there is no byte encoding/decoding per block. The digest is identical
// to bnposeidon2.NewMerkleDamgardHasher fed with the big-endian encodings.
func (h *BNPackedHasher) Sum() fr.Element {
	n := len(h.buffer)
	var length, state fr.Element
	length.SetUint64(uint64(n))
	if h.domain == BNPackedLengthInIV {
		checkBNPackedNonEmpty(n)
		state = length
	}
	for start := 0; start < n; start += BNPackedNbKoalaPerLimb {
		end := min(start+BNPackedNbKoalaPerLimb, n)
		state = bnPackedCompress(state, bnPackChunk(h.buffer[start:end]))
	}
	if h.domain == BNPackedLengthSuffix {
		state = bnPackedCompress(state, length)
	}
	return state
}

func checkBNPackedDomain(domain BNPackedDomain) {
	if domain != BNPackedLengthInIV && domain != BNPackedLengthSuffix {
		panic("poseidon2: unsupported BN-packed domain separation mode")
	}
}

func checkBNPackedNonEmpty(n int) {
	if n == 0 {
		panic("poseidon2: BN-packed hasher with length in IV cannot hash an empty input")
	}
}

func bnPackedCompress(state, block fr.Element) fr.Element {
	x := [2]fr.Element{state, block}
	if err := bnPackedPerm.Permutation(x[:]); err != nil {
		panic(err)
	}
	x[1].Add(&x[1], &block)
	return x[1]
}

// BNPackKoala packs KoalaBear elements into BN254 elements, 8 per element
// (the last chunk may be shorter), using the layout described above.
func BNPackKoala(xs []field.Element) []fr.Element {
	res := make([]fr.Element, 0, (len(xs)+BNPackedNbKoalaPerLimb-1)/BNPackedNbKoalaPerLimb)
	for start := 0; start < len(xs); start += BNPackedNbKoalaPerLimb {
		end := min(start+BNPackedNbKoalaPerLimb, len(xs))
		res = append(res, bnPackChunk(xs[start:end]))
	}
	return res
}

// bnPackChunk packs up to 8 KoalaBear elements into one BN254 element without
// big.Int: the 31-bit slots are written into four 64-bit words, which are then
// converted to Montgomery form once.
func bnPackChunk(xs []field.Element) fr.Element {
	var w [4]uint64 // little-endian words of the packed integer
	k := len(xs)
	for i, x := range xs {
		v := x.Uint64()
		off := BNPackedBitsPerKoala * (k - 1 - i) // big-endian slot order
		word, shift := off/64, uint(off%64)
		w[word] |= v << shift
		if shift+BNPackedBitsPerKoala > 64 {
			w[word+1] |= v >> (64 - shift)
		}
	}
	var b [fr.Bytes]byte
	binary.BigEndian.PutUint64(b[0:8], w[3])
	binary.BigEndian.PutUint64(b[8:16], w[2])
	binary.BigEndian.PutUint64(b[16:24], w[1])
	binary.BigEndian.PutUint64(b[24:32], w[0])
	e, err := fr.BigEndian.Element(&b) // < 2^248, always canonical
	if err != nil {
		panic(err)
	}
	return e
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
	domain   BNPackedDomain
	buffer   []circuit.Element
}

// NewBNPackedGnarkHasher creates a new in-circuit BN-packed hasher.
func NewBNPackedGnarkHasher(api frontend.API, domain BNPackedDomain) *BNPackedGnarkHasher {
	checkBNPackedDomain(domain)
	return &BNPackedGnarkHasher{api: api, koalaAPI: circuit.NewAPI(api), domain: domain}
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
//
// The length n is a compile-time constant, so with [BNPackedLengthInIV] the
// initial chaining value costs no constraints.
func (h *BNPackedGnarkHasher) Sum() frontend.Variable {
	n := len(h.buffer)
	perm, err := stdperm.NewPoseidon2(h.api)
	if err != nil {
		panic(err)
	}
	var iv frontend.Variable = 0
	if h.domain == BNPackedLengthInIV {
		checkBNPackedNonEmpty(n)
		iv = n
	}
	md := gnarkhash.NewMerkleDamgardHasher(h.api, perm, iv)
	md.Write(h.pack()...)
	if h.domain == BNPackedLengthSuffix {
		md.Write(n)
	}
	return md.Sum()
}

func (h *BNPackedGnarkHasher) pack() []frontend.Variable {
	return packKoalaCanonical(h.api, h.koalaAPI, h.buffer)
}

// packKoalaCanonical packs KoalaBear elements into native BN254 variables, 8
// per variable, with the layout of [BNPackKoala]. Each element is first
// brought to its canonical value in [0, p) by [canonicalKoala]: this keeps
// the encoding unique (x and x+p must not hash differently) and guarantees
// each slot fits in 31 bits.
func packKoalaCanonical(api frontend.API, koalaAPI *circuit.KoalaBearAPI, xs []circuit.Element) []frontend.Variable {
	rc := rangecheck.New(api)
	shift := new(big.Int).Lsh(big.NewInt(1), BNPackedBitsPerKoala)

	res := make([]frontend.Variable, 0, (len(xs)+BNPackedNbKoalaPerLimb-1)/BNPackedNbKoalaPerLimb)
	for start := 0; start < len(xs); start += BNPackedNbKoalaPerLimb {
		end := min(start+BNPackedNbKoalaPerLimb, len(xs))
		var acc frontend.Variable = 0
		for i := start; i < end; i++ {
			x := canonicalKoala(api, koalaAPI, rc, xs[i])
			acc = api.Add(api.Mul(acc, shift), x)
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

// reducedLimb returns the single limb of e after the weak reduction, as a
// native variable. A constant zero has no limb at all, and gives 0.
func reducedLimb(koalaAPI *circuit.KoalaBearAPI, e circuit.Element) frontend.Variable {
	reduced := koalaAPI.EmulatedField().Reduce(&e.EV)
	if len(reduced.Limbs) == 0 {
		return 0
	}
	return reduced.Limbs[0]
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
	v := reducedLimb(koalaAPI, e)

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
