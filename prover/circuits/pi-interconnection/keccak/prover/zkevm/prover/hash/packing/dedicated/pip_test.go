package dedicated

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Partitioned inner-product, for testing
func makeTestCasePIP() (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	round := 0
	size := 8
	var colA, colB, partition ifaces.Column

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP
		// commit to the colA,colB,partition,ipTracker
		colA = comp.InsertCommit(round, ifaces.ColIDf("ColA"), size)
		colB = comp.InsertCommit(round, ifaces.ColIDf("ColB"), size)
		partition = comp.InsertCommit(round, ifaces.ColIDf("Partition"), size)
		InsertPartitionedIP(comp, "PIP", colA, colB, partition)

	}
	prover = func(run *wizard.ProverRuntime) {
		// assign the columns
		run.AssignColumn(colA.GetColID(), smartvectors.ForTest(1, 0, 2, 3, 1, 4, 3, 2))
		run.AssignColumn(colB.GetColID(), smartvectors.ForTest(0, 0, 0, 1, 1, 1, 2, 1))
		run.AssignColumn(partition.GetColID(), smartvectors.ForTest(1, 0, 0, 1, 0, 0, 1, 1))
	}
	return define, prover
}
func TestPIPModule(t *testing.T) {
	define, prover := makeTestCasePIP()
	comp := wizard.Compile(define, dummy.Compile)

	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}
