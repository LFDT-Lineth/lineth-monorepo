package execution

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
)

// checkPublicInputs checks that the values in fi are consistent with the
// wizard.VerifierCircuit
func checkPublicInputs(
	api frontend.API,
	wvc *wizard.VerifierCircuit,
	gnarkFuncInp FunctionalPublicInputSnark,
	limitlessMode bool,
) {

	var (
		lastRollingHash  = internal.CombineBytesIntoElements(api, gnarkFuncInp.FinalRollingHashUpdate)
		firstRollingHash = internal.CombineBytesIntoElements(api, gnarkFuncInp.InitialRollingHashUpdate)
		execDataHash     = execDataHash(api, wvc, limitlessMode)
	)

	// As we have this issue, the execDataHash will not match what we have in the
	// functional input (the txnrlp is incorrect). It should be converted into
	// an [api.AssertIsEqual] once this is resolved.
	//
	api.AssertIsEqual(execDataHash, gnarkFuncInp.DataChecksum)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.L2MessageHash, limitlessMode),
		// TODO: this operation is done a second time when computing the final
		// public input which is wasteful although not dramatic (~8000 unused
		// constraints)
		gnarkFuncInp.L2MessageHashes.CheckSumMiMC(api),
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.InitialStateRootHash, limitlessMode),
		gnarkFuncInp.InitialStateRootHash,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.InitialBlockNumber, limitlessMode),
		gnarkFuncInp.InitialBlockNumber,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.InitialBlockTimestamp, limitlessMode),
		gnarkFuncInp.InitialBlockTimestamp,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.FirstRollingHashUpdate_0, limitlessMode),
		firstRollingHash[0],
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.FirstRollingHashUpdate_1, limitlessMode),
		firstRollingHash[1],
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.FirstRollingHashUpdateNumber, limitlessMode),
		gnarkFuncInp.FirstRollingHashUpdateNumber,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.FinalStateRootHash, limitlessMode),
		gnarkFuncInp.FinalStateRootHash,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.FinalBlockNumber, limitlessMode),
		gnarkFuncInp.FinalBlockNumber,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.FinalBlockTimestamp, limitlessMode),
		gnarkFuncInp.FinalBlockTimestamp,
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.LastRollingHashUpdate_0, limitlessMode),
		lastRollingHash[0],
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.LastRollingHashUpdate_1, limitlessMode),
		lastRollingHash[1],
	)

	api.AssertIsEqual(
		getPublicInput(api, wvc, publicInput.LastRollingHashNumberUpdate, limitlessMode),
		gnarkFuncInp.LastRollingHashUpdateNumber,
	)

	var (
		twoPow128     = new(big.Int).SetInt64(1)
		twoPow112     = new(big.Int).SetInt64(1)
		_             = twoPow128.Lsh(twoPow128, 128)
		_             = twoPow112.Lsh(twoPow112, 112)
		bridgeAddress = api.Add(
			api.Mul(
				twoPow128,
				getPublicInput(api, wvc, publicInput.L2MessageServiceAddrHi, limitlessMode),
			),
			getPublicInput(api, wvc, publicInput.L2MessageServiceAddrLo, limitlessMode),
		)
	)

	// In principle, we should enforce a strict equality between the purported
	// chainID and the one extracted from the traces. But in case, the executed
	// block has only legacy transactions (e.g. transactions without a specified
	// chainID) then the traces will return a chainID of zero.
	//
	// The constraint ensures that either:
	// 1. wizard chainID is 0 (legacy transactions case), OR
	// 2. wizard chainID equals functional chainID
	wizardChainID := getPublicInput(api, wvc, publicInput.ChainID, limitlessMode)
	functionalChainID := gnarkFuncInp.ChainID

	// (wizardChainID - functionalChainID) * wizardChainID == 0
	// This is satisfied when wizardChainID == 0 OR wizardChainID == functionalChainID
	api.AssertIsEqual(
		api.Mul(
			api.Sub(wizardChainID, functionalChainID),
			wizardChainID,
		),
		0,
	)

	api.AssertIsEqual(bridgeAddress, gnarkFuncInp.L2MessageServiceAddr)

}

// execDataHash hash the execution-data with its length so that we can guard
// against padding attack (although the padding attacks are not possible to
// being with due to the encoding of the plaintext)
func execDataHash(
	api frontend.API,
	wvc *wizard.VerifierCircuit,
	limitlessMode bool,
) frontend.Variable {

	hsh, err := mimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}

	hsh.Write(
		getPublicInput(api, wvc, publicInput.DataNbBytes, limitlessMode),
		getPublicInput(api, wvc, publicInput.DataChecksum, limitlessMode),
	)

	return hsh.Sum()
}

// getPublicInput is a wrapper around the public input getter.
func getPublicInput(api frontend.API, wvc *wizard.VerifierCircuit, key string, limitlessMode bool) frontend.Variable {
	if limitlessMode {
		key = "functional." + key
	}
	return wvc.GetPublicInput(api, key)
}
