package mock

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// TestContext is used to hold the data needed for running tests
type TestContext struct {
	addresses                                  []types.EthAddress
	storageKeys, storageVals, keccakCodeHashes []types.FullBytes32
	poseidon2CodeHashes                        []types.KoalaOctuplet
	blockNumber                                int
	state                                      State
	// tMessages are test messages for the tFunc functions
	tMessages [13]string
	// tFunc are test functions that will be called by a composite test
	tFunc [13]func(t *testing.T, tContext *TestContext) [][]StateAccessLog
}

// InitializeContext returns a new context for testing
func InitializeContext() *TestContext {
	var (
		tMessages = [...]string{
			"A transfers some eths",
			"B is created after receiving a transfer",
			"Reading over an existing contract and its storage",
			"Reading over an existing account",
			"Writing over an existing account",
			"Reading over a missing account",
			"Deploying a new contract C",
			"Read before deploying a new contract C",
			"Delete an account",
			"Ephemeral account",
			"Far-fetched ephemeral account",
			"Multi-block with same account",
			"Redeploy"}
		tFunc = [...]func(t *testing.T, tContext *TestContext) [][]StateAccessLog{
			TestingTransfer,
			TestingCreationUponTransfer,
			TestingReadAccAndStorage,
			TestingReadingOverExisting,
			TestingWritingOverExisting,
			TestingReadingOverMissingAccount,
			TestingDeployingANewContract,
			TestingReadBeforeDeployingNewContract,
			TestingDelete,
			TestingEphemeral,
			TestingFarFetchedEphemeral,
			TestingMultiBlockSameAccount,
			TestingRedeploy}
	)
	var (
		addresses = []types.EthAddress{
			types.DummyAddress(57),
			types.DummyAddress(89),
			types.DummyAddress(89083),
			types.DummyAddress(8544),
		}
		storageKeys = []types.FullBytes32{
			types.DummyFullByte(0),
			types.DummyFullByte(1),
			types.DummyFullByte(2),
			types.DummyFullByte(1000),
		}
		storageVals = []types.FullBytes32{
			types.DummyFullByte(4000),
			types.DummyFullByte(4001),
			types.DummyFullByte(4002),
			types.DummyFullByte(4003),
		}
		keccakCodeHashes = []types.FullBytes32{
			types.DummyFullByte(5000),
			types.DummyFullByte(5001),
			types.DummyFullByte(5002),
			types.DummyFullByte(5003),
		}
		poseidon2CodeHashes = []types.KoalaOctuplet{
			types.DummyKoalaOctuplet(6000),
			types.DummyKoalaOctuplet(6001),
			types.DummyKoalaOctuplet(6002),
			types.DummyKoalaOctuplet(6003),
		}
	)
	state := State{}
	state.InsertEOA(addresses[0], 3, big.NewInt(500))
	state.InsertContract(addresses[1], poseidon2CodeHashes[1], keccakCodeHashes[1], 1001)
	state.SetStorage(addresses[1], storageKeys[0], storageVals[0])
	state.SetStorage(addresses[1], storageKeys[1], storageVals[1])
	return &TestContext{
		addresses:           addresses,
		storageKeys:         storageKeys,
		storageVals:         storageVals,
		keccakCodeHashes:    keccakCodeHashes,
		poseidon2CodeHashes: poseidon2CodeHashes,
		state:               state,
		blockNumber:         1002,
		tMessages:           tMessages,
		tFunc:               tFunc,
	}
}

// TestingTransfer checks when account A transfers some eths
func TestingTransfer(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[0]).
		IncNonce().
		WriteBalance(big.NewInt(300))
	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingCreationUponTransfer checks what happens when account B is created after receiving a transfer
func TestingCreationUponTransfer(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[3]).
		InitEoa().
		WriteBalance(big.NewInt(200))

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingReadAccAndStorage checks reading over an account data and its storage
func TestingReadAccAndStorage(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[1]).
		ReadBalance().
		ReadCodeHash().
		ReadCodeSize().
		ReadNonce().
		ReadStorage(tContext.storageKeys[0]).
		ReadStorage(tContext.storageKeys[1])

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingReadingOverExisting checks reading the data of an existing account
func TestingReadingOverExisting(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[1]).
		ReadBalance().
		ReadCodeHash().
		ReadCodeSize().
		ReadNonce()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingWritingOverExisting checks writing operations over an existing account
func TestingWritingOverExisting(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[1]).
		WriteBalance(big.NewInt(200)).
		WriteStorage(tContext.storageKeys[0], types.FullBytes32{}).
		WriteStorage(tContext.storageKeys[1], tContext.storageVals[2]).
		ReadCodeSize().
		ReadNonce()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingReadingOverMissingAccount checks reading operations over a missing (non-existing) account
func TestingReadingOverMissingAccount(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[3]).
		ReadBalance().
		ReadCodeHash().
		ReadCodeSize().
		ReadNonce()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingDeployingANewContract checks the deployment of a new account, and then write/read operations over its storage
func TestingDeployingANewContract(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[2]).
		InitContract(31, tContext.keccakCodeHashes[2], tContext.poseidon2CodeHashes[2]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[0]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[3]).
		WriteStorage(tContext.storageKeys[1], tContext.storageVals[0]).
		WriteStorage(tContext.storageKeys[1], types.FullBytes32{}). // = storage slot deletion
		ReadStorage(tContext.storageKeys[0]).
		ReadStorage(tContext.storageKeys[1]).
		ReadStorage(tContext.storageKeys[2])

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingReadBeforeDeployingNewContract checks a reading operations followed by the actual deployment
func TestingReadBeforeDeployingNewContract(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[2]).
		ReadBalance().
		InitContract(31, tContext.keccakCodeHashes[2], tContext.poseidon2CodeHashes[2]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[0]).
		ReadBalance()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingDelete checks writes into the storage of an existing account followed by a deletion
func TestingDelete(t *testing.T, tContext *TestContext) [][]StateAccessLog {

	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[1]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[0]).
		WriteStorage(tContext.storageKeys[1], tContext.storageVals[3]).
		EraseAccount()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingEphemeral checks the creation of an account that gets deleted afterwards
func TestingEphemeral(t *testing.T, tContext *TestContext) [][]StateAccessLog {

	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[2]).
		InitContract(31, tContext.keccakCodeHashes[2], tContext.poseidon2CodeHashes[2]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[0]).
		EraseAccount()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingFarFetchedEphemeral checks the scenario when an account gets created, then deleted. Afterward, it is created and deleted once more.
func TestingFarFetchedEphemeral(t *testing.T, tContext *TestContext) [][]StateAccessLog {

	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[2]).
		ReadBalance().
		InitContract(31, tContext.keccakCodeHashes[2], tContext.poseidon2CodeHashes[2]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[0]).
		EraseAccount().
		ReadNonce().
		InitContract(31, tContext.keccakCodeHashes[2], tContext.poseidon2CodeHashes[2]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[0]).
		EraseAccount()

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingMultiBlockSameAccount checks the scenario in which the same account gets written again in a subsequent block
func TestingMultiBlockSameAccount(t *testing.T, tContext *TestContext) [][]StateAccessLog {
	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[0]).
		IncNonce().
		WriteBalance(big.NewInt(400))

	builder.WithAddress(tContext.addresses[0]).
		IncNonce().
		WriteBalance(big.NewInt(400))

	builder.GoNextBlock()

	builder.WithAddress(tContext.addresses[0]).
		IncNonce().
		WriteBalance(big.NewInt(300))

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestingRedeploy checks the redeployment of an account. It is similar to the ephemeral case, but the account persists and still exists at the end of the test
func TestingRedeploy(t *testing.T, tContext *TestContext) [][]StateAccessLog {

	builder := NewStateLogBuilder(tContext.blockNumber, tContext.state)
	builder.WithAddress(tContext.addresses[1]).
		WriteStorage(tContext.storageKeys[0], tContext.storageVals[1]).
		WriteStorage(tContext.storageKeys[1], types.FullBytes32{}).
		ReadStorage(tContext.storageKeys[2]).
		EraseAccount().
		ReadBalance().
		InitContract(100, tContext.keccakCodeHashes[3], tContext.poseidon2CodeHashes[3]).
		ReadStorage(tContext.storageKeys[0]).
		WriteStorage(tContext.storageKeys[1], tContext.storageVals[3])

	builder.WithAddress(tContext.addresses[0]).
		IncNonce().
		WriteBalance(big.NewInt(400))

	frames := builder.Done()
	AssertShomeiAgree(t, tContext.state, frames)
	return frames
}

// TestShomeiTraceConversion perform all the tests above one after the other
func TestShomeiTraceConversion(t *testing.T) {
	tContext := InitializeContext()
	for i := range tContext.tMessages {
		t.Run(tContext.tMessages[i], func(t *testing.T) {
			tContext.tFunc[i](t, tContext)
		})
	}
}
