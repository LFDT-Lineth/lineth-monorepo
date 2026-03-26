package keccak_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

// Sizes of the inputs for which we want to compare the results of keccak.
var sizes = []int{0, 1, 135, 136, 137, 2*136 - 1, 2 * 136, 2*136 + 1}

// Test the toy implementation against crypto/sha3 as a reference
func TestFullHashAgainstRef(t *testing.T) {

	// Create a deterministic random number generator for reproducibility.
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	refHasher := sha3.NewLegacyKeccak256()
	numCases := 50

	// One subtest per size
	for _, s := range sizes {
		subTestName := fmt.Sprintf("testcase-size-%d", s)
		t.Run(subTestName, func(t *testing.T) {

			for _n := 0; _n < numCases; _n++ {
				// Populate the sample with random values
				inp := make([]byte, s)
				_, err := utils.ReadPseudoRand(rng, inp)
				require.NoError(t, err)

				// Hash the input stream using the reference hasher
				refHasher.Reset()
				refHasher.Write(inp)
				outRef := refHasher.Sum(nil)

				// Hash the input stream using our local implementation
				oldInp := append([]byte{}, inp...)
				outOur := keccak.Hash(inp, nil)

				// The input should be left unchanged by the hashing operation
				require.Equalf(t, outRef, outOur[:], "mismatch with reference")
				require.Equalf(t, oldInp, inp, "the input has changed")
			}
		})
	}
}

// Test the toy implementation against crypto/sha3 as a reference using
// random sized strings.
func TestFullHashAgainstRefFullRand(t *testing.T) {

	// Create a deterministic random number generator for reproducibility.
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	refHasher := sha3.NewLegacyKeccak256()
	numCases := 500
	maxSize := 1024

	for _n := 0; _n < numCases; _n++ {
		// Populate the sample with random values
		inp := make([]byte, rng.IntN(maxSize))
		_, err := utils.ReadPseudoRand(rng, inp)
		require.NoError(t, err)

		// Hash the input stream using the reference hasher
		refHasher.Reset()
		refHasher.Write(inp)
		outRef := refHasher.Sum(nil)

		// Hash the input stream using our local implementation
		oldInp := append([]byte{}, inp...)
		outOur := keccak.Hash(inp)

		// The input should be left unchanged by the hashing operation
		require.Equalf(t, outRef, outOur[:], "mismatch with reference")
		require.Equalf(t, oldInp, inp, "the input has changed")
	}

}
