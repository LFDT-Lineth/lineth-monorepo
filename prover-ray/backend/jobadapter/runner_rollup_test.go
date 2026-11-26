package jobadapter

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_RunRollup(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{
			JobID: job.ID, Status: backend.ResultStatusOK, ProofBytes: []byte{0xab, 0xcd}, ProgramVK: filledHash(0x33),
		}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID:   "rollup-1",
		Type: backend.ProofTypeRollup,
		Body: readSpecFixture(t, referenceRollupRequest),
	})

	require.Equal(t, RunStatusSuccess, result.Status)
	require.NoError(t, result.Err)
	require.Len(t, mock.jobs, 1)
	job := mock.jobs[0]
	assert.Equal(t, backend.ProofTypeRollup, job.Type)
	assert.Equal(t, uint64(10), job.StartBlock)
	assert.Equal(t, uint64(14), job.EndBlock)

	resp, ok := result.ResponseBody.(rollupResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion, resp.ProverVersion)
	assert.Equal(t, repeatHex(0x33), resp.ProgramVk,
		"programVk should be the VK of the prover's proof, not the request's guestProgramId")
	assert.Equal(t, "0xabcd", resp.ProofHex)
	assert.Equal(t, uint64(10), resp.StartBlockNumber)
}

// TestRunner_RunRollup_GuestInput checks that the modes that run the rollup
// guest pass it the input built from the request, and that dev-mock passes none.
func TestRunner_RunRollup_GuestInput(t *testing.T) {
	golden, err := os.ReadFile("../../utils/ssz/testdata/rollup_input.ssz")
	require.NoError(t, err)

	for mode, want := range map[backend.ProverMode][]byte{
		backend.ProverModeDevZkVM:    golden,
		backend.ProverModeDevPartial: golden,
		backend.ProverModeDevMock:    nil,
	} {
		t.Run(string(mode), func(t *testing.T) {
			mock := &mockProver{}
			runner, err := NewRunner(mock, testProverVersion, WithMode(mode))
			require.NoError(t, err)

			result := runner.Run(context.Background(), RunRequest{
				ID: "rollup-input", Type: backend.ProofTypeRollup, Body: readSpecFixture(t, referenceRollupRequest),
			})

			require.Equal(t, RunStatusSuccess, result.Status, "error: %v", result.Err)
			require.Len(t, mock.jobs, 1)
			assert.Equal(t, want, mock.jobs[0].Payload, "the payload should be the spec's encoding of the request")
		})
	}
}

func TestRunner_RunRollup_Malformed(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID: "bad-rollup", Type: backend.ProofTypeRollup, Body: []byte("not json"),
	})

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	assert.Empty(t, mock.jobs)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Contains(t, failure.Error, "parsing JSON")
}

func TestRunner_RunRollup_ProverFailure(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusFailed, Err: errors.New("prove boom")}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID: "rollup-fail", Type: backend.ProofTypeRollup, Body: readSpecFixture(t, referenceRollupRequest),
	})

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInternalError, result.FailureCode)
	require.Len(t, mock.jobs, 1)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Contains(t, failure.Error, "prove boom")
}

func TestRunner_RunAggregation(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusOK, ProofBytes: []byte{0xab, 0xcd}}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID:   "agg-1",
		Type: backend.ProofTypeRollupAggregation,
		Body: readSpecFixture(t, referenceAggregationRequest),
	})

	require.Equal(t, RunStatusSuccess, result.Status)
	require.NoError(t, result.Err)
	require.Len(t, mock.jobs, 1)
	job := mock.jobs[0]
	assert.Equal(t, backend.ProofTypeRollupAggregation, job.Type)
	assert.Equal(t, uint64(10), job.StartBlock)
	assert.Equal(t, uint64(18), job.EndBlock)

	resp, ok := result.ResponseBody.(aggregationResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion, resp.ProverVersion)
	assert.Equal(t, "0xabcd", resp.ProofHex)
	assert.Equal(t, uint64(10), resp.StartBlockNumber)
}

func TestRunner_RunAggregation_Malformed(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID: "bad-agg", Type: backend.ProofTypeRollupAggregation, Body: []byte("not json"),
	})

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	assert.Empty(t, mock.jobs)
}

// TestRunner_RunRollup_InvalidGuestOutput checks that the modes that run the
// rollup guest reject an embedded guestOutput that is not an l2-execution
// guest output, and that dev-mock, which builds no input, accepts it.
func TestRunner_RunRollup_InvalidGuestOutput(t *testing.T) {
	withGuestOutput := func(mutate func(out []byte) []byte) []byte {
		obj := unmarshalObject(t, readSpecFixture(t, rollupRequestGuestOutput))
		proof := obj[proofRequestKey].(map[string]any)[l2ExecutionProofsKey].([]any)[0].(map[string]any)
		proof[guestOutputKey] = hexBytes(mutate(mustHex(t, proof[guestOutputKey].(string))))
		return marshal(t, obj)
	}
	for name, body := range map[string][]byte{
		"empty": withGuestOutput(func([]byte) []byte { return nil }),
		"wrong hash": withGuestOutput(func(out []byte) []byte {
			out[0] ^= 1
			return out
		}),
	} {
		t.Run(name, func(t *testing.T) {
			for mode, wantStatus := range map[backend.ProverMode]RunStatus{
				backend.ProverModeDevZkVM:    RunStatusFailed,
				backend.ProverModeDevPartial: RunStatusFailed,
				backend.ProverModeDevMock:    RunStatusSuccess,
			} {
				runner, err := NewRunner(&mockProver{}, testProverVersion, WithMode(mode))
				require.NoError(t, err)

				result := runner.Run(context.Background(), RunRequest{ID: "rollup", Type: backend.ProofTypeRollup, Body: body})

				require.Equal(t, wantStatus, result.Status, "%s: %v", mode, result.Err)
				if wantStatus == RunStatusFailed {
					assert.Equal(t, FailureCodeInvalidInput, result.FailureCode, mode)
					assert.ErrorContains(t, result.Err, "l2ExecutionProofs[0].guestOutput", mode)
				}
			}
		})
	}
}
