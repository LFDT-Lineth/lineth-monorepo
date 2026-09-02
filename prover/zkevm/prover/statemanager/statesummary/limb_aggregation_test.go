package statesummary

import (
	"encoding/binary"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mock"
)

// storageValueMatchingZeroHashOnOneLimb hashes to a digest sharing limb 7 with
// hashOfZeroStorage and differing on the other seven. It is a regular non-zero
// value: the coincidence is on a single limb only.
func storageValueMatchingZeroHashOnOneLimb() types.FullBytes32 {
	var v types.FullBytes32
	binary.BigEndian.PutUint32(v[28:], 0x02d3b00e)
	return v
}

// TestStorageZeroizationUsesWholeHash guards STATE_SUMMARY_OLD_STORAGE_ZEROIZATION
// against aggregating the per-limb OldValueIsZero flags with an OR: overwriting a
// slot whose old value merely shares one limb with the zero-storage hash is an
// update, not a read-zero or an insert.
func TestStorageZeroizationUsesWholeHash(t *testing.T) {

	var (
		address    = types.DummyAddress(32)
		storageKey = types.DummyFullByte(102)
		oldValue   = storageValueMatchingZeroHashOnOneLimb()
		newValue   = types.DummyFullByte(2002)
	)

	state := mock.State{}
	state.InsertContract(address, types.DummyKoalaOctuplet(67), types.DummyFullByte(56), 100)
	state.SetStorage(address, storageKey, oldValue)

	var (
		shomeiState = mock.InitShomeiState(state)
		logs        = mock.NewStateLogBuilder(15, state).
				WithAddress(address).
				WriteStorage(storageKey, newValue).
				Done()
		shomeiTraces = mock.StateLogsToShomeiTraces(shomeiState, logs)
		ss           Module
	)

	define := func(b *wizard.Builder) {
		ss = NewModule(b.CompiledIOP, 1<<6)
	}

	prove := func(run *wizard.ProverRuntime) {
		ss.Assign(run, shomeiTraces)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)

	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatalf("verification failed: %v", err)
	}
}
