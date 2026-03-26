package keccakfkoalabear

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
	"github.com/stretchr/testify/assert"
)

func TestTheta(t *testing.T) {

	const numCases int = 30
	maxKeccaf := 10

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))

	// Every time the prover function is called, the traces will be updated.
	// Likewise, run will be set by the prover.
	var run *wizard.ProverRuntime

	// Parametrizes the wizard and the input generator.
	builder, prover, mod := thetaTestingModule(maxKeccaf)

	comp := wizard.Compile(builder, dummy.Compile)

	for i := 0; i < numCases; i++ {

		// Generate new traces
		traces := genKeccakfTrace(rnd, maxKeccaf)

		// Recall that this will set the values of `traces` and ``
		proof := wizard.Prove(comp, prover(traces, &run))
		assert.NoErrorf(t, wizard.Verify(comp, proof), "verifier failed")

		effNumKeccak := len(traces.KeccakFInps)

		for permId := 0; permId < effNumKeccak; permId++ {

			// Copy the corresponding input state and apply the rho
			// transformation.
			state := traces.KeccakFInps[permId]
			state.Theta()

			// Reconstruct the same state from the assignment of the prover
			reconstructed := keccak.State{}
			recomposed := [64]field.Element{}
			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {
					for z := 0; z < 64; z++ {
						// Recompose the slice into a complete base 2 representation
						recomposed[z] = mod.Theta.StateNext[x][y][z].GetColAssignmentAt(run,
							permId*keccak.NumRound)

					}
					reconstructed[x][y] = reconstructU64(recomposed)
				}
			}

			assert.Equal(t, state, reconstructed,
				"could not reconstruct the state. permutation %v", permId)

			// Exiting on the first failed case to not spam the test logs
			if t.Failed() {
				t.Fatalf("stopping here as we encountered errors")
			}
		}
	}
}

// a module definition method specifically for testing the rho submodule
func thetaTestingModule(
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
	var (
		mod       = &Module{}
		size      = int(utils.NextPowerOfTwo(uint64(maxNumKeccakf) * common.NumRounds))
		stateCurr = state{} // input to the theta module base 4
	)

	/*
		Initializes the builder function
	*/
	builder := func(b *wizard.Builder) {
		comp := b.CompiledIOP

		// Initializes the input current state
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				for z := 0; z < 8; z++ {
					stateCurr[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("THETA_STATE_CURR_%v_%v_%v", x, y, z), size, true)
				}
			}
		}

		mod.Theta = newTheta(comp, size, stateCurr)
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
			stateCurrWit := [5][5][8][]field.Element{}
			for permId := 0; permId < numKeccakf; permId++ {
				state := traces.KeccakFInps[permId]

				for rnd := 0; rnd < keccak.NumRound; rnd++ {
					// Convert the state in sliced from in base 2
					for x := 0; x < 5; x++ {
						for y := 0; y < 5; y++ {
							a := stateBase4(state[x][y])
							for k := 0; k < 8; k++ {
								// If the column is not already assigned, then
								// allocate it with the proper length.
								if stateCurrWit[x][y][k] == nil {
									stateCurrWit[x][y][k] = make(
										[]field.Element,
										size,
									)
								}

								r := keccak.NumRound*permId + rnd
								stateCurrWit[x][y][k][r] = field.NewElement(uint64(a[k]))
							}
						}
					}
				}
			}

			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {
					for k := 0; k < 8; k++ {
						run.AssignColumn(
							stateCurr[x][y][k].GetColID(),
							smartvectors.RightZeroPadded(
								stateCurrWit[x][y][k],
								size,
							),
						)
					}
				}
			}

			// Then assign all the columns of the theta module
			mod.Theta.assignTheta(run, stateCurr)
		}
	}

	return builder, prover, mod
}

// in convert uint64 to base 4 representation stored in 8 uint32
func stateBase4(in uint64) [8]uint32 {
	var res [8]uint32
	for i := 0; i < 8; i++ {
		var v uint32
		for k := 0; k < 8; k++ {
			bit := (in >> (8*i + k)) & 1
			v += uint32(bit) << (2 * k) // multiply by 4^k = 2^(2k)
		}
		res[i] = v
	}
	return res
}
