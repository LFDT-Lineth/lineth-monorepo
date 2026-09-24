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

// fakeProver stands in for the subprocess prover: it records calls, runs an
// optional hook (to observe the claim), writes a canned response, and returns a
// configurable exit code or run error.
type fakeProver struct {
	exitCode int
	runErr   error
	response []byte
	onProve  func(reqPath string)
	calls    int
}

func (f *fakeProver) Prove(_ context.Context, reqPath, respPath string) (int, error) {
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

func newAdapter(t *testing.T, prover Prover) (*Adapter, string) {
	t.Helper()
	root := t.TempDir()
	a, err := New(Config{RequestsRootDir: root, PollInterval: 5 * time.Millisecond}, prover)
	require.NoError(t, err)
	return a, root
}

func placeRequest(t *testing.T, root, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, "requests", name), []byte("{}"), 0o600))
}

// TestAdapter_Success: the prover writes a response and exits 0; the response is
// published and the request archived as .success.
func TestAdapter_Success(t *testing.T) {
	prover := &fakeProver{response: []byte(`{"ok":true}`)}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, singleReqName)

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 1, prover.calls)

	data, err := os.ReadFile(filepath.Join(root, "responses", singleReqName))
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(data))

	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.FileExists(t, filepath.Join(root, "requests-done", singleReqName+".success"))
}

// TestAdapter_ClaimsBeforeProving: the request is .inprogress by the time the
// prover runs, so a second worker cannot pick it up.
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

	_, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, claimed, "request must be claimed as .inprogress before Prove")
	assert.True(t, originalGone, "original request name must be gone once claimed")
}

// TestAdapter_ProveFailure: a non-zero exit archives the request as
// .failure.<code>, and the response the worker wrote is still published.
func TestAdapter_ProveFailure(t *testing.T) {
	prover := &fakeProver{exitCode: 2, response: []byte(`{"status":"failed"}`)}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, singleReqName)

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	assert.FileExists(t, filepath.Join(root, "responses", singleReqName))
	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.FileExists(t, filepath.Join(root, "requests-done", singleReqName+".failure.2"))
}

// TestAdapter_WorkerCannotRun: when the prover cannot be run at all, the request
// is left for the next scan (not archived, no response).
func TestAdapter_WorkerCannotRun(t *testing.T) {
	prover := &fakeProver{runErr: errors.New("exec boom")}
	a, root := newAdapter(t, prover)
	placeRequest(t, root, singleReqName)

	handled, err := a.processRequest(context.Background(), singleReqName)
	require.NoError(t, err)
	assert.False(t, handled)

	assert.FileExists(t, filepath.Join(root, "requests", singleReqName))
	assert.NoFileExists(t, filepath.Join(root, "requests", singleReqName+".inprogress"))
	assert.NoFileExists(t, filepath.Join(root, "responses", singleReqName))
}

func TestAdapter_SkipsInProgress(t *testing.T) {
	prover := &fakeProver{}
	a, root := newAdapter(t, prover)
	inProgress := filepath.Join(root, "requests", singleReqName+".inprogress")
	require.NoError(t, os.WriteFile(inProgress, []byte("{}"), 0o600))

	n, err := a.processOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, prover.calls)
	assert.FileExists(t, inProgress)
}

func TestAdapter_LostClaim(t *testing.T) {
	prover := &fakeProver{}
	a, root := newAdapter(t, prover)

	handled, err := a.processRequest(context.Background(), singleReqName)
	require.NoError(t, err)
	assert.False(t, handled)
	assert.Equal(t, 0, prover.calls)
	assert.NoFileExists(t, filepath.Join(root, "responses", singleReqName))
}

func TestNew_Validation(t *testing.T) {
	t.Run("EmptyRequestsRootDir", func(t *testing.T) {
		_, err := New(Config{}, &fakeProver{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RequestsRootDir")
	})
	t.Run("NilProver", func(t *testing.T) {
		_, err := New(Config{RequestsRootDir: t.TempDir()}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prover")
	})
	t.Run("DefaultsPollInterval", func(t *testing.T) {
		a, err := New(Config{RequestsRootDir: t.TempDir()}, &fakeProver{})
		require.NoError(t, err)
		assert.Equal(t, defaultPollInterval, a.cfg.PollInterval)
	})
}

func TestNew_MkdirFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	_, err := New(Config{RequestsRootDir: file}, &fakeProver{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating")
}

func TestAdapter_ReadDirError(t *testing.T) {
	a, root := newAdapter(t, &fakeProver{})
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests")))

	_, err := a.processOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requests")
}

// TestAdapter_ArchiveError: a failure moving the request to requests-done/
// surfaces as an error without stranding the request as .inprogress.
func TestAdapter_ArchiveError(t *testing.T) {
	a, root := newAdapter(t, &fakeProver{response: []byte("{}")})
	placeRequest(t, root, singleReqName)
	require.NoError(t, os.RemoveAll(filepath.Join(root, "requests-done")))

	_, err := a.processOnce(context.Background())
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
