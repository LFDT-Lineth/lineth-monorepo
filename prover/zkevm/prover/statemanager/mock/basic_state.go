package mock

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// State is a basic in-memory data-structure storing the state of the EVM. It
// is used to initialize and track the state of the EVM in our mock.
type State map[types.EthAddress]*AccountState

// AccountState stores all the informations of an account in a basic fashion,
// it is meant to be used by the StateAccessFrame builder internally to
// initialize the state and keep track of the state.
type AccountState struct {
	Nonce            int64
	Balance          *big.Int
	KeccakCodeHash   types.FullBytes32
	LineaCodeHash    types.KoalaOctuplet // Poseidon2 code hash
	CodeSize         int64
	Storage          map[types.FullBytes32]types.FullBytes32
	DeploymentNumber int64
}

// deepCopyState makes a hard copy of the state so that modifications of the
// copy will not affect the original state.
func (s State) DeepCopy() State {
	res := State{}
	for k, v := range s {
		vCopy := *v
		vCopy.Storage = map[types.FullBytes32]types.FullBytes32{}
		for sk, sv := range v.Storage {
			vCopy.Storage[sk] = sv
		}
		res[k] = &vCopy
	}
	return res
}

// InsertEOA inserts an EOA into the state
func (s State) InsertEOA(a types.EthAddress, nonce int64, balance *big.Int) {
	if _, ok := s[a]; ok {
		panic("account already exists for the address")
	}

	s[a] = &AccountState{
		Nonce:          nonce,
		Balance:        balance,
		LineaCodeHash:  statemanager.EmptyCodeHash(),
		KeccakCodeHash: types.AsFullBytes32(statemanager.LEGACY_KECCAK_EMPTY_CODEHASH),
		CodeSize:       0,
	}
}

// InsertContract inserts an empty contract into the account. The contract has
// empty storage and no balance
func (s State) InsertContract(a types.EthAddress, lineaCodeHash types.KoalaOctuplet, keccakCodeHash types.FullBytes32, codeSize int64) {
	if _, ok := s[a]; ok {
		panic("account already exists for the address")
	}

	s[a] = &AccountState{
		Nonce:          0,
		Balance:        &big.Int{},
		LineaCodeHash:  lineaCodeHash,
		KeccakCodeHash: keccakCodeHash,
		CodeSize:       codeSize,
		Storage:        map[types.FullBytes32]types.FullBytes32{},
	}
}

// SetNonce initializes the nonce of an account. If the account does not exists
// in the map it will be created.
func (s State) SetNonce(a types.EthAddress, nonce int64) {
	s.initAccountIfNil(a)
	s[a].Nonce = nonce
}

// SetBalance initializes the balance of an account. If the account does not
// already exists, it will be initialized as empty.
func (s State) SetBalance(a types.EthAddress, balance *big.Int) {
	s.initAccountIfNil(a)
	s[a].Balance = balance
}

// SetCodeHash initializes the keccak code hash of an account and initializes
// an empty account with the value. If the account does not already exist.
func (s State) SetCodeHash(a types.EthAddress, codeHash types.FullBytes32) {
	s.initAccountIfNil(a)
	s[a].KeccakCodeHash = codeHash
}

// SetPoseidon2CodeHash initializes the Poseidon2 code hash of an account and initializes
// an empty account with the value. If the account does not already exist.
func (s State) SetPoseidon2CodeHash(a types.EthAddress, codeHash types.KoalaOctuplet) {
	s.initAccountIfNil(a)
	s[a].LineaCodeHash = codeHash
}

// SetCodeSize initializes the code size of an account and initializes the
// account to an empty value if it does not exist.
func (s State) SetCodeSize(a types.EthAddress, codeSize int64) {
	s.initAccountIfNil(a)
	s[a].CodeSize = codeSize
}

// SetCodeSize initializes the code size of an account and initializes the
// account to an empty value if it does not exist.
func (s State) SetStorage(a types.EthAddress, key, value types.FullBytes32) {
	s.initAccountIfNil(a)
	s[a].Storage[key] = value
}

// GetNonce returns the nonce of the account. Returns 0 if the account does
// not exists.
func (s State) GetNonce(a types.EthAddress) int64 {
	s.initAccountIfNil(a)
	return s[a].Nonce
}

// GetBalance returns the balance of the account. Returns 0 if the account does
// not exist.
func (s State) GetBalance(a types.EthAddress) *big.Int {
	s.initAccountIfNil(a)
	return s[a].Balance
}

// GetCodeHash returns the code hash of an account and zero if the account does
// not exist.
func (s State) GetCodeHash(a types.EthAddress) types.FullBytes32 {
	s.initAccountIfNil(a)
	return s[a].KeccakCodeHash
}

// GetMimcCodeHash returns the Mimc code hash of an account and zero if the account does
// not exist.
func (s State) GetPoseidon2CodeHash(a types.EthAddress) types.KoalaOctuplet {
	s.initAccountIfNil(a)
	return s[a].LineaCodeHash
}

// SetCodeSize returns the code size of an account and initializes the
// account to an empty value if it does not exist.
func (s State) GetCodeSize(a types.EthAddress) int64 {
	s.initAccountIfNil(a)
	return s[a].CodeSize
}

// GetStorage returns a storage value for the given address and returns 0 if the
// account does not exist.
func (s State) GetStorage(a types.EthAddress, key types.FullBytes32) types.FullBytes32 {
	s.initAccountIfNil(a)
	return s[a].Storage[key]
}

func (s State) initAccountIfNil(a types.EthAddress) {
	if _, ok := s[a]; !ok {
		s[a] = emptyAccount()
	}
}

func emptyAccount() *AccountState {
	return &AccountState{
		Balance:        &big.Int{},
		Storage:        map[types.FullBytes32]types.FullBytes32{},
		KeccakCodeHash: keccak.Hash([]byte{}),
	}
}
