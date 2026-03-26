//go:build !fuzzlight

package keccakf

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/specialqueries"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/assert"
)

func keccakfTestingModule(
	maxNumKeccakf int,
) (
	define wizard.DefineFunc,
	prover func(t *testing.T, traces keccak.PermTraces) wizard.MainProverStep,
) {

	mod := &Module{}
	round := 0 // The round is always 0

	// The testing wizard uniquely calls the keccakf module
	define = func(b *wizard.Builder) {
		// This declares all the columns of the keccakf module
		*mod = NewModule(b.CompiledIOP, round, maxNumKeccakf)
	}

	// And the prover (instanciated for traces) is called
	prover = func(
		t *testing.T,
		traces keccak.PermTraces,
	) wizard.MainProverStep {
		return func(run *wizard.ProverRuntime) {
			// Assigns the module
			mod.Assign(run, traces)

			// Asserts that the last value in aIota is the correct one. `pos` is
			// the last active row of the module (given the traces we got). We
			// use it to reconstruct what the module "believes" to be the final
			// keccak state. Then, we compare this value with one generated in
			// the traces.
			numPerm := len(traces.KeccakFInps)
			pos := numPerm*keccak.NumRound - 1
			expectedState := traces.KeccakFOuts[numPerm-1]
			extractedState := keccak.State{}
			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {
					z := mod.
						PiChiIota.
						AIotaBaseB[x][y].
						GetColAssignmentAt(run, pos)
					extractedState[x][y] = BaseXToU64(z, &BaseBFr, 1)
				}
			}

			assert.Equal(t, expectedState, extractedState)
		}
	}

	return define, prover
}

func TestKeccakf(t *testing.T) {

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	numCases := 15
	maxNumKeccakf := 5
	// The -1 is here to prevent the generation of a padding block
	maxInputSize := maxNumKeccakf*keccak.Rate - 1

	definer, prover := keccakfTestingModule(maxNumKeccakf)
	comp := wizard.Compile(definer, dummy.Compile)

	for i := 0; i < numCases; i++ {
		// Generate a random piece of data
		dataSize := rng.IntN(maxInputSize + 1)
		data := make([]byte, dataSize)
		utils.ReadPseudoRand(rng, data)

		// Generate permutation traces for the data
		traces := keccak.PermTraces{}
		keccak.Hash(data, &traces)

		proof := wizard.Prove(comp, prover(t, traces))
		assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
	}
}

func BenchmarkDataTransferModule(b *testing.B) {
	b.Skip()
	maxNumKeccakF := []int{
		1 << 13,
		// 1 << 16,
		// 1 << 18,
		// 1 << 20,
	}
	once := &sync.Once{}

	for _, numKeccakF := range maxNumKeccakF {

		b.Run(fmt.Sprintf("%v-numKeccakF", numKeccakF), func(b *testing.B) {

			define := func(build *wizard.Builder) {
				comp := build.CompiledIOP
				mod := &Module{}
				*mod = NewModule(comp, 0, numKeccakF)
			}

			var (
				compiled = wizard.Compile(
					define,
					specialqueries.RangeProof,
					specialqueries.CompileFixedPermutations,
					permutation.CompileViaGrandProduct,
					logderivativesum.CompileLookups,
					innerproduct.Compile(),
				)
				numCells = 0
				numCols  = 0
			)

			for _, colID := range compiled.Columns.AllKeys() {
				numCells += compiled.Columns.GetSize(colID)
				numCols += 1
			}

			b.ReportMetric(float64(numCells), "#cells")
			b.ReportMetric(float64(numCols), "#columns")

			once.Do(func() {

				for _, colID := range compiled.Columns.AllKeys() {
					fmt.Printf("%v, %v\n", colID, compiled.Columns.GetSize(colID))
				}

			})

		})

	}
}
