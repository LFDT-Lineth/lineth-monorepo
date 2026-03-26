package dedicated

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Packing module, for testing
func makeTestCaseLaneAllocDoublyMax(tc testCases) (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {

	var isActive ifaces.Column
	acc := &AccumulateUpToDoublyMaxCtx{}

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP

		colA := comp.InsertCommit(0, ifaces.ColIDf("COL_A"), tc.size, true)
		isActive = comp.InsertCommit(0, ifaces.ColIDf("IS_ACTIVE"), tc.size, true)

		acc = AccumulateUpToDoublyMax(comp, tc.maxValue, tc.maxAtRow, colA, isActive)

	}
	prover = func(run *wizard.ProverRuntime) {

		run.AssignColumn(acc.Inputs.ColA.GetColID(), smartvectors.RightZeroPadded(smartvectors.ForTest(tc.col...).IntoRegVecSaveAlloc(), tc.size))
		run.AssignColumn(isActive.GetColID(), smartvectors.RightZeroPadded(vector.Repeat(field.One(), tc.len), tc.size))

		// assigning the submodule
		acc.Run(run)
		// print the columns for debugging

	}
	return define, prover
}

func TestLaneAllocDoublyMax(t *testing.T) {
	for _, tc := range testc {
		define, prover := makeTestCaseLaneAllocDoublyMax(tc)
		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prover)
		assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
	}
}

type testCases struct {
	name     string
	len      int
	size     int
	maxAtRow int
	maxValue int
	col      []int
}

var testc = []testCases{
	{
		name:     "test case 1",
		len:      16,
		size:     16,
		maxAtRow: 3,
		maxValue: 10,
		col:      []int{3, 3, 2, 1, 1, 1, 2, 2, 1, 3, 1, 3, 2, 1, 3, 1},
	},
	{
		name:     "test case 2",
		len:      18,
		size:     32,
		maxAtRow: 4,
		maxValue: 15,
		col:      []int{4, 4, 1, 3, 3, 1, 3, 2, 2, 2, 1, 1, 3, 4, 2, 2, 4, 3},
	},
}
