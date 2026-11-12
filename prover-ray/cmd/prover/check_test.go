package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile writes a file with the given mode in a new folder and returns its
// path.
func writeFile(t *testing.T, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "l2_execution_runner")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), mode)) //nolint:gosec // G306: test stub
	return path
}

// TestCheckPipeline_Rejects checks the configs that would fail every job.
func TestCheckPipeline_Rejects(t *testing.T) {
	runner := writeFile(t, 0o700)
	cases := map[string]struct {
		typ     backend.ProofType
		p       pipeline
		wantErr string
	}{
		"invalid mode": {
			backend.ProofTypeL2Execution, pipeline{mode: "bogus"}, "invalid prover_mode",
		},
		"dev-zkvm for rollup": {
			backend.ProofTypeRollup, pipeline{mode: backend.ProverModeDevZkVM}, "not supported for rollup",
		},
		"dev-partial for aggregation": {
			backend.ProofTypeRollupAggregation, pipeline{mode: backend.ProverModeDevPartial},
			"not supported for rollup-aggregation",
		},
		"dev-zkvm without a native runner": {
			backend.ProofTypeL2Execution, pipeline{mode: backend.ProverModeDevZkVM}, "native_runner_bin must be set",
		},
		"missing native runner": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevPartial, nativeRunnerBin: filepath.Join(t.TempDir(), "missing")},
			"native_runner_bin",
		},
		"native runner is a folder": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: t.TempDir()},
			"not a file",
		},
		"native runner is not executable": {
			backend.ProofTypeL2Execution,
			pipeline{mode: backend.ProverModeDevZkVM, nativeRunnerBin: writeFile(t, 0o600)},
			"not executable",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := checkPipeline(tc.typ, tc.p)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("valid", func(t *testing.T) {
		for typ, p := range map[backend.ProofType]pipeline{
			backend.ProofTypeL2Execution:       {mode: backend.ProverModeDevZkVM, nativeRunnerBin: runner},
			backend.ProofTypeRollup:            {mode: backend.ProverModeDevMock},
			backend.ProofTypeRollupAggregation: {mode: backend.ProverModeFull},
		} {
			warnings, err := checkPipeline(typ, p)
			require.NoError(t, err, "%s", typ)
			assert.Empty(t, warnings, "%s", typ)
		}
	})
}

// TestCheckPipeline_Warns checks the settings a mode ignores.
func TestCheckPipeline_Warns(t *testing.T) {
	runner := writeFile(t, 0o700)
	cases := map[string]struct {
		p    pipeline
		want string
	}{
		"shard_size in dev-mock": {
			pipeline{mode: backend.ProverModeDevMock, shardSize: 500000}, "shard_size is ignored in prover_mode \"dev-mock\"",
		},
		"shard_size in dev-zkvm": {
			pipeline{mode: backend.ProverModeDevZkVM, shardSize: 500000, nativeRunnerBin: runner},
			"shard_size is ignored in prover_mode \"dev-zkvm\"",
		},
		"native runner in dev-mock": {
			pipeline{mode: backend.ProverModeDevMock, nativeRunnerBin: runner},
			"native_runner_bin is ignored in prover_mode \"dev-mock\"",
		},
		"native runner in full": {
			pipeline{mode: backend.ProverModeFull, nativeRunnerBin: runner},
			"native_runner_bin is ignored in prover_mode \"full\"",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			warnings, err := checkPipeline(backend.ProofTypeL2Execution, tc.p)
			require.NoError(t, err)
			assert.Equal(t, []string{"execution: " + tc.want}, warnings)
		})
	}

	t.Run("used settings", func(t *testing.T) {
		for _, p := range []pipeline{
			{mode: backend.ProverModeDevPartial, shardSize: 500000, nativeRunnerBin: runner},
			{mode: backend.ProverModeFull, shardSize: 500000},
		} {
			warnings, err := checkPipeline(backend.ProofTypeL2Execution, p)
			require.NoError(t, err)
			assert.Empty(t, warnings, "%s", p.mode)
		}
	})
}

// TestRun_RejectsDevModeForRollup checks that the queue adapter rejects at
// startup a rollup pipeline in a mode that would fail every job.
func TestRun_RejectsDevModeForRollup(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(cfg, []byte("version = \"t\"\n"+
		"[execution]\nprover_mode = \"dev-mock\"\nrequests_root_dir = \""+dir+"\"\n"+
		"[rollup]\nprover_mode = \"dev-partial\"\nrequests_root_dir = \""+dir+"\"\n"), 0o600))

	err := run([]string{flagConfig, cfg})
	assert.ErrorContains(t, err, "not supported for rollup")
}
