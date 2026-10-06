package jobadapter

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"

// aggregationResponse is the getZkRollupAggregationProofV1 success body. Unlike
// the rollup response it has no top-level programVk (the combined programVks set
// lives in publicInputs) and adds l2MessagingBlocksOffsets. Public inputs,
// proof, and revealed arrays are placeholders until the guest lands.
type aggregationResponse struct {
	ProverVersion            string                     `json:"proverVersion"`
	ProofHex                 string                     `json:"proof"`
	StartBlockNumber         uint64                     `json:"startBlockNumber"`
	PublicInputs             rollupResponsePublicInputs `json:"publicInputs"`
	L2L1Roots                []string                   `json:"l2L1Roots"`
	FilteredAddresses        []string                   `json:"filteredAddresses"`
	L2MessagingBlocksOffsets []uint64                   `json:"l2MessagingBlocksOffsets"`
}

func newAggregationResponse(
	result backend.Result, startBlockNumber uint64, proverVersion string,
) aggregationResponse {
	return aggregationResponse{
		ProverVersion:            proverVersion,
		ProofHex:                 hexBytes(result.ProofBytes),
		StartBlockNumber:         startBlockNumber,
		PublicInputs:             placeholderRollupPublicInputs(),
		L2L1Roots:                []string{},
		FilteredAddresses:        []string{},
		L2MessagingBlocksOffsets: []uint64{},
	}
}
