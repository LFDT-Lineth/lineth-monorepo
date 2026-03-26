package selector_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/selector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestSubsampleSingleCols(t *testing.T) {

	var large, small ifaces.Column

	define := func(b *wizard.Builder) {
		large = b.RegisterCommit("LARGE", 16)
		small = b.RegisterCommit("SMALL", 4)
		selector.CheckSubsample(b.CompiledIOP, "SUBSAMPLE", []ifaces.Column{large}, []ifaces.Column{small}, 0)
	}

	prove := func(run *wizard.ProverRuntime) {
		run.AssignColumn(large.GetColID(), smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
		run.AssignColumn(small.GetColID(), smartvectors.ForTest(0, 4, 8, 12))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	require.NoError(t, wizard.Verify(comp, proof))
}

func TestSubsample3Cols(t *testing.T) {

	var large, small [3]ifaces.Column

	define := func(b *wizard.Builder) {
		large[0] = b.RegisterCommit("LARGE_0", 16)
		large[1] = b.RegisterCommit("LARGE_1", 16)
		large[2] = b.RegisterCommit("LARGE_2", 16)
		small[0] = b.RegisterCommit("SMALL_0", 4)
		small[1] = b.RegisterCommit("SMALL_1", 4)
		small[2] = b.RegisterCommit("SMALL_2", 4)
		selector.CheckSubsample(b.CompiledIOP, "SUBSAMPLE", large[:], small[:], 0)
	}

	prove := func(run *wizard.ProverRuntime) {
		run.AssignColumn(large[0].GetColID(), smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
		run.AssignColumn(large[1].GetColID(), smartvectors.ForTest(20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 210, 211, 212, 213, 214, 215))
		run.AssignColumn(large[2].GetColID(), smartvectors.ForTest(420, 421, 422, 423, 424, 425, 426, 427, 428, 429, 4210, 4211, 4212, 4213, 4214, 4215))
		run.AssignColumn(small[0].GetColID(), smartvectors.ForTest(0, 4, 8, 12))
		run.AssignColumn(small[1].GetColID(), smartvectors.ForTest(20, 24, 28, 212))
		run.AssignColumn(small[2].GetColID(), smartvectors.ForTest(420, 424, 428, 4212))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	require.NoError(t, wizard.Verify(comp, proof))
}

func TestSubsample3ColsWithOffset(t *testing.T) {

	var large, small [3]ifaces.Column

	define := func(b *wizard.Builder) {
		large[0] = b.RegisterCommit("LARGE_0", 16)
		large[1] = b.RegisterCommit("LARGE_1", 16)
		large[2] = b.RegisterCommit("LARGE_2", 16)
		small[0] = b.RegisterCommit("SMALL_0", 4)
		small[1] = b.RegisterCommit("SMALL_1", 4)
		small[2] = b.RegisterCommit("SMALL_2", 4)
		selector.CheckSubsample(b.CompiledIOP, "SUBSAMPLE", large[:], small[:], 3)
	}

	prove := func(run *wizard.ProverRuntime) {
		run.AssignColumn(large[0].GetColID(), smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
		run.AssignColumn(large[1].GetColID(), smartvectors.ForTest(20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 210, 211, 212, 213, 214, 215))
		run.AssignColumn(large[2].GetColID(), smartvectors.ForTest(420, 421, 422, 423, 424, 425, 426, 427, 428, 429, 4210, 4211, 4212, 4213, 4214, 4215))
		run.AssignColumn(small[0].GetColID(), smartvectors.ForTest(3, 7, 11, 15))
		run.AssignColumn(small[1].GetColID(), smartvectors.ForTest(23, 27, 211, 215))
		run.AssignColumn(small[2].GetColID(), smartvectors.ForTest(423, 427, 4211, 4215))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	require.NoError(t, wizard.Verify(comp, proof))
}
