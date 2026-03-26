package statesummary

import (
	poseidon2kb "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	dedicatedposeidon2 "github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
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
	OldValueIsZero, NewValueIsZero [dedicatedposeidon2.BlockSize]ifaces.Column

	// ComputeOldValueIsZero and ComputeNewValueIsZero are respectively
	// responsible for assigning OldValueIsZero and NewValueIsZero.
	ComputeOldValueIsZero, ComputeNewValueIsZero [dedicatedposeidon2.BlockSize]wizard.ProverAction

	// KeyLimbs represents the key in limb decomposition.
	KeyLimbs [dedicatedposeidon2.BlockSize]byte32cmp.LimbColumns

	// ComputeKeyLimbs and ComputeKeyLimbLo are responsible for computing the
	// "hi" and the "lo" limbs of the KeyLimbs.
	ComputeKeyLimbs [dedicatedposeidon2.BlockSize]wizard.ProverAction

	// KeyIncreased is a column indicating whether the current storage
	// key is strictly greater than the previous one.
	KeyIncreased ifaces.Column

	// ComputeKeyIncreased computes KeyIncreased.
	ComputeKeyIncreased wizard.ProverAction

	// OldValueHash and NewValueHash store the hash of the storage
	// peek. It is not passed to the accumulator statement directly as we have
	// special handling for the case where the storage value is zero.
	OldValueHash, NewValueHash [dedicatedposeidon2.BlockSize]ifaces.Column

	// ComputeOldValueHash and ComputeNewStorageValue hash compute
	// respectively OldStorageValueHash and NewStorageValueHash
	ComputeOldValueHash, ComputeNewValueHash *dedicatedposeidon2.HashingCtx

	// KeyHash stores the hash of the storage keys
	KeyHash [dedicatedposeidon2.BlockSize]ifaces.Column

	// ComputeStorageKeyHash computes the KeyHash column.
	ComputeKeyHash *dedicatedposeidon2.HashingCtx

	// OldAndNewAreEqual is an indicator column telling whether the old and the
	// new storage value are equal with a 1 and 0 else.
	OldAndNewValuesAreEqual [dedicatedposeidon2.BlockSize]ifaces.Column

	// ComputeOldAndNewValuesAreEqual computes OldAndNewValuesAreEqual
	ComputeOldAndNewValuesAreEqual [dedicatedposeidon2.BlockSize]wizard.ProverAction
}

// newStoragePeek returns a new StoragePeek object with initialized and
// unconstrained columns.
func newStoragePeek(comp *wizard.CompiledIOP, size int, name string) StoragePeek {
	res := StoragePeek{
		Key:      common.NewHiLoColumns(comp, size, name+"_KEY"),
		OldValue: common.NewHiLoColumns(comp, size, name+"_OLD_VALUE"),
		NewValue: common.NewHiLoColumns(comp, size, name+"_NEW_VALUE"),
	}

	// Use StorageHash helper which matches FullBytes32.WriteTo format (Hi first, then Lo)
	res.ComputeOldValueHash = StorageHash(comp, res.OldValue, "STORAGE_VALUE_OLD_HASHING")
	res.OldValueHash = res.ComputeOldValueHash.Result()

	res.ComputeNewValueHash = StorageHash(comp, res.NewValue, "STORAGE_VALUE_NEW_HASHING")
	res.NewValueHash = res.ComputeNewValueHash.Result()

	for i := range dedicatedposeidon2.BlockSize {
		res.OldAndNewValuesAreEqual[i], res.ComputeOldAndNewValuesAreEqual[i] = dedicated.IsZero(
			comp,
			sym.Sub(res.OldValueHash[i], res.NewValueHash[i]),
		).GetColumnAndProverAction()
	}

	// Use StorageHash helper which matches FullBytes32.WriteTo format (Hi first, then Lo)
	res.ComputeKeyHash = StorageHash(comp, res.Key, "STORAGE_KEY_HASHING")

	res.KeyHash = res.ComputeKeyHash.Result()
	zeroStorageHash := hashOfZeroStorage()

	for i := range dedicatedposeidon2.BlockSize {
		res.OldValueIsZero[i], res.ComputeOldValueIsZero[i] = dedicated.IsZero(
			comp,
			sym.Sub(res.OldValueHash[i], zeroStorageHash[i]),
		).GetColumnAndProverAction()

		res.NewValueIsZero[i], res.ComputeNewValueIsZero[i] = dedicated.IsZero(
			comp,
			sym.Sub(res.NewValueHash[i], zeroStorageHash[i]),
		).GetColumnAndProverAction()
	}

	keyLimbColumbs := byte32cmp.LimbColumns{LimbBitSize: common.LimbBytes * 8, IsBigEndian: true}
	shiftedLimbColumbs := byte32cmp.LimbColumns{LimbBitSize: common.LimbBytes * 8, IsBigEndian: true}
	for i := range dedicatedposeidon2.BlockSize {
		res.KeyLimbs[i], res.ComputeKeyLimbs[i] = byte32cmp.Decompose(comp, res.KeyHash[i], 2, common.LimbBytes*8)

		// Decompose returns limbs in little-endian order (LSB at index 0).
		// For big-endian comparison (matching hex string comparison), we need to:
		// 1. Process KeyHash[0] first (most significant part of hash)
		// 2. Reverse each KeyHash's limbs so MSB comes first
		// Limbs[1] is MSB, Limbs[0] is LSB - append MSB first
		keyLimbColumbs.Limbs = append(keyLimbColumbs.Limbs, res.KeyLimbs[i].Limbs[1], res.KeyLimbs[i].Limbs[0])
		shifted := res.KeyLimbs[i].Shift(-1)
		shiftedLimbColumbs.Limbs = append(shiftedLimbColumbs.Limbs, shifted.Limbs[1], shifted.Limbs[0])
	}

	res.KeyIncreased, _, _, res.ComputeKeyIncreased = byte32cmp.CmpMultiLimbs(
		comp,
		keyLimbColumbs, shiftedLimbColumbs,
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
	var keyLimbs [common.NbLimbU256][]byte
	copy(keyLimbs[:], common.SplitBytes(key[:]))

	sh.key.Push(keyLimbs)
	sh.oldValue.PushZeroes()
	sh.newValue.PushZeroes()
}

// pushOnlyOld pushes a row where the keys and the old value are the one provided
// by the caller and the new value is zero.
func (sh *storagePeekAssignmentBuilder) pushOnlyOld(key, oldVal types.FullBytes32) {
	var keyLimbs [common.NbLimbU256][]byte
	copy(keyLimbs[:], common.SplitBytes(key[:]))

	var oldValueLimbs [common.NbLimbU256][]byte
	copy(oldValueLimbs[:], common.SplitBytes(oldVal[:]))

	sh.key.Push(keyLimbs)
	sh.oldValue.Push(oldValueLimbs)
	sh.newValue.PushZeroes()
}

// pushOnlyNew pushes a row where the key and the new value are the one provided
// by the caller and the old value is set to zero.
func (sh *storagePeekAssignmentBuilder) pushOnlyNew(key, newVal types.FullBytes32) {
	var keyLimbs [common.NbLimbU256][]byte
	copy(keyLimbs[:], common.SplitBytes(key[:]))

	var newValueLimbs [common.NbLimbU256][]byte
	copy(newValueLimbs[:], common.SplitBytes(newVal[:]))

	sh.key.Push(keyLimbs)
	sh.oldValue.PushZeroes()
	sh.newValue.Push(newValueLimbs)
}

// push pushes a row where the key, the old value and the new value are the
// one provided by the caller.
func (sh *storagePeekAssignmentBuilder) push(key, oldVal, newVal types.FullBytes32) {
	var keyLimbs [common.NbLimbU256][]byte
	copy(keyLimbs[:], common.SplitBytes(key[:]))

	var oldValueLimbs [common.NbLimbU256][]byte
	copy(oldValueLimbs[:], common.SplitBytes(oldVal[:]))

	var newValueLimbs [common.NbLimbU256][]byte
	copy(newValueLimbs[:], common.SplitBytes(newVal[:]))

	sh.key.Push(keyLimbs)
	sh.oldValue.Push(oldValueLimbs)
	sh.newValue.Push(newValueLimbs)
}

// padAssign pads and assigns the columns of the storage peek into `run`.
func (sh *storagePeekAssignmentBuilder) padAssign(run *wizard.ProverRuntime) {
	sh.key.PadAssign(run, [common.NbLimbU256][]byte{})
	sh.oldValue.PadAssign(run, [common.NbLimbU256][]byte{})
	sh.newValue.PadAssign(run, [common.NbLimbU256][]byte{})
}

// hashOfZeroStorage returns the hash of (0, 0) which is what we use for empty
// storage slots. This matches FullBytes32.WriteTo format for a zero value:
// 32 zero bytes -> LeftPadded -> 64 zero bytes -> 16 zero field elements
func hashOfZeroStorage() []field.Element {
	hasher := poseidon2kb.NewMDHasher()
	// FullBytes32.WriteTo for zero: LeftPadded([0;32]) = [0;64] = 16 zero field elements
	zeroBytes := make([]byte, 64) // 16 field elements * 4 bytes
	hasher.Write(zeroBytes)
	hashBytes := hasher.Sum(nil)
	// Convert hash output to field elements
	res := make([]field.Element, dedicatedposeidon2.BlockSize)
	for i := range dedicatedposeidon2.BlockSize {
		res[i].SetBytes(hashBytes[i*4 : (i+1)*4])
	}
	return res
}

// StorageHash computes the hash of a HiLoColumns in a format consistent with
// FullBytes32.WriteTo serialization (Hi bytes first, then Lo bytes).
func StorageHash(comp *wizard.CompiledIOP, hilo common.HiLoColumns, name string) *dedicatedposeidon2.HashingCtx {
	var hashInputs []ifaces.Column
	// WriteTo serializes Hi bytes (0-15) first, then Lo bytes (16-31)
	hashInputs = append(hashInputs, hilo.Hi[:]...)
	hashInputs = append(hashInputs, hilo.Lo[:]...)
	return dedicatedposeidon2.HashOf(comp, hashInputs, name)
}
