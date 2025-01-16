package base_conversion

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/assert"
)

func makeTestCaseDecomposeBE() (
	define wizard.DefineFunc,
	prover wizard.ProverStep,
) {
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rand := rand.New(utils.NewRandSource(0))
	size := 16
	d := &decompositionCtx{}
	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP
		inp := DecompositionInputs{
			name:          "TEST_DEC_BE",
			col:           comp.InsertCommit(0, ifaces.ColIDf("COL"), size),
			numLimbs:      4,
			bytesPerLimbs: 2,
		}
		d = DecomposeBE(comp, inp)
	}
	prover = func(run *wizard.ProverRuntime) {
		var (
			col  = common.NewVectorBuilder(d.Inputs.col)
			size = d.Inputs.col.Size()
		)
		for row := 0; row < size; row++ {
			b := make([]byte, 8)
			utils.ReadPseudoRand(rand, b)
			f := *new(field.Element).SetBytes(b)
			col.PushField(f)
		}
		col.PadAndAssign(run)
		d.Run(run)
	}
	return define, prover
}

func TestDecomposeBE(t *testing.T) {
	define, prover := makeTestCaseDecomposeBE()
	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}
