package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runnerName is the file name of the stub native runner.
const runnerName = "l2_execution_runner"

// writeRunner writes a stub native runner with the given mode in a new folder
// and returns its path.
func writeRunner(t *testing.T, mode os.FileMode) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("execute bits and shell stubs are POSIX-only")
	}
	path := filepath.Join(t.TempDir(), runnerName)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), mode)) //nolint:gosec // G306: test stub
	return path
}

// TestCheckPipeline_Rejects checks the configs that would fail every job.
func TestCheckPipeline_Rejects(t *testing.T) {
	cases := map[string]struct {
		typ     backend.ProofType
		p       pipeline
		wantErr string
	}{
		"invalid mode": {
			backend.ProofTypeL2Execution, pipeline{mode: "bogus"}, "invalid prover_mode \"bogus\"",
		},
		"dev-zkvm for rollup": {
			backend.ProofTypeRollup, pipeline{mode: backend.ProverModeDevZkVM},
			"rollup.prover_mode \"dev-zkvm\" is not supported yet",
		},
		"dev-partial for rollup": {
			backend.ProofTypeRollup, pipeline{mode: backend.ProverModeDevPartial},
			"rollup.prover_mode \"dev-partial\" is not supported yet",
		},
		"dev-zkvm for aggregation": {
			backend.ProofTypeRollupAggregation, pipeline{mode: backend.ProverModeDevZkVM},
			"aggregation.prover_mode \"dev-zkvm\" is not supported yet",
		},
		"dev-partial for aggregation": {
			backend.ProofTypeRollupAggregation, pipeline{mode: backend.ProverModeDevPartial},
			"aggregation.prover_mode \"dev-partial\" is not supported yet",
		},
		"dev-zkvm without a native runner": {
			backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevZkVM},
			"execution.native_runner_bin must be set in prover_mode \"dev-zkvm\"",
		},
		"missing native runner": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevPartial, nativeRunnerBin: filepath.Join(t.TempDir(), "missing")},
			"no such file",
		},
		"native runner is a folder": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: t.TempDir()},
			"is a directory",
		},
		"native runner is not executable": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: writeRunner(t, 0o600)},
			"permission denied",
		},
		"native runner name is not on PATH": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: "no-such-runner-on-path"},
			"not found in $PATH",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := checkPipeline(tc.typ, tc.p)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestCheckPipeline_Accepts checks the configs that work, with no warning.
func TestCheckPipeline_Accepts(t *testing.T) {
	runner := writeRunner(t, 0o700)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(runner, link))
	// The runner is also found by its bare name, as exec finds it.
	t.Setenv("PATH", filepath.Dir(runner))

	cases := map[string]struct {
		typ backend.ProofType
		p   pipeline
	}{
		"execution in dev-mock":            {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevMock}},
		"dev-zkvm with a native runner":    {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: runner}},
		"native runner is a symlink":       {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: link}},
		"native runner is a name on PATH":  {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: runnerName}},
		"dev-partial without a runner":     {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevPartial}},
		"dev-partial with runner, sharded": {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevPartial, nativeRunnerBin: runner, shardSize: 500000}},
		"full, sharded":                    {backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeFull, shardSize: 500000}},
		"rollup in dev-mock":               {backend.ProofTypeRollup, pipeline{mode: backend.ProverModeDevMock}},
		"aggregation in dev-mock":          {backend.ProofTypeRollupAggregation, pipeline{mode: backend.ProverModeDevMock}},
		// full is not ready on any layer, but stays startable while it is built.
		"aggregation in full": {backend.ProofTypeRollupAggregation, pipeline{mode: backend.ProverModeFull}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			warnings, err := checkPipeline(tc.typ, tc.p)
			require.NoError(t, err)
			assert.Empty(t, warnings)
		})
	}
}

// TestCheckPipeline_Warns checks the settings a mode ignores. A mode that
// ignores the native runner does not check its path either.
func TestCheckPipeline_Warns(t *testing.T) {
	runner := writeRunner(t, 0o700)
	missing := filepath.Join(t.TempDir(), "missing")
	cases := map[string]struct {
		p    pipeline
		want string
	}{
		"shard_size in dev-mock": {
			pipeline{mode: backend.ProverModeDevMock, shardSize: 500000},
			"execution.shard_size is ignored in prover_mode \"dev-mock\"",
		},
		"shard_size in dev-zkvm": {
			pipeline{mode: backend.ProverModeDevZkVM, shardSize: 500000, nativeRunnerBin: runner},
			"execution.shard_size is ignored in prover_mode \"dev-zkvm\"",
		},
		"native runner in dev-mock": {
			pipeline{mode: backend.ProverModeDevMock, nativeRunnerBin: runner},
			"execution.native_runner_bin is ignored in prover_mode \"dev-mock\"",
		},
		"native runner in full": {
			pipeline{mode: backend.ProverModeFull, nativeRunnerBin: runner},
			"execution.native_runner_bin is ignored in prover_mode \"full\"",
		},
		"missing native runner in dev-mock": {
			pipeline{mode: backend.ProverModeDevMock, nativeRunnerBin: missing},
			"execution.native_runner_bin is ignored in prover_mode \"dev-mock\"",
		},
		"missing native runner in full": {
			pipeline{mode: backend.ProverModeFull, nativeRunnerBin: missing},
			"execution.native_runner_bin is ignored in prover_mode \"full\"",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			warnings, err := checkPipeline(backend.ProofTypeL2Execution, tc.p)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, warnings)
		})
	}
}

// TestModeTables_CoverEveryMode checks that every valid mode is in both
// tables, and only valid modes: a mode missing from one is rejected as invalid.
func TestModeTables_CoverEveryMode(t *testing.T) {
	modes := []backend.ProverMode{
		backend.ProverModeDevMock, backend.ProverModeDevZkVM, backend.ProverModeDevPartial, backend.ProverModeFull,
	}
	for mode := range modeLayers {
		assert.True(t, mode.Valid(), "%s is in modeLayers but is not a valid mode", mode)
	}
	for mode := range modeSettings {
		assert.True(t, mode.Valid(), "%s is in modeSettings but is not a valid mode", mode)
	}
	for _, mode := range modes {
		assert.Contains(t, modeLayers, mode)
		assert.Contains(t, modeSettings, mode)
	}
}

// rollupDevPartialConfig writes a config whose rollup pipeline is in a mode
// that is not ready for rollup.
func rollupDevPartialConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("version = \"t\"\n"+
		"[execution]\nprover_mode = \"dev-mock\"\nrequests_root_dir = \""+dir+"\"\n"+
		"[rollup]\nprover_mode = \"dev-partial\"\nrequests_root_dir = \""+dir+"\"\n"), 0o600))
	return path
}

// TestPipelineQueues checks that the queue adapter rejects at startup a
// pipeline in a mode that would fail every job, and returns the queues of a
// config that works.
func TestPipelineQueues(t *testing.T) {
	path := rollupDevPartialConfig(t)
	cfg, err := config.NewConfigFromFile(path)
	require.NoError(t, err)
	_, err = pipelineQueues(cfg, path)
	require.ErrorContains(t, err, "invalid config")
	require.ErrorContains(t, err, "rollup.prover_mode \"dev-partial\" is not supported yet")

	cfg.Rollup.ProverMode = string(backend.ProverModeDevMock)
	queues, err := pipelineQueues(cfg, path)
	require.NoError(t, err)
	assert.Len(t, queues, 2, "execution and rollup are configured")
}

// TestRunProve_RejectsDevModeForRollup checks that a one-shot prove rejects
// the same config before it runs the job.
func TestRunProve_RejectsDevModeForRollup(t *testing.T) {
	path := rollupDevPartialConfig(t)
	in := filepath.Join(t.TempDir(), "1-1-getZkRollupProofV1.json")
	require.NoError(t, os.WriteFile(in, []byte("{}"), 0o600))

	err := run([]string{cmdProve, flagConfig, path, flagIn, in, flagOut, filepath.Join(t.TempDir(), "out.json")})
	require.ErrorContains(t, err, "invalid config")
	require.ErrorContains(t, err, "rollup.prover_mode \"dev-partial\" is not supported yet")
}
