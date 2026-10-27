package jobadapter

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"

// rollupResponse is the getZkRollupProofV1 success body. programVk is echoed
// from the request envelope. The public inputs and proof are placeholders until
// the rollup guest and proof serialization land.
type rollupResponse struct {
	ProverVersion    string                     `json:"proverVersion"`
	ProgramVk        string                     `json:"programVk"`
	ProofHex         string                     `json:"proof"`
	StartBlockNumber uint64                     `json:"startBlockNumber"`
	PublicInputs     rollupResponsePublicInputs `json:"publicInputs"`
}

// rollupResponsePublicInputs is the rollup PI tuple.
type rollupResponsePublicInputs struct {
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
	FilteredAddresses                     []string `json:"filteredAddresses"`
	ProgramVks                            []string `json:"programVks"`
	BlockCount                            uint64   `json:"blockCount"`
	L2MessagingBlocksOffsets              []uint16 `json:"l2MessagingBlocksOffsets"`
	L2L1Messages                          []string `json:"l2L1Messages"`
}

func newRollupResponse(
	result backend.Result, startBlockNumber uint64, proverVersion string, programVk []byte,
) rollupResponse {
	return rollupResponse{
		ProverVersion:    proverVersion,
		ProgramVk:        hexBytes(programVk),
		ProofHex:         hexBytes(result.ProofBytes),
		StartBlockNumber: startBlockNumber,
		PublicInputs:     placeholderRollupPublicInputs(),
	}
}

// placeholderRollupPublicInputs is a zero-valued but schema-valid rollup PI.
// Populated from backend.Result once rollup public-input extraction exists.
func placeholderRollupPublicInputs() rollupResponsePublicInputs {
	zero := hexHash([32]byte{})
	return rollupResponsePublicInputs{
		EndL1L2BridgeRollingHash: zero,
		DynamicChainConfigHash:   zero,
		ParentFtxRollingHash:     zero,
		EndFtxRollingHash:        zero,
		ParentDataRollingHash:    zero,
		EndDataRollingHash:       zero,
		ParentBlockHash:          zero,
		EndBlockHash:             zero,
		FilteredAddresses:        []string{},
		ProgramVks:               []string{},
		L2MessagingBlocksOffsets: []uint16{},
		L2L1Messages:             []string{},
	}
}
