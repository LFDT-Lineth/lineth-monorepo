package main

import (
	"fmt"
	"os"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// sectionNames are the config sections of the proof types.
var sectionNames = map[backend.ProofType]string{
	backend.ProofTypeL2Execution:       "execution",
	backend.ProofTypeRollup:            "rollup",
	backend.ProofTypeRollupAggregation: "aggregation",
}

// checkPipeline checks the config of proof type t before any job runs: it
// rejects a config that would fail every job, and returns a warning for each
// setting the mode ignores.
func checkPipeline(t backend.ProofType, p pipeline) ([]string, error) {
	if !p.mode.Valid() {
		return nil, fmt.Errorf("invalid prover_mode %q for %s", p.mode, t)
	}
	runsGuest := p.mode == backend.ProverModeDevZkVM || p.mode == backend.ProverModeDevPartial
	// Rollup and aggregation have no guest input format yet.
	if runsGuest && t != backend.ProofTypeL2Execution {
		return nil, fmt.Errorf("prover_mode %q is not supported for %s yet: use dev-mock", p.mode, t)
	}
	if p.mode == backend.ProverModeDevZkVM && p.nativeRunnerBin == "" {
		return nil, fmt.Errorf("execution.native_runner_bin must be set in prover_mode %q", p.mode)
	}
	if p.nativeRunnerBin != "" {
		if err := checkExecutable(p.nativeRunnerBin); err != nil {
			return nil, fmt.Errorf("execution.native_runner_bin: %w", err)
		}
	}

	var warnings []string
	ignored := func(key string) {
		warnings = append(warnings, fmt.Sprintf("%s: %s is ignored in prover_mode %q", sectionNames[t], key, p.mode))
	}
	if p.shardSize > 0 && (p.mode == backend.ProverModeDevMock || p.mode == backend.ProverModeDevZkVM) {
		ignored("shard_size")
	}
	if p.nativeRunnerBin != "" && !runsGuest {
		ignored("native_runner_bin")
	}
	return warnings, nil
}

// checkExecutable checks that path is a file that can be run.
func checkExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a file", path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%q is not executable", path)
	}
	return nil
}
