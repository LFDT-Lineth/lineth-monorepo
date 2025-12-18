package main

import (
	"encoding/base64"
	"encoding/hex"
	"math/rand"

	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// RandDataGen generates random data for the smart-contract
type RandDataGen struct {
	*rand.Rand
}

// Rand request spec. This is a union struct for either blob submissions or
// aggregations requests.
type RandGenSpec struct {
	// Optional, if set indicates that this should generate a blob submission
	BlobSubmissionSpec BlobSubmissionSpec `json:"blobSubmissionSpec"`
	// Optional, if set indicates that this should generate a proof aggregation
	AggregationSpec AggregationSpec `json:"aggregationSpec"`
	// Optional, if set specifies the dynamic chain configuration to use
	DynamicChainConfigurationSpec DynamicChainConfigurationSpec `json:"dynamicChainConfigurationSpec"`
}

// DynamicChainConfiguration spec, specifies the chain configuration parameters
type DynamicChainConfigurationSpec struct {
	ChainID              uint64 `json:"chainID"`
	BaseFee              uint64 `json:"baseFee"`
	CoinBase             string `json:"coinBase"`
	L2MessageServiceAddr string `json:"l2MessageServiceAddr"`
}

// BlobSubmission spec, specifies how to generate a random blob submission
// prover output
type BlobSubmissionSpec struct {

	// If the IgnoreBefore flag is set to true, this tells the generator not
	// overwrite the provided parameters with the previous blob submission.
	// This is useful for generating invalid cases for the tests.
	IgnoreBefore bool `json:"ignoreBefore"`

	// L2 block numbers where the blob sequence starts and ends
	StartFromL2Block   int    `json:"startFromL2Block"`
	NumConflation      int    `json:"numConflation"`
	BlockPerConflation int    `json:"blockPerConflation"`
	ParentDataHash     string `json:"parentDataHash"`

	// Compressed data size
	CompressedDataSize int `json:"compressedDataSize"`
	// Parent shnarf
	ParentShnarf     string `json:"parentShnarf"`
	ParentZkRootHash string `json:"parentZkStateRootHash"`

	// isEip4844 enabled tells the generator to specify that the response it
	// using EIP-4844.
	Eip4844Enabled bool `json:"eip4844Enabled"`
}

// Aggregation spec
type AggregationSpec struct {

	// If the IgnoreBefore flag is set to true, this tells the generator not
	// overwrite the provided parameters with the previous blob submission.
	// This is useful for generating invalid cases for the tests.
	IgnoreBefore bool `json:"ignoreBefore"`

	// Finalized shnarf
	FinalShnarf                  string `json:"finalizedShnarf"`
	ParentAggregationFinalShnarf string `json:"parentAggregationFinalShnarf"`

	// Parent data hash and the list of data hashes to be finalized
	DataHashes     []string `json:"dataHashes"`
	DataParentHash string   `json:"dataParentHash"`

	// Parent zk root hash of the state over which we want to finalize. In 0x
	// prefixed hexstring.
	ParentStateRootHash string `json:"parentStateRootHash"`

	// Timestamp of the last already finalized L2 block
	LastFinalizedTimestamp uint `json:"lastFinalizedTimestamp"`

	// Timestamp of the last L2 block to be finalized
	FinalTimestamp uint `json:"finalTimestamp"`

	// Rolling hash of the L1 messages received on L2 so far for the state to be
	// currently finalized. In 0x prefixed Hexstring
	L1RollingHash              string `json:"l1RollingHash"`
	LastFinalizedL1RollingHash string `json:"lastFinalizedL1RollingHash"`

	// Number of L1 messages received on L2 so far for the state to be
	// currently finalized. Messaging Feedback loop messaging number -
	// part of public input
	L1RollingHashMessageNumber              uint `json:"l1RollingHashMessageNumber"`
	LastFinalizedL1RollingHashMessageNumber uint `json:"lastFinalizedL1RollingHashMessageNumber"`

	// L2 Merkle roots for L2 -> L1 messages - The prover will be calculating
	// Merkle roots for trees of a set depth
	HowManyL2Msgs uint `json:"howManyL2Msgs"`

	// The depth of the Merkle tree for the Merkle roots being anchored. This is
	// not used as part of the public inputs but is nonetheless helpful on the
	// contracts
	L2MsgTreeDepth uint `json:"treeDepth"`

	// Bytes array indicating which L2 blocks have “MessageSent” events. The
	// index starting from 1 + currentL2BlockNumber. If the value contains 1,
	// it means that that block has events
	L2MessagingBlocksOffsets string `json:"l2MessagingBlocksOffsets"`

	// Final block number
	LastFinalizedBlockNumber uint `json:"lastFinalizedBlockNumber"`
	FinalBlockNumber         uint `json:"finalBlockNumber"`
}

// Generates a random request file for a blob submission
func RandBlobSubmission(rng *rand.Rand, spec BlobSubmissionSpec) *blobsubmission.Request {
	rdg := RandDataGen{rng}
	return &blobsubmission.Request{
		CompressedData: rdg.Base64RandLen(spec.CompressedDataSize, -1),
		ConflationOrder: struct {
			StartingBlockNumber int   "json:\"startingBlockNumber\""
			UpperBoundaries     []int "json:\"upperBoundaries\""
		}{
			StartingBlockNumber: spec.StartFromL2Block,
			UpperBoundaries: rdg.AscendingSeq(
				spec.StartFromL2Block,
				spec.NumConflation,
				spec.BlockPerConflation,
			),
		},
		PrevShnarf:          spec.ParentShnarf,
		ParentStateRootHash: spec.ParentZkRootHash,
		FinalStateRootHash:  rdg.HexString(32),
		DataParentHash:      spec.ParentDataHash,
		Eip4844Enabled:      spec.Eip4844Enabled,
	}
}

// Generates a random request file for an aggregation request
func RandAggregation(rng *rand.Rand, spec AggregationSpec) *aggregation.CollectedFields {

	cf := &aggregation.CollectedFields{
		ParentAggregationFinalShnarf:            spec.ParentAggregationFinalShnarf,
		FinalShnarf:                             spec.FinalShnarf,
		ParentStateRootHash:                     spec.ParentStateRootHash,
		DataHashes:                              spec.DataHashes,
		DataParentHash:                          spec.DataParentHash,
		ParentAggregationLastBlockTimestamp:     spec.LastFinalizedTimestamp,
		FinalTimestamp:                          spec.FinalTimestamp,
		LastFinalizedL1RollingHash:              spec.LastFinalizedL1RollingHash,
		L1RollingHash:                           spec.L1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: spec.LastFinalizedL1RollingHashMessageNumber,
		L1RollingHashMessageNumber:              spec.L1RollingHashMessageNumber,
		L2MsgTreeDepth:                          spec.L2MsgTreeDepth,
		HowManyL2Msgs:                           spec.HowManyL2Msgs,
		L2MessagingBlocksOffsets:                string(spec.L2MessagingBlocksOffsets),
		LastFinalizedBlockNumber:                spec.LastFinalizedBlockNumber,
		FinalBlockNumber:                        spec.FinalBlockNumber,
	}

	rdg := RandDataGen{rng}

	// Generate the L2 Messages Merkle root hashes. The right hand of the
	// addition is to ensure that we get a divCeil and the right-shift is to
	// divide by the number of elements in the tree;
	numL2MsgTrees := cf.HowManyL2Msgs + (1 << cf.L2MsgTreeDepth) - 1
	numL2MsgTrees >>= cf.L2MsgTreeDepth
	for _i := uint(0); _i < numL2MsgTrees; _i++ {
		cf.L2MsgRootHashes = append(cf.L2MsgRootHashes, rdg.HexString(32))
	}

	return cf
}

// Returns a slice of random length in base64. If to is smaller than from, then
// return a slice of deterministic length `from`.
func (rdg *RandDataGen) Base64RandLen(from, to int) string {
	n := from
	if to > from {
		// #nosec G404 --we don't need a cryptographic RNG for testing purpose
		n = from + rand.Intn(to-from)
	}
	n = utils.DivCeil(n, 32) * 32 // round up to the next multiple of 32
	res := make([]byte, n)
	rdg.Read(res)

	// Also zeroize all the bytes that are at position multiples of 32. This
	// ensures that we will not have overflow when casting to the bls12 scalar
	// field.
	for i := 0; i < n; i += 32 {
		res[i] = 0
	}

	return base64.StdEncoding.EncodeToString(res)
}

// Returns a slice of random bytes in hexadecimal. The length is deterministic
// and the returned hex string is 0x prefixed.
func (rdg *RandDataGen) HexString(n int) string {
	res := make([]byte, n)
	rdg.Read(res)
	return "0x" + hex.EncodeToString(res)
}

// Returns a sequence of random ascending numbers with a given length, with a
// maximal increment between consecutive numbers.
func (rdg *RandDataGen) AscendingSeq(startFrom, nTerms, maxInc int) []int {
	ret := make([]int, nTerms)
	s0 := startFrom
	for i := range ret {
		// It needs to increase at least by one.
		incBy := 1
		if maxInc > incBy {
			incBy += rdg.Intn(maxInc - incBy)
		}
		// Increments s0 and assigns it
		s0 = s0 + incBy
		ret[i] = s0
	}

	return ret
}
