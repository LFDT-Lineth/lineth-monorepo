package main

// DCC (Dynamic Chain Configuration) public input hash calculation.
// This is a self-contained implementation for testcase generation only.
// This file can be deleted when the real DCC PR lands.

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/crypto/sha3"
)

// DCCConfig holds the Dynamic Chain Configuration parameters
type DCCConfig struct {
	ChainID              uint64
	BaseFee              uint64
	CoinBase             common.Address
	L2MessageServiceAddr common.Address
}

// Global DCC config, set from the aggregation spec file
var dccConfig = &DCCConfig{}

// computeChainConfigurationHash computes the MiMC hash of chain configuration
func computeChainConfigurationHash(chainID uint64, baseFee uint64, coinBase common.Address, l2MessageServiceAddr common.Address) [32]byte {
	h := mimc.NewMiMC()
	h.Reset()

	// Helper to write value to MiMC
	writeValue := func(value *big.Int) {
		var b [32]byte
		value.FillBytes(b[:])
		h.Write(b[:])
	}

	// Process chain ID
	writeValue(new(big.Int).SetUint64(chainID))

	// Process base fee
	writeValue(new(big.Int).SetUint64(baseFee))

	// Process coin base address
	var coinBaseBytes [32]byte
	copy(coinBaseBytes[12:], coinBase[:])
	h.Write(coinBaseBytes[:])

	// Process L2 message service address
	var addrBytes [32]byte
	copy(addrBytes[12:], l2MessageServiceAddr[:]) // address is 20 bytes. Padding to 32 bytes
	h.Write(addrBytes[:])

	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// computeDCCPublicInputHash computes the aggregation public input hash with DCC support.
// This matches the hash calculation on main branch that includes chain configuration.
func computeDCCPublicInputHash(resp *aggregation.Response, l2MsgMerkleTreeDepth int, lastFinalizedL1RollingHash string, lastFinalizedL1RollingHashMsgNum uint) string {
	hsh := sha3.NewLegacyKeccak256()

	writeHex := func(hex string) {
		b, err := utils.HexDecodeString(hex)
		if err != nil {
			panic(err)
		}
		hsh.Write(b)
	}

	writeInt := func(i int) {
		b := utils.FmtInt32Bytes(i)
		hsh.Write(b[:])
	}

	writeUint := func(i uint) {
		b := utils.FmtUint32Bytes(i)
		hsh.Write(b[:])
	}

	// First, hash L2 merkle roots
	hsh.Reset()
	for _, hex := range resp.L2MerkleRoots {
		writeHex(hex)
	}
	l2Msgs := hsh.Sum(nil)

	// Compute chain configuration hash using MiMC
	chainConfigHash := computeChainConfigurationHash(
		dccConfig.ChainID,
		dccConfig.BaseFee,
		dccConfig.CoinBase,
		dccConfig.L2MessageServiceAddr,
	)

	// Now compute the full public input hash
	hsh.Reset()
	writeHex(resp.ParentAggregationFinalShnarf)
	writeHex(resp.FinalShnarf)
	writeUint(resp.ParentAggregationLastBlockTimestamp)
	writeUint(resp.FinalTimestamp)
	writeUint(resp.LastFinalizedBlockNumber)
	writeUint(resp.FinalBlockNumber)
	writeHex(lastFinalizedL1RollingHash)
	writeHex(resp.L1RollingHash)
	writeUint(lastFinalizedL1RollingHashMsgNum)
	writeUint(resp.L1RollingHashMessageNumber)
	writeInt(l2MsgMerkleTreeDepth)
	hsh.Write(l2Msgs)
	// Add the chain configuration hash - exactly 32 bytes (DCC addition)
	hsh.Write(chainConfigHash[:])

	// Represent canonically as a bn254 scalar
	var x bn254fr.Element
	x.SetBytes(hsh.Sum(nil))
	res := x.Bytes()

	return utils.HexEncodeToString(res[:])
}

// DCCAggregationResponse extends the standard response with DCC fields
type DCCAggregationResponse struct {
	*aggregation.Response

	// DCC fields that are not in the standard response
	LastFinalizedL1RollingHash              string `json:"lastFinalizedL1RollingHash"`
	LastFinalizedL1RollingHashMessageNumber uint   `json:"lastFinalizedL1RollingHashMessageNumber"`
}

// computeDCCPublicInputHashFromDCCResponse computes hash from DCCAggregationResponse
func computeDCCPublicInputHashFromDCCResponse(resp *DCCAggregationResponse, l2MsgMerkleTreeDepth int) string {
	return computeDCCPublicInputHash(
		resp.Response,
		l2MsgMerkleTreeDepth,
		resp.LastFinalizedL1RollingHash,
		resp.LastFinalizedL1RollingHashMessageNumber,
	)
}

// applyDCCToConfig applies the Dynamic Chain Configuration from the spec
func applyDCCToConfig(dccSpec *DynamicChainConfigurationSpec) {
	if dccSpec == nil {
		printlnAndExit("aggregation spec must include dynamicChainConfigurationSpec")
	}
	dccConfig.ChainID = dccSpec.ChainID
	dccConfig.BaseFee = dccSpec.BaseFee
	dccConfig.CoinBase = common.HexToAddress(dccSpec.CoinBase)
	dccConfig.L2MessageServiceAddr = common.HexToAddress(dccSpec.L2MessageServiceAddr)

	// Also set on the standard cfg for aggregation.CraftResponse to work
	cfg.Layer2.ChainID = uint(dccSpec.ChainID)
	cfg.Layer2.MsgSvcContractStr = dccSpec.L2MessageServiceAddr
	cfg.Layer2.MsgSvcContract = common.HexToAddress(dccSpec.L2MessageServiceAddr)
}

// wrapResponseWithDCC wraps the standard response with DCC fields
func wrapResponseWithDCC(resp *aggregation.Response, lastFinalizedL1RollingHash string, lastFinalizedL1RollingHashMsgNum uint) *DCCAggregationResponse {
	return &DCCAggregationResponse{
		Response:                                resp,
		LastFinalizedL1RollingHash:              lastFinalizedL1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: lastFinalizedL1RollingHashMsgNum,
	}
}

// l2MsgMerkleTreeDepth is the depth used for L2 message merkle trees
const l2MsgMerkleTreeDepth = 5

// makeDCCDummyProof generates a dummy proof for the given public input.
// This mirrors the logic in prover/backend/aggregation/prove.go:makeDummyProof
func makeDCCDummyProof(publicInput string) string {
	srsProvider, err := circuits.NewSRSStore(cfg.PathForSRS())
	if err != nil {
		printlnAndExit("could not create SRS provider: %v", err)
	}

	setup, err := dummy.MakeUnsafeSetup(srsProvider, circuits.MockCircuitIDEmulation, ecc.BN254.ScalarField())
	if err != nil {
		printlnAndExit("could not create dummy setup: %v", err)
	}

	var x bn254fr.Element
	xBytes, _ := hexutil.Decode(publicInput)
	x.SetBytes(xBytes)

	return dummy.MakeProof(&setup, x, circuits.MockCircuitIDEmulation)
}

// recalculateDCCPublicInputAndProof recalculates the public input hash with DCC support
// and regenerates the proof for that public input.
func recalculateDCCPublicInputAndProof(resp *DCCAggregationResponse) {
	// First compute the DCC public input
	resp.AggregatedProofPublicInput = computeDCCPublicInputHashFromDCCResponse(resp, l2MsgMerkleTreeDepth)
	// Then regenerate the proof for that public input
	resp.AggregatedProof = makeDCCDummyProof(resp.AggregatedProofPublicInput)
}
