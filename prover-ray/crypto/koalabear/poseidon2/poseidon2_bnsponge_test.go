package poseidon2

import (
	"fmt"
	"testing"

	kbcircuit "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

type bnSpongeCircuit struct {
	Inputs []kbcircuit.Element
	Output frontend.Variable
}

func (c *bnSpongeCircuit) Define(api frontend.API) error {
	h := NewBNSpongeGnarkHasher(api, BNSpongeLengthInCapacity)
	h.Write(c.Inputs...)
	api.AssertIsEqual(h.Sum(), c.Output)
	return nil
}

func bnSpongeNative(vals []field.Element) fr.Element {
	h := NewBNSpongeHasher(BNSpongeLengthInCapacity)
	h.Write(vals...)
	return h.Sum()
}

// TestBNSpongeCircuitMatchesNative checks native vs circuit digests on lengths
// around the chunk (8) and block (16) boundaries.
func TestBNSpongeCircuitMatchesNative(t *testing.T) {
	for _, n := range []int{1, 7, 8, 9, 15, 16, 17, 24, 32, 33, 100} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			vals := randKoala(n)
			vals[0].SetUint64(2130706432) // p-1: largest slot value
			digest := bnSpongeNative(vals)

			circuit := &bnSpongeCircuit{Inputs: make([]kbcircuit.Element, n)}
			witness := &bnSpongeCircuit{Inputs: make([]kbcircuit.Element, n), Output: digest}
			for i := range vals {
				witness.Inputs[i] = kbcircuit.NewElementFromKoala(vals[i])
			}
			require.NoError(t, test.IsSolved(circuit, witness, ecc.BN254.ScalarField()))

			bad := digest
			bad.SetUint64(42)
			witness.Output = bad
			require.ErrorContains(t, test.IsSolved(circuit, witness, ecc.BN254.ScalarField()), "bnSpongeCircuit).Define")
		})
	}
}

// TestBNSpongeLengthSeparation checks that zero padding does not cause
// collisions across lengths, at both the packing and the block level.
func TestBNSpongeLengthSeparation(t *testing.T) {
	var x, zero field.Element
	x.SetUint64(5)

	block := randKoala(16)
	cases := map[string][]field.Element{
		"[x]":         {x},
		"[0, x]":      {zero, x},
		"[x, 0]":      {x, zero},
		"block":       block,
		"block + [0]": append(append([]field.Element{}, block...), zero),
		"block[:8]":   block[:8],
		"block[:8]+0": append(append([]field.Element{}, block[:8]...), zero),
	}
	seen := map[fr.Element]string{}
	for name, in := range cases {
		d := bnSpongeNative(in)
		if other, ok := seen[d]; ok {
			t.Fatalf("collision between %s and %s", name, other)
		}
		seen[d] = name
	}
}

func TestBNSpongeRejectsEmpty(t *testing.T) {
	require.PanicsWithValue(t, "poseidon2: BN sponge with length in capacity cannot hash an empty input",
		func() { bnSpongeNative(nil) })
}

type bnNodeCircuit struct {
	Left, Right, Output frontend.Variable
}

func (c *bnNodeCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(BNNodeCompressGnark(api, c.Left, c.Right), c.Output)
	return nil
}

func TestBNNodeCompressCircuitMatchesNative(t *testing.T) {
	var l, r fr.Element
	_, _ = l.SetRandom()
	_, _ = r.SetRandom()
	out := BNNodeCompress(l, r)
	require.NoError(t, test.IsSolved(&bnNodeCircuit{},
		&bnNodeCircuit{Left: l, Right: r, Output: out}, ecc.BN254.ScalarField()))
	require.ErrorContains(t, test.IsSolved(&bnNodeCircuit{},
		&bnNodeCircuit{Left: r, Right: l, Output: out}, ecc.BN254.ScalarField()), "bnNodeCircuit).Define")
}

// TestBNSpongeConstraints prints PLONK constraint counts of the emulated
// KoalaBear hasher, the width-2 BN-packed Merkle-Damgard hasher and the
// width-3 BN-packed sponge, plus one node compression.
func TestBNSpongeConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles circuits")
	}
	count := func(c frontend.Circuit) int {
		cs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, c)
		require.NoError(t, err)
		return cs.GetNbConstraints()
	}
	for _, n := range []int{8, 16, 64, 512} {
		emu := count(&KoalagnarkMDHasherCircuit{Inputs: make([]kbcircuit.Element, n)})
		mdSuffix := count(&bnPackedCircuit{Inputs: make([]kbcircuit.Element, n), Domain: BNPackedLengthSuffix})
		mdIV := count(&bnPackedCircuit{Inputs: make([]kbcircuit.Element, n), Domain: BNPackedLengthInIV})
		sp := count(&bnSpongeCircuit{Inputs: make([]kbcircuit.Element, n)})
		t.Logf("n=%4d  emulated=%8d  width2-MD(len suffix)=%6d  width2-MD(len in IV)=%6d  width3-sponge=%6d  (IV vs emulated %.1fx)",
			n, emu, mdSuffix, mdIV, sp, float64(emu)/float64(mdIV))
	}
	t.Logf("node compress (width 2): %d", count(&bnNodeCircuit{}))
}
