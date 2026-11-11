package jobadapter

import (
	"context"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// partialGuestOutput stands in for the guest output. The runner does not read
// it; it only compares it byte for byte with the native runner's output.
var partialGuestOutput = []byte("guest output")

// partialProver returns what partial returns: a proof, the public inputs
// decoded from the guest output, and that output.
func partialProver(guestOutput []byte) *mockProver {
	return &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{
			JobID:        job.ID,
			Status:       backend.ResultStatusOK,
			ProofBytes:   []byte("devmode proof"),
			PublicInputs: backend.PublicInputs{EndBlockNumber: 1, BlockCount: 1, ParentBlockHash: [32]byte{0xee}},
			GuestOutput:  guestOutput,
		}
	}}
}

// partial fills the response from the prover's result, and runs the guest on
// the same extended input as the native runner.
func TestRunner_Partial_ResponseFromResult(t *testing.T) {
	body := readFixture(t, "request_single_block.json")
	mock := partialProver(partialGuestOutput)
	runner, err := NewRunner(mock, testProverVersion, WithMode(backend.ProverModePartial))
	require.NoError(t, err)

	result := runner.Run(context.Background(), l2ExecutionRunRequest("p-1", body))

	require.Equal(t, RunStatusSuccess, result.Status, "run err: %v", result.Err)
	require.Len(t, mock.jobs, 1)
	req, err := DecodeL2ExecutionRequest(body)
	require.NoError(t, err)
	assert.Equal(t, ssz.EncodeExtendedInput(buildExtendedInput(req)), mock.jobs[0].Payload,
		"partial must run the guest on the extended input")
	assert.Equal(t, requestGuestProgramID(t, body), mock.jobs[0].GuestProgramID)

	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion+"-partial", resp.ProverVersion)
	assert.Equal(t, hexBytes([]byte("devmode proof")), resp.ProofHex)
	assert.Equal(t, req.Payloads[0].BlockNumber, resp.StartBlockNumber, "the start block should come from the request")
	assert.Equal(t, publicInputs(backend.PublicInputs{EndBlockNumber: 1, BlockCount: 1, ParentBlockHash: [32]byte{0xee}}),
		resp.PublicInputs, "the public inputs should come from the prover's result")
	assert.Equal(t, guestProgramID, resp.ProgramVk)
}

// With a native runner set, partial also checks the guest output against it.
func TestRunner_Partial_NativeCheckPasses(t *testing.T) {
	runner, err := NewRunner(partialProver(partialGuestOutput), testProverVersion,
		WithMode(backend.ProverModePartial),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, partialGuestOutput)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("p-2", readFixture(t, "request_single_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status, "run err: %v", result.Err)
}

func TestRunner_Partial_NativeCheckFails(t *testing.T) {
	wrong := append([]byte(nil), partialGuestOutput...)
	wrong[2] ^= 0xff
	runner, err := NewRunner(partialProver(wrong), testProverVersion,
		WithMode(backend.ProverModePartial),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, partialGuestOutput)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("p-3", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.ErrorContains(t, result.Err, "native runner check failed")
}
