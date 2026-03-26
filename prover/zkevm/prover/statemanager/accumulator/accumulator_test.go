package accumulator

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func accumulatorTestingModule(maxNumProofs int) (
	define wizard.DefineFunc,
	prover func(traces []statemanager.DecodedTrace) wizard.MainProverStep,
) {

	var mod Module

	// The testing wizard uniquely calls the accumulator module
	define = func(b *wizard.Builder) {
		mod = NewModule(b.CompiledIOP, Settings{
			Name:            "ACCUMULATOR_TEST",
			MaxNumProofs:    maxNumProofs,
			MerkleTreeDepth: 40,
		})
	}

	// And the prover (instanciated for traces) is called
	prover = func(
		traces []statemanager.DecodedTrace,
	) wizard.MainProverStep {
		return func(run *wizard.ProverRuntime) {
			// Assigns the module
			mod.Assign(run, traces)
		}
	}

	return define, prover
}

func TestShomei(t *testing.T) {
	// Generate trace file for insert
	decodedTrace := make([]statemanager.DecodedTrace, 0, 10)
	acc := statemanager.NewStorageTrie(types.EthAddress{})
	traceInsert := acc.InsertAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x12"))
	insertDecodedTrace := statemanager.DecodedTrace{
		Location:   "this is the first trace",
		Type:       2,
		Underlying: traceInsert,
		IsSkipped:  false,
	}
	decodedTrace = append(decodedTrace, insertDecodedTrace)
	// Generate a trace file for update
	traceUpdate := acc.UpdateAndProve(types.FullBytes32FromHex("0x32"), types.FullBytes32FromHex("0x34"))
	updateDecodedTrace := statemanager.DecodedTrace{
		Location:   "this is the second trace",
		Type:       3,
		Underlying: traceUpdate,
		IsSkipped:  false,
	}
	decodedTrace = append(decodedTrace, updateDecodedTrace)
	// Generate a trace file for delete
	acc.InsertAndProve(types.FullBytes32FromHex("0x43"), types.FullBytes32FromHex("0x12"))
	traceDelete := acc.DeleteAndProve(types.FullBytes32FromHex("0x43"))
	deleteDecodedTrace := statemanager.DecodedTrace{
		Location:   "this is the third trace",
		Type:       4,
		Underlying: traceDelete,
		IsSkipped:  false,
	}
	decodedTrace = append(decodedTrace, deleteDecodedTrace)
	//  Generate a trace file for read zero
	traceReadZero := acc.ReadZeroAndProve(types.FullBytes32FromHex("0x93"))
	readZeroDecodedTrace := statemanager.DecodedTrace{
		Location:   "this is the fourth trace",
		Type:       1,
		Underlying: traceReadZero,
		IsSkipped:  false,
	}
	decodedTrace = append(decodedTrace, readZeroDecodedTrace)
	// Generate a trace file for read non zero
	traceReadNonZero := acc.ReadNonZeroAndProve(types.FullBytes32FromHex("0x32"))
	readNonZeroDecodedTrace := statemanager.DecodedTrace{
		Location:   "this is the fifth trace",
		Type:       0,
		Underlying: traceReadNonZero,
		IsSkipped:  false,
	}
	decodedTrace = append(decodedTrace, readNonZeroDecodedTrace)
	logrus.Infof("decoded trace length: %d", len(decodedTrace))
	definer, prover := accumulatorTestingModule(1024)
	comp := wizard.Compile(definer, dummy.Compile)
	proof := wizard.Prove(comp, prover(decodedTrace))
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid accumulator proof")
}
