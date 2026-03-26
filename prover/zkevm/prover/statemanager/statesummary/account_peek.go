package statesummary

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	types2 "github.com/ethereum/go-ethereum/core/types"
)

func initEmptyCodeHash() types.KoalaOctuplet {
	return statemanager.EmptyCodeHash()
}

var (
	emptyCodeHash = initEmptyCodeHash()
)

// AccountPeek contains the view of the State-summary module regarding accounts.
// Namely, it stores all the account-related columns: the peeked address, the
// initial account and the final account.
type AccountPeek struct {
	// Initial and Final stores the view of the account at the beginning of an
	// account sub-segmenet and the at the current row.
	Initial, Final Account

	// HashInitial, HashFinal stores the hash of the initial account and the
	// hash of the final account
	HashInitial, HashFinal [poseidon2.BlockSize]ifaces.Column

	// ComputeHashInitial and ComputeHashFinal are [wizard.ProverAction]
	// responsible for hashing the accounts.
	ComputeHashInitial, ComputeHashFinal *poseidon2.HashingCtx

	// InitialAndFinalAreSame is an indicator column set to 1 when the
	// initial and final account share the same hash and 0 otherwise.
	InitialAndFinalAreSame [poseidon2.BlockSize]ifaces.Column

	// ComputeInitialAndFinalAreSame is a [wizard.ProverAction] responsible for
	// computing the column InitialAndFinalAreSame
	ComputeInitialAndFinalAreSame [poseidon2.BlockSize]wizard.ProverAction

	// Address represents which account is being peeked by the module.
	// It is assigned by providing
	Address [common.NbLimbEthAddress]ifaces.Column

	// AddressHash is the hash of the account address
	AddressHash [poseidon2.BlockSize]ifaces.Column

	// ComputeAddressHash is responsible for computing the AddressHash
	ComputeAddressHash *poseidon2.HashingCtx

	// AddressHashLimbs stores the limbs of the address
	AddressHashLimbs [poseidon2.BlockSize]byte32cmp.LimbColumns

	// ComputeAddressLimbs computes the [AddressLimbs] column.
	ComputeAddressLimbs [poseidon2.BlockSize]wizard.ProverAction

	// HasSameAddressAsPrev is an indicator column telling whether the previous
	// row has the same AccountAddress value as the current one.
	//
	// HasGreaterAddressAsPrev tells of the current address represents a larger
	// number than the previous one.
	HasSameAddressAsPrev, HasGreaterAddressAsPrev ifaces.Column

	// ComputeAddressComparison computes the HashSameAddressAsPrev and
	// HasGreaterAddressAsPrev.
	ComputeAddressComparison wizard.ProverAction
}

// newAccountPeek initializes all the columns related to the account and returns
// an [AccountPeek] object containing all of them. It does not generate
// constraints beyond the one coming from the dedicated wizard.
//
// The function also instantiates the dedicated columns for hashing the account,
// and operating limb-based comparisons.
func newAccountPeek(comp *wizard.CompiledIOP, size int) AccountPeek {

	createCol := func(subName string) ifaces.Column {
		return comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_ACCOUNTS_%v", subName),
			size,
			true,
		)
	}

	accPeek := AccountPeek{
		Initial: newAccount(comp, size, "OLD_ACCOUNT"),
		Final:   newAccount(comp, size, "NEW_ACCOUNT"),
	}

	for i := range common.NbLimbEthAddress {
		accPeek.Address[i] = createCol(fmt.Sprintf("ACCOUNT_%v", i))
	}

	accPeek.ComputeHashInitial = accPeek.Initial.AccountHash(comp, "OLD")
	accPeek.HashInitial = accPeek.ComputeHashInitial.Result()

	accPeek.ComputeHashFinal = accPeek.Final.AccountHash(comp, "NEW")
	accPeek.HashFinal = accPeek.ComputeHashFinal.Result()

	for i := range poseidon2.BlockSize {
		accPeek.InitialAndFinalAreSame[i], accPeek.ComputeInitialAndFinalAreSame[i] = dedicated.IsZero(
			comp,
			sym.Sub(accPeek.HashInitial[i], accPeek.HashFinal[i]),
		).GetColumnAndProverAction()
	}

	accPeek.ComputeAddressHash = poseidon2.HashOf(
		comp,
		accPeek.Address[:],
		"ACCOUNT_ADDRESS_HASHING",
	)

	accPeek.AddressHash = accPeek.ComputeAddressHash.Result()

	addrHashLimbColumbs := byte32cmp.LimbColumns{LimbBitSize: common.LimbBytes * 8, IsBigEndian: true}
	shiftedAddrHashLimbColumbs := byte32cmp.LimbColumns{LimbBitSize: common.LimbBytes * 8, IsBigEndian: true}
	for i := range poseidon2.BlockSize {
		accPeek.AddressHashLimbs[i], accPeek.ComputeAddressLimbs[i] = byte32cmp.Decompose(comp, accPeek.AddressHash[i], 2, common.LimbBytes*8)

		// Decompose returns limbs in little-endian order (LSB at index 0).
		// For big-endian comparison (matching hex string comparison), reverse the limbs.
		// Limbs[1] is MSB, Limbs[0] is LSB - append MSB first
		addrHashLimbColumbs.Limbs = append(addrHashLimbColumbs.Limbs, accPeek.AddressHashLimbs[i].Limbs[1], accPeek.AddressHashLimbs[i].Limbs[0])
		shifted := accPeek.AddressHashLimbs[i].Shift(-1)
		shiftedAddrHashLimbColumbs.Limbs = append(shiftedAddrHashLimbColumbs.Limbs, shifted.Limbs[1], shifted.Limbs[0])
	}

	accPeek.HasGreaterAddressAsPrev, accPeek.HasSameAddressAsPrev, _, accPeek.ComputeAddressComparison = byte32cmp.CmpMultiLimbs(
		comp,
		addrHashLimbColumbs,
		shiftedAddrHashLimbColumbs,
	)

	return accPeek
}

// Account provides the columns to store the values of an account that
// we are peeking at.
type Account struct {
	// Nonce, Balance, Poseidon2CodeHash and CodeSize store the account field on a
	// single column each.
	Exists          ifaces.Column
	Nonce, CodeSize [common.NbLimbU64]ifaces.Column
	StorageRoot     [poseidon2.BlockSize]ifaces.Column
	LineaCodeHash   [poseidon2.BlockSize]ifaces.Column
	Balance         [common.NbLimbU256]ifaces.Column
	// KeccakCodeHash stores the keccak code hash of the account.
	KeccakCodeHash common.HiLoColumns
	// ExpectedHubCodeHash is almost the same as the KeccakCodeHash, with the difference
	// than when the account does not exist, it contains the keccak hash of the empty string
	ExpectedHubCodeHash common.HiLoColumns
	// HasEmptyCodeHash is an indicator column indicating whether the current
	// account has an empty codehash
	HasEmptyCodeHash             [common.NbLimbU64]ifaces.Column
	CptHasEmptyCodeHash          [common.NbLimbU64]wizard.ProverAction
	ExistsAndHasNonEmptyCodeHash ifaces.Column
}

// newAccount returns a new AccountPeek with initialized and unconstrained
// columns.
func newAccount(comp *wizard.CompiledIOP, size int, name string) Account {

	createCol := func(subName string) ifaces.Column {
		return comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_%v", name, subName),
			size,
			true,
		)
	}

	acc := Account{
		Exists:                       createCol("EXISTS"),
		KeccakCodeHash:               common.NewHiLoColumns(comp, size, name+"_KECCAK_CODE_HASH"),
		ExpectedHubCodeHash:          common.NewHiLoColumns(comp, size, name+"_EXPECTED_HUB_CODE_HASH"),
		ExistsAndHasNonEmptyCodeHash: createCol("EXISTS_AND_NON_EMPTY_CODEHASH"),
	}

	for i := range common.NbLimbU64 {
		acc.Nonce[i] = createCol(fmt.Sprintf("NONCE_%v", i))
		acc.CodeSize[i] = createCol(fmt.Sprintf("CODESIZE_%v", i))
	}

	for i := range common.NbLimbU256 {
		acc.Balance[i] = createCol(fmt.Sprintf("BALANCE_%v", i))
	}

	for i := range poseidon2.BlockSize {
		acc.StorageRoot[i] = createCol(fmt.Sprintf("STORAGE_ROOT_%d", i))
	}

	for i := range poseidon2.BlockSize {
		acc.LineaCodeHash[i] = createCol(fmt.Sprintf("LINEA_CODE_HASH_%d", i))
	}

	// There is no need for an IsActive mask here because the column will be
	// multiplied by Exists which is already zero when inactive.
	for i := range common.NbLimbU64 {
		acc.HasEmptyCodeHash[i], acc.CptHasEmptyCodeHash[i] = dedicated.IsZero(comp, acc.CodeSize[i]).GetColumnAndProverAction()
	}

	hasEmptyCodeHashExpressions := make([]any, 0, common.NbLimbU64)
	for i := range common.NbLimbU64 {
		hasEmptyCodeHashExpressions = append(hasEmptyCodeHashExpressions, acc.HasEmptyCodeHash[i])
	}

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_%v_CPT_EXIST_AND_NONEMPTY_CODE", name),
		sym.Sub(
			acc.ExistsAndHasNonEmptyCodeHash,
			sym.Mul(
				sym.Sub(1, sym.Mul(hasEmptyCodeHashExpressions...)),
				acc.Exists,
			),
		),
	)

	for i := range poseidon2.BlockSize {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_%v_LINEA_CODE_HASH_FOR_EXISTING_BUT_EMPTY_CODE_%v", name, i),
			sym.Mul(
				acc.Exists,
				sym.Mul(hasEmptyCodeHashExpressions...),
				acc.LineaCodeHash[i], // for delegated accounts, the codehash is zero
				sym.Sub(acc.LineaCodeHash[i], emptyCodeHash[i]),
			),
		)
	}

	return acc
}

// accountPeekAssignmentBuilder is a convenience structure storing column
// builders relating to AccountPeek
type accountPeekAssignmentBuilder struct {
	initial, final accountAssignmentBuilder
	address        [common.NbLimbEthAddress]*common.VectorBuilder
}

// newAccountPeekAssignmentBuilder initializes a fresh accountPeekAssignmentBuilder
func newAccountPeekAssignmentBuilder(ap *AccountPeek) accountPeekAssignmentBuilder {
	res := accountPeekAssignmentBuilder{
		initial: newAccountAssignmentBuilder(&ap.Initial),
		final:   newAccountAssignmentBuilder(&ap.Final),
	}

	for i := range common.NbLimbEthAddress {
		res.address[i] = common.NewVectorBuilder(ap.Address[i])
	}

	return res
}

// accountAssignmentBuilder is a convenience structure storing the column
// builders relating to the an Account.
type accountAssignmentBuilder struct {
	exists                       *common.VectorBuilder
	nonce, codeSize              [common.NbLimbU64]*common.VectorBuilder
	balance                      [common.NbLimbU256]*common.VectorBuilder
	storageRoot, lineaCodeHash   [poseidon2.BlockSize]*common.VectorBuilder
	keccakCodeHash               common.HiLoAssignmentBuilder
	expectedHubCodeHash          common.HiLoAssignmentBuilder
	existsAndHasNonEmptyCodeHash *common.VectorBuilder
}

// newAccountAssignmentBuilder returns a new [accountAssignmentBuilder] bound
// to an [Account].
func newAccountAssignmentBuilder(ap *Account) accountAssignmentBuilder {
	res := accountAssignmentBuilder{
		exists:                       common.NewVectorBuilder(ap.Exists),
		existsAndHasNonEmptyCodeHash: common.NewVectorBuilder(ap.ExistsAndHasNonEmptyCodeHash),
		keccakCodeHash:               common.NewHiLoAssignmentBuilder(ap.KeccakCodeHash),
		expectedHubCodeHash:          common.NewHiLoAssignmentBuilder(ap.ExpectedHubCodeHash),
	}

	for i := range common.NbLimbU64 {
		res.codeSize[i] = common.NewVectorBuilder(ap.CodeSize[i])
		res.nonce[i] = common.NewVectorBuilder(ap.Nonce[i])
	}

	for i := range common.NbLimbU256 {
		res.balance[i] = common.NewVectorBuilder(ap.Balance[i])
	}

	for i := range poseidon2.BlockSize {
		res.storageRoot[i] = common.NewVectorBuilder(ap.StorageRoot[i])
		res.lineaCodeHash[i] = common.NewVectorBuilder(ap.LineaCodeHash[i])
	}

	return res
}

// pushAll stacks the value of a [types.Account] as a new row on the receiver.
func (ss *accountAssignmentBuilder) pushAll(acc types.Account) {

	// accountExists is telling whether the intent is to push an empty account
	accountExists := acc.Balance != nil

	nonceBytes := int64ToByteLimbs(acc.Nonce)
	for i := range common.NbLimbU64 {
		ss.nonce[i].PushBytes(nonceBytes[i])
	}

	// This is telling us whether the intent is to push an empty account
	if accountExists {
		balanceBytes := acc.Balance.Bytes()
		balancePadBytes := make([]byte, common.NbLimbU256*common.LimbBytes-len(balanceBytes))
		balancePaddedBytes := append(balancePadBytes, balanceBytes...)

		balanceLimbs := common.SplitBytes(balancePaddedBytes)
		for i := range common.NbLimbU256 {
			limbBytes := common.LeftPadToFrBytes(balanceLimbs[i])
			ss.balance[i].PushBytes(limbBytes)
		}

		ss.exists.PushOne()

		var keccakCodeHashLimbs [common.NbLimbU256][]byte
		copy(keccakCodeHashLimbs[:], common.SplitBytes(acc.KeccakCodeHash[:]))

		ss.keccakCodeHash.Push(keccakCodeHashLimbs)
		// if account exists push the same Keccak code hash
		ss.expectedHubCodeHash.Push(keccakCodeHashLimbs)
	} else {
		for i := range common.NbLimbU256 {
			ss.balance[i].PushZero()
		}

		ss.exists.PushZero()
		ss.keccakCodeHash.PushZeroes()
		// if account does not exist push empty codehash
		var emptyCodeHashLimbs [common.NbLimbU256][]byte
		copy(emptyCodeHashLimbs[:], common.SplitBytes(types2.EmptyCodeHash[:]))
		ss.expectedHubCodeHash.Push(emptyCodeHashLimbs)
	}

	codesizeBytes := int64ToByteLimbs(acc.CodeSize)
	for i := range common.NbLimbU64 {
		ss.codeSize[i].PushBytes(codesizeBytes[i])
	}

	for i := range acc.LineaCodeHash {
		ss.lineaCodeHash[i].PushField(acc.LineaCodeHash[i])
	}

	for i := range acc.StorageRoot {
		ss.storageRoot[i].PushField(acc.StorageRoot[i])
	}

	ss.existsAndHasNonEmptyCodeHash.PushBoolean(accountExists && acc.CodeSize > 0)
}

// pushOverrideStorageRoot is as [accountAssignmentBuilder.pushAll] but allows
// the caller to override the StorageRoot field with the provided one.
func (ss *accountAssignmentBuilder) pushOverrideStorageRoot(
	acc types.Account,
	storageRoot types.KoalaOctuplet,
) {
	newAcc := acc
	newAcc.StorageRoot = storageRoot
	ss.pushAll(newAcc)
}

// PadAndAssign terminates the receiver by padding all the columns representing
// the account with "zeroes" rows up to the target size of the column and then
// assigning the underlying [ifaces.Column] object with it.
func (ss *accountAssignmentBuilder) PadAndAssign(run *wizard.ProverRuntime) {
	ss.exists.PadAndAssign(run)

	for i := range common.NbLimbU64 {
		ss.codeSize[i].PadAndAssign(run)
		ss.nonce[i].PadAndAssign(run)
	}

	for i := range common.NbLimbU256 {
		ss.balance[i].PadAndAssign(run)
	}

	ss.keccakCodeHash.PadAssign(run, [common.NbLimbU256][]byte{})
	ss.expectedHubCodeHash.PadAssign(run, [common.NbLimbU256][]byte{})

	for i := range poseidon2.BlockSize {
		ss.lineaCodeHash[i].PadAndAssign(run)
		ss.storageRoot[i].PadAndAssign(run)
	}

	ss.existsAndHasNonEmptyCodeHash.PadAndAssign(run)
}

func int64ToByteLimbs(num int64) [][]byte {
	nonceBuffer := new(bytes.Buffer)

	err := binary.Write(nonceBuffer, binary.BigEndian, num)
	if err != nil {
		panic(err)
	}

	res := make([][]byte, common.NbLimbU64)
	nonceLimbs := common.SplitBytes(nonceBuffer.Bytes())
	for i := range common.NbLimbU64 {
		padding := make([]byte, field.Bytes-len(nonceLimbs[i]))
		res[i] = append(padding, nonceLimbs[i]...)
	}

	return res
}
func (ac Account) AccountHash(comp *wizard.CompiledIOP, name string) *poseidon2.HashingCtx {
	var (
		hashInputs []ifaces.Column
		size       = ac.Nonce[0].Size()
		zeroColumn = verifiercol.NewConstantCol(field.Zero(), size, "ZERO_COLUMN")
	)
	hashInputs = append(hashInputs, padd(ac.Nonce[:], 16, zeroColumn)...)
	hashInputs = append(hashInputs, padd(ac.Balance[:], 16, zeroColumn)...)
	hashInputs = append(hashInputs, ac.StorageRoot[:]...)
	hashInputs = append(hashInputs, ac.LineaCodeHash[:]...)
	hashInputs = append(hashInputs, ac.KeccakCodeHash.Hi[:]...)
	hashInputs = append(hashInputs, ac.KeccakCodeHash.Lo[:]...)
	hashInputs = append(hashInputs, padd(ac.CodeSize[:], 16, zeroColumn)...)
	res := poseidon2.HashOf(comp, hashInputs, "ACCOUNT_HASH_"+name)
	return res
}

func padd(cols []ifaces.Column, size int, padding ifaces.Column) (res []ifaces.Column) {

	for range size - len(cols) {
		res = append(res, padding)
	}
	res = append(res, cols...)
	return res
}
