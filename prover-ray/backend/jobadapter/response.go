package jobadapter

import (
	"encoding/hex"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/nativerunner"
)

// executionResponse is the success response body for getZkL2ExecutionProofV1.
// It mirrors proof_io_v1.py::encode_response structurally. The adapter maps
// proof bytes and publicInputs from backend.Result. Those values are
// provisional until proof serialization and public-input extraction are wired.
type executionResponse struct {
	ProverVersion    string                `json:"proverVersion"`
	ProofHex         string                `json:"proof"`
	StartBlockNumber uint64                `json:"startBlockNumber"`
	PublicInputs     executionPublicInputs `json:"publicInputs"`
	ProgramVk        string                `json:"programVk"`
}

type executionPublicInputs struct {
	ParentBlockHash                       string   `json:"parentBlockHash"`
	EndBlockHash                          string   `json:"endBlockHash"`
	EndBlockNumber                        uint64   `json:"endBlockNumber"`
	EndBlockTimestamp                     uint64   `json:"endBlockTimestamp"`
	L2L1Messages                          []string `json:"l2L1Messages"`
	EndL1L2BridgeRollingHash              string   `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber uint64   `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                string   `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                  string   `json:"parentFtxRollingHash"`
	ParentFtxNumber                       uint64   `json:"parentFtxNumber"`
	EndFtxRollingHash                     string   `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                 uint64   `json:"endProcessedFtxNumber"`
	FilteredAddresses                     []string `json:"filteredAddresses"`
	TxFromsHash                           string   `json:"txFromsHash"`
	BlockCount                            uint64   `json:"blockCount"`
	L2MessagingBlocksOffsets              []uint16 `json:"l2MessagingBlocksOffsets"`
}

// failureResponseBody is a temporary adapter-owned operational format, not a
// reference-defined coordinator response. Treat it as provisional until the
// coordinator/backend failure semantics are agreed.
type failureResponseBody struct {
	JobID       string      `json:"jobId"`
	Status      RunStatus   `json:"status"`
	FailureCode FailureCode `json:"failureCode,omitempty"`
	Error       string      `json:"error,omitempty"`
}

func newExecutionResponse(
	result backend.Result, startBlockNumber uint64, proverVersion string, programVk []byte,
) executionResponse {
	return executionResponse{
		ProverVersion:    proverVersion,
		ProofHex:         hexBytes(result.ProofBytes),
		StartBlockNumber: startBlockNumber,
		PublicInputs:     publicInputs(result.PublicInputs),
		ProgramVk:        hexBytes(programVk),
	}
}

// newExecutionResponseFromNative builds a response from the native runner's
// output (dev-zkvm): the public inputs, a placeholder marker proof (dev-zkvm
// produces no real proof), and the echoed programVk.
func newExecutionResponseFromNative(
	out nativerunner.Output, proverVersion string, programVk []byte,
) executionResponse {
	return executionResponse{
		ProverVersion:    proverVersion,
		ProofHex:         hexBytes(backend.DevMarkerProof(backend.ProverModeDevZkVM)),
		StartBlockNumber: out.StartBlockNumber,
		PublicInputs:     publicInputs(out.PublicInputs),
		ProgramVk:        hexBytes(programVk),
	}
}

func hexHashList(hs [][32]byte) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = hexHash(h)
	}
	return out
}

func hexAddressList(as [][20]byte) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = hexBytes(a[:])
	}
	return out
}

func publicInputs(pi backend.PublicInputs) executionPublicInputs {
	return executionPublicInputs{
		ParentBlockHash:                       hexHash(pi.ParentBlockHash),
		EndBlockHash:                          hexHash(pi.EndBlockHash),
		EndBlockNumber:                        pi.EndBlockNumber,
		EndBlockTimestamp:                     pi.EndBlockTimestamp,
		L2L1Messages:                          hexHashList(pi.L2L1Messages),
		EndL1L2BridgeRollingHash:              hexHash(pi.EndL1L2BridgeRollingHash),
		EndL1L2BridgeRollingHashMessageNumber: pi.EndL1L2BridgeRollingHashMessageNumber,
		DynamicChainConfigHash:                hexHash(pi.DynamicChainConfigHash),
		ParentFtxRollingHash:                  hexHash(pi.ParentFtxRollingHash),
		ParentFtxNumber:                       pi.ParentFtxNumber,
		EndFtxRollingHash:                     hexHash(pi.EndFtxRollingHash),
		EndProcessedFtxNumber:                 pi.EndProcessedFtxNumber,
		FilteredAddresses:                     hexAddressList(pi.FilteredAddresses),
		TxFromsHash:                           hexHash(pi.TxFromsHash),
		BlockCount:                            pi.BlockCount,
		// non-nil so an empty list marshals to [] not null.
		L2MessagingBlocksOffsets: append([]uint16{}, pi.L2MessagingBlocksOffsets...),
	}
}

func failureResponse(id string, code FailureCode, err error) failureResponseBody {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return failureResponseBody{JobID: id, Status: RunStatusFailed, FailureCode: code, Error: msg}
}

func hexHash(v [32]byte) string {
	return hexBytes(v[:])
}

func hexBytes(v []byte) string {
	return fmt.Sprintf("0x%s", hex.EncodeToString(v))
}
