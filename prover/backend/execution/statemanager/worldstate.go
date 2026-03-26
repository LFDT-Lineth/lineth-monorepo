package statemanager

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
)

const WS_LOCATION = "0x"

// Legacy keccak code hash of an empty account
// 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
var LEGACY_KECCAK_EMPTY_CODEHASH = common.FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")

type WorldState struct {
	AccountTrie  *AccountTrie
	StorageTries collection.Mapping[Address, *StorageTrie]
}

func NewWorldState() *WorldState {
	return &WorldState{
		AccountTrie:  accumulator.InitializeProverState[Address, Account](WS_LOCATION),
		StorageTries: collection.NewMapping[Address, *accumulator.ProverState[FullBytes32, FullBytes32]](),
	}
}

// Returns an empty code hash : the hash of an empty bytes32
func EmptyCodeHash() Digest {
	hasher := poseidon2_koalabear.NewMDHasher()
	hasher.Write(make([]byte, 32))
	return types.MustBytesToKoalaOctuplet(hasher.Sum(nil))
}

// Returns an EOA account
func NewEOA(nonce int64, balance *big.Int) Account {
	return Account{
		Account: types.Account{
			Nonce:          nonce,
			Balance:        balance,
			StorageRoot:    EmptyStorageTrieHash(), // The eth
			LineaCodeHash:  EmptyCodeHash(),
			KeccakCodeHash: types.AsFullBytes32(LEGACY_KECCAK_EMPTY_CODEHASH),
			CodeSize:       0,
		},
	}
}

// Returns an empty storage contract
func NewContractEmptyStorage(
	nonce int64,
	balance *big.Int,
	codeHash Digest,
	keccakCodeHash FullBytes32,
	codeSize int64,
) Account {
	return Account{
		Account: types.Account{
			Nonce:          nonce,
			Balance:        balance,
			StorageRoot:    EmptyStorageTrieHash(),
			LineaCodeHash:  codeHash,
			KeccakCodeHash: keccakCodeHash,
			CodeSize:       codeSize,
		},
	}
}

var ZKHASH_EMPTY_STORAGE = EmptyStorageTrieHash()

func NewAccountTrie() *AccountTrie {
	return accumulator.InitializeProverState[Address, Account](WS_LOCATION)
}

func NewStorageTrie(address Address) *StorageTrie {
	return accumulator.InitializeProverState[FullBytes32, FullBytes32](address.Hex())
}

func EmptyStorageTrieHash() Digest {
	// the EthAddress does not contribute to the hash so
	// it is fine to send an empty one.
	trie := NewStorageTrie(Address{})
	return trie.TopRoot()
}
