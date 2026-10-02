package jobadapter

import (
	"context"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dev-mock accepts the multi-block fixture (rejected by full) and labels the response.
func TestRunner_DevMock_AcceptsMultiBlockAndForcedTx(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusOK, ProofBytes: []byte("dev-proof:dev-mock")}
	}}
	runner, err := NewRunner(mock, testProverVersion, WithMode(backend.ProverModeDevMock))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dev-1", readFixture(t, "request_multi_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status)
	require.NoError(t, result.Err)
	require.Len(t, mock.jobs, 1)
	assert.Equal(t, uint64(1000501), mock.jobs[0].StartBlock)
	assert.Equal(t, uint64(1000502), mock.jobs[0].EndBlock, "range end is the last payload block")

	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion+"-dev-mock", resp.ProverVersion)
	assert.Equal(t, uint64(1000501), resp.StartBlockNumber)
}

// full keeps the single-block restriction.
func TestRunner_FullMode_StillRejectsMultiBlock(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion) // defaults to full
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("full-1", readFixture(t, "request_multi_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	assert.Empty(t, mock.jobs)
}

// TestResponseVersion_DevSuffixOnlyForDevModes checks the version labeling.
func TestResponseVersion_DevSuffixOnlyForDevModes(t *testing.T) {
	mock := &mockProver{}

	full, err := NewRunner(mock, "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", full.responseVersion())

	dev, err := NewRunner(mock, "1.0.0", WithMode(backend.ProverModeDevZkVM))
	require.NoError(t, err)
	assert.Equal(t, "1.0.0-dev-zkvm", dev.responseVersion())
}
