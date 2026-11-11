package poseidon2

import (
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/rangecheck"
)

// BN-packed Merkle digests carried as KoalaBear octuplets.
//
// The FRI/PCS code stores every Merkle digest as a field.Octuplet. To hash
// with BN254 Poseidon2 without changing those types, a BN254 digest d is
// encoded as an octuplet of 8 limbs of 30 bits taken from the low 240 bits of
// d, o[0] holding the highest:
//
//	o[i] = (d >> 30*(7-i)) mod 2^30
//
// Every limb is below 2^30 < p, so the octuplet is always canonical, and the
// digest keeps 240 bits (about 2^120 collision resistance, against about
// 2^124 for a native KoalaBear octuplet).
//
// To hash an octuplet again (a Merkle node), it is packed back into one BN254
// element with [BNPackKoala]'s 31-bit slots, o[0] highest. The 31-bit slots
// keep the packing injective for any canonical octuplet, not only for digest
// encodings, since some trees use non-digest octuplets as leaves (e.g. the
// FRI layers' extension values).

const (
	bnDigestLimbBits = 30
	bnDigestNbLimbs  = 8
	bnDigestHighBits = fr.Bits - bnDigestLimbBits*bnDigestNbLimbs // 14
)

// BNDigestToOctuplet encodes a BN254 digest as an octuplet: the low 240 bits
// in 8 limbs of 30 bits, highest first.
func BNDigestToOctuplet(d fr.Element) field.Octuplet {
	w := d.Bits() // regular form, little-endian words
	var o field.Octuplet
	for i := range bnDigestNbLimbs {
		off := bnDigestLimbBits * (bnDigestNbLimbs - 1 - i)
		word, shift := off/64, uint(off%64)
		v := w[word] >> shift
		if shift+bnDigestLimbBits > 64 {
			v |= w[word+1] << (64 - shift)
		}
		o[i].SetUint64(v & (1<<bnDigestLimbBits - 1))
	}
	return o
}

// BNPackOctuplet packs an octuplet into one BN254 element, 31 bits per slot,
// o[0] highest. It is injective on canonical octuplets.
func BNPackOctuplet(o field.Octuplet) fr.Element {
	return bnPackChunk(o[:])
}

// BNCompressOctuplet is the Merkle node hash of BN-packed trees on octuplet
// digests: both children are packed, compressed with [BNNodeCompress] and the
// result encoded back with [BNDigestToOctuplet].
func BNCompressOctuplet(left, right field.Octuplet) field.Octuplet {
	return BNDigestToOctuplet(BNNodeCompress(BNPackOctuplet(left), BNPackOctuplet(right)))
}

// BNOctupletMDHasher is a leaf hasher with the same interface shape as
// [MDHasher]: it hashes KoalaBear elements with the BN-packed Merkle-Damgard
// hasher (length as a suffix, so it accepts any input) and returns the digest
// encoded as an octuplet.
type BNOctupletMDHasher struct {
	h *BNPackedHasher
}

// NewBNOctupletMDHasher returns an empty BN-packed leaf hasher.
func NewBNOctupletMDHasher() *BNOctupletMDHasher {
	return &BNOctupletMDHasher{h: NewBNPackedHasher(BNPackedLengthSuffix)}
}

// Reset clears the buffer.
func (h *BNOctupletMDHasher) Reset() { h.h.Reset() }

// WriteElements appends KoalaBear elements.
func (h *BNOctupletMDHasher) WriteElements(data ...field.Element) { h.h.Write(data...) }

// SumDigest returns the octuplet encoding of the BN-packed digest.
func (h *BNOctupletMDHasher) SumDigest() field.Octuplet {
	return BNDigestToOctuplet(h.h.Sum())
}

// ---------------------------------------------------------------------------
// Circuit
// ---------------------------------------------------------------------------

func init() {
	solver.RegisterHint(bnDigestSplitHint)
}

// bnDigestSplitHint splits d into its 8 limbs of 30 bits, highest first, and
// the remaining high part d >> 240.
func bnDigestSplitHint(_ *big.Int, in, out []*big.Int) error {
	d := new(big.Int).Set(in[0])
	mask := new(big.Int).Lsh(big.NewInt(1), bnDigestLimbBits)
	mask.Sub(mask, big.NewInt(1))
	for i := bnDigestNbLimbs - 1; i >= 0; i-- {
		out[i].And(d, mask)
		d.Rsh(d, bnDigestLimbBits)
	}
	out[bnDigestNbLimbs].Set(d)
	return nil
}

// BNDigestToOctupletGnark mirrors [BNDigestToOctuplet]. The limbs are
// range-checked to 30 bits and the high part to 14 bits, and d is recomposed
// from them. The integer recomposition is below 2^254, which exceeds the
// BN254 modulus r, so d and d + r may both decompose when d + r < 2^254; the
// prover can then pick between two octuplets. This does not weaken binding:
// any octuplet it outputs is a function of d, and reaching a committed root
// from a different one needs a Poseidon2 collision.
func BNDigestToOctupletGnark(api frontend.API, d frontend.Variable) circuit.Octuplet {
	out, err := api.Compiler().NewHint(bnDigestSplitHint, bnDigestNbLimbs+1, d)
	if err != nil {
		panic(err)
	}
	rc := rangecheck.New(api)
	var (
		acc frontend.Variable = out[bnDigestNbLimbs]
		o   circuit.Octuplet
	)
	rc.Check(out[bnDigestNbLimbs], bnDigestHighBits)
	for i := range bnDigestNbLimbs {
		rc.Check(out[i], bnDigestLimbBits)
		acc = api.Add(api.Mul(acc, 1<<bnDigestLimbBits), out[i])
		o[i] = circuit.WrapFrontendVariable(out[i])
	}
	api.AssertIsEqual(acc, d)
	return o
}

// BNPackOctupletGnark mirrors [BNPackOctuplet] for any octuplet. Each limb is
// brought to its canonical value first, as in the BN-packed hasher.
func BNPackOctupletGnark(api frontend.API, o circuit.Octuplet) frontend.Variable {
	return packKoalaCanonical(api, circuit.NewAPI(api), o[:])[0]
}

// BNPackDigestGnark mirrors [BNPackOctuplet] for an octuplet that encodes a
// BN digest ([BNDigestToOctuplet]), and is cheaper than [BNPackOctupletGnark]:
// after the weak reduction each limb is range-checked to 30 bits, which makes
// it its canonical value (< 2^30 < p) without the canonical check. An
// octuplet with a limb >= 2^30 is never a digest encoding and is rejected.
func BNPackDigestGnark(api frontend.API, o circuit.Octuplet) frontend.Variable {
	koalaAPI := circuit.NewAPI(api)
	rc := rangecheck.New(api)
	var acc frontend.Variable = 0
	for i := range o {
		v := reducedLimb(koalaAPI, o[i])
		rc.Check(v, bnDigestLimbBits)
		acc = api.Add(api.Mul(acc, 1<<BNPackedBitsPerKoala), v)
	}
	return acc
}

// BNCompressOctupletGnark mirrors [BNCompressOctuplet] when both children are
// digest encodings, which holds for every Merkle node above the leaves. Use
// [BNCompressPackedGnark] with [BNPackOctupletGnark] for a child that is a raw
// octuplet (e.g. an FRI layer's extension value used as a leaf).
func BNCompressOctupletGnark(api frontend.API, left, right circuit.Octuplet) circuit.Octuplet {
	return BNCompressPackedGnark(api, BNPackDigestGnark(api, left), BNPackDigestGnark(api, right))
}

// BNCompressPackedGnark compresses two already packed children and encodes
// the result as an octuplet.
func BNCompressPackedGnark(api frontend.API, left, right frontend.Variable) circuit.Octuplet {
	return BNDigestToOctupletGnark(api, BNNodeCompressGnark(api, left, right))
}

// BNOctupletGnarkMDHasher mirrors [BNOctupletMDHasher].
type BNOctupletGnarkMDHasher struct {
	api frontend.API
	h   *BNPackedGnarkHasher
}

// NewBNOctupletGnarkMDHasher returns an empty in-circuit BN-packed leaf
// hasher.
func NewBNOctupletGnarkMDHasher(api frontend.API) *BNOctupletGnarkMDHasher {
	return &BNOctupletGnarkMDHasher{api: api, h: NewBNPackedGnarkHasher(api, BNPackedLengthSuffix)}
}

// Reset clears the buffer.
func (h *BNOctupletGnarkMDHasher) Reset() { h.h.Reset() }

// Write appends KoalaBear elements.
func (h *BNOctupletGnarkMDHasher) Write(data ...circuit.Element) { h.h.Write(data...) }

// WriteOctuplet appends octuplets.
func (h *BNOctupletGnarkMDHasher) WriteOctuplet(data ...circuit.Octuplet) { h.h.WriteOctuplet(data...) }

// Sum returns the octuplet encoding of the BN-packed digest.
func (h *BNOctupletGnarkMDHasher) Sum() circuit.Octuplet {
	return BNDigestToOctupletGnark(h.api, h.h.Sum())
}
