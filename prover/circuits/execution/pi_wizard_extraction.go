package execution

import (
	"math/big"
	"reflect"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	poseidon2permutation "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
)

// checkPublicInputs checks that the values in fi are consistent with the
// wizard.VerifierCircuit
func checkPublicInputs(
	api frontend.API,
	wvc *wizard.VerifierCircuit,
	gnarkFuncInp FunctionalPublicInputSnark,
	execData [1 << 17]frontend.Variable,
) {

	// Checking the state root hash concomittance
	checkStateRootHash(api, wvc, gnarkFuncInp)

	// Checking the block number concomittance
	checkBlockNumber(api, wvc, gnarkFuncInp)

	// Checking the block timestamp concomittance
	checkBlockTimestamp(api, wvc, gnarkFuncInp)

	// Checking the rolling hash concomittance
	checkRollingHash(api, wvc, gnarkFuncInp)

	// Checking the rolling hash number concomittance
	checkRollingHashNumber(api, wvc, gnarkFuncInp)

	// Checking the concomittance of the dynamic chain config (L2MsgService,
	// BaseFee, CoinBase, ChainID)
	checkDynamicChainConfig(api, wvc, gnarkFuncInp)

	// Checking the execution data
	checkExecutionData(api, wvc, gnarkFuncInp, execData)

	// Checking the L2 Msg hash
	checkL2MSgHashes(api, wvc, gnarkFuncInp)
}

// checkStateRootHash checks the concomittance of the state root hashes between
// the functional inputs and the public inputs extracted from the wizard circuit.
func checkStateRootHash(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		pie = getPublicInputExtractor(wvc)

		extrInitialStateRootHashWords = getPublicInputArr(api, wvc, pie.InitialStateRootHash[:])
		extrFinalStateRootHashWords   = getPublicInputArr(api, wvc, pie.FinalStateRootHash[:])
	)

	combineKoala := func(api frontend.API, vs []frontend.Variable) frontend.Variable {
		p32 := big.NewInt(1)
		p32.Lsh(p32, 32)
		return compress.ReadNum(api, vs, p32)
	}

	api.AssertIsEqual(
		gnarkFuncInp.InitialStateRootHash[0],
		combineKoala(api, extrInitialStateRootHashWords[:4]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.InitialStateRootHash[1],
		combineKoala(api, extrInitialStateRootHashWords[4:]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.FinalStateRootHash[0],
		combineKoala(api, extrFinalStateRootHashWords[:4]),
	)

	api.AssertIsEqual(
		gnarkFuncInp.FinalStateRootHash[1],
		combineKoala(api, extrFinalStateRootHashWords[4:]),
	)
}

// checkBlockNumber checks the concomittance of the block number between the
// functional inputs and the public inputs extracted from the wizard circuit.
func checkBlockNumber(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		pie = getPublicInputExtractor(wvc)

		extrInitialBlockNumberWords = getPublicInputArr(api, wvc, pie.InitialBlockNumber[:])
		extrFinalBlockNumberWords   = getPublicInputArr(api, wvc, pie.FinalBlockNumber[:])
	)

	api.AssertIsEqual(
		gnarkFuncInp.InitialBlockNumber,
		internal.CombineWordsIntoElements(api, extrInitialBlockNumberWords),
	)

	api.AssertIsEqual(
		gnarkFuncInp.FinalBlockNumber,
		internal.CombineWordsIntoElements(api, extrFinalBlockNumberWords),
	)
}

// checkBlockTimestamp checks the concomittance of the block timestamp between the
// functional inputs and the public inputs extracted from the wizard circuit.
func checkBlockTimestamp(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		pie = getPublicInputExtractor(wvc)

		extrInitialTimestampWords = getPublicInputArr(api, wvc, pie.InitialBlockTimestamp[:])
		extrFinalTimestampWords   = getPublicInputArr(api, wvc, pie.FinalBlockTimestamp[:])
	)

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.InitialBlockTimestamp,
		internal.CombineWordsIntoElements(api, extrInitialTimestampWords),
	)

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.FinalBlockTimestamp,
		internal.CombineWordsIntoElements(api, extrFinalTimestampWords),
	)
}

// checkRollingHash checks the concomittance of the rolling hash between the
// functional inputs and the public inputs extracted from the wizard circuit.
func checkRollingHash(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		pie = getPublicInputExtractor(wvc)

		extrInitialRollingHashWordsHi = getPublicInputArr(api, wvc, pie.FirstRollingHashUpdate[:8])
		extrInitialRollingHashWordsLo = getPublicInputArr(api, wvc, pie.FirstRollingHashUpdate[8:])
		extrFinalRollingHashWordsHi   = getPublicInputArr(api, wvc, pie.LastRollingHashUpdate[:8])
		extrFinalRollingHashWordsLo   = getPublicInputArr(api, wvc, pie.LastRollingHashUpdate[8:])

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
func checkRollingHashNumber(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		pie = getPublicInputExtractor(wvc)

		extrInitialRollingHashNumberWords = getPublicInputArr(api, wvc, pie.FirstRollingHashUpdateNumber[:])
		extrFinalRollingHashNumberWords   = getPublicInputArr(api, wvc, pie.LastRollingHashUpdateNumber[:])
	)

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.FirstRollingHashUpdateNumber,
		internal.CombineWordsIntoElements(api, extrInitialRollingHashNumberWords),
	)

	mustBeEqualIfExtractedIsNonZero(api,
		gnarkFuncInp.LastRollingHashUpdateNumber,
		internal.CombineWordsIntoElements(api, extrFinalRollingHashNumberWords),
	)
}

// checkDynamicChainConfig checks the concomittance of the dynamic chain config
// between the functional inputs and the public inputs extracted from the wizard
// circuit.
func checkDynamicChainConfig(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	var (
		pie = getPublicInputExtractor(wvc)

		extrChainIDLimbs    = getPublicInputArr(api, wvc, pie.ChainID[:])
		extrBaseFeeLimbs    = getPublicInputArr(api, wvc, pie.BaseFee[:])
		extrCoinBaseLimbs   = getPublicInputArr(api, wvc, pie.CoinBase[:])
		extrMsgServiceLimbs = getPublicInputArr(api, wvc, pie.L2MessageServiceAddr[:])

		chainID    = internal.CombineWordsIntoElements(api, extrChainIDLimbs)
		baseFee    = internal.CombineWordsIntoElements(api, extrBaseFeeLimbs)
		coinBase   = internal.CombineWordsIntoElements(api, extrCoinBaseLimbs)
		msgService = internal.CombineWordsIntoElements(api, extrMsgServiceLimbs)
	)

	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.ChainID, chainID)
	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.BaseFee, baseFee)
	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.CoinBase, coinBase)
	mustBeEqualIfExtractedIsNonZero(api, gnarkFuncInp.L2MessageServiceAddr, msgService)
}

// checkExecutionData computes the BLS execution data hash and checks it is
// consistent with the public input extracted from the wizard circuit using the
// multilateral commitment.
func checkExecutionData(api frontend.API, wvc *wizard.VerifierCircuit,
	gnarkFuncInp FunctionalPublicInputSnark, execData [1 << 17]frontend.Variable,
) {

	hsh, err := poseidon2permutation.NewCompressor(api)
	if err != nil {
		panic(err)
	}

	var (
		pie = getPublicInputExtractor(wvc)

		extrSZX       = getPublicInputExt(api, wvc, pie.DataSZX)
		extrSZY       = getPublicInputExt(api, wvc, pie.DataSZY)
		extrKoalaHash = getPublicInputArr(api, wvc, pie.DataChecksum[:])
		extrDataNByte = getPublicInput(api, wvc, pie.DataNbBytes)
	)

	// @alex: in theory we could simplify a little bit the code by just not
	// asking the user to provider execDataNByte, but doing it this way allows
	// easily diagnosing if there is a mismatching between what is extracted
	// from the inner-proof and what is provided by the user.
	api.AssertIsEqual(extrDataNByte, gnarkFuncInp.DataChecksum.Length)

	recoveredX, recoveredY, hashBLS := public_input.CheckExecDataMultiCommitmentOpeningGnark(
		api, execData, extrDataNByte, [8]frontend.Variable(extrKoalaHash), hsh,
	)

	for i := range extrSZX {
		api.AssertIsEqual(extrSZX[i], recoveredX[i])
		api.AssertIsEqual(extrSZY[i], recoveredY[i])
	}

	api.AssertIsEqual(gnarkFuncInp.DataChecksum.PartialHash, hashBLS)

	if err := gnarkFuncInp.DataChecksum.Check(api); err != nil {
		panic(err)
	}
}

// checkL2MSgHashes checks the concomittance of the L2 message hashes extracted
// from the wvc to their purported BLS hash held in the gnarkFuncInp.
func checkL2MSgHashes(api frontend.API, wvc *wizard.VerifierCircuit, gnarkFuncInp FunctionalPublicInputSnark) {

	pie := getPublicInputExtractor(wvc)

	if len(pie.L2Messages) != len(gnarkFuncInp.L2MessageHashes.Values) {
		utils.Panic("L2MessageHashes length mismatch: %d != %d", len(pie.L2Messages), len(gnarkFuncInp.L2MessageHashes.Values))
	}

	// This converts the provided L2MsgHash (in 8-bits words) into 16-bytes and
	// then directly compare with the public input extracted from the circuit.

	for i := range gnarkFuncInp.L2MessageHashes.Values {

		var (
			funcL2MessageHashBytes = gnarkFuncInp.L2MessageHashes.Values[i]
			funcL2MessageHashWords = internal.CombineByteIntoWords(api, funcL2MessageHashBytes[:])
			extrL2MessageHashWords = getPublicInputArr(api, wvc, pie.L2Messages[i][:])
		)

		if len(funcL2MessageHashWords) != len(extrL2MessageHashWords) {
			utils.Panic("L2MessageHashes[%d] length mismatch: %d != %d", i, len(funcL2MessageHashWords), len(extrL2MessageHashWords))
		}

		for j := range funcL2MessageHashWords {
			api.AssertIsEqual(funcL2MessageHashWords[j], extrL2MessageHashWords[j])
		}
	}
}

// getPublicInputExtractor extracts the public input from the wizard circuit
func getPublicInputExtractor(wvc *wizard.VerifierCircuit) *publicInput.FunctionalInputExtractor {
	extraData, extraDataFound := wvc.Spec.ExtraData[publicInput.PublicInputExtractorMetadata]
	if !extraDataFound {
		panic("public input extractor not found")
	}
	pie, ok := extraData.(*publicInput.FunctionalInputExtractor)
	if !ok {
		panic("public input extractor not of the right type: " + reflect.TypeOf(extraData).String())
	}
	return pie
}

// getPublicInputExt returns a field extension public input coordinates in array
// form
func getPublicInputExt(api frontend.API, wvc *wizard.VerifierCircuit, pi wizard.PublicInput) [4]frontend.Variable {

	name := pi.Name
	if !wvc.HasPublicInput(name) {
		name = "functional." + name
	}

	// this prefixing is needed because the full-prover circuit is being recursed
	res := wvc.GetPublicInputExt(api, name)
	return [4]frontend.Variable{
		res.B0.A0.Native(),
		res.B0.A1.Native(),
		res.B1.A0.Native(),
		res.B1.A1.Native(),
	}
}

// getPublicInputArr returns a slice of values from the public input
func getPublicInputArr(api frontend.API, wvc *wizard.VerifierCircuit, pis []wizard.PublicInput) []frontend.Variable {
	res := make([]frontend.Variable, len(pis))
	for i := range pis {

		// When the outer-proof runs on top of the limitless prover. Then, the
		// name of the "functional" public-inputs (e.g. the one that corresponds
		// to the actual statement of the execution instance) is prefixed with
		// "functional."
		name := pis[i].Name
		if !wvc.HasPublicInput(name) {
			name = "functional." + name
		}

		r := wvc.GetPublicInput(api, name)
		res[i] = r.Native()
	}
	return res
}

// getPublicInput returns a value from the public input
func getPublicInput(api frontend.API, wvc *wizard.VerifierCircuit, pi wizard.PublicInput) frontend.Variable {

	name := pi.Name
	if !wvc.HasPublicInput(name) {
		name = "functional." + name
	}

	r := wvc.GetPublicInput(api, name)
	return r.Native()
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
