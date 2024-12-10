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
	wvc *wizard.WizardVerifierCircuit,
	gnarkFuncInp FunctionalPublicInputSnark,
	wizardFuncInp publicInput.FunctionalInputExtractor,
) {

	var (
		lastRollingHash  = internal.CombineBytesIntoElements(api, gnarkFuncInp.FinalRollingHashUpdate)
		firstRollingHash = internal.CombineBytesIntoElements(api, gnarkFuncInp.InitialRollingHashUpdate)
		execDataHash     = execDataHash(api, wvc, wizardFuncInp)
	)

	// As we have this issue, the execDataHash will not match what we have in the
	// functional input (the txnrlp is incorrect). It should be converted into
	// an [api.AssertIsEqual] once this is resolved.
	//
	shouldBeEqual(api, execDataHash, gnarkFuncInp.DataChecksum)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.L2MessageHash.ID).Y,
		// TODO: this operation is done a second time when computing the final
		// public input which is wasteful although not dramatic (~8000 unused
		// constraints)
		gnarkFuncInp.L2MessageHashes.CheckSumMiMC(api),
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.InitialStateRootHash.ID).Y,
		gnarkFuncInp.InitialStateRootHash,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.InitialBlockNumber.ID).Y,
		gnarkFuncInp.InitialBlockNumber,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.InitialBlockTimestamp.ID).Y,
		gnarkFuncInp.InitialBlockTimestamp,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.FirstRollingHashUpdate[0].ID).Y,
		firstRollingHash[0],
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.FirstRollingHashUpdate[1].ID).Y,
		firstRollingHash[1],
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.FirstRollingHashUpdateNumber.ID).Y,
		gnarkFuncInp.FirstRollingHashUpdateNumber,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.FinalStateRootHash.ID).Y,
		gnarkFuncInp.FinalStateRootHash,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.FinalBlockNumber.ID).Y,
		gnarkFuncInp.FinalBlockNumber,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.FinalBlockTimestamp.ID).Y,
		gnarkFuncInp.FinalBlockTimestamp,
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.LastRollingHashUpdate[0].ID).Y,
		lastRollingHash[0],
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.LastRollingHashUpdate[1].ID).Y,
		lastRollingHash[1],
	)

	api.AssertIsEqual(
		wvc.GetLocalPointEvalParams(wizardFuncInp.LastRollingHashUpdateNumber.ID).Y,
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
				wizardFuncInp.L2MessageServiceAddrHi.GetFrontendVariable(api, wvc),
			),
			wizardFuncInp.L2MessageServiceAddrLo.GetFrontendVariable(api, wvc),
		)
	)

	// In principle, we should enforce a strict equality between the purported
	// chainID and the one extracted from the traces. But in case, the executed
	// block has only legacy transactions (e.g. transactions without a specified
	// chainID) then the traces will return a chainID of zero.
	api.AssertIsEqual(
		api.Mul(
			wvc.GetLocalPointEvalParams(wizardFuncInp.ChainID.ID).Y,
			api.Sub(
				api.Div(
					wvc.GetLocalPointEvalParams(wizardFuncInp.ChainID.ID).Y,
					twoPow112,
				),
				gnarkFuncInp.ChainID,
			),
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
	wvc *wizard.WizardVerifierCircuit,
	wFuncInp publicInput.FunctionalInputExtractor,
) frontend.Variable {

	hsh, err := mimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}

	hsh.Write(
		wvc.GetLocalPointEvalParams(wFuncInp.DataNbBytes.ID).Y,
		wvc.GetLocalPointEvalParams(wFuncInp.DataChecksum.ID).Y,
	)

	return hsh.Sum()
}

// shouldBeEqual is a placeholder dummy function that generate fake constraints
// as a replacement for what should be an api.AssertIsEqual. If we just commented
// out the api.AssertIsEqual we might have an unconstrained variable.
func shouldBeEqual(api frontend.API, a, b frontend.Variable) {
	_ = api.Sub(a, b)
}
