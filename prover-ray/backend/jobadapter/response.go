package jobadapter

import (
	"encoding/hex"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// executionResponse is the success response body for getZkL2ExecutionProofV1.
// It mirrors proof_io_v1.py::encode_response structurally. The adapter maps
// proof bytes and publicInputs from backend.Result; Core returns placeholder
// values until proof serialization and public-input extraction are wired.
type executionResponse struct {
	ProverVersion     string                `json:"proverVersion"`
	ProofHex          string                `json:"proof"`
	StartBlockNumber  uint64                `json:"startBlockNumber"`
	PublicInputs      executionPublicInputs `json:"publicInputs"`
	L2L1Messages      []string              `json:"l2L1Messages"`
	TxFroms           []string              `json:"txFroms"`
	FilteredAddresses []string              `json:"filteredAddresses"`
}

type executionPublicInputs struct {
	ParentBlockHash                          string `json:"parentBlockHash"`
	EndBlockHash                             string `json:"endBlockHash"`
	EndBlockNumber                           uint64 `json:"endBlockNumber"`
	EndBlockTimestamp                        uint64 `json:"endBlockTimestamp"`
	L2L1MessagesHash                         string `json:"l2L1MessagesHash"`
	ParentL1L2BridgeRollingHash              string `json:"parentL1L2BridgeRollingHash"`
	ParentL1L2BridgeRollingHashMessageNumber uint64 `json:"parentL1L2BridgeRollingHashMessageNumber"`
	EndL1L2BridgeRollingHash                 string `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber    uint64 `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                   string `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                     string `json:"parentFtxRollingHash"`
	ParentProcessedFtxNumber                 uint64 `json:"parentProcessedFtxNumber"`
	EndFtxRollingHash                        string `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                    uint64 `json:"endProcessedFtxNumber"`
	FilteredAddressesHash                    string `json:"filteredAddressesHash"`
	TxFromsHash                              string `json:"txFromsHash"`
}

// failureResponseBody is a temporary adapter-owned operational format, not a
// reference-defined coordinator response. Treat it as provisional until the
// coordinator/backend failure semantics are agreed.
type failureResponseBody struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func newExecutionResponse(result backend.Result, startBlockNumber uint64, proverVersion string) executionResponse {
	return executionResponse{
		ProverVersion:     proverVersion,
		ProofHex:          hexBytes(result.ProofBytes),
		StartBlockNumber:  startBlockNumber,
		PublicInputs:      publicInputs(result.PublicInputs),
		L2L1Messages:      []string{},
		TxFroms:           []string{},
		FilteredAddresses: []string{},
	}
}

func publicInputs(pi backend.PublicInputs) executionPublicInputs {
	return executionPublicInputs{
		ParentBlockHash:                          hexHash(pi.ParentBlockHash),
		EndBlockHash:                             hexHash(pi.EndBlockHash),
		EndBlockNumber:                           pi.EndBlockNumber,
		EndBlockTimestamp:                        pi.EndBlockTimestamp,
		L2L1MessagesHash:                         hexHash(pi.L2L1MessagesHash),
		ParentL1L2BridgeRollingHash:              hexHash(pi.ParentL1L2BridgeRollingHash),
		ParentL1L2BridgeRollingHashMessageNumber: pi.ParentL1L2BridgeRollingHashMessageNumber,
		EndL1L2BridgeRollingHash:                 hexHash(pi.EndL1L2BridgeRollingHash),
		EndL1L2BridgeRollingHashMessageNumber:    pi.EndL1L2BridgeRollingHashMessageNumber,
		DynamicChainConfigHash:                   hexHash(pi.DynamicChainConfigHash),
		ParentFtxRollingHash:                     hexHash(pi.ParentFtxRollingHash),
		ParentProcessedFtxNumber:                 pi.ParentProcessedFtxNumber,
		EndFtxRollingHash:                        hexHash(pi.EndFtxRollingHash),
		EndProcessedFtxNumber:                    pi.EndProcessedFtxNumber,
		FilteredAddressesHash:                    hexHash(pi.FilteredAddressesHash),
		TxFromsHash:                              hexHash(pi.TxFromsHash),
	}
}

func failureResponse(id string, err error) failureResponseBody {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return failureResponseBody{JobID: id, Status: statusFailed, Error: msg}
}

func hexHash(v [32]byte) string {
	return hexBytes(v[:])
}

func hexBytes(v []byte) string {
	return fmt.Sprintf("0x%s", hex.EncodeToString(v))
}
