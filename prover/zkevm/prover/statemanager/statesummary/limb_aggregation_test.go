package statesummary

import (
	"encoding/binary"
	"math/big"
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

// TestAccountEqualityUsesWholeHash guards STATE_SUMMARY_OLD_NEW_ACCOUNT_EQUAL
// against aggregating the per-limb InitialAndFinalAreSame flags with an OR. The
// two balances below hash to account digests sharing limb 2 and differing on the
// other seven, so the write must not be taken for a read.
func TestAccountEqualityUsesWholeHash(t *testing.T) {

	var (
		address    = types.DummyAddress(32)
		oldBalance = big.NewInt(1_000)
		newBalance = big.NewInt(91_763_027)
	)

	state := mock.State{}
	state.InsertEOA(address, 1, oldBalance)

	var (
		shomeiState = mock.InitShomeiState(state)
		logs        = mock.NewStateLogBuilder(15, state).
				WithAddress(address).
				WriteBalance(newBalance).
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

// storageValueHashingToAZeroLimb hashes to a digest whose limb 3 is zero and
// whose other seven limbs are not. FinalHValIsZero compares against zero rather
// than against the zero-storage hash, so this is the value that exercises it.
func storageValueHashingToAZeroLimb() types.FullBytes32 {
	var v types.FullBytes32
	binary.BigEndian.PutUint32(v[28:], 0x07b89647)
	return v
}

// TestNewStorageZeroizationUsesWholeHash guards
// STATE_SUMMARY_NEW_STORAGE_ZEROIZATION against aggregating the per-limb
// FinalHValIsZero flags with an OR: writing a value whose hash merely happens to
// have one zero limb is an update, not a read-zero or a deletion.
func TestNewStorageZeroizationUsesWholeHash(t *testing.T) {

	var (
		address    = types.DummyAddress(32)
		storageKey = types.DummyFullByte(102)
		oldValue   = types.DummyFullByte(2002)
		newValue   = storageValueHashingToAZeroLimb()
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
