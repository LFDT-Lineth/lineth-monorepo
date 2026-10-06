package jobadapter

import (
	"context"
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_RunRollup(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusOK, ProofBytes: []byte{0xab, 0xcd}}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID:   "rollup-1",
		Type: backend.ProofTypeRollup,
		Body: readFixture(t, rollupRequestFixture),
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
	assert.Equal(t, "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d", resp.ProgramVk)
	assert.Equal(t, "0xabcd", resp.ProofHex)
	assert.Equal(t, uint64(10), resp.StartBlockNumber)
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
		ID: "rollup-fail", Type: backend.ProofTypeRollup, Body: readFixture(t, rollupRequestFixture),
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
		Body: readFixture(t, aggregationRequestFixture),
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
