package jobadapter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProver struct {
	result func(backend.Job) backend.Result
	jobs   []backend.Job
}

const testProverVersion = "runner-test-version"

func (m *mockProver) Prove(_ context.Context, job backend.Job) backend.Result {
	m.jobs = append(m.jobs, job)
	if m.result != nil {
		return m.result(job)
	}
	return backend.Result{JobID: job.ID, Status: backend.ResultStatusOK}
}

func l2ExecutionRunRequest(id string, body []byte) RunRequest {
	return RunRequest{ID: id, Type: backend.ProofTypeL2Execution, Body: body}
}

func TestRunner_RunSingleBlock(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{
			JobID:  job.ID,
			Status: backend.ResultStatusOK,
			PublicInputs: backend.PublicInputs{
				ParentBlockHash: filledHash(0x22),
				EndBlockNumber:  job.EndBlock,
			},
			ProofBytes: []byte{0xab, 0xcd},
		}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), l2ExecutionRunRequest("job-1", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusSuccess, result.Status)
	assert.Empty(t, result.FailureCode)
	require.NoError(t, result.Err)
	require.Len(t, mock.jobs, 1)
	job := mock.jobs[0]
	assert.Equal(t, "job-1", job.ID)
	assert.Equal(t, backend.ProofTypeL2Execution, job.Type)
	assert.Equal(t, uint64(1000501), job.StartBlock)
	assert.Equal(t, uint64(1000501), job.EndBlock)
	assert.NotEmpty(t, job.Payload)

	execResp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, testProverVersion, execResp.ProverVersion)
	assert.Equal(t, "0xabcd", execResp.ProofHex)
	assert.Equal(t, uint64(1000501), execResp.StartBlockNumber)
	assert.Equal(t, uint64(1000501), execResp.PublicInputs.EndBlockNumber)
	assert.Equal(t, repeatHex(0x22), execResp.PublicInputs.ParentBlockHash)
}

func TestRunner_RunMalformedRequest(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), l2ExecutionRunRequest("bad-job", []byte("not json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	require.Error(t, result.Err)
	assert.Empty(t, mock.jobs)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Equal(t, "bad-job", failure.JobID)
	assert.Equal(t, RunStatusFailed, failure.Status)
	assert.Equal(t, FailureCodeInvalidInput, failure.FailureCode)
	assert.Contains(t, failure.Error, "parsing JSON")
}

func TestRunner_RunUnsupportedProofType(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(context.Background(), RunRequest{
		ID:   "blob-job",
		Type: backend.ProofType("blob"),
		Body: readFixture(t, "request_single_block.json"),
	})

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	require.Error(t, result.Err)
	assert.Empty(t, mock.jobs)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Equal(t, "blob-job", failure.JobID)
	assert.Equal(t, FailureCodeInvalidInput, failure.FailureCode)
	assert.Contains(t, failure.Error, "proof type")
	assert.Contains(t, failure.Error, backend.ErrNotImplemented.Error())
}

func TestRunner_RunMultiBlockNotImplemented(t *testing.T) {
	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(
		context.Background(),
		l2ExecutionRunRequest("multi-block-job", readFixture(t, "request_multi_block.json")),
	)

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	require.Error(t, result.Err)
	assert.Empty(t, mock.jobs)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Equal(t, "multi-block-job", failure.JobID)
	assert.Equal(t, FailureCodeInvalidInput, failure.FailureCode)
	assert.Contains(t, failure.Error, "multi-block")
}

func TestRunner_RunForcedTransactionsNotImplemented(t *testing.T) {
	var obj map[string]any
	require.NoError(t, json.Unmarshal(readFixture(t, "request_single_block.json"), &obj))
	payload := obj[proofRequestKey].(map[string]any)[payloadsKey].([]any)[0].(map[string]any)
	payload[rollupExtensionKey].(map[string]any)[forcedTransactionsKey] = []any{
		map[string]any{
			numberKey:      16,
			deadlineKey:    1000599,
			signedTxRlpKey: "0x02f86b",
			acceptanceKey:  forcedTxIncluded,
		},
	}
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	mock := &mockProver{}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)
	result := runner.Run(context.Background(), l2ExecutionRunRequest("forced-tx-job", raw))

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	require.Error(t, result.Err)
	assert.Empty(t, mock.jobs)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Equal(t, "forced-tx-job", failure.JobID)
	assert.Equal(t, FailureCodeInvalidInput, failure.FailureCode)
	assert.Contains(t, failure.Error, "forced transactions")
	assert.Contains(t, failure.Error, backend.ErrNotImplemented.Error())
}

func TestRunner_RunProverFailure(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusFailed, Err: errors.New("prove boom")}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(
		context.Background(),
		l2ExecutionRunRequest("prove-fail-job", readFixture(t, "request_single_block.json")),
	)

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInternalError, result.FailureCode)
	require.Error(t, result.Err)
	require.Len(t, mock.jobs, 1)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Equal(t, "prove-fail-job", failure.JobID)
	assert.Equal(t, FailureCodeInternalError, failure.FailureCode)
	assert.Contains(t, failure.Error, "prove boom")
}

func TestRunner_RunProverFailureWithoutError(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusFailed}
	}}
	runner, err := NewRunner(mock, testProverVersion)
	require.NoError(t, err)

	result := runner.Run(
		context.Background(),
		l2ExecutionRunRequest("prove-fail-job", readFixture(t, "request_single_block.json")),
	)

	assert.Equal(t, RunStatusFailed, result.Status)
	assert.Equal(t, FailureCodeInternalError, result.FailureCode)
	require.Error(t, result.Err)
	require.Len(t, mock.jobs, 1)
	failure, ok := result.ResponseBody.(failureResponseBody)
	require.True(t, ok)
	assert.Equal(t, "prove-fail-job", failure.JobID)
	assert.Equal(t, FailureCodeInternalError, failure.FailureCode)
	assert.Contains(t, failure.Error, "prover returned status failed")
}

func TestNewRunner_Validation(t *testing.T) {
	_, err := NewRunner(nil, testProverVersion)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prover")

	_, err = NewRunner(&mockProver{}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proverVersion")
}
