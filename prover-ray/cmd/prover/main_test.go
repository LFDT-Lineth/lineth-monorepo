package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../backend/jobadapter/testdata"

// TestIntegration_AdapterSpawnsWorker builds the binary and runs the adapter,
// which spawns a "prover prove" worker per request and writes the response.
func TestIntegration_AdapterSpawnsWorker(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary and spawns worker processes")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "prover")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	queue := filepath.Join(dir, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(queue, "requests"), 0o750))
	cfg := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(
		"version = \"t\"\n[execution]\nprover_mode = \"dev-mock\"\nrequests_root_dir = \""+queue+"\"\n"), 0o600))

	name := "10-11-getZkL2ExecutionProofV1.json"
	reqData, err := os.ReadFile(filepath.Join(fixtureDir, "request_single_block.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(queue, "requests", name), reqData, 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--config", cfg)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill() }()

	respPath := filepath.Join(queue, "responses", name)
	require.Eventually(t, func() bool {
		_, statErr := os.Stat(respPath)
		return statErr == nil
	}, 30*time.Second, 200*time.Millisecond, "adapter must spawn a worker and produce a response")

	resp := readJSON(t, respPath)
	assert.Equal(t, "t-dev-mock", resp["proverVersion"])
	assert.FileExists(t, filepath.Join(queue, "requests-done", name+".success"))
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
	assert.Contains(t, err.Error(), "invalid prover_mode")
}

func devMockConfig(t *testing.T, dir string) string {
	t.Helper()
	cfg := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(
		"version = \"t\"\n[execution]\nprover_mode = \"dev-mock\"\nrequests_root_dir = \""+dir+"\"\n"), 0o600))
	return cfg
}

// TestRunProve_DevMock runs the one-shot worker: read a request file, prove it,
// write the response file.
func TestRunProve_DevMock(t *testing.T) {
	dir := t.TempDir()
	cfg := devMockConfig(t, dir)

	reqData, err := os.ReadFile(filepath.Join(fixtureDir, "request_single_block.json"))
	require.NoError(t, err)
	inPath := filepath.Join(dir, "request.json")
	require.NoError(t, os.WriteFile(inPath, reqData, 0o600))
	outPath := filepath.Join(dir, "response.json")

	require.NoError(t, run([]string{
		"prove", "--config", cfg, "--in", inPath, "--out", outPath,
	}))

	resp := readJSON(t, outPath)
	assert.Equal(t, "t-dev-mock", resp["proverVersion"])
	pi, ok := resp["publicInputs"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, pi, 16, "all 16 public-input fields present (placeholder zeros)")
}

func TestRunProve_RequiresInOut(t *testing.T) {
	cfg := devMockConfig(t, t.TempDir())
	err := run([]string{"prove", "--config", cfg})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--in and --out")
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}
