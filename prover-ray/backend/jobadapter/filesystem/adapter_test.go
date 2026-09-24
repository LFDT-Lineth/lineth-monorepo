package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const singleReqName = "1000501-1000501-getZkL2ExecutionProofV1.json"

// fakeProver stands in for the real spawn: it records calls, runs an optional
// hook (to observe the claim / order), writes a canned response, and returns a
// configurable exit code or run error. Its run method is a RunProver.
type fakeProver struct {
	exitCode int
	runErr   error
	response []byte
	onProve  func(reqPath string)
	calls    int
}

func (f *fakeProver) run(_ context.Context, reqPath, respPath string) (int, error) {
	f.calls++
	if f.onProve != nil {
		f.onProve(reqPath)
	}
	if f.runErr != nil {
		return -1, f.runErr
	}
	if f.response != nil {
		if err := os.WriteFile(respPath, f.response, 0o600); err != nil {
			return -1, err
		}
	}
	return f.exitCode, nil
}

// newAdapter builds a one-queue adapter over a fresh temp root.
func newAdapter(t *testing.T, prover *fakeProver) (*Adapter, string) {
	t.Helper()
	root := t.TempDir()
	a, err := New(Config{Queues: []Queue{{RequestsRootDir: root}}, PollInterval: 5 * time.Millisecond}, prover.run)
	require.NoError(t, err)
	return a, root
}

func placeRequest(t *testing.T, root, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, "requests", name), []byte("{}"), 0o600))
}

func TestEndBlock(t *testing.T) {
	assert.Equal(t, 1000502, endBlock("1000501-1000502-getZkL2ExecutionProofV1.json"))
	assert.Equal(t, 18, endBlock("10-18-getZkRollupAggregationProofV1.json"))
	assert.Equal(t, 0, endBlock("bad.json"))
}

// TestAdapter_Success: the prover writes a response and exits 0; the response is
// published and the request archived as .success.
func TestAdapter_Success(t *testing.T) {
	prover := &fakeProver{response: []byte(`{"ok":true}`)}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, singleReqName)

	processed, err := a.processNext(context.Background())
	require.NoError(t, err)
	assert.True(t, processed)
	assert.Equal(t, 1, prover.calls)

	data, err := os.ReadFile(filepath.Join(root, "responses", singleReqName))
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(data))

	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.FileExists(t, filepath.Join(root, "requests-done", singleReqName+".success"))
}

// TestAdapter_Priority_EndBlock: the earliest end block is processed first.
func TestAdapter_Priority_EndBlock(t *testing.T) {
	var order []string
	prover := &fakeProver{response: []byte("{}"), onProve: func(reqPath string) {
		order = append(order, filepath.Base(reqPath))
	}}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, "10-20-getZkL2ExecutionProofV1.json")
	placeRequest(t, root, "10-14-getZkL2ExecutionProofV1.json")

	// Two calls: each processNext handles the current best.
	_, err := a.processNext(context.Background())
	require.NoError(t, err)
	_, err = a.processNext(context.Background())
	require.NoError(t, err)

	require.Len(t, order, 2)
	assert.Contains(t, order[0], "10-14", "earliest end block runs first")
	assert.Contains(t, order[1], "10-20")
}

// TestAdapter_Priority_QueueTiebreaker: at equal end block, the lower-priority
// queue runs first.
func TestAdapter_Priority_QueueTiebreaker(t *testing.T) {
	var first string
	prover := &fakeProver{response: []byte("{}"), onProve: func(reqPath string) {
		if first == "" {
			first = reqPath
		}
	}}
	high := t.TempDir() // priority 0
	low := t.TempDir()  // priority 2
	a, err := New(Config{
		Queues: []Queue{
			{RequestsRootDir: low, Priority: 2},
			{RequestsRootDir: high, Priority: 0},
		},
		PollInterval: 5 * time.Millisecond,
	}, prover.run)
	require.NoError(t, err)

	name := "10-14-getZkL2ExecutionProofV1.json"
	placeRequest(t, low, name)
	placeRequest(t, high, name)

	_, err = a.processNext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, first, high, "the priority-0 queue runs first at equal end block")
}

// TestAdapter_ClaimsBeforeProving: the request is .inprogress by the time the
// prover runs.
func TestAdapter_ClaimsBeforeProving(t *testing.T) {
	var claimed, originalGone bool
	var root string
	prover := &fakeProver{onProve: func(_ string) {
		_, errClaim := os.Stat(filepath.Join(root, "requests", singleReqName+".inprogress"))
		claimed = errClaim == nil
		_, errOrig := os.Stat(filepath.Join(root, "requests", singleReqName))
		originalGone = errors.Is(errOrig, os.ErrNotExist)
	}}
	a, r := newAdapter(t, prover)
	root = r
	placeRequest(t, root, singleReqName)

	_, err := a.processNext(context.Background())
	require.NoError(t, err)
	assert.True(t, claimed, "request must be claimed as .inprogress before the prover runs")
	assert.True(t, originalGone)
}

// TestAdapter_ProveFailure: a non-zero exit archives the request as
// .failure.<code>, and the response the prover wrote is still published.
func TestAdapter_ProveFailure(t *testing.T) {
	prover := &fakeProver{exitCode: 2, response: []byte(`{"status":"failed"}`)}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, singleReqName)

	processed, err := a.processNext(context.Background())
	require.NoError(t, err)
	assert.True(t, processed)

	assert.FileExists(t, filepath.Join(root, "responses", singleReqName))
	assert.FileExists(t, filepath.Join(root, "requests-done", singleReqName+".failure.2"))
}

// TestAdapter_ProverCannotRun: when the prover cannot be run, the request is left
// for the next scan (not archived, no response).
func TestAdapter_ProverCannotRun(t *testing.T) {
	prover := &fakeProver{runErr: errors.New("exec boom")}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, singleReqName)

	claimed, err := a.processRequest(context.Background(), root, singleReqName)
	require.NoError(t, err)
	assert.False(t, claimed)

	assert.FileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.NoFileExists(t, filepath.Join(root, "responses", singleReqName))
}

func TestAdapter_SkipsInProgress(t *testing.T) {
	prover := &fakeProver{}
	a, root := newAdapter(t, prover)
	inProgress := filepath.Join(root, "requests", singleReqName+".inprogress")
	require.NoError(t, os.WriteFile(inProgress, []byte("{}"), 0o600))

	processed, err := a.processNext(context.Background())
	require.NoError(t, err)
	assert.False(t, processed)
	assert.Equal(t, 0, prover.calls)
	assert.FileExists(t, inProgress)
}

func TestAdapter_LostClaim(t *testing.T) {
	prover := &fakeProver{}
	a, root := newAdapter(t, prover)

	claimed, err := a.processRequest(context.Background(), root, singleReqName)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, 0, prover.calls)
}

func TestNew_Validation(t *testing.T) {
	t.Run("NoQueues", func(t *testing.T) {
		_, err := New(Config{}, (&fakeProver{}).run)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "queue")
	})
	t.Run("NilSpawn", func(t *testing.T) {
		_, err := New(Config{Queues: []Queue{{RequestsRootDir: t.TempDir()}}}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spawn")
	})
	t.Run("DefaultsPollInterval", func(t *testing.T) {
		a, err := New(Config{Queues: []Queue{{RequestsRootDir: t.TempDir()}}}, (&fakeProver{}).run)
		require.NoError(t, err)
		assert.Equal(t, defaultPollInterval, a.cfg.PollInterval)
	})
}

func TestNew_MkdirFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	_, err := New(Config{Queues: []Queue{{RequestsRootDir: file}}}, (&fakeProver{}).run)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating")
}

func TestAdapter_ReadDirError(t *testing.T) {
	a, root := newAdapter(t, &fakeProver{})
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests")))

	_, err := a.processNext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requests")
}

// TestAdapter_ArchiveError: a failure moving the request to requests-done/
// surfaces as an error without stranding the request as .inprogress.
func TestAdapter_ArchiveError(t *testing.T) {
	a, root := newAdapter(t, &fakeProver{response: []byte("{}")})
	placeRequest(t, root, singleReqName)
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests-done")))

	_, err := a.processNext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archiving")
	assert.FileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName+".inprogress"))
}

func TestAdapter_Run_ReturnsProcessError(t *testing.T) {
	a, root := newAdapter(t, &fakeProver{})
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests")))

	err := a.Run(context.Background())
	require.Error(t, err)
}

// TestAdapter_Run_Shutdown: Run processes pending work and returns cleanly when
// its context is cancelled.
func TestAdapter_Run_Shutdown(t *testing.T) {
	a, root := newAdapter(t, &fakeProver{response: []byte("{}")})
	placeRequest(t, root, singleReqName)

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
