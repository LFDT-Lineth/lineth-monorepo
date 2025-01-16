package statesummary

import (
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

var (
	emptyCodeHash = statemanager.EmptyCodeHash(statemanager.MIMC_CONFIG)
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
	HashInitial, HashFinal ifaces.Column

	// ComputeHashInitial and ComputeHashFinal are [wizard.ProverAction]
	// responsible for hashing the accounts.
	ComputeHashInitial, ComputeHashFinal wizard.ProverAction

	// InitialAndFinalAreSame is an indicator column set to 1 when the
	// initial and final account share the same hash and 0 otherwise.
	InitialAndFinalAreSame ifaces.Column

	// ComputeInitialAndFinalAreSame is a [wizard.ProverAction] responsible for
	// computing the column InitialAndFinalAreSame
	ComputeInitialAndFinalAreSame wizard.ProverAction

	// Address represents which account is being peeked by the module.
	// It is assigned by providing
	Address ifaces.Column

	// AddressHash is the hash of the account address
	AddressHash ifaces.Column

	// ComputeAddressHash is responsible for computing the AddressHash
	ComputeAddressHash wizard.ProverAction

	// AddressHashLimbs stores the limbs of the address
	AddressHashLimbs byte32cmp.LimbColumns

	// ComputeAddressLimbs computes the [AddressLimbs] column.
	ComputeAddressLimbs wizard.ProverAction

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
		)
	}

	accPeek := AccountPeek{
		Initial: newAccount(comp, size, "OLD_ACCOUNT"),
		Final:   newAccount(comp, size, "NEW_ACCOUNT"),
		Address: createCol("ADDRESS"),
	}

	accPeek.HashInitial, accPeek.ComputeHashInitial = common.HashOf(
		comp,
		[]ifaces.Column{
			accPeek.Initial.Nonce,
			accPeek.Initial.Balance,
			accPeek.Initial.StorageRoot,
			accPeek.Initial.MiMCCodeHash,
			accPeek.Initial.KeccakCodeHash.Lo,
			accPeek.Initial.KeccakCodeHash.Hi,
			accPeek.Initial.CodeSize,
		},
	)

	accPeek.HashFinal, accPeek.ComputeHashFinal = common.HashOf(
		comp,
		[]ifaces.Column{
			accPeek.Final.Nonce,
			accPeek.Final.Balance,
			accPeek.Final.StorageRoot,
			accPeek.Final.MiMCCodeHash,
			accPeek.Final.KeccakCodeHash.Lo,
			accPeek.Final.KeccakCodeHash.Hi,
			accPeek.Final.CodeSize,
		},
	)

	accPeek.InitialAndFinalAreSame, accPeek.ComputeInitialAndFinalAreSame = dedicated.IsZero(
		comp,
		sym.Sub(accPeek.HashInitial, accPeek.HashFinal),
	)

	accPeek.AddressHash, accPeek.ComputeAddressHash = common.HashOf(
		comp,
		[]ifaces.Column{
			accPeek.Address,
		},
	)

	accPeek.AddressHashLimbs, accPeek.ComputeAddressLimbs = byte32cmp.Decompose(
		comp,
		accPeek.AddressHash,
		16, // numLimbs so that we have 20 bytes
		16, // number of bits per limbs (= 2 bytes)
	)

	accPeek.HasGreaterAddressAsPrev, accPeek.HasSameAddressAsPrev, _, accPeek.ComputeAddressComparison = byte32cmp.CmpMultiLimbs(
		comp,
		accPeek.AddressHashLimbs,
		accPeek.AddressHashLimbs.Shift(-1),
	)

	return accPeek
}

// Account provides the columns to store the values of an account that
// we are peeking at.
type Account struct {
	// Nonce, Balance, MiMCCodeHash and CodeSize store the account field on a
	// single column each.
	Exists, Nonce, Balance, MiMCCodeHash, CodeSize, StorageRoot ifaces.Column
	// KeccakCodeHash stores the keccak code hash of the account.
	KeccakCodeHash common.HiLoColumns
	// HasEmptyCodeHash is an indicator column indicating whether the current
	// account has an empty codehash
	HasEmptyCodeHash             ifaces.Column
	CptHasEmptyCodeHash          wizard.ProverAction
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
		)
	}

	acc := Account{
		Exists:                       createCol("EXISTS"),
		Nonce:                        createCol("NONCE"),
		Balance:                      createCol("BALANCE"),
		MiMCCodeHash:                 createCol("MIMC_CODEHASH"),
		CodeSize:                     createCol("CODESIZE"),
		StorageRoot:                  createCol("STORAGE_ROOT"),
		KeccakCodeHash:               common.NewHiLoColumns(comp, size, name+"_KECCAK_CODE_HASH"),
		ExistsAndHasNonEmptyCodeHash: createCol("EXISTS_AND_NON_EMPTY_CODEHASH"),
	}

	// There is no need for an IsActive mask here because the column will be
	// multiplied by Exists which is already zero when inactive.
	acc.HasEmptyCodeHash, acc.CptHasEmptyCodeHash = dedicated.IsZero(comp, acc.CodeSize)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_%v_CPT_EXIST_AND_NONEMPTY_CODE", name),
		sym.Sub(
			acc.ExistsAndHasNonEmptyCodeHash,
			sym.Mul(
				sym.Sub(1, acc.HasEmptyCodeHash),
				acc.Exists,
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_%v_MIMC_CODEHASH_FOR_EXISTING_BUT_EMPTY_CODE", name),
		sym.Mul(
			acc.Exists,
			acc.HasEmptyCodeHash,
			sym.Sub(acc.MiMCCodeHash, *new(field.Element).SetBytes(emptyCodeHash[:])),
		),
	)

	return acc
}

// accountPeekAssignmentBuilder is a convenience structure storing column
// builders relating to AccountPeek
type accountPeekAssignmentBuilder struct {
	initial, final accountAssignmentBuilder
	address        *common.VectorBuilder
}

// newAccountPeekAssignmentBuilder initializes a fresh accountPeekAssignmentBuilder
func newAccountPeekAssignmentBuilder(ap *AccountPeek) accountPeekAssignmentBuilder {
	return accountPeekAssignmentBuilder{
		initial: newAccountAssignmentBuilder(&ap.Initial),
		final:   newAccountAssignmentBuilder(&ap.Final),
		address: common.NewVectorBuilder(ap.Address),
	}
}

// accountAssignmentBuilder is a convenience structure storing the column
// builders relating to the an Account.
type accountAssignmentBuilder struct {
	exists, nonce, balance, miMCCodeHash, codeSize, storageRoot *common.VectorBuilder
	keccakCodeHash                                              common.HiLoAssignmentBuilder
	existsAndHasNonEmptyCodeHash                                *common.VectorBuilder
}

// newAccountAssignmentBuilder returns a new [accountAssignmentBuilder] bound
// to an [Account].
func newAccountAssignmentBuilder(ap *Account) accountAssignmentBuilder {
	return accountAssignmentBuilder{
		exists:                       common.NewVectorBuilder(ap.Exists),
		nonce:                        common.NewVectorBuilder(ap.Nonce),
		balance:                      common.NewVectorBuilder(ap.Balance),
		miMCCodeHash:                 common.NewVectorBuilder(ap.MiMCCodeHash),
		codeSize:                     common.NewVectorBuilder(ap.CodeSize),
		storageRoot:                  common.NewVectorBuilder(ap.StorageRoot),
		existsAndHasNonEmptyCodeHash: common.NewVectorBuilder(ap.ExistsAndHasNonEmptyCodeHash),
		keccakCodeHash:               common.NewHiLoAssignmentBuilder(ap.KeccakCodeHash),
	}
}

// pushAll stacks the value of a [types.Account] as a new row on the receiver.
func (ss *accountAssignmentBuilder) pushAll(acc types.Account) {
	// accountExists is telling whether the intent is to push an empty account
	accountExists := acc.Balance != nil

	ss.nonce.PushInt(int(acc.Nonce))

	// This is telling us whether the intent is to push an empty account
	if accountExists {
		ss.balance.PushBytes32(types.LeftPadToBytes32(acc.Balance.Bytes()))
		ss.exists.PushOne()
		ss.keccakCodeHash.Push(acc.KeccakCodeHash)
	} else {
		ss.balance.PushZero()
		ss.exists.PushZero()
		ss.keccakCodeHash.Push(types.FullBytes32(statemanager.LEGACY_KECCAK_EMPTY_CODEHASH))
	}

	ss.codeSize.PushInt(int(acc.CodeSize))
	ss.miMCCodeHash.PushBytes32(acc.MimcCodeHash)
	ss.storageRoot.PushBytes32(acc.StorageRoot)
	ss.existsAndHasNonEmptyCodeHash.PushBoolean(accountExists && acc.CodeSize > 0)
}

// pushOverrideStorageRoot is as [accountAssignmentBuilder.pushAll] but allows
// the caller to override the StorageRoot field with the provided one.
func (ss *accountAssignmentBuilder) pushOverrideStorageRoot(
	acc types.Account,
	storageRoot types.Bytes32,
) {
	// accountExists is telling whether the intent is to push an empty account
	accountExists := acc.Balance != nil

	ss.nonce.PushInt(int(acc.Nonce))

	// This is telling us whether the intent is to push an empty account
	if accountExists {
		ss.balance.PushBytes32(types.LeftPadToBytes32(acc.Balance.Bytes()))
		ss.exists.PushOne()
		ss.keccakCodeHash.Push(acc.KeccakCodeHash)
	} else {
		ss.balance.PushZero()
		ss.exists.PushZero()
		ss.keccakCodeHash.Push(types.FullBytes32(statemanager.LEGACY_KECCAK_EMPTY_CODEHASH))
	}

	ss.codeSize.PushInt(int(acc.CodeSize))
	ss.miMCCodeHash.PushBytes32(acc.MimcCodeHash)
	ss.storageRoot.PushBytes32(storageRoot)
	ss.existsAndHasNonEmptyCodeHash.PushBoolean(accountExists && acc.CodeSize > 0)
}

// PadAndAssign terminates the receiver by padding all the columns representing
// the account with "zeroes" rows up to the target size of the column and then
// assigning the underlying [ifaces.Column] object with it.
func (ss *accountAssignmentBuilder) PadAndAssign(run *wizard.ProverRuntime) {
	ss.exists.PadAndAssign(run)
	ss.nonce.PadAndAssign(run)
	ss.balance.PadAndAssign(run)
	ss.keccakCodeHash.PadAssign(run, types.FullBytes32{})
	ss.miMCCodeHash.PadAndAssign(run)
	ss.storageRoot.PadAndAssign(run)
	ss.codeSize.PadAndAssign(run)
	ss.existsAndHasNonEmptyCodeHash.PadAndAssign(run)
}
