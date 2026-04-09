package execution

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	poseidon2permutation "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// checkPublicInputs checks that the values in gnarkFuncInp are consistent with
// the public inputs extracted from the inner wizard proof (extr).
func checkPublicInputs(
	api frontend.API,
	extr *ExtractedPISnark,
	gnarkFuncInp FunctionalPublicInputSnark,
	execData [1 << 17]frontend.Variable,
) {

	// Checking the state root hash concomittance
	checkStateRootHash(api, extr, gnarkFuncInp)

	// Checking the block number concomittance
	checkBlockNumber(api, extr, gnarkFuncInp)

	// Checking the block timestamp concomittance
	checkBlockTimestamp(api, extr, gnarkFuncInp)

	// Checking the rolling hash concomittance
	checkRollingHash(api, extr, gnarkFuncInp)

	// Checking the rolling hash number concomittance
	checkRollingHashNumber(api, extr, gnarkFuncInp)

	// Checking the concomittance of the dynamic chain config (L2MsgService,
	// BaseFee, CoinBase, ChainID)
	checkDynamicChainConfig(api, extr, gnarkFuncInp)

	// Checking the execution data
	checkExecutionData(api, extr, gnarkFuncInp, execData)

	// Checking the L2 Msg hash
	checkL2MSgHashes(api, extr, gnarkFuncInp)
}

// checkStateRootHash checks the concomittance of the state root hashes between
// the functional inputs and the public inputs extracted from the wizard circuit.
func checkStateRootHash(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	combineKoala := func(api frontend.API, vs []frontend.Variable) frontend.Variable {
		p32 := big.NewInt(1)
		p32.Lsh(p32, 32)
		return compress.ReadNum(api, vs, p32)
	}

	api.AssertIsEqual(
		gnarkFuncInp.InitialStateRootHash[0],
		combineKoala(api, extr.InitialStateRootHash[:4]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.InitialStateRootHash[1],
		combineKoala(api, extr.InitialStateRootHash[4:]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.FinalStateRootHash[0],
		combineKoala(api, extr.FinalStateRootHash[:4]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.FinalStateRootHash[1],
		combineKoala(api, extr.FinalStateRootHash[4:]),
	)
}

// checkBlockNumber checks the concomittance of the block number between the
// functional inputs and the public inputs extracted from the wizard circuit.
func checkBlockNumber(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	api.AssertIsEqual(
		gnarkFuncInp.InitialBlockNumber,
		internal.CombineWordsIntoElements(api, extr.InitialBlockNumber[:]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.FinalBlockNumber,
		internal.CombineWordsIntoElements(api, extr.FinalBlockNumber[:]),
	)
}

// checkBlockTimestamp checks the concomittance of the block timestamp between the
// functional inputs and the public inputs extracted from the wizard circuit.
func checkBlockTimestamp(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.InitialBlockTimestamp,
		internal.CombineWordsIntoElements(api, extr.InitialBlockTimestamp[:]),
	)

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.FinalBlockTimestamp,
		internal.CombineWordsIntoElements(api, extr.FinalBlockTimestamp[:]),
	)
}

// checkRollingHash checks the concomittance of the rolling hash between the
// functional inputs and the public inputs extracted from the wizard circuit.
func checkRollingHash(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		extrInitialRollingHashWordsHi = extr.FirstRollingHashUpdate[:8]
		extrInitialRollingHashWordsLo = extr.FirstRollingHashUpdate[8:]
		extrFinalRollingHashWordsHi   = extr.LastRollingHashUpdate[:8]
		extrFinalRollingHashWordsLo   = extr.LastRollingHashUpdate[8:]

		funcInpInitialRollingHashWordsHi = internal.CombineByteIntoWords(api, gnarkFuncInp.InitialRollingHashUpdate[:16])
		funcInpInitialRollingHashWordsLo = internal.CombineByteIntoWords(api, gnarkFuncInp.InitialRollingHashUpdate[16:])
		funcInpFinalRollingHashWordsHi   = internal.CombineByteIntoWords(api, gnarkFuncInp.FinalRollingHashUpdate[:16])
		funcInpFinalRollingHashWordsLo   = internal.CombineByteIntoWords(api, gnarkFuncInp.FinalRollingHashUpdate[16:])

		foundNonZeroInitialRollingHash = api.Sub(1, areAllZeroes(api, append(extrInitialRollingHashWordsHi, extrInitialRollingHashWordsLo...)))
		foundNonZeroFinalRollingHash   = api.Sub(1, areAllZeroes(api, append(extrFinalRollingHashWordsHi, extrFinalRollingHashWordsLo...)))
	)

	for i := range extrInitialRollingHashWordsHi {
		mustBeEqualIf(api, foundNonZeroInitialRollingHash, funcInpInitialRollingHashWordsHi[i], extrInitialRollingHashWordsHi[i])
		mustBeEqualIf(api, foundNonZeroInitialRollingHash, funcInpInitialRollingHashWordsLo[i], extrInitialRollingHashWordsLo[i])
		mustBeEqualIf(api, foundNonZeroFinalRollingHash, funcInpFinalRollingHashWordsHi[i], extrFinalRollingHashWordsHi[i])
		mustBeEqualIf(api, foundNonZeroFinalRollingHash, funcInpFinalRollingHashWordsLo[i], extrFinalRollingHashWordsLo[i])
	}
}

// checkRollingHashNumber checks the concomittance of the rolling hash number
// between the functional inputs and the public inputs extracted from the wizard
// circuit.
func checkRollingHashNumber(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.FirstRollingHashUpdateNumber,
		internal.CombineWordsIntoElements(api, extr.FirstRollingHashUpdateNumber[:]),
	)

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.LastRollingHashUpdateNumber,
		internal.CombineWordsIntoElements(api, extr.LastRollingHashUpdateNumber[:]),
	)
}

// checkDynamicChainConfig checks the concomittance of the dynamic chain config
// between the functional inputs and the public inputs extracted from the wizard
// circuit.
func checkDynamicChainConfig(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		chainID    = internal.CombineWordsIntoElements(api, extr.ChainID[:])
		baseFee    = internal.CombineWordsIntoElements(api, extr.BaseFee[:])
		coinBase   = internal.CombineWordsIntoElements(api, extr.CoinBase[:])
		msgService = internal.CombineWordsIntoElements(api, extr.L2MessageServiceAddr[:])
	)

	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.ChainID, chainID)
	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.BaseFee, baseFee)
	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.CoinBase, coinBase)
	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.L2MessageServiceAddr, msgService)
}

// checkExecutionData computes the BLS execution data hash and checks it is
// consistent with the public input extracted from the wizard circuit using the
// multilateral commitment.
func checkExecutionData(api frontend.API, extr *ExtractedPISnark,
	gnarkFuncInp FunctionalPublicInputSnark, execData [1 << 17]frontend.Variable,
) {

	hsh, err := poseidon2permutation.NewCompressor(api)
	if err != nil {
		panic(err)
	}

	// @alex: in theory we could simplify a little bit the code by just not
	// asking the user to provider execDataNByte, but doing it this way allows
	// easily diagnosing if there is a mismatching between what is extracted
	// from the inner-proof and what is provided by the user.
	api.AssertIsEqual(extr.DataNbBytes, gnarkFuncInp.DataChecksum.Length)

	recoveredX, recoveredY, hashBLS := public_input.CheckExecDataMultiCommitmentOpeningGnark(
		api, execData, extr.DataNbBytes, extr.DataChecksum, hsh,
	)

	for i := range extr.DataSZX {
		api.AssertIsEqual(extr.DataSZX[i], recoveredX[i])
		api.AssertIsEqual(extr.DataSZY[i], recoveredY[i])
	}

	api.AssertIsEqual(gnarkFuncInp.DataChecksum.PartialHash, hashBLS)

	if err := gnarkFuncInp.DataChecksum.Check(api); err != nil {
		panic(err)
	}
}

// checkL2MSgHashes checks the concomittance of the L2 message hashes extracted
// from the inner proof to their purported BLS hash held in the gnarkFuncInp.
func checkL2MSgHashes(api frontend.API, extr *ExtractedPISnark, gnarkFuncInp FunctionalPublicInputSnark) {

	if len(extr.L2Messages) != len(gnarkFuncInp.L2MessageHashes.Values) {
		utils.Panic("L2MessageHashes length mismatch: %d != %d", len(extr.L2Messages), len(gnarkFuncInp.L2MessageHashes.Values))
	}

	// This converts the provided L2MsgHash (in 8-bits words) into 16-bytes and
	// then directly compare with the public input extracted from the circuit.

	for i := range gnarkFuncInp.L2MessageHashes.Values {

		var (
			funcL2MessageHashBytes = gnarkFuncInp.L2MessageHashes.Values[i]
			funcL2MessageHashWords = internal.CombineByteIntoWords(api, funcL2MessageHashBytes[:])
			extrL2MessageHashWords = extr.L2Messages[i][:]
		)

		if len(funcL2MessageHashWords) != len(extrL2MessageHashWords) {
			utils.Panic("L2MessageHashes[%d] length mismatch: %d != %d", i, len(funcL2MessageHashWords), len(extrL2MessageHashWords))
		}

		for j := range funcL2MessageHashWords {
			api.AssertIsEqual(funcL2MessageHashWords[j], extrL2MessageHashWords[j])
		}
	}
}

// mustBeEqualIfExtractedIsNonZero enforces that either "fn == ex" OR "ex == 0".
// This is commonly used because in many places, the extraction of a value might
// just return zero because there is nothing to extract in the proved EVM
// execution instance. This can happen for various reasons. For instance, the
// initialTimestamp might be missing because the timestamp opcode is never
// called.
func mustBeEqualIfExtractedIsNonZero(api frontend.API, fn, ex frontend.Variable) {
	mustBeEqualIf(api, ex, fn, ex)
}

// mustBeEqualIf checks if either cond==0 or x==y
func mustBeEqualIf(api frontend.API, cond, x, y frontend.Variable) {
	api.AssertIsEqual(api.Mul(cond, api.Sub(x, y)), 0)
}

// areAllZeroes returns a frontend.Variable constrained to be one if all the
// inputs are zero and zero otherwise.
func areAllZeroes(api frontend.API, xs []frontend.Variable) frontend.Variable {

	if len(xs) == 0 {
		panic("no inputs provided")
	}

	res := frontend.Variable(1)
	for _, x := range xs {
		xIsZero := api.IsZero(x)
		res = api.Mul(res, xIsZero)
	}

	return res
}
