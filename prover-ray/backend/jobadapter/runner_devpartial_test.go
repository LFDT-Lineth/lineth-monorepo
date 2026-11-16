package jobadapter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// devPartialGuestOutput stands in for the guest output. The runner does not read
// it; it only compares it byte for byte with the native runner's output.
var devPartialGuestOutput = []byte("guest output")

// devPartialPublicInputs are the public inputs of a single-block job ending at
// endBlock.
func devPartialPublicInputs(endBlock uint64) backend.PublicInputs {
	return backend.PublicInputs{EndBlockNumber: endBlock, BlockCount: 1, ParentBlockHash: [32]byte{0xee}}
}

// devPartialProver returns what dev-partial returns: a proof, the public inputs
// decoded from the guest output, and that output.
func devPartialProver(guestOutput []byte) *mockProver {
	return devPartialProverWith(guestOutput, func(*backend.PublicInputs) {})
}

// devPartialProverWith is devPartialProver with the public inputs changed by
// edit.
func devPartialProverWith(guestOutput []byte, edit func(*backend.PublicInputs)) *mockProver {
	return &mockProver{result: func(job backend.Job) backend.Result {
		pi := devPartialPublicInputs(job.EndBlock)
		edit(&pi)
		return backend.Result{
			JobID:        job.ID,
			Status:       backend.ResultStatusOK,
			ProofBytes:   []byte("devmode proof"),
			PublicInputs: pi,
			GuestOutput:  guestOutput,
		}
	}}
}

// dev-partial fills the response from the prover's result, and runs the guest on
// the same extended input as the native runner.
func TestRunner_DevPartial_ResponseFromResult(t *testing.T) {
	body := readFixture(t, "request_single_block.json")
	mock := devPartialProver(devPartialGuestOutput)
	runner, err := NewRunner(mock, testProverVersion, WithMode(backend.ProverModeDevPartial))
	require.NoError(t, err)

	result := runner.Run(context.Background(), l2ExecutionRunRequest("p-1", body))

	require.Equal(t, RunStatusSuccess, result.Status, "run err: %v", result.Err)
	require.Len(t, mock.jobs, 1)
	req, err := DecodeL2ExecutionRequest(body)
	require.NoError(t, err)
	assert.Equal(t, ssz.EncodeExtendedInput(buildExtendedInput(req)), mock.jobs[0].Payload,
		"dev-partial must run the guest on the extended input")
	assert.Equal(t, requestGuestProgramID(t, body), mock.jobs[0].GuestProgramID)

	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion+"-dev-partial", resp.ProverVersion)
	assert.Equal(t, hexBytes([]byte("devmode proof")), resp.ProofHex)
	assert.Equal(t, req.Payloads[0].BlockNumber, resp.StartBlockNumber, "the start block should come from the request")
	assert.Equal(t, publicInputs(devPartialPublicInputs(req.Payloads[0].BlockNumber)),
		resp.PublicInputs, "the public inputs should come from the prover's result")
	assert.Equal(t, guestProgramID, resp.ProgramVk)
}

// With a native runner set, dev-partial also checks the guest output against it.
func TestRunner_DevPartial_NativeCheckPasses(t *testing.T) {
	runner, err := NewRunner(devPartialProver(devPartialGuestOutput), testProverVersion,
		WithMode(backend.ProverModeDevPartial),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, devPartialGuestOutput)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("p-2", readFixture(t, "request_single_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status, "run err: %v", result.Err)
}

func TestRunner_DevPartial_NativeCheckFails(t *testing.T) {
	wrong := append([]byte(nil), devPartialGuestOutput...)
	wrong[2] ^= 0xff
	runner, err := NewRunner(devPartialProver(wrong), testProverVersion,
		WithMode(backend.ProverModeDevPartial),
		WithNativeRunnerBin(fakeRunnerJSONSSZ(t, devZkVMRunnerJSON, devPartialGuestOutput)))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("p-3", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.ErrorContains(t, result.Err, "native runner check failed")
}

// The guest's end block and block count must match the request.
func TestRunner_DevPartial_PublicInputsMatchRequest(t *testing.T) {
	cases := map[string]struct {
		edit    func(*backend.PublicInputs)
		wantErr string
	}{
		"end block":   {func(pi *backend.PublicInputs) { pi.EndBlockNumber++ }, "end block"},
		"block count": {func(pi *backend.PublicInputs) { pi.BlockCount = 2 }, "block count"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			runner, err := NewRunner(devPartialProverWith(devPartialGuestOutput, tc.edit), testProverVersion,
				WithMode(backend.ProverModeDevPartial))
			require.NoError(t, err)

			result := runner.Run(context.Background(),
				l2ExecutionRunRequest("p-4", readFixture(t, "request_single_block.json")))

			assert.Equal(t, RunStatusFailed, result.Status)
			assert.ErrorContains(t, result.Err, tc.wantErr)
		})
	}
}

// A native runner that cannot run fails the job before the guest is proved.
func TestRunner_DevPartial_NativeRunnerFailsFirst(t *testing.T) {
	mock := devPartialProver(devPartialGuestOutput)
	runner, err := NewRunner(mock, testProverVersion, WithMode(backend.ProverModeDevPartial),
		WithNativeRunnerBin(filepath.Join(t.TempDir(), "missing-runner")))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("p-5", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Empty(t, mock.jobs, "the guest should not be proved")
}
