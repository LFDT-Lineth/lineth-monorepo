package poseidon2

import (
	"math/big"
	"testing"

	kbcircuit "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

func randDigestOctuplet() field.Octuplet {
	var d fr.Element
	_, _ = d.SetRandom()
	return BNDigestToOctuplet(d)
}

// TestBNCompressOctupletRejectsNonDigest checks that a child limb >= 2^30,
// which no digest encoding has, is rejected.
//
// The 30-bit range check makes each limb canonical (< 2^30 < p), so a prover
// cannot pass limb + p, which packs differently, in place of limb.
func TestBNCompressOctupletRejectsNonDigest(t *testing.T) {
	l, r := randDigestOctuplet(), randDigestOctuplet()
	// Override only the first limb of l with 2^30, one past the 30-bit digest
	// limb range. l[1..7] and all of r stay valid digest limbs, so the range
	// check on l[0] is the only thing that can fail.
	l[0].SetUint64(1 << 30)
	out := BNCompressOctuplet(l, r)
	w := &bnCompressOctCircuit{Left: octupletWitness(l), Right: octupletWitness(r), Output: octupletWitness(out)}
	require.ErrorContains(t, test.IsSolved(&bnCompressOctCircuit{}, w, ecc.BN254.ScalarField()), "rangecheck")
}

// bnChainCircuit chains N node compressions, to measure the marginal cost of
// one node without the range checker's fixed cost.
type bnChainCircuit struct {
	Start, Sib kbcircuit.Octuplet
	N          int  `gnark:"-"`
	Emulated   bool `gnark:"-"`
}

func (c *bnChainCircuit) Define(api frontend.API) error {
	kb := kbcircuit.NewAPI(api)
	cur := c.Start
	for range c.N {
		if c.Emulated {
			cur = KoalagnarkCompress(kb, cur, c.Sib)
		} else {
			cur = BNCompressOctupletGnark(api, cur, c.Sib)
		}
	}
	kb.AssertIsDifferent(cur[0], kb.Const(-1))
	return nil
}

func octupletWitness(o field.Octuplet) kbcircuit.Octuplet {
	var res kbcircuit.Octuplet
	for i := range o {
		res[i] = kbcircuit.NewElementFromKoala(o[i])
	}
	return res
}

func TestBNDigestToOctuplet(t *testing.T) {
	for range 20 {
		var d fr.Element
		_, _ = d.SetRandom()
		o := BNDigestToOctuplet(d)

		// reference: low 240 bits, 30 per limb, highest first
		var bi big.Int
		d.BigInt(&bi)
		for i := 7; i >= 0; i-- {
			want := new(big.Int).And(&bi, big.NewInt(1<<30-1)).Uint64()
			require.Equal(t, want, o[i].Uint64())
			bi.Rsh(&bi, 30)
		}
	}
}

type bnCompressOctCircuit struct {
	Left, Right, Output kbcircuit.Octuplet
}

func (c *bnCompressOctCircuit) Define(api frontend.API) error {
	kb := kbcircuit.NewAPI(api)
	res := BNCompressOctupletGnark(api, c.Left, c.Right)
	for i := range res {
		kb.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestBNCompressOctupletCircuit(t *testing.T) {
	l, r := randDigestOctuplet(), randDigestOctuplet()
	out := BNCompressOctuplet(l, r)
	w := &bnCompressOctCircuit{Left: octupletWitness(l), Right: octupletWitness(r), Output: octupletWitness(out)}
	require.NoError(t, test.IsSolved(&bnCompressOctCircuit{}, w, ecc.BN254.ScalarField()))

	w.Left, w.Right = w.Right, w.Left
	require.ErrorContains(t, test.IsSolved(&bnCompressOctCircuit{}, w, ecc.BN254.ScalarField()), "bnCompressOctCircuit).Define")
}

type bnOctLeafCircuit struct {
	Inputs []kbcircuit.Element
	Output kbcircuit.Octuplet
}

func (c *bnOctLeafCircuit) Define(api frontend.API) error {
	kb := kbcircuit.NewAPI(api)
	h := NewBNOctupletGnarkMDHasher(api)
	h.Write(c.Inputs...)
	res := h.Sum()
	for i := range res {
		kb.AssertIsEqual(res[i], c.Output[i])
	}
	return nil
}

func TestBNOctupletLeafCircuit(t *testing.T) {
	for _, n := range []int{1, 8, 11, 40} {
		vals := randKoala(n)
		h := NewBNOctupletMDHasher()
		h.WriteElements(vals...)
		out := h.SumDigest()

		w := &bnOctLeafCircuit{Inputs: make([]kbcircuit.Element, n), Output: octupletWitness(out)}
		for i := range vals {
			w.Inputs[i] = kbcircuit.NewElementFromKoala(vals[i])
		}
		require.NoError(t, test.IsSolved(&bnOctLeafCircuit{Inputs: make([]kbcircuit.Element, n)}, w, ecc.BN254.ScalarField()))
	}
}

// TestBNCompressOctupletConstraints compares one Merkle node in the circuit:
// emulated KoalaBear compression vs the BN-packed one on octuplet digests.
func TestBNCompressOctupletConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles circuits")
	}
	count := func(c frontend.Circuit) int {
		cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, c)
		require.NoError(t, err)
		return cs.GetNbConstraints()
	}
	marginal := func(emulated bool) int {
		return (count(&bnChainCircuit{N: 9, Emulated: emulated}) - count(&bnChainCircuit{N: 1, Emulated: emulated})) / 8
	}
	emu, bn := marginal(true), marginal(false)
	t.Logf("node compress (marginal per node): emulated=%d  bn-octuplet=%d  (%.1fx)", emu, bn, float64(emu)/float64(bn))
}

type bnDigestSplitCircuit struct {
	D frontend.Variable
}

func (c *bnDigestSplitCircuit) Define(api frontend.API) error {
	BNDigestToOctupletGnark(api, c.D)
	return nil
}

// TestBNDigestToOctupletRejectsNonCanonical checks that the circuit rejects
// the split of d + r, which also fits in 254 bits for small d and would give
// another octuplet than the native encoding of d.
func TestBNDigestToOctupletRejectsNonCanonical(t *testing.T) {
	w := &bnDigestSplitCircuit{D: 5}
	require.NoError(t, test.IsSolved(&bnDigestSplitCircuit{}, w, ecc.BN254.ScalarField()))

	// Replace the hint with one that splits d + r instead of d.
	plusR := func(m *big.Int, in, out []*big.Int) error {
		return bnDigestSplitHint(m, []*big.Int{new(big.Int).Add(in[0], fr.Modulus())}, out)
	}
	err := test.IsSolved(&bnDigestSplitCircuit{}, w, ecc.BN254.ScalarField(),
		test.WithReplacementHint(solver.GetHintID(bnDigestSplitHint), plusR))
	require.ErrorContains(t, err, "rangecheck")
}
