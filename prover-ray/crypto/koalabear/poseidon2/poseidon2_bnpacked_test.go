package poseidon2

import (
	"fmt"
	"testing"

	kbcircuit "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

type bnPackedCircuit struct {
	Inputs []kbcircuit.Element
	Output frontend.Variable
}

func (c *bnPackedCircuit) Define(api frontend.API) error {
	h := NewBNPackedGnarkHasher(api)
	h.Write(c.Inputs...)
	api.AssertIsEqual(h.Sum(), c.Output)
	return nil
}

func randKoala(n int) []field.Element {
	vals := make([]field.Element, n)
	for i := range vals {
		if _, err := vals[i].SetRandom(); err != nil {
			panic(err)
		}
	}
	return vals
}

func TestBNPackedHasherCircuit(t *testing.T) {
	for _, n := range []int{0, 1, 7, 8, 9, 16, 23} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			vals := randKoala(n)
			nh := NewBNPackedHasher()
			nh.Write(vals...)
			digest := nh.Sum()

			circuit := &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n)}
			witness := &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n), Output: digest}
			for i := range vals {
				witness.Inputs[i] = kbcircuit.NewElementFromKoala(vals[i])
			}
			require.NoError(t, test.IsSolved(circuit, witness, ecc.BN254.ScalarField()))

			// wrong digest must fail
			var bad = digest
			bad.SetUint64(42)
			witness.Output = bad
			require.Error(t, test.IsSolved(circuit, witness, ecc.BN254.ScalarField()))
		})
	}
}

func TestBNPackedHasherInjective(t *testing.T) {
	var x field.Element
	x.SetUint64(5)
	var zero field.Element

	h1 := NewBNPackedHasher()
	h1.Write(x)
	h2 := NewBNPackedHasher()
	h2.Write(zero, x)
	a, b := h1.Sum(), h2.Sum()
	require.False(t, a.Equal(&b))
}

// TestBNPackedVsEmulatedConstraints prints the PLONK constraint counts of the
// emulated KoalaBear Poseidon2 hasher vs the BN-packed one.
func TestBNPackedVsEmulatedConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles circuits")
	}
	for _, n := range []int{8, 64, 512} {
		emu := &KoalagnarkMDHasherCircuit{Inputs: make([]kbcircuit.Element, n)}
		csEmu, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, emu)
		require.NoError(t, err)

		packed := &bnPackedCircuit{Inputs: make([]kbcircuit.Element, n)}
		csPacked, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, packed)
		require.NoError(t, err)

		t.Logf("n=%4d  emulated=%9d  bn-packed=%9d  ratio=%.1fx",
			n, csEmu.GetNbConstraints(), csPacked.GetNbConstraints(),
			float64(csEmu.GetNbConstraints())/float64(csPacked.GetNbConstraints()))
	}
}
