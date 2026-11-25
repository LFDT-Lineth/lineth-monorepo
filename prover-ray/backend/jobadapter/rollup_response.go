package jobadapter

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"

// rollupResponse is the getZkRollupProofV1 success body. programVk is the VK
// of the proof, from the result, not the request's guestProgramId.
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
	ParentSharedBlobCarryOverTailBytes    uint64   `json:"parentSharedBlobCarryOverTailBytes"`
	FinalSharedBlobCarryOverTailBytes     uint64   `json:"finalSharedBlobCarryOverTailBytes"`
	FilteredAddresses                     []string `json:"filteredAddresses"`
	ProgramVks                            []string `json:"programVks"`
	BlockCount                            uint64   `json:"blockCount"`
	L2MessagingBlocksOffsets              []uint16 `json:"l2MessagingBlocksOffsets"`
	L2L1Messages                          []string `json:"l2L1Messages"`
}

func newRollupResponse(result backend.Result, startBlockNumber uint64, proverVersion string) rollupResponse {
	return rollupResponse{
		ProverVersion:    proverVersion,
		ProgramVk:        hexHash(result.ProgramVK),
		ProofHex:         hexBytes(result.ProofBytes),
		StartBlockNumber: startBlockNumber,
		PublicInputs:     rollupPublicInputs(result.RollupPublicInputs),
	}
}

func rollupPublicInputs(pi backend.RollupPublicInputs) rollupResponsePublicInputs {
	return rollupResponsePublicInputs{
		EndBlockNumber:                        pi.EndBlockNumber,
		EndBlockTimestamp:                     pi.EndBlockTimestamp,
		EndL1L2BridgeRollingHash:              hexHash(pi.EndL1L2BridgeRollingHash),
		EndL1L2BridgeRollingHashMessageNumber: pi.EndL1L2BridgeRollingHashMessageNumber,
		DynamicChainConfigHash:                hexHash(pi.DynamicChainConfigHash),
		ParentFtxRollingHash:                  hexHash(pi.ParentFtxRollingHash),
		ParentFtxNumber:                       pi.ParentFtxNumber,
		EndFtxRollingHash:                     hexHash(pi.EndFtxRollingHash),
		EndProcessedFtxNumber:                 pi.EndProcessedFtxNumber,
		ParentDataRollingHash:                 hexHash(pi.ParentDataRollingHash),
		EndDataRollingHash:                    hexHash(pi.EndDataRollingHash),
		ParentBlockHash:                       hexHash(pi.ParentBlockHash),
		EndBlockHash:                          hexHash(pi.EndBlockHash),
		ParentSharedBlobCarryOverTailBytes:    pi.ParentSharedBlobCarryOverTailBytes,
		FinalSharedBlobCarryOverTailBytes:     pi.FinalSharedBlobCarryOverTailBytes,
		FilteredAddresses:                     hexAddressList(pi.FilteredAddresses),
		ProgramVks:                            hexHashList(pi.ProgramVks),
		BlockCount:                            pi.BlockCount,
		// non-nil so an empty list marshals to [] not null.
		L2MessagingBlocksOffsets: append([]uint16{}, pi.L2MessagingBlocksOffsets...),
		L2L1Messages:             hexHashList(pi.L2L1Messages),
	}
}
