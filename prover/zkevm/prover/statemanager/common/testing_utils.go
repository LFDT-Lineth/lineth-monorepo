package common

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mock"
)

type TestCase struct {
	Explainer     string
	StateLogsGens func(initState mock.State) [][]mock.StateAccessLog
}

type TestContext struct {
	Addresses                []types.EthAddress
	storageKeys, storageVals []types.FullBytes32
	State                    mock.State
	TestCases                []TestCase
}

func InitializeContext(initialBlock int) *TestContext {
	// fieldOne := field.One()
	// oneBytes := fieldOne.Bytes()
	var (
		addresses = []types.EthAddress{
			types.DummyAddress(32),
			types.DummyAddress(64),
			types.DummyAddress(54),
			types.DummyAddress(23),
		}

		storageKeys = []types.FullBytes32{
			types.DummyFullByte(102),
			types.DummyFullByte(1002),
			types.DummyFullByte(1012),
			types.DummyFullByte(1023),
		}

		storageValues = []types.FullBytes32{
			types.DummyFullByte(202),
			types.DummyFullByte(2002),
			types.DummyFullByte(2012),
			types.DummyFullByte(2023),
			types.DummyFullByte(1),
		}
	)

	state := mock.State{}
	state.InsertEOA(addresses[0], 0, big.NewInt(400))
	state.InsertEOA(addresses[1], 0, big.NewInt(200))
	state.InsertContract(addresses[2], types.DummyKoalaOctuplet(67), types.DummyFullByte(56), 100)
	state.InsertContract(addresses[3], types.DummyKoalaOctuplet(76), types.DummyFullByte(57), 102)
	state.SetStorage(addresses[2], storageKeys[0], storageValues[0])
	state.SetStorage(addresses[2], storageKeys[1], storageValues[1])
	state.SetStorage(addresses[3], storageKeys[2], storageValues[2])
	state.SetStorage(addresses[3], storageKeys[3], storageValues[3])

	testCases := []TestCase{
		{
			Explainer: "Reverted transaction",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[0]).
					IncNonce().
					WriteBalance(big.NewInt(100)).
					Done()
			},
		},
		{
			Explainer: "Update contract and its storage",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					ReadStorage(storageKeys[0]).
					WriteStorage(storageKeys[1], storageValues[0]).
					Done()
			},
		},
		{
			Explainer: "Reading two contracts",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				// Note: addresses are ordered by their Poseidon2 hash (not raw value)
				// addresses[3] hash < addresses[2] hash in Poseidon2
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[3]).
					ReadStorage(storageKeys[0]).
					ReadStorage(storageKeys[1]).
					ReadStorage(storageKeys[2]).
					ReadStorage(storageKeys[3]).
					WithAddress(addresses[2]).
					ReadStorage(storageKeys[0]).
					ReadStorage(storageKeys[1]).
					ReadStorage(storageKeys[2]).
					ReadStorage(storageKeys[3]).
					Done()
			},
		},
		{
			Explainer: "Contract deletion",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[1], storageValues[3]).
					WriteStorage(storageKeys[2], storageValues[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					Done()

			},
		},
		{
			Explainer: "one contract, two blocks",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					ReadStorage(storageKeys[0]).
					WriteStorage(storageKeys[1], storageValues[0]).
					GoNextBlock().
					WithAddress(addresses[2]).
					ReadStorage(storageKeys[0]).
					WriteStorage(storageKeys[1], storageValues[3]).
					Done()
			},
		},
		{
			Explainer: "Redeployed contract",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[1], storageValues[3]).
					WriteStorage(storageKeys[2], storageValues[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)).
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					Done()
			},
		},
		{
			Explainer: "Contract created then deleted in another account",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[1], storageValues[3]).
					WriteStorage(storageKeys[2], storageValues[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					GoNextBlock().
					WithAddress(addresses[2]).
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)).
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					Done()
			},
		},
		{
			Explainer: "Reading a non-existing contract",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					// Note: with the 0x0 address, it will not work because
					// there is a constraint ensuring that the address
					// transition from non-zero to zero when going inactive.
					WithAddress(types.DummyAddress(1)).
					ReadBalance().Done()
			},
		},
		{
			Explainer: "Multi-Block: Redeployment, erasing account, Reading non-existent contract, ",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]). // account at Addresses[2] already exists in the initial state, deployment number 0
					WriteStorage(storageKeys[1], storageValues[3]).
					WriteStorage(storageKeys[2], storageValues[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // deployment number should now be 1
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]). // storageKeys[0] will appear on a row with deployment number 1. minDeplBlock will be 0, and will not be equal to 1 here (meaning that this row might be filtered out depending on the filter)
					GoNextBlock().
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[1], storageValues[3]).
					WriteStorage(storageKeys[2], storageValues[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					GoNextBlock().
					WithAddress(types.DummyAddress(1)).
					ReadBalance().
					Done()
			},
		},
		{
			Explainer: "Multi-Block 2: Read account from initial state, readST and WS deletion in the next block, deployment in the next block, deletion and deployment in the last block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]). // block 0 deplNo 0
					WriteStorage(storageKeys[1], storageValues[3]).
					GoNextBlock().
					WithAddress(addresses[2]). // block 1
					ReadStorage(storageKeys[3]).
					EraseAccount(). // Erase account 2
					GoNextBlock().
					WithAddress(addresses[2]).                                                  // block 2
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // redeploy at address 2 deplNo 1
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					GoNextBlock().
					WithAddress(addresses[2]). // block 3, account 2
					ReadStorage(storageKeys[3]).
					EraseAccount().                                                             //delete account 2
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // redeploy 2 deplNo 2
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					Done()
			},
		},
		{
			Explainer: "Multi-Block 3: read and erase account, deploy it in the next block and read another account in that same block, read the second account in the last block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[0], types.DummyFullByte(1000)).
					ReadStorage(storageKeys[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}). // delete ST
					EraseAccount().
					GoNextBlock().
					WithAddress(addresses[2]).
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)).
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					WithAddress(addresses[3]). // switch to another account
					ReadStorage(storageKeys[3]).
					WriteStorage(storageKeys[2], types.FullBytes32{}). // delete ST
					ReadStorage(storageKeys[2]).
					GoNextBlock().
					WithAddress(addresses[3]).
					WriteStorage(storageKeys[2], types.DummyFullByte(100)). // delete ST
					Done()
			},
		},
		{
			Explainer: "Multi-Block 4: Erase account, then read another account in the same block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					EraseAccount().
					WithAddress(addresses[3]).
					ReadStorage(storageKeys[0]).
					Done()
			},
		},
		{
			Explainer: "Multi-Block 5: Erase account, redeploy another account in the same block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[3]).
					EraseAccount().            // erase account 3
					WithAddress(addresses[2]). // switch to account 2
					WriteStorage(storageKeys[1], storageValues[3]).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // redeploy 2
					Done()
			},
		},
		{
			Explainer: "Multi-Block 6: Existing contract 2 deleted in another block, re-creating it. deleting account 3 in a subsequent block, reading the storage of 2 in subsequent block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]).
					EraseAccount().                                                             // Erase account 2
					GoNextBlock().                                                              // block 1
					WithAddress(addresses[2]).                                                  // switch to account 2
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // deploy at address 2
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					GoNextBlock().               // block 2
					WithAddress(addresses[3]).   // switch to account 3
					ReadStorage(storageKeys[1]). // READ_ZERO
					EraseAccount().              // erase account 3
					WithAddress(addresses[2]).   // switch to account 2
					WriteStorage(storageKeys[1], storageValues[4]).
					ReadCodeSize().
					Done()
			},
		},
		{
			Explainer: "Multi-Block 7: Existing contract 2 deleted in another block, redeployed, reading from another account 3. deleting account 3 in a subsequent block, redeploying account 2 in subsequent block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[2]). // switch to account 2
					WriteStorage(storageKeys[1], storageValues[3]).
					WriteStorage(storageKeys[2], storageValues[3]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					GoNextBlock(). // block 1
					WithAddress(addresses[2]).
					EraseAccount().                                                             // Erase account 2
					GoNextBlock().                                                              // block 2
					WithAddress(addresses[2]).                                                  // switch to account 2
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // deploy at address 2
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					ReadStorage(storageKeys[3]).
					WithAddress(addresses[3]). // switch to account 3
					ReadStorage(storageKeys[0]).
					ReadStorage(storageKeys[1]).
					ReadStorage(storageKeys[3]).
					GoNextBlock().               // block 3
					WithAddress(addresses[3]).   // switch to account 3
					ReadStorage(storageKeys[1]). // READ_ZERO
					EraseAccount().              // erase account 3
					WithAddress(addresses[2]).   // switch to account 2
					WriteStorage(storageKeys[1], storageValues[3]).
					ReadStorage(storageKeys[3]).
					EraseAccount().                                                             // erase account 2
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)). // redeploy 2
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					Done()
			},
		},
		{
			Explainer: "Multi-Block 8: read EOA at Addresses[0] and contract accounts in the first block, erase EOA and redeploy it as contract account in the second block, " +
				"delete contract account second block, read a non-existing account and an existing one (at Addresses[0]) in the last block",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(addresses[0]).
					IncNonce().
					WithAddress(addresses[1]).
					ReadBalance().
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[2], storageValues[1]).
					GoNextBlock(). // block number 1
					WithAddress(addresses[0]).
					EraseAccount().
					WithAddress(addresses[3]).
					EraseAccount().
					WithAddress(addresses[2]).
					ReadStorage(storageKeys[0]).
					WithAddress(addresses[0]).
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)).
					WriteStorage(storageKeys[2], storageValues[1]).
					EraseAccount().
					InitContract(15, types.DummyFullByte(9111), types.DummyKoalaOctuplet(555)).
					WriteStorage(storageKeys[2], storageValues[2]).
					GoNextBlock(). // block number 2
					WithAddress(addresses[3]).
					ReadCodeHash(). // Read Zero
					ReadNonce().
					WithAddress(addresses[0]).
					ReadStorage(storageKeys[2]). // Read NONZERO
					Done()
			},
		},
		{
			Explainer: "Multi-Block 9: Interleaved reads and deletes, EOAs become are deleted and turned intro contract accounts, and the other way around",
			StateLogsGens: func(initState mock.State) [][]mock.StateAccessLog {
				return mock.NewStateLogBuilder(initialBlock, initState).
					WithAddress(types.DummyAddress(1)).
					ReadBalance().
					ReadCodeHash().
					WithAddress(addresses[2]).
					WriteStorage(storageKeys[0], types.FullBytes32{}).
					ReadStorage(storageKeys[3]).
					EraseAccount().
					GoNextBlock().
					WithAddress(addresses[1]).
					EraseAccount().
					WithAddress(addresses[2]).
					InitContract(45, types.DummyFullByte(5679), types.DummyKoalaOctuplet(346)).
					WriteStorage(storageKeys[2], storageValues[1]).
					ReadStorage(storageKeys[0]).
					WithAddress(types.DummyAddress(1)).
					ReadBalance().
					ReadCodeHash().
					ReadNonce().
					WithAddress(addresses[2]).
					ReadNonce().
					GoNextBlock().
					WithAddress(addresses[1]).
					InitContract(5, types.DummyFullByte(9), types.DummyKoalaOctuplet(6111)).
					WriteStorage(storageKeys[2], storageValues[1]).
					WithAddress(addresses[2]).
					EraseAccount().
					InitEoa().
					Done()
			},
		},
	}

	return &TestContext{
		Addresses:   addresses,
		storageKeys: storageKeys,
		storageVals: storageValues,
		State:       state,
		TestCases:   testCases,
	}

}
