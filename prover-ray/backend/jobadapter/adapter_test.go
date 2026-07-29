package jobadapter

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// singleReqName follows the coordinator's <start>-<end>-getZk...json convention;
// the adapter treats the base name as the job id and response name.
const singleReqName = "1000501-1000501-getZkL2ExecutionProofV1.json"

const zeroHashHex = "0x0000000000000000000000000000000000000000000000000000000000000000"

// mockProver stands in for backend.Core. It records the jobs it receives, runs
// an optional hook mid-Prove (to observe the claim), and returns a configurable
// result (defaulting to success).
type mockProver struct {
	result  func(backend.Job) backend.Result
	onProve func(backend.Job)
	jobs    []backend.Job
}

func (m *mockProver) Prove(_ context.Context, job backend.Job) backend.Result {
	m.jobs = append(m.jobs, job)
	if m.onProve != nil {
		m.onProve(job)
	}
	if m.result != nil {
		return m.result(job)
	}
	return backend.Result{JobID: job.ID, Status: backend.ResultStatusOK, ProofBytes: []byte{0xde, 0xad}}
}

func newAdapter(t *testing.T, prover Prover) (*Adapter, string) {
	t.Helper()
	root := t.TempDir()
	a, err := New(Config{RootDir: root, PollInterval: 5 * time.Millisecond}, prover)
	require.NoError(t, err)
	return a, root
}

func placeRequest(t *testing.T, root, name, fixture string) {
	t.Helper()
	data := readFixture(t, fixture)
	require.NoError(t, os.WriteFile(filepath.Join(root, "requests", name), data, 0o600))
}

func readExecutionResponse(t *testing.T, root, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "responses", name))
	require.NoError(t, err)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	return resp
}

func readFailureResponse(t *testing.T, root, name string) FailureResponse {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "responses", name))
	require.NoError(t, err)
	var resp FailureResponse
	require.NoError(t, json.Unmarshal(data, &resp))
	return resp
}

func writeRequestObject(t *testing.T, root, name string, obj map[string]any) {
	t.Helper()
	raw, err := json.Marshal(obj)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "requests", name), raw, 0o600))
}

// TestAdapter_SingleBlock_Success is the happy path: a single-block request is
// decoded, proved, its response written, and the request archived.
func TestAdapter_SingleBlock_Success(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	placeRequest(t, root, singleReqName, "request_single_block.json")

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	require.Len(t, mock.jobs, 1)
	job := mock.jobs[0]
	assert.Equal(t, backend.ProofTypeL2Execution, job.Type)
	assert.Equal(t, uint64(1000501), job.StartBlock)
	assert.Equal(t, uint64(1000501), job.EndBlock)
	wantSSZ, err := os.ReadFile("testdata/single_block_expected.ssz")
	require.NoError(t, err)
	assert.Equal(t, wantSSZ, job.Payload, "job payload must be the framed SSZ")

	resp := readExecutionResponse(t, root, singleReqName)
	assertResponseShapeMatchesReference(t, resp)
	assert.Equal(t, defaultProverVersion, resp["proverVersion"])
	assert.Equal(t, "0xdead", resp["proof"])
	startBlockNumber, ok := resp["startBlockNumber"].(float64)
	require.True(t, ok)
	assert.Equal(t, 1000501, int(startBlockNumber))
	assert.NotContains(t, resp, "status")
	assert.NotContains(t, resp, "jobId")
	assert.Empty(t, resp["l2L1Messages"])
	assert.Empty(t, resp["txFroms"])
	assert.Empty(t, resp["filteredAddresses"])
	publicInputs, ok := resp["publicInputs"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, publicInputs, 16)
	assert.Equal(t, zeroHashHex, publicInputs["parentBlockHash"])

	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName+".inprogress"))
	assert.FileExists(t, filepath.Join(root, "requests-done", singleReqName))
}

// TestAdapter_ClaimsBeforeProving verifies the request is renamed to
// .inprogress before the Prover runs, so a second worker cannot pick it up.
func TestAdapter_ClaimsBeforeProving(t *testing.T) {
	var claimed, originalGone bool
	var root string
	mock := &mockProver{onProve: func(_ backend.Job) {
		_, errClaim := os.Stat(filepath.Join(root, "requests", singleReqName+".inprogress"))
		claimed = errClaim == nil
		_, errOrig := os.Stat(filepath.Join(root, "requests", singleReqName))
		originalGone = errors.Is(errOrig, os.ErrNotExist)
	}}
	a, r := newAdapter(t, mock)
	root = r
	placeRequest(t, root, singleReqName, "request_single_block.json")

	_, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, claimed, "request must be claimed as .inprogress before Prove")
	assert.True(t, originalGone, "original request name must be gone once claimed")
}

// TestAdapter_ProveFailure verifies a failed proof is recorded as a failure
// response and archived, without crashing the loop.
func TestAdapter_ProveFailure(t *testing.T) {
	mock := &mockProver{result: func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusFailed, Err: errors.New("prove boom")}
	}}
	a, root := newAdapter(t, mock)
	placeRequest(t, root, singleReqName, "request_single_block.json")

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	resp := readFailureResponse(t, root, singleReqName)
	assert.Equal(t, "failed", resp.Status)
	assert.Contains(t, resp.Error, "prove boom")

	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.FileExists(t, filepath.Join(root, "requests-done", singleReqName+".failed"))
}

// TestAdapter_MalformedRequest verifies undecodable JSON never reaches the
// Prover, produces a failure response, and is archived so the loop continues.
func TestAdapter_MalformedRequest(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	require.NoError(t, os.WriteFile(filepath.Join(root, "requests", "bad.json"), []byte("not json"), 0o600))

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Empty(t, mock.jobs, "Prover must not be called for a malformed request")

	resp := readFailureResponse(t, root, "bad.json")
	assert.Equal(t, "failed", resp.Status)
	assert.FileExists(t, filepath.Join(root, "requests-done", "bad.json.failed"))
}

// TestAdapter_ForcedTransactions_NotImplemented verifies that the adapter does
// not silently drop rollupExtension.forcedTransactions while the backend has no
// field for them yet.
func TestAdapter_ForcedTransactions_NotImplemented(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)

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
	writeRequestObject(t, root, singleReqName, obj)

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Empty(t, mock.jobs, "forced transactions must not be dropped before proving")

	resp := readFailureResponse(t, root, singleReqName)
	assert.Equal(t, "failed", resp.Status)
	assert.Contains(t, resp.Error, "forced transactions")
}

// TestAdapter_MultiBlock verifies a multi-block request is rejected (not yet
// supported) without invoking the Prover.
func TestAdapter_MultiBlock(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	placeRequest(t, root, "1000501-1000502-getZkL2ExecutionProofV1.json", "request_multi_block.json")

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Empty(t, mock.jobs, "multi-block request must not reach the Prover")

	resp := readFailureResponse(t, root, "1000501-1000502-getZkL2ExecutionProofV1.json")
	assert.Equal(t, "failed", resp.Status)
}

// TestAdapter_SkipsInProgress verifies a file already claimed by another worker
// (ending in .inprogress) is ignored.
func TestAdapter_SkipsInProgress(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	inProgress := filepath.Join(root, "requests", singleReqName+".inprogress")
	require.NoError(t, os.WriteFile(inProgress, []byte("{}"), 0o600))

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, mock.jobs)
	assert.FileExists(t, inProgress, "an .inprogress file must be left untouched")
}

// TestNew_Validation covers New's argument checks and the PollInterval default.
func TestNew_Validation(t *testing.T) {
	t.Run("EmptyRootDir", func(t *testing.T) {
		_, err := New(Config{}, &mockProver{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RootDir")
	})
	t.Run("NilProver", func(t *testing.T) {
		_, err := New(Config{RootDir: t.TempDir()}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prover")
	})
	t.Run("DefaultsPollInterval", func(t *testing.T) {
		a, err := New(Config{RootDir: t.TempDir()}, &mockProver{})
		require.NoError(t, err)
		assert.Equal(t, defaultPollInterval, a.cfg.PollInterval)
	})
	t.Run("DefaultsProverVersion", func(t *testing.T) {
		a, err := New(Config{RootDir: t.TempDir()}, &mockProver{})
		require.NoError(t, err)
		assert.Equal(t, defaultProverVersion, a.cfg.ProverVersion)
	})
}

// TestNew_MkdirFailure verifies New reports an error when its subdirectories
// cannot be created (here RootDir is a regular file).
func TestNew_MkdirFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	_, err := New(Config{RootDir: file}, &mockProver{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating")
}

// TestAdapter_ReadDirError verifies a missing requests/ directory surfaces as
// an error from processOnce (and therefore aborts Run).
func TestAdapter_ReadDirError(t *testing.T) {
	a, root := newAdapter(t, &mockProver{})
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests")))

	_, err := a.processOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requests")
}

// TestAdapter_WriteResponseError verifies a failure writing the response
// aborts processing with an error rather than losing the claimed request.
func TestAdapter_WriteResponseError(t *testing.T) {
	a, root := newAdapter(t, &mockProver{})
	placeRequest(t, root, singleReqName, "request_single_block.json")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "responses")))

	_, err := a.processOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response")
}

// TestAdapter_ArchiveError verifies a failure moving the request to
// requests-done/ surfaces as an error.
func TestAdapter_ArchiveError(t *testing.T) {
	a, root := newAdapter(t, &mockProver{})
	placeRequest(t, root, singleReqName, "request_single_block.json")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests-done")))

	_, err := a.processOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archiving")
}

// TestAdapter_Run_ReturnsProcessError verifies Run surfaces an infrastructure
// error from its scan instead of looping forever.
func TestAdapter_Run_ReturnsProcessError(t *testing.T) {
	a, root := newAdapter(t, &mockProver{})
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests")))

	err := a.Run(context.Background())
	require.Error(t, err)
}

// TestAdapter_Run_Shutdown verifies Run processes pending work and returns
// cleanly when its context is cancelled.
func TestAdapter_Run_Shutdown(t *testing.T) {
	mock := &mockProver{}
	a, root := newAdapter(t, mock)
	placeRequest(t, root, singleReqName, "request_single_block.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(root, "responses", singleReqName))
		return err == nil
	}, time.Second, 5*time.Millisecond, "response must be written while Run polls")

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "Run must return nil on graceful shutdown")
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
