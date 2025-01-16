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
func piChiIotaTestingModule(
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

		// Initializes the Rho columns
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				// The output of the rho module that serves as input for the
				// chi module.
				mod.rho.aRho[x][y] = comp.InsertCommit(
					round,
					deriveName("A_RHO", x, y),
					numRows(maxNumKeccakf),
				)
			}
		}

		mod.lookups = newLookUpTables(comp, maxNumKeccakf)
		mod.IO.declareColumnsInput(comp, maxNumKeccakf)
		mod.piChiIota = newPiChiIota(comp, round, maxNumKeccakf, *mod)
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

			// Initializes the input columns
			aRho := [5][5][]field.Element{}
			colSize := numKeccakf * keccak.NumRound
			for permId := 0; permId < numKeccakf; permId++ {
				state := traces.KeccakFInps[permId]

				for rnd := 0; rnd < keccak.NumRound; rnd++ {
					// Pre-permute using the theta transformation before running
					// the rho permutation.
					state.Theta()
					state.Rho()

					// Convert the state in sliced from in base 2
					for x := 0; x < 5; x++ {
						for y := 0; y < 5; y++ {
							// If the column is not already assigned, then
							// allocate it with the proper length.
							if aRho[x][y] == nil {
								aRho[x][y] = make([]field.Element, colSize)
							}

							r := keccak.NumRound*permId + rnd
							aRho[x][y][r] = U64ToBaseX(state[x][y], &BaseBFr)
						}
					}

					// Then finalize the permutation normally
					b := state.Pi()
					state.Chi(&b)
					state.Iota(rnd)
				}
			}

			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {
					run.AssignColumn(
						mod.rho.aRho[x][y].GetColID(),
						smartvectors.RightZeroPadded(
							aRho[x][y],
							numRows(maxNumKeccakf),
						),
					)
				}
			}

			// Then assigns all the columns of the rho module
			mod.assignStateAndBlocks(run, traces, numKeccakf)
			mod.IO.assignBlockFlags(run, traces)
			mod.piChiIota.assign(run, numKeccakf, mod.lookups, mod.rho.aRho,
				mod.Blocks, mod.IO.IsBlockBaseB)
		}
	}

	return builder, prover, mod
}

// Test the correctness of the theta wizard function
func TestPiChiIota(t *testing.T) {

	const numCases int = 30
	maxKeccaf := 10

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))

	// Every time the prover function is called, the traces will be updated.
	// Likewise, run will be set by the prover.
	var run *wizard.ProverRuntime

	// Parametrizes the wizard and the input generator.
	provider := testInputProvider(rnd, maxKeccaf)
	builder, prover, mod := piChiIotaTestingModule(maxKeccaf)

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

		for permId := 0; permId < effNumKeccak; permId++ {

			// Copy the corresponding input state and apply the theta
			// transformation. We only check the round zero of the permutation
			expectedAIota := traces.KeccakFInps[permId]

			for rnd := 0; rnd < keccak.NumRound; rnd++ {

				pos := permId*keccak.NumRound + rnd
				expectedAIota.ApplyKeccakfRound(rnd)

				// In that case, aIOTA should be in fact the next input because
				// the iota step will be responsible for xoring in the inputs.
				if rnd == 23 && permId+1 < len(traces.KeccakFInps) {
					if !traces.IsNewHash[permId+1] {
						expectedAIota = traces.KeccakFInps[permId+1]
					}
				}

				// Reconstruct the same state from the assignment of the prover
				aIotaFromModeBase1Sliced := keccak.State{}
				for x := 0; x < 5; x++ {
					for y := 0; y < 5; y++ {
						// Extract the slices
						slice := [numSlice]field.Element{}
						for k := 0; k < numSlice; k++ {
							colid := mod.piChiIota.aIotaBaseASliced[x][y][k].GetColID()
							slice[k] = run.GetColumnAt(colid, pos)
						}

						// Recompose the slice into a complete base 1 representation
						// of aTheta[x][y] in base 1
						recomposed := BaseRecompose(slice[:], &BaseAPow4Fr)

						// And cast it back to a u64
						aIotaFromModeBase1Sliced[x][y] = BaseXToU64(recomposed, &BaseAFr)
					}
				}

				assert.Equal(t, expectedAIota, aIotaFromModeBase1Sliced,
					"could not reconstruct the state. permutation %v/ round %v",
					permId, rnd,
				)
			}

			// Exiting on the first failed case to not (over) spam the test logs
			if t.Failed() {
				t.Fatalf("stopping here as we encountered errors")
			}
		}
	}
}
