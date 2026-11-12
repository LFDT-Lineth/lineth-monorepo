package multisethashing

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

// hashGnarkCircuit checks that HashGnark(Root) equals the Expected digest.
type hashGnarkCircuit struct {
	Root     circuit.Octuplet
	Expected [MSetHashSizeNumFieldElement]circuit.Element
}

// Define hashes Root in-circuit and asserts each element equals Expected.
func (c *hashGnarkCircuit) Define(api frontend.API) error {
	koalaAPI := circuit.NewAPI(api)
	got := HashGnark(api, c.Root)
	for i := range got {
		koalaAPI.AssertIsEqual(c.Expected[i], got[i])
	}
	return nil
}

// hashGnarkWitness builds an assignment from a random root and its native [Hash].
func hashGnarkWitness(t *testing.T) *hashGnarkCircuit {
	t.Helper()
	var root field.Octuplet
	for i := range root {
		_, err := root[i].SetRandom()
		require.NoError(t, err)
	}
	digest := Hash(root)

	var w hashGnarkCircuit
	for i := range root {
		w.Root[i] = circuit.NewElementFromKoala(root[i])
	}
	for i := range digest {
		w.Expected[i] = circuit.NewElementFromKoala(digest[i])
	}
	return &w
}

// TestHashGnark checks that [HashGnark] matches the native [Hash] element for
// element, and that a single tampered element is rejected.
func TestHashGnark(t *testing.T) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &hashGnarkCircuit{})
	require.NoError(t, err)
	t.Logf("constraints: %d", ccs.GetNbConstraints())

	w := hashGnarkWitness(t)
	require.NoError(t, test.IsSolved(&hashGnarkCircuit{}, w, ecc.BN254.ScalarField()))

	bad := *w
	var tampered field.Element
	tampered.SetUint64(1)
	bad.Expected[MSetHashSizeNumFieldElement-1] = circuit.NewElementFromKoala(tampered)
	require.Error(t, test.IsSolved(&hashGnarkCircuit{}, &bad, ecc.BN254.ScalarField()))
}
