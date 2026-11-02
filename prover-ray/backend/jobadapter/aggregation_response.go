package jobadapter

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"

// aggregationResponse is the getZkRollupAggregationProofV1 success body. Unlike
// the rollup response it has no top-level programVk (the combined programIds set
// lives in publicInputs). Public inputs and proof are placeholders until the
// guest lands.
type aggregationResponse struct {
	ProverVersion    string                          `json:"proverVersion"`
	ProofHex         string                          `json:"proof"`
	StartBlockNumber uint64                          `json:"startBlockNumber"`
	PublicInputs     aggregationResponsePublicInputs `json:"publicInputs"`
}

// aggregationResponsePublicInputs is the aggregation PI tuple.
type aggregationResponsePublicInputs struct {
	EndBlockNumber                        uint64   `json:"endBlockNumber"`
	EndBlockTimestamp                     uint64   `json:"endBlockTimestamp"`
	EndL1L2BridgeRollingHash              string   `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber uint64   `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                string   `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                  string   `json:"parentFtxRollingHash"`
	ParentFtxNumber                       uint64   `json:"parentFtxNumber"`
	EndFtxRollingHash                     string   `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                 uint64   `json:"endProcessedFtxNumber"`
	ParentDataRollingHash                 string   `json:"parentDataRollingHash"`
	EndDataRollingHash                    string   `json:"endDataRollingHash"`
	ParentBlockHash                       string   `json:"parentBlockHash"`
	EndBlockHash                          string   `json:"endBlockHash"`
	ParentDataTailTakeBytes               uint64   `json:"parentDataTailTakeBytes"`
	FinalDataTailDiscardBytes             uint64   `json:"finalDataTailDiscardBytes"`
	L2L1TreeDepth                         uint64   `json:"l2L1TreeDepth"`
	L2L1Roots                             []string `json:"l2L1Roots"`
	FilteredAddresses                     []string `json:"filteredAddresses"`
	ProgramIDs                            []string `json:"programIds"`
	L2MessagingBlocksOffsets              []uint16 `json:"l2MessagingBlocksOffsets"`
}

func newAggregationResponse(
	result backend.Result, startBlockNumber uint64, proverVersion string,
) aggregationResponse {
	return aggregationResponse{
		ProverVersion:    proverVersion,
		ProofHex:         hexBytes(result.ProofBytes),
		StartBlockNumber: startBlockNumber,
		PublicInputs:     placeholderAggregationPublicInputs(),
	}
}

// placeholderAggregationPublicInputs is a zero-valued but schema-valid
// aggregation PI. Populated from backend.Result once extraction exists.
func placeholderAggregationPublicInputs() aggregationResponsePublicInputs {
	zero := hexHash([32]byte{})
	return aggregationResponsePublicInputs{
		EndL1L2BridgeRollingHash: zero,
		DynamicChainConfigHash:   zero,
		ParentFtxRollingHash:     zero,
		EndFtxRollingHash:        zero,
		ParentDataRollingHash:    zero,
		EndDataRollingHash:       zero,
		ParentBlockHash:          zero,
		EndBlockHash:             zero,
		L2L1Roots:                []string{},
		FilteredAddresses:        []string{},
		ProgramIDs:               []string{},
		L2MessagingBlocksOffsets: []uint16{},
	}
}
