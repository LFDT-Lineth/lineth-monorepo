package keccakfkoalabear

import (
	"encoding/binary"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	kcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/iokeccakf"
	"github.com/stretchr/testify/assert"
)

func TestKeccakf(t *testing.T) {

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	numCases := 30
	maxNumKeccakf := 2
	// The -1 is here to prevent the generation of a padding block
	maxInputBytes := maxNumKeccakf*keccak.Rate - 1

	definer, prover := keccakfTestingModule(maxNumKeccakf)
	comp := wizard.Compile(definer, dummy.Compile)

	for i := 0; i < numCases; i++ {
		// Generate a random piece of data
		dataSize := rng.IntN(maxInputBytes + 1)
		data := make([]byte, dataSize)
		utils.ReadPseudoRand(rng, data)

		// Generate permutation traces for the data
		traces := keccak.PermTraces{}
		keccak.Hash(data, &traces)

		proof := wizard.Prove(comp, prover(t, traces))
		assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
	}
}

func keccakfTestingModule(
	maxNumKeccakf int,
) (
	define wizard.DefineFunc,
	prover func(t *testing.T, traces keccak.PermTraces) wizard.MainProverStep,
) {

	var (
		mod    = &Module{}
		size   = NumRows(maxNumKeccakf)
		blocks = make([][kcommon.NumSlices]ifaces.Column, kcommon.NumLanesInBlock)
	)

	// The testing wizard uniquely calls the keccakf module
	define = func(b *wizard.Builder) {

		comp := b.CompiledIOP
		for m := 0; m < kcommon.NumLanesInBlock; m++ {
			for z := 0; z < kcommon.NumSlices; z++ {
				blocks[m][z] = comp.InsertCommit(0, ifaces.ColIDf("BLOCK_%v_%v", m, z), size, true)
			}
		}

		mod = NewModule(b.CompiledIOP, KeccakfInputs{
			Blocks:       blocks,
			IsBlock:      comp.InsertCommit(0, "IS_BLOCK", size, true),
			IsFirstBlock: comp.InsertCommit(0, "IS_FIRST_BLOCK", size, true),
			IsBlockBaseB: comp.InsertCommit(0, "IS_BLOCK_BASEB", size, true),
			IsActive:     comp.InsertCommit(0, "IS_ACTIVE", size, true),
			KeccakfSize:  size,
		})
	}

	// And the prover (instanciated for traces) is called
	prover = func(
		t *testing.T,
		traces keccak.PermTraces,
	) wizard.MainProverStep {
		return func(run *wizard.ProverRuntime) {
			// assign the input columns
			var (
				keccakfBlocks = iokeccakf.KeccakFBlocks{
					Blocks:        mod.Inputs.Blocks,
					IsBlockActive: mod.Inputs.IsActive,
					IsBlock:       mod.Inputs.IsBlock,
					IsFirstBlock:  mod.Inputs.IsFirstBlock,
					IsBlockBaseB:  mod.Inputs.IsBlockBaseB,
					KeccakfSize:   mod.Inputs.KeccakfSize,
				}
			)

			keccakfBlocks.AssignBlocks(run, traces)
			keccakfBlocks.AssignBlockFlags(run, traces)

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
					var a [8]uint8
					for z := 0; z < kcommon.NumSlices; z++ {
						v := mod.BackToThetaOrOutput.StateNext[x][y][z].GetColAssignmentAt(run, pos)
						a[z] = uint8(v.Uint64())
					}
					extractedState[x][y] = binary.LittleEndian.Uint64(a[:])
				}
			}

			assert.Equal(t, expectedState, extractedState)
		}
	}

	return define, prover
}
