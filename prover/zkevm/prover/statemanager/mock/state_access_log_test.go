package mock

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
)

func TestEoaTransfer(t *testing.T) {

	var (
		initialState = State{}
		addA, _      = types.AddressFromHex("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
		addB, _      = types.AddressFromHex("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		blockNumber  = 1002
	)

	// This initializes the account of A with some Eth to transfer
	initialState.SetBalance(addA, big.NewInt(500))

	// This simulate the balance update of both accounts and the nonce increasal
	// of the first account.
	frames := NewStateLogBuilder(blockNumber, initialState).
		WithAddress(addA).
		IncNonce().
		WriteBalance(big.NewInt(200)).
		WithAddress(addB).
		WriteBalance(big.NewInt(300)).
		Done()

	assert.Len(t, frames, 1)
	assert.Len(t, frames[0], 3)
	assert.Equal(t, frames[0][0], StateAccessLog{Address: addA, Block: blockNumber, Type: Nonce, Value: int64(1), OldValue: int64(0), IsWrite: true})
	assert.Equal(t, frames[0][1], StateAccessLog{Address: addA, Block: blockNumber, Type: Balance, Value: big.NewInt(200), OldValue: big.NewInt(500), IsWrite: true})
	assert.Equal(t, frames[0][2], StateAccessLog{Address: addB, Block: blockNumber, Type: Balance, Value: big.NewInt(300), OldValue: big.NewInt(0), IsWrite: true})

}

func TestContractDeployUpdate(t *testing.T) {

	var (
		initialState      = State{}
		addA, _           = types.AddressFromHex("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
		addB, _           = types.AddressFromHex("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		blockNumber       = 1002
		codeSize          = int64(100)
		codeHash          = types.FullBytes32FromHex("0xaaaaaa") // This will do the padding if necessary
		poseidon2CodeHash = types.MustHexToKoalabearOctuplet("0x000000000000000000000000000000000000000000000000000000000000bbbb")
		stoKey            = types.FullBytes32FromHex("0xcccccc")
		stoVal            = types.FullBytes32FromHex("0xdddddd")
	)

	// This initializes the account of A with some Eth to transfer
	initialState.SetBalance(addA, big.NewInt(500))

	frames := NewStateLogBuilder(blockNumber, initialState).
		WithAddress(addA).
		IncNonce().
		WriteBalance(big.NewInt(450)). // Simulate the fees
		WithAddress(addB).
		InitContract(codeSize, codeHash, poseidon2CodeHash).
		ReadStorage(stoKey).
		WriteStorage(stoKey, stoVal).
		ReadStorage(stoKey).
		Done()

	assert.Len(t, frames, 1)
	assert.Len(t, frames[0], 6)
	assert.Equal(t, frames[0][0], StateAccessLog{Address: addA, Block: blockNumber, Type: Nonce, Value: int64(1), OldValue: int64(0), IsWrite: true})
	assert.Equal(t, frames[0][1], StateAccessLog{Address: addA, Block: blockNumber, Type: Balance, Value: big.NewInt(450), OldValue: big.NewInt(500), IsWrite: true})
	assert.Equal(t, frames[0][2], StateAccessLog{Address: addB, Block: blockNumber, Type: AccountInit, Value: []any{codeSize, codeHash, poseidon2CodeHash}, IsWrite: true})
	assert.Equal(t, frames[0][3], StateAccessLog{Address: addB, Block: blockNumber, Type: Storage, Key: stoKey, Value: types.FullBytes32FromHex("0x00")})
	assert.Equal(t, frames[0][4], StateAccessLog{Address: addB, Block: blockNumber, Type: Storage, Key: stoKey, Value: stoVal, OldValue: types.FullBytes32FromHex("0x00"), IsWrite: true})
	assert.Equal(t, frames[0][5], StateAccessLog{Address: addB, Block: blockNumber, Type: Storage, Key: stoKey, Value: stoVal})

}
