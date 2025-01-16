//go:build !fuzzlight

package keccakf

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

// a module definition method specifically for testing the theta submodule
func thetaTestingModule(
	// parameters for the wizard
	maxNumKeccakf int,
) (
	wizard.DefineFunc, // the define function of testing wizard
	func(
		traces keccak.PermTraces,
		runRet **wizard.ProverRuntime,
	) wizard.ProverStep,
	*Module,
) {

	// The module is only used a placeholder to let us the `assignInput`
	// function
	mod := &Module{}
	round := 0 // The round is always zero

	/*
		Initializes the builder function
	*/
	builder := func(b *wizard.Builder) {
		comp := b.CompiledIOP

		mod.declareColumns(comp, round, maxNumKeccakf)

		// Initializes the lookup columns
		mod.lookups = newLookUpTables(comp, maxNumKeccakf)

		// Then initializes the submodules : declare the colums and all the
		// constraints.
		mod.theta = newTheta(comp, round, maxNumKeccakf, mod.state, mod.lookups)
	}

	prover := func(
		traces keccak.PermTraces,
		// pointers to access the runtime once they are available, when the
		// prover has been run. This allows the caller test to "open" the box
		// and checks that the assigned columns are consistent with the traces.
		runRet **wizard.ProverRuntime,
	) wizard.ProverStep {
		return func(run *wizard.ProverRuntime) {
			*runRet = run

			// Number of permutation used for the current instance
			numKeccakf := len(traces.KeccakFInps)

			// If the number of keccakf constraints is larger than what the
			// module is sized for, then, we cannot prove everything.
			if numKeccakf > maxNumKeccakf {
				utils.Panic("Too many keccakf %v > %v", numKeccakf, maxNumKeccakf)
			}

			// Then assigns all the columns
			mod.assignStateAndBlocks(run, traces, numKeccakf)
			mod.theta.assign(run, mod.state, mod.lookups, numKeccakf)
		}
	}

	return builder, prover, mod
}

// Input provider for the tests. Return traces corresponding to random hashes
func testInputProvider(rnd *rand.Rand, maxNumKeccakf int) InputWitnessProvider {

	return func() keccak.PermTraces {
		res := keccak.PermTraces{}
		// The number of effective permutation is a random fraction of the
		// max number of keccakf.
		effNumKeccak := rnd.Int() % maxNumKeccakf
		for effNumKeccak == 0 {
			effNumKeccak = rnd.Int() % maxNumKeccakf
		}

		for len(res.Blocks) < effNumKeccak {
			// Each hash is for a random string taking at most 3 permutations
			streamLen := rnd.IntN(3*keccak.Rate-1) + 1
			stream := make([]byte, streamLen)
			utils.ReadPseudoRand(rnd, stream)

			for i := 0; i < effNumKeccak; i++ {
				keccak.Hash(stream, &res)
			}
		}

		// And trim a posteriori the excess permutation so that we have exactly
		// effNumKeccak. This will not be very realistic for the last permutation
		// since this will ignore the padding. Fortunately, the padding is out
		// of scope for this module. So, this should not matter in practice.
		res.Blocks = res.Blocks[:effNumKeccak]
		res.IsNewHash = res.IsNewHash[:effNumKeccak]
		res.KeccakFInps = res.KeccakFInps[:effNumKeccak]
		res.KeccakFOuts = res.KeccakFOuts[:effNumKeccak]

		return res
	}
}

// Test the correctness of the theta wizard function
func TestTheta(t *testing.T) {

	const numCases int = 30
	maxKeccaf := 10

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))

	// Every time the prover function is called, the traces will be updated.
	// Likewise, run will be set by the prover.
	var run *wizard.ProverRuntime

	// Parametrizes the wizard and the input generator.
	provider := testInputProvider(rnd, maxKeccaf)
	builder, prover, mod := thetaTestingModule(maxKeccaf)

	comp := wizard.Compile(builder, dummy.Compile)

	for i := 0; i < numCases; i++ {

		// Generate new traces
		traces := provider()

		// Recall that this will set the values of `traces` and ``
		proof := wizard.Prove(comp, prover(traces, &run))
		assert.NoErrorf(t, wizard.Verify(comp, proof), "verifier failed")

		// When we extract the slices representing aTheta in base 2, we should
		// recover the state after aTheta.
		effNumKeccak := len(traces.KeccakFInps)
		base2Pow4 := field.NewElement(uint64(BaseB * BaseB * BaseB * BaseB))
		base2 := field.NewElement(uint64(BaseB))

		for permId := 0; permId < effNumKeccak; permId++ {

			// Copy the corresponding input state and apply the theta
			// transformation.
			expectedATheta := traces.KeccakFInps[permId]
			expectedATheta.Theta()

			// Reconstruct the same state from the assignment of the prover
			reconstructed := keccak.State{}
			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {

					// Extract the slices
					slice := [numSlice]field.Element{}
					pos := permId * keccak.NumRound
					for k := 0; k < numSlice; k++ {
						colid := mod.theta.aThetaSlicedBaseB[x][y][k].GetColID()
						slice[k] = run.GetColumnAt(colid, pos)
					}

					// Recompose the slice into a complete base 2 representation
					// of aTheta[x][y] in base 2
					recomposed := BaseRecompose(slice[:], &base2Pow4)

					// And cast it back to a u64
					reconstructed[x][y] = BaseXToU64(recomposed, &base2)
				}
			}

			assert.Equal(t, expectedATheta, reconstructed,
				"could not reconstruct the state. permutation %v", permId)
		}

		// Exiting on the first failed case to not spam the test logs
		if t.Failed() {
			t.Fatalf("stopping here as we encountered errors")
		}
	}
}
