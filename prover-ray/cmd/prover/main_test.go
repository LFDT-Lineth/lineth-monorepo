package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../backend/jobadapter/testdata"

// TestDevMockSmoke runs the dev-mock binary path over a request folder end to end.
func TestDevMockSmoke(t *testing.T) {
	root := t.TempDir()

	core, err := backend.New(backend.Config{Mode: backend.ProverModeDevMock})
	require.NoError(t, err)
	adapter, err := filesystem.New(filesystem.Config{
		RequestsRootDir: root,
		ProverVersion:   "0.0.0-test",
		PollInterval:    5 * time.Millisecond,
		Mode:            backend.ProverModeDevMock,
	}, core)
	require.NoError(t, err)

	drop(t, root, "single.json", "request_single_block.json")
	drop(t, root, "multi.json", "request_multi_block.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- adapter.Run(ctx) }()

	for _, name := range []string{"single.json", "multi.json"} {
		require.Eventually(t, func() bool {
			_, err := os.Stat(filepath.Join(root, "responses", name))
			return err == nil
		}, 2*time.Second, 5*time.Millisecond, "response for %s must be written", name)
	}
	cancel()
	require.NoError(t, <-done, "Run must return cleanly on shutdown")

	// Single-block response is schema-shaped, dev-labelled, with a marker proof.
	resp := readJSON(t, filepath.Join(root, "responses", "single.json"))
	assert.Equal(t, "0.0.0-test-dev-mock", resp["proverVersion"])
	proof, ok := resp["proof"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(proof, "0x"))
	assert.Greater(t, len(proof), 2, "proof marker is non-empty")
	pi, ok := resp["publicInputs"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, pi, 16, "all 16 public-input fields present (placeholder zeros)")
	assert.Empty(t, resp["l2L1Messages"])

	// Conflated response carries the range start.
	multi := readJSON(t, filepath.Join(root, "responses", "multi.json"))
	start, ok := multi["startBlockNumber"].(float64)
	require.True(t, ok)
	assert.Equal(t, 1000501, int(start))
}

func TestRun_RequiresConfig(t *testing.T) {
	t.Setenv("CONFIG_FILE", "")
	err := run([]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestRun_RejectsInvalidMode(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(
		"version = \"t\"\n[execution]\nprover_mode = \"bogus\"\nrequests_root_dir = \"/tmp\"\n"), 0o600))
	err := run([]string{"--config", cfg})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid execution.prover_mode")
}

func drop(t *testing.T, root, name, fixture string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, fixture))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "requests", name), data, 0o600))
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}
