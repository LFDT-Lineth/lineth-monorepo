//go:build !fuzzlight

package keccakf

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

// a module definition method specifically for testing the theta submodule
func rhoTestingModule(
	// parameters for the wizard
	maxNumKeccakf int,
) (
	wizard.DefineFunc, // the define function of testing wizard
	func(
		traces keccak.PermTraces,
		runRet **wizard.ProverRuntime,
	) wizard.MainProverStep,
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

		// Initializes the input columns
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				for k := 0; k < numSlice; k++ {
					mod.Theta.AThetaSlicedBaseB[x][y][k] = comp.InsertCommit(
						round,
						deriveName("A_THETA_BASE2_SLICED", x, y, k),
						numRows(maxNumKeccakf),
					)
				}
			}
		}

		mod.Rho = newRho(comp, round, maxNumKeccakf, mod.Theta.AThetaSlicedBaseB)
	}

	prover := func(
		traces keccak.PermTraces,
		// pointers to access the runtime once they are available, when the
		// prover has been run. This allows the caller test to "open" the box
		// and checks that the assigned columns are consistent with the traces.
		runRet **wizard.ProverRuntime,
	) wizard.MainProverStep {
		return func(run *wizard.ProverRuntime) {
			*runRet = run

			// Number of permutation used for the current instance
			numKeccakf := len(traces.KeccakFInps)

			// If the number of keccakf constraints is larger than what the
			// module is sized for, then, we cannot prove everything.
			if numKeccakf > maxNumKeccakf {
				utils.Panic("Too many keccakf %v > %v", numKeccakf, maxNumKeccakf)
			}

			// Initializes the input columns
			aTheta := [5][5][numSlice][]field.Element{}
			for permId := 0; permId < numKeccakf; permId++ {
				state := traces.KeccakFInps[permId]

				for rnd := 0; rnd < keccak.NumRound; rnd++ {
					// Pre-permute using the theta transformation before running
					// the rho permutation.
					state.Theta()

					// Convert the state in sliced from in base 2
					for x := 0; x < 5; x++ {
						for y := 0; y < 5; y++ {
							z := state[x][y]
							zF := U64ToBaseX(z, &BaseBFr)

							// Slice decomposition
							slice := DecomposeFr(zF, BaseBPow4, numSlice)
							for k := 0; k < numSlice; k++ {
								// If the column is not already assigned, then
								// allocate it with the proper length.
								if aTheta[x][y][k] == nil {
									aTheta[x][y][k] = make(
										[]field.Element,
										numKeccakf*keccak.NumRound,
									)
								}

								r := keccak.NumRound*permId + rnd
								aTheta[x][y][k][r] = slice[k]
							}
						}
					}

					// Then finalize the permutation normally
					state.Rho()
					b := state.Pi()
					state.Chi(&b)
					state.Iota(rnd)
				}
			}

			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {
					for k := 0; k < numSlice; k++ {
						run.AssignColumn(
							mod.Theta.AThetaSlicedBaseB[x][y][k].GetColID(),
							smartvectors.RightZeroPadded(
								aTheta[x][y][k],
								numRows(maxNumKeccakf),
							),
						)
					}
				}
			}

			// Then assigns all the columns of the rho module
			mod.Rho.assign(run, mod.Theta.AThetaSlicedBaseB, numKeccakf)
		}
	}

	return builder, prover, mod
}

// Test the correctness of the theta wizard function
func TestRho(t *testing.T) {

	const numCases int = 30
	maxKeccaf := 10

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))

	// Every time the prover function is called, the traces will be updated.
	// Likewise, run will be set by the prover.
	var run *wizard.ProverRuntime

	// Parametrizes the wizard and the input generator.
	provider := testInputProvider(rnd, maxKeccaf)
	builder, prover, mod := rhoTestingModule(maxKeccaf)

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
		base2 := field.NewElement(uint64(BaseB))

		for permId := 0; permId < effNumKeccak; permId++ {

			// Copy the corresponding input state and apply the theta
			// transformation.
			expectedARho := traces.KeccakFInps[permId]
			expectedARho.Theta()
			expectedARho.Rho()

			// Reconstruct the same state from the assignment of the prover
			reconstructed := keccak.State{}
			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {
					// Recompose the slice into a complete base 2 representation
					// of aTheta[x][y] in base 2
					recomposed := mod.Rho.ARho[x][y].GetColAssignmentAt(
						run,
						permId*keccak.NumRound,
					)
					// And cast it back to a u64
					reconstructed[x][y] = BaseXToU64(recomposed, &base2)
				}
			}

			assert.Equal(t, expectedARho, reconstructed,
				"could not reconstruct the state. permutation %v", permId)

			// Exiting on the first failed case to not spam the test logs
			if t.Failed() {
				t.Fatalf("stopping here as we encountered errors")
			}
		}
	}
}
