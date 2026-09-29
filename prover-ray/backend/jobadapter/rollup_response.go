package jobadapter

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"

// rollupResponse is the getZkRollupProofV1 success body. programVk is echoed
// from the request envelope. The public inputs, proof, and revealed arrays are
// placeholders until the rollup guest and proof serialization land.
type rollupResponse struct {
	ProverVersion     string                     `json:"proverVersion"`
	ProgramVk         string                     `json:"programVk"`
	ProofHex          string                     `json:"proof"`
	StartBlockNumber  uint64                     `json:"startBlockNumber"`
	PublicInputs      rollupResponsePublicInputs `json:"publicInputs"`
	L2L1Roots         []string                   `json:"l2L1Roots"`
	FilteredAddresses []string                   `json:"filteredAddresses"`
}

// rollupResponsePublicInputs is the 20-field rollup PI tuple (§2.4), shared by
// the rollup and aggregation responses.
type rollupResponsePublicInputs struct {
	EndBlockNumber                           uint64   `json:"endBlockNumber"`
	EndBlockTimestamp                        uint64   `json:"endBlockTimestamp"`
	L2L1BridgeTransactionTree                string   `json:"l2L1BridgeTransactionTree"`
	ParentL1L2BridgeRollingHash              string   `json:"parentL1L2BridgeRollingHash"`
	ParentL1L2BridgeRollingHashMessageNumber uint64   `json:"parentL1L2BridgeRollingHashMessageNumber"`
	EndL1L2BridgeRollingHash                 string   `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber    uint64   `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                   string   `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                     string   `json:"parentFtxRollingHash"`
	ParentFtxNumber                          uint64   `json:"parentFtxNumber"`
	EndFtxRollingHash                        string   `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                    uint64   `json:"endProcessedFtxNumber"`
	FilteredAddressesHash                    string   `json:"filteredAddressesHash"`
	ParentDataRollingHash                    string   `json:"parentDataRollingHash"`
	EndDataRollingHash                       string   `json:"endDataRollingHash"`
	ParentBlockHash                          string   `json:"parentBlockHash"`
	EndBlockHash                             string   `json:"endBlockHash"`
	StartOffset                              uint64   `json:"startOffset"`
	EndOffset                                uint64   `json:"endOffset"`
	ProgramVks                               []string `json:"programVks"`
}

func newRollupResponse(
	result backend.Result, startBlockNumber uint64, proverVersion string, programVk []byte,
) rollupResponse {
	return rollupResponse{
		ProverVersion:     proverVersion,
		ProgramVk:         hexBytes(programVk),
		ProofHex:          hexBytes(result.ProofBytes),
		StartBlockNumber:  startBlockNumber,
		PublicInputs:      placeholderRollupPublicInputs(),
		L2L1Roots:         []string{},
		FilteredAddresses: []string{},
	}
}

// placeholderRollupPublicInputs is a zero-valued but schema-valid rollup PI.
// Populated from backend.Result once rollup public-input extraction exists.
func placeholderRollupPublicInputs() rollupResponsePublicInputs {
	zero := hexHash([32]byte{})
	return rollupResponsePublicInputs{
		L2L1BridgeTransactionTree:   zero,
		ParentL1L2BridgeRollingHash: zero,
		EndL1L2BridgeRollingHash:    zero,
		DynamicChainConfigHash:      zero,
		ParentFtxRollingHash:        zero,
		EndFtxRollingHash:           zero,
		FilteredAddressesHash:       zero,
		ParentDataRollingHash:       zero,
		EndDataRollingHash:          zero,
		ParentBlockHash:             zero,
		EndBlockHash:                zero,
		ProgramVks:                  []string{},
	}
}
