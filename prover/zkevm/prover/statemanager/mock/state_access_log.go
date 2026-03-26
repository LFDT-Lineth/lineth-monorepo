package mock

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// OperationType specifies the part of an account that is touched by a state
// access. The 0 value corresponds to an invalid OperationType.
type OperationType int

const (
	// Storage denotes an access to the storage of an account
	Storage OperationType = iota + 1
	// Balance denotes an access to the balance of an account
	Balance
	// Nonce denotes an access to the nonce of an account
	Nonce
	// CodeSize denotes an access to the code size of an account. Only for
	// read-only operations. Otherwise, it would be a deploy.
	Codesize
	// CodeHash denotes an access to the code hash of an account. Only for
	// read-only operations. Otherwise, it would be a deploy.
	Codehash
	// SquashedAccount denotes an summary of all the accesses made on an account
	// aside from its storage. This is meant to be used by the Shomei trace
	// generator.
	SquashedAccount
	// AccountErasal indicates that the account deleted from the state. This
	// happens at the end of a transaction calling selfdestruct when the account
	// was existing before the transaction or during a transaction when the
	// selfdestruct is called on an ephemeral account (i.e. an account created
	// by the same transaction).
	AccountErasal
	// AccountInit indicates that the account has been (re)deployed or simply
	// initialized. This can happen when CREATE or CREATE2 are called or when
	// a non-existing account receives a transfer.
	AccountInit
)

// StateAccessLog represents an atomic operation over an account, the
// operation can relate to the account itself or to the storage of the account.
// Stacking StateAccessLog in a slice allows representing a succession of
// operation on the state of Ethereum. However, this is different from a
// sequence of EVM opcode: StateAccessLog have to be understood as "immediately"
// effective while EVM opcodes can have deferred effect on the state.
//
// An example of these two being differents is that self-destruct does not
// immediately remove the account from the state if the account pre-existed
// the transaction.
type StateAccessLog struct {
	// Address indicates the address of the account being accessed by the
	// current state log.
	Address types.EthAddress
	// Block indicates the block in which the access takes place.
	Block int
	// Type adds information about which part of the account is being accessed
	// or the type of access.
	Type OperationType
	// Key is only used for storage accesses and indicates the storage key
	// being accessed.
	Key types.FullBytes32
	// Value contains the "read" value if the log is a read-only log or the
	// new value if the log is write operation.
	Value any
	// OldValue is used only for write operations and indicates the value
	// of the accessed state item prior to being updated.
	OldValue any
	// IsWrite tells whether the logged state access is a write operation or
	// a read-only operation.
	IsWrite bool // false means read
}

// StateLogBuilder is a utility tool to construct sequences [StateAccessLog]
// spanning across several blocks and account. It is meant to be used to
// generate mocked shomei and arithmetization traces
//
// @alex: add wayyyyy more doc
type StateLogBuilder struct {
	logsByBlock *[][]StateAccessLog
	currBlock   int
	currAddress types.EthAddress
	currState   State
}

// NewStateLogBuilder creates a [StateLogBuilder] that is ready to be used.
// The returned object still needs to be given a "current" account to work on.
// For this reason, the Builder should call [WithAddress] immediately after
// being initialized.
func NewStateLogBuilder(currentBlock int, initialState State) *StateLogBuilder {
	return &StateLogBuilder{
		logsByBlock: &[][]StateAccessLog{{}},
		currBlock:   currentBlock,
		currState:   initialState.DeepCopy(),
	}
}

// Done returns the state frames built so far.
func (b *StateLogBuilder) Done() [][]StateAccessLog {
	return *b.logsByBlock
}

// GoNextBlock tells the StateFrameBuilder that the traces coming next belong
// to a new block.
func (b *StateLogBuilder) GoNextBlock() *StateLogBuilder {
	*b.logsByBlock = append(*b.logsByBlock, []StateAccessLog{})
	b.currBlock++
	return b
}

// WithAddress specifies in which account the StateLogBuilder must generate
// the following StateAccessLogs. Meaning, to what account will the next logs
// be operating on.
func (b *StateLogBuilder) WithAddress(address types.EthAddress) *StateLogBuilder {
	b.currAddress = address
	return b
}

// ReadStorage generates StateAccessLog corresponding to a SLOAD in the
// current account and the current block.
func (b *StateLogBuilder) ReadStorage(key types.FullBytes32) *StateLogBuilder {
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    Storage,
			Key:     key,
			Value:   b.currState.GetStorage(b.currAddress, key),
			IsWrite: false,
		},
	)
	return b
}

// WriteStorage generates a new StateAccessLog corresponding to an SSTORE in
// the current account.
func (b *StateLogBuilder) WriteStorage(key types.FullBytes32, value types.FullBytes32) *StateLogBuilder {
	oldValue := b.currState.GetStorage(b.currAddress, key)
	b.currState.SetStorage(b.currAddress, key, value)
	b.pushFrame(
		StateAccessLog{
			Block:    b.currBlock,
			Address:  b.currAddress,
			Type:     Storage,
			Key:      key,
			Value:    value,
			OldValue: oldValue,
			IsWrite:  true,
		},
	)
	return b
}

// ReadBalance instruct the StateFrameBuilder to generate a frame representing
// an account balance read access.
func (b *StateLogBuilder) ReadBalance() *StateLogBuilder {
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    Balance,
			Value:   b.currState.GetBalance(b.currAddress),
			IsWrite: false,
		},
	)
	return b
}

// WriteBalance instructs the StateFrameBuilder to generate a frame representing a
// update the balance of an account. This panics over a negative value.
func (b *StateLogBuilder) WriteBalance(balance *big.Int) *StateLogBuilder {
	if balance.Cmp(&big.Int{}) < 0 {
		panic("got a negative balance parameter")
	}

	oldBalance := b.currState.GetBalance(b.currAddress)
	b.currState.SetBalance(b.currAddress, balance)

	b.pushFrame(
		StateAccessLog{
			Block:    b.currBlock,
			Address:  b.currAddress,
			Type:     Balance,
			IsWrite:  true,
			Value:    balance,
			OldValue: oldBalance,
		},
	)

	return b
}

// ReadNonce instructs the StateFrameBuilder to generate a frame representing
// the read of the nonce of the current account.
func (b *StateLogBuilder) ReadNonce() *StateLogBuilder {
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    Nonce,
			Value:   b.currState.GetNonce(b.currAddress),
			IsWrite: false,
		},
	)
	return b
}

// IncNonce instructs the StateFrameBuilder to generate a frame representing
// the incrementation of the nonce.
func (b *StateLogBuilder) IncNonce() *StateLogBuilder {
	oldNonce := b.currState.GetNonce(b.currAddress)
	newNonce := oldNonce + 1
	b.currState.SetNonce(b.currAddress, newNonce)
	b.pushFrame(
		StateAccessLog{
			Block:    b.currBlock,
			Address:  b.currAddress,
			Type:     Nonce,
			Value:    newNonce,
			OldValue: oldNonce,
			IsWrite:  true,
		},
	)
	return b
}

// ReadCodeSize instructs the StateFrameBuilder to generate a frame for reading
// the codesize of the current contract.
func (b *StateLogBuilder) ReadCodeSize() *StateLogBuilder {
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    Codesize,
			Value:   b.currState.GetCodeSize(b.currAddress),
			IsWrite: false,
		},
	)
	return b
}

// ReadCodeHash instructs the StateFrameBuilder to generate a frame for reading
// the (keccak) code hash of the current contract.
func (b *StateLogBuilder) ReadCodeHash() *StateLogBuilder {
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    Codehash,
			Value:   b.currState.GetCodeHash(b.currAddress),
			IsWrite: false,
		},
	)
	return b
}

// EraseAccount instructs the StateFrameBuilder to generate a frame for
// erasing the account. Its use-case are, either at the end of a SELFDESTRUCT
// calling transaction when the account pre-existed or during the transaction
// if the account is transient.
func (b *StateLogBuilder) EraseAccount() *StateLogBuilder {
	// Note that an empty account cannot destroy itself since it needs to at
	// least have code to do so.
	b.currState[b.currAddress] = emptyAccount()
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    AccountErasal,
			IsWrite: true,
		},
	)
	return b
}

// Create instructs the StateFrameBuilder that a new contract was deployed to
// the current address.
func (b *StateLogBuilder) InitContract(codeSize int64, keccakCodeHash types.FullBytes32, poseidonCodeHash types.KoalaOctuplet) *StateLogBuilder {
	state := b.currState
	address := b.currAddress
	state.SetCodeSize(address, codeSize)
	state.SetCodeHash(address, keccakCodeHash)
	state.SetPoseidon2CodeHash(address, poseidonCodeHash)
	b.pushFrame(
		StateAccessLog{
			Block:   b.currBlock,
			Address: b.currAddress,
			Type:    AccountInit,
			Value:   []any{codeSize, keccakCodeHash, poseidonCodeHash},
			IsWrite: true,
		},
	)
	return b
}

// SetNoCode initializes an EOA with no balance and a nonce of zero. This is
// a short-hand for "Deploy" with no code.
func (b *StateLogBuilder) InitEoa() *StateLogBuilder {
	return b.InitContract(
		0,
		types.AsFullBytes32(statemanager.LEGACY_KECCAK_EMPTY_CODEHASH),
		statemanager.EmptyCodeHash(),
	)
}

// pushFrame appends the input StateAccessLog to the list of generated
// frames.
func (b *StateLogBuilder) pushFrame(f StateAccessLog) {
	lastBlock := len(*b.logsByBlock) - 1
	confRange := *b.logsByBlock
	confRange[lastBlock] = append(confRange[lastBlock], f)
}
