package statesummary

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mock"
)

// TestStateSummaryInternal tests only the StateSummary module internally, without any connectors
func TestStateSummaryInternal(t *testing.T) {

	tContext := common.InitializeContext(100)
	var ss Module

	for i, tCase := range tContext.TestCases {

		t.Run(fmt.Sprintf("test-case-%v", i), func(t *testing.T) {

			t.Logf("Test case explainer: %v", tCase.Explainer)

			define := func(b *wizard.Builder) {
				ss = NewModule(b.CompiledIOP, 1<<6)
			}

			prove := func(run *wizard.ProverRuntime) {

				var (
					initState    = tContext.State
					shomeiState  = mock.InitShomeiState(initState)
					stateLogs    = tCase.StateLogsGens(initState)
					shomeiTraces = mock.StateLogsToShomeiTraces(shomeiState, stateLogs)
				)

				ss.Assign(run, shomeiTraces)
			}

			comp := wizard.Compile(define, dummy.Compile)
			proof := wizard.Prove(comp, prove)
			err := wizard.Verify(comp, proof)

			if err != nil {
				t.Fatalf("verification failed: %v", err)
			}
		})
	}
}

func TestStateSummaryReadNonZeroShomei(t *testing.T) {

	var (
		addresses = []types.EthAddress{
			types.DummyAddress(32),
			types.DummyAddress(64),
			types.DummyAddress(23),
			types.DummyAddress(54),
		}

		storageKeys = []types.FullBytes32{
			types.DummyFullByte(102),
			types.DummyFullByte(1002),
			types.DummyFullByte(1012),
			types.DummyFullByte(1023),
		}

		storageValues = []types.FullBytes32{
			types.DummyFullByte(202),
			types.DummyFullByte(2002),
			types.DummyFullByte(2012),
			types.DummyFullByte(2023),
		}
	)

	state := mock.State{}
	state.InsertContract(addresses[2], types.DummyKoalaOctuplet(67), types.DummyFullByte(56), 100)
	state.InsertContract(addresses[3], types.DummyKoalaOctuplet(76), types.DummyFullByte(57), 102)
	state.SetStorage(addresses[2], storageKeys[0], storageValues[0])
	state.SetStorage(addresses[2], storageKeys[1], storageValues[1])
	state.SetStorage(addresses[3], storageKeys[2], storageValues[2])
	state.SetStorage(addresses[3], storageKeys[3], storageValues[3])

	var (
		shomeiState = mock.InitShomeiState(state)
		logs        = mock.NewStateLogBuilder(15, state).
				WithAddress(addresses[2]).
				IncNonce().
				WithAddress(addresses[3]).
				ReadStorage(storageKeys[2]).
				ReadStorage(storageKeys[3]).
				Done()
		shomeiTraces = mock.StateLogsToShomeiTraces(shomeiState, logs)
		ss           Module
	)

	// Shuffle the logs to ensure they will be in the same order as shomei's
	newTraces := [][]statemanager.DecodedTrace{make([]statemanager.DecodedTrace, 4)}
	newTraces[0][0] = shomeiTraces[0][2]
	newTraces[0][1] = shomeiTraces[0][0]
	newTraces[0][2] = shomeiTraces[0][1]
	newTraces[0][3] = shomeiTraces[0][3]

	define := func(b *wizard.Builder) {
		ss = NewModule(b.CompiledIOP, 1<<6)
	}

	prove := func(run *wizard.ProverRuntime) {
		ss.Assign(run, newTraces)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

}
