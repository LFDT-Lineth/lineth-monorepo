package statesummary

import (
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// StoragePeek provides the columns to represent a peek to an account storage
// slot.
type StoragePeek struct {
	// StorageKey stores the storage key of the peeked slot
	Key common.HiLoColumns
	// OldValue represents the storage value that is being peeked at.
	OldValue common.HiLoColumns
	// NewValue represents the new value of the storage slot.
	NewValue common.HiLoColumns

	// OldValueIsZero and NewValueIsZero are indicator columns set to 1 when the
	// old/new value is set to zero and 0 otherwise.
	OldValueIsZero, NewValueIsZero ifaces.Column

	// ComputeOldValueIsZero and ComputeNewValueIsZero are respectively
	// responsible for assigning OldValueIsZero and NewValueIsZero.
	ComputeOldValueIsZero, ComputeNewValueIsZero wizard.ProverAction

	// KeyLimbs represents the key in limb decomposition.
	KeyLimbs byte32cmp.LimbColumns

	// ComputeKeyLimbs and ComputeKeyLimbLo are responsible for computing the
	// "hi" and the "lo" limbs of the KeyLimbs.
	ComputeKeyLimbs wizard.ProverAction

	// KeyIncreased is a column indicating whether the current storage
	// key is strictly greater than the previous one.
	KeyIncreased ifaces.Column

	// ComputeKeyIncreased computes KeyIncreased.
	ComputeKeyIncreased wizard.ProverAction

	// OldValueHash and NewValueHash store the hash of the storage
	// peek. It is not passed to the accumulator statement directly as we have
	// special handling for the case where the storage value is zero.
	OldValueHash, NewValueHash ifaces.Column

	// ComputeOldValueHash and ComputeNewStorageValue hash compute
	// respectively OldStorageValueHash and NewStorageValueHash
	ComputeOldValueHash, ComputeNewValueHash wizard.ProverAction

	// KeyHash stores the hash of the storage keys
	KeyHash ifaces.Column

	// ComputeStorageKeyHash computes the KeyHash column.
	ComputeKeyHash wizard.ProverAction

	// OldAndNewAreEqual is an indicator column telling whether the old and the
	// new storage value are equal with a 1 and 0 else.
	OldAndNewValuesAreEqual ifaces.Column

	// ComputeOldAndNewValuesAreEqual computes OldAndNewValuesAreEqual
	ComputeOldAndNewValuesAreEqual wizard.ProverAction
}

// newStoragePeek returns a new StoragePeek object with initialized and
// unconstrained columns.
func newStoragePeek(comp *wizard.CompiledIOP, size int, name string) StoragePeek {
	res := StoragePeek{
		Key:      common.NewHiLoColumns(comp, size, name+"_KEY"),
		OldValue: common.NewHiLoColumns(comp, size, name+"_OLD_VALUE"),
		NewValue: common.NewHiLoColumns(comp, size, name+"_NEW_VALUE"),
	}

	res.OldValueHash, res.ComputeOldValueHash = common.HashOf(
		comp,
		[]ifaces.Column{
			res.OldValue.Lo,
			res.OldValue.Hi,
		},
	)

	res.NewValueHash, res.ComputeNewValueHash = common.HashOf(
		comp,
		[]ifaces.Column{
			res.NewValue.Lo,
			res.NewValue.Hi,
		},
	)

	res.OldAndNewValuesAreEqual, res.ComputeOldAndNewValuesAreEqual = dedicated.IsZero(
		comp,
		sym.Sub(res.OldValueHash, res.NewValueHash),
	)

	res.KeyHash, res.ComputeKeyHash = common.HashOf(
		comp,
		[]ifaces.Column{
			res.Key.Lo,
			res.Key.Hi,
		},
	)

	res.OldValueIsZero, res.ComputeOldValueIsZero = dedicated.IsZero(
		comp,
		sym.Sub(res.OldValueHash, hashOfZeroStorage()),
	)

	res.NewValueIsZero, res.ComputeNewValueIsZero = dedicated.IsZero(
		comp,
		sym.Sub(res.NewValueHash, hashOfZeroStorage()),
	)

	res.KeyLimbs, res.ComputeKeyLimbs = byte32cmp.Decompose(comp, res.KeyHash, 16, 16)

	res.KeyIncreased, _, _, res.ComputeKeyIncreased = byte32cmp.CmpMultiLimbs(
		comp,
		res.KeyLimbs, res.KeyLimbs.Shift(-1),
	)

	return res
}

// storagePeekAssignmentBuilder is a convenience structure storing the column
// builders relating to a StoragePeek.
type storagePeekAssignmentBuilder struct {
	key, oldValue, newValue common.HiLoAssignmentBuilder
}

// newStoragePeekAssignmentBuilder constructs a fresh [storagePeekAssignmentBuilder]
// with empty columns.
func newStoragePeekAssignmentBuilder(sp *StoragePeek) storagePeekAssignmentBuilder {
	return storagePeekAssignmentBuilder{
		key:      common.NewHiLoAssignmentBuilder(sp.Key),
		oldValue: common.NewHiLoAssignmentBuilder(sp.OldValue),
		newValue: common.NewHiLoAssignmentBuilder(sp.NewValue),
	}
}

// pushAllZeroes pushes a zero row onto the receiver
func (sh *storagePeekAssignmentBuilder) pushAllZeroes() {
	sh.key.PushZeroes()
	sh.oldValue.PushZeroes()
	sh.newValue.PushZeroes()
}

// pushOnlyKey pushes the key onto the the "key" columns builder and zero on
// the others
func (sh *storagePeekAssignmentBuilder) pushOnlyKey(key types.FullBytes32) {
	sh.key.Push(key)
	sh.oldValue.PushZeroes()
	sh.newValue.PushZeroes()
}

// pushOnlyOld pushes a row where the keys and the old value are the one provided
// by the caller and the new value is zero.
func (sh *storagePeekAssignmentBuilder) pushOnlyOld(key, oldVal types.FullBytes32) {
	sh.key.Push(key)
	sh.oldValue.Push(oldVal)
	sh.newValue.PushZeroes()
}

// pushOnlyNew pushes a row where the key and the new value are the one provided
// by the caller and the old value is set to zero.
func (sh *storagePeekAssignmentBuilder) pushOnlyNew(key, newVal types.FullBytes32) {
	sh.key.Push(key)
	sh.oldValue.PushZeroes()
	sh.newValue.Push(newVal)
}

// push pushes a row where the key, the old value and the new value are the
// one provided by the caller.
func (sh *storagePeekAssignmentBuilder) push(key, oldVal, newVal types.FullBytes32) {
	sh.key.Push(key)
	sh.oldValue.Push(oldVal)
	sh.newValue.Push(newVal)
}

// padAssign pads and assigns the columns of the storage peek into `run`.
func (sh *storagePeekAssignmentBuilder) padAssign(run *wizard.ProverRuntime) {
	sh.key.PadAssign(run, types.FullBytes32{})
	sh.oldValue.PadAssign(run, types.FullBytes32{})
	sh.newValue.PadAssign(run, types.FullBytes32{})
}

// hashOfZeroStorage returns the hash of (0, 0) which is what we use for empty
// storage slots.
func hashOfZeroStorage() field.Element {
	res := field.Zero()
	res = mimc.BlockCompression(res, field.Zero())
	return mimc.BlockCompression(res, field.Zero())
}
