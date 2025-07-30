package keccakf

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

func MakeTestCaseInputOutputModule(maxNumKeccakF int) (
	define wizard.DefineFunc,
	prover func(permTrace keccak.PermTraces) wizard.MainProverStep,
) {
	round := 0
	mod := &Module{}
	mod.MaxNumKeccakf = maxNumKeccakF
	mod.State = [5][5]ifaces.Column{}
	define = func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		mod.Lookups = newLookUpTables(comp, maxNumKeccakF)
		mod.declareColumns(comp, round, maxNumKeccakF)
		mod.Theta.declareColumn(comp, round, maxNumKeccakF)
		mod.Rho.declareColumns(comp, round, maxNumKeccakF)
		mod.PiChiIota.declareColumns(comp, round, maxNumKeccakF)
		mod.IO.newInput(comp, maxNumKeccakF, *mod)
		mod.IO.newOutput(comp, maxNumKeccakF, *mod)
	}

	prover = func(permTrace keccak.PermTraces) wizard.MainProverStep {
		return func(run *wizard.ProverRuntime) {
			mod.Assign(run, permTrace)
		}
	}
	return define, prover
}

func TestInputOutputModule(t *testing.T) {
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewChaCha8([32]byte{}))
	numCases := 2
	maxNumKeccakf := 64
	// The -1 is here to prevent the generation of a padding block
	maxInputSize := maxNumKeccakf*keccak.Rate - 1

	definer, prover := MakeTestCaseInputOutputModule(maxNumKeccakf)
	comp := wizard.Compile(definer, dummy.Compile)

	for i := 0; i < numCases; i++ {
		// Generate a random piece of data
		dataSize := rng.IntN(maxInputSize + 1)
		data := make([]byte, dataSize)
		utils.ReadPseudoRand(rng, data)

		// Generate permutation traces for the data
		traces := keccak.PermTraces{}
		keccak.Hash(data, &traces)

		proof := wizard.Prove(comp, prover(traces))
		assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
	}
}
