package base_conversion

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf"
	"github.com/stretchr/testify/assert"
)

func makeTestCaseBaseConversionOutput() (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	maxNumBlocks := 4
	b := &HashBaseConversion{}
	define = func(build *wizard.Builder) {
		var (
			comp      = build.CompiledIOP
			size      = utils.NextPowerOfTwo(maxNumBlocks)
			createCol = common.CreateColFn(comp, "BASE_CONVERSION_TEST", size, pragmas.RightPadded)
			limbsHi   = make([]ifaces.Column, numLimbsOutput)
			limbsLo   = make([]ifaces.Column, numLimbsOutput)
		)

		for j := 0; j < numLimbsOutput; j++ {
			limbsHi[j] = createCol("LIMBS_HI_B_%v", j)
			limbsLo[j] = createCol("LIMBS_LO_B_%v", j)
		}

		inp := HashBaseConversionInput{
			LimbsHiB:      limbsHi,
			LimbsLoB:      limbsLo,
			MaxNumKeccakF: maxNumBlocks,

			Lookup: NewLookupTables(comp),
		}

		b = NewHashBaseConversion(comp, inp)

	}
	prover = func(run *wizard.ProverRuntime) {

		b.assignInputs(run)
		b.Run(run)

	}
	return define, prover
}

func TestBaseConversionOutput(t *testing.T) {
	define, prover := makeTestCaseBaseConversionOutput()
	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

func (b *HashBaseConversion) assignInputs(run *wizard.ProverRuntime) {

	var (
		sliceHiB = make([]*common.VectorBuilder, numLimbsOutput)
		sliceLoB = make([]*common.VectorBuilder, numLimbsOutput)
		size     = b.Size
	)

	for j := range sliceHiB {
		sliceHiB[j] = common.NewVectorBuilder(b.Inputs.LimbsHiB[j])
		sliceLoB[j] = common.NewVectorBuilder(b.Inputs.LimbsLoB[j])
	}

	// #nosec G404 -- we don't need a cryptographic PRNG for testing purposes
	rng := rand.New(utils.NewRandSource(678988))

	max := keccakf.BaseBPow4
	for j := range sliceHiB {
		for row := 0; row < size; row++ {
			// generate a random value in baseB
			sliceHiB[j].PushInt(rng.IntN(max))
			sliceLoB[j].PushInt(rng.IntN(max))
		}
	}

	for j := range sliceHiB {
		sliceHiB[j].PadAndAssign(run, field.Zero())
		sliceLoB[j].PadAndAssign(run, field.Zero())
	}

}
