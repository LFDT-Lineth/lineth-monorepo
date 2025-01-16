package keccak_test

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/require"
)

func TestTraces(t *testing.T) {

	numCases := 100
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	maxSize := 1024

	for i := 0; i < numCases; i++ {

		// Populate a random string
		data := make([]byte, rng.IntN(maxSize))
		utils.ReadPseudoRand(rng, data)

		// Initialize an empty trace
		traces := keccak.PermTraces{}

		// Pass some traces
		outWoTraces := keccak.Hash(data)
		outWiTraces := keccak.Hash(data, &traces)
		lastKeccakfOut := traces.KeccakFOuts[len(traces.KeccakFOuts)-1]

		require.Equal(
			t, len(traces.Blocks), len(traces.KeccakFInps),
			"inconsistent number of block and keccakf inputs",
		)

		require.Equal(
			t, len(traces.Blocks), len(traces.KeccakFOuts),
			"inconsistent number of block and keccakf outputs",
		)

		require.Equal(
			t, outWoTraces, outWiTraces,
			"passing the traces changes the result",
		)

		require.Equal(
			t, outWiTraces, lastKeccakfOut.ExtractDigest(),
			"the output from the traces does not match the output of hash",
		)

		// For each entry of the traces, check that the input and the output
		// of the transformation are consistents.
		for p := range traces.KeccakFInps {
			in := traces.KeccakFInps[p]
			out := traces.KeccakFOuts[p]
			in.Permute(nil)
			require.Equal(t, in, out, "in and out are inconsistent")
		}

	}
}
