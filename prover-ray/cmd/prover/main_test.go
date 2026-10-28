package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter/subprocess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureDir = "../../backend/jobadapter/testdata"

const (
	cmdProve   = "prove"
	flagConfig = "--config"
	flagIn     = "--in"
	flagOut    = "--out"
)

// TestIntegration_AdapterSpawnsWorker builds the binary and runs the adapter,
// which spawns a "prover prove" worker per request and writes the response.
func TestIntegration_AdapterSpawnsWorker(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary and spawns worker processes")
	}
	dir := t.TempDir()
	bin := buildProver(t)

	queue := filepath.Join(dir, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(queue, "requests"), 0o750))
	cfg := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(
		"version = \"t\"\n[execution]\nprover_mode = \"dev-mock\"\nrequests_root_dir = \""+queue+"\"\n"), 0o600))

	name := "10-11-getZkL2ExecutionProofV1.json"
	reqData, err := os.ReadFile(filepath.Join(fixtureDir, "request_single_block.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(queue, "requests", name), reqData, 0o600)) //nolint:gosec // G703: test-controlled path

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, flagConfig, cfg)
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
	//nolint:goconst
	err := run([]string{flagConfig, cfg})
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
	require.NoError(t, os.WriteFile(inPath, reqData, 0o600)) //nolint:gosec // G703: test-controlled path
	outPath := filepath.Join(dir, "response.json")

	require.NoError(t, run([]string{
		cmdProve, flagConfig, cfg, flagIn, inPath, flagOut, outPath,
	}))

	resp := readJSON(t, outPath)
	assert.Equal(t, "t-dev-mock", resp["proverVersion"])
	pi, ok := resp["publicInputs"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, pi, 18, "all 18 public-input fields present (placeholder zeros)")
	assert.Equal(t, resp["startBlockNumber"], pi["endBlockNumber"],
		"single-block request: endBlockNumber must match the request, not stay zero")
}

func TestRunProve_RequiresInOut(t *testing.T) {
	cfg := devMockConfig(t, t.TempDir())
	err := run([]string{cmdProve, flagConfig, cfg})
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

func buildProver(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "prover")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// proveOnlyConfig has no queue folder, which a "prover prove" child does not need.
func proveOnlyConfig(t *testing.T) string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte(
		"version = \"t\"\n[execution]\nprover_mode = \"dev-mock\"\n"), 0o600))
	return cfg
}

func TestIntegration_ProveStdinStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := buildProver(t)
	reqData, err := os.ReadFile(filepath.Join(fixtureDir, "request_single_block.json"))
	require.NoError(t, err)

	cmd := exec.Command(bin, cmdProve, flagConfig, proveOnlyConfig(t),
		"--type", "l2-execution", "--id", "job-1", flagIn, "-", flagOut, "-")
	cmd.Stdin = bytes.NewReader(reqData)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp), "stdout must be the response JSON only")
	assert.Equal(t, "t-dev-mock", resp["proverVersion"])
}

func TestIntegration_SubprocessCommand_DevMock(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary and spawns child processes")
	}
	cmd := subprocess.Command{Bin: buildProver(t), ConfigPath: proveOnlyConfig(t)}
	reqData, err := os.ReadFile(filepath.Join(fixtureDir, "request_single_block.json"))
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		res := cmd.Run(context.Background(),
			jobadapter.RunRequest{ID: "job-ok", Type: backend.ProofTypeL2Execution, Body: reqData})
		require.Equal(t, jobadapter.RunStatusSuccess, res.Status, "err: %v", res.Err)
		raw, ok := res.ResponseBody.(json.RawMessage)
		require.True(t, ok)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(raw, &resp))
		assert.Equal(t, "t-dev-mock", resp["proverVersion"])
	})

	t.Run("invalid request", func(t *testing.T) {
		res := cmd.Run(context.Background(),
			jobadapter.RunRequest{ID: "job-bad", Type: backend.ProofTypeL2Execution, Body: []byte(`{}`)})
		assert.Equal(t, jobadapter.RunStatusFailed, res.Status)
		assert.Equal(t, jobadapter.FailureCodeInvalidInput, res.FailureCode,
			"the child's own failure code must reach the parent")
	})
}

func TestRunProve_StdinRequiresType(t *testing.T) {
	err := run([]string{cmdProve, flagConfig, proveOnlyConfig(t), flagIn, "-", flagOut, "-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--type is required")
}

func TestRunProve_NoQueueFolder(t *testing.T) {
	dir := t.TempDir()
	reqData, err := os.ReadFile(filepath.Join(fixtureDir, "request_single_block.json"))
	require.NoError(t, err)
	inPath := filepath.Join(dir, "10-10-getZkL2ExecutionProofV1.json")
	require.NoError(t, os.WriteFile(inPath, reqData, 0o600)) //nolint:gosec // G703: test-controlled path
	outPath := filepath.Join(dir, "response.json")

	require.NoError(t, run([]string{cmdProve, flagConfig, proveOnlyConfig(t), flagIn, inPath, flagOut, outPath}))
	assert.Equal(t, "t-dev-mock", readJSON(t, outPath)["proverVersion"])
}

func TestRun_AdapterRequiresQueueFolder(t *testing.T) {
	err := run([]string{flagConfig, proveOnlyConfig(t)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requests_root_dir")
}

func TestRequestIdentity(t *testing.T) {
	tests := map[string]struct {
		in, typ, id string
		wantType    backend.ProofType
		wantID      string
		wantErr     string
	}{
		"from file name": {
			in: "/q/10-11-getZkRollupProofV1.json", wantType: backend.ProofTypeRollup, wantID: "10-11-getZkRollupProofV1",
		},
		"flags win over file name": {
			in: "/q/10-11-getZkRollupProofV1.json", typ: "rollup-aggregation", id: "x",
			wantType: backend.ProofTypeRollupAggregation, wantID: "x",
		},
		"stdin with type":    {in: "-", typ: "l2-execution", wantType: backend.ProofTypeL2Execution, wantID: "stdin"},
		"stdin without type": {in: "-", wantErr: "--type is required"},
		"unknown type":       {in: "-", typ: "blob", wantErr: "invalid --type"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotType, gotID, err := requestIdentity(tc.in, tc.typ, tc.id)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantType, gotType)
			assert.Equal(t, tc.wantID, gotID)
		})
	}
}
