package main

import (
	"fmt"
	"os/exec"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// use says how a prover mode treats a config setting.
type use int

const (
	// ignored: the mode never reads the setting. Setting it is a warning.
	ignored use = iota
	// optional: the mode uses the setting when it is set.
	optional
	// required: the mode does not work without the setting.
	required
)

// layers says on which layers the prover may start in a mode. A layer is false
// when every job of that layer would fail in the mode.
type layers struct{ execution, rollup, aggregation bool }

// modeLayers says on which layers each mode may run:
//
//	mode                 execution  rollup  aggregation
//	dev-mock             true       true    true
//	dev-zkvm             true       false   false
//	dev-partial          true       false   false
//	full                 true       true    true
//	dev-full-dummy-wrap  false      false   true         (planned)
//
// It follows the "Status per layer" table of docs/prover-modes.md: rollup and
// aggregation have no guest input format yet, so dev-zkvm and dev-partial
// cannot run there. full is not ready on any layer, but stays allowed while it
// is being built.
var modeLayers = map[backend.ProverMode]layers{
	backend.ProverModeDevMock:    {execution: true, rollup: true, aggregation: true},
	backend.ProverModeDevZkVM:    {execution: true, rollup: false, aggregation: false},
	backend.ProverModeDevPartial: {execution: true, rollup: false, aggregation: false},
	backend.ProverModeFull:       {execution: true, rollup: true, aggregation: true},
	// Planned, not a mode yet: real proofs, but a dummy BN254 proof for L1.
	// backend.ProverModeDevFullDummyWrap: {execution: false, rollup: false, aggregation: true},
}

// allows reports whether the prover may start for proof type t.
func (l layers) allows(t backend.ProofType) bool {
	switch t {
	case backend.ProofTypeL2Execution:
		return l.execution
	case backend.ProofTypeRollup:
		return l.rollup
	case backend.ProofTypeRollupAggregation:
		return l.aggregation
	}
	return false
}

// settings says how a mode treats each [execution] setting.
type settings struct{ nativeRunner, shardSize use }

// modeSettings says how each mode treats the [execution] settings:
//
//	mode                 native_runner_bin  shard_size
//	dev-mock             ignored            ignored
//	dev-zkvm             required           ignored
//	dev-partial          optional           optional
//	full                 ignored            optional
//	dev-full-dummy-wrap  -                  -           (planned, aggregation only)
var modeSettings = map[backend.ProverMode]settings{
	backend.ProverModeDevMock:    {nativeRunner: ignored, shardSize: ignored},
	backend.ProverModeDevZkVM:    {nativeRunner: required, shardSize: ignored},
	backend.ProverModeDevPartial: {nativeRunner: optional, shardSize: optional},
	backend.ProverModeFull:       {nativeRunner: ignored, shardSize: optional},
	// Planned, not a mode yet. It runs on aggregation only, so the
	// [execution] settings do not apply to it.
	// backend.ProverModeDevFullDummyWrap: {nativeRunner: ignored, shardSize: ignored},
}

// sectionNames are the config sections of the proof types.
var sectionNames = map[backend.ProofType]string{
	backend.ProofTypeL2Execution:       "execution",
	backend.ProofTypeRollup:            "rollup",
	backend.ProofTypeRollupAggregation: "aggregation",
}

// checkPipeline checks the config of proof type t before any job runs. An
// error is a config that would fail every job. A warning is a setting that is
// set but ignored by the mode.
func checkPipeline(t backend.ProofType, p pipeline) (warnings []string, err error) {
	// 1. The mode must exist.
	allowed, knownLayers := modeLayers[p.mode]
	uses, knownSettings := modeSettings[p.mode]
	if !p.mode.Valid() || !knownLayers || !knownSettings {
		return nil, fmt.Errorf("invalid prover_mode %q for %s", p.mode, t)
	}

	// 2. The mode must be allowed on this layer (modeLayers).
	if !allowed.allows(t) {
		return nil, fmt.Errorf("%s.prover_mode %q is not supported yet: use dev-mock", sectionNames[t], p.mode)
	}

	// 3. The native runner must be set when required (modeSettings). When the
	// mode uses it, it must be a program that can be run: a path, or a name
	// found on PATH.
	runnerSet := p.nativeRunnerBin != ""
	if uses.nativeRunner == required && !runnerSet {
		return nil, fmt.Errorf("execution.native_runner_bin must be set in prover_mode %q", p.mode)
	}
	if uses.nativeRunner != ignored && runnerSet {
		// The same lookup as the one that starts the runner.
		if _, err := exec.LookPath(p.nativeRunnerBin); err != nil {
			return nil, fmt.Errorf("execution.native_runner_bin: %w", err)
		}
	}

	// 4. Warn about each setting that is set but ignored. Only [execution]
	// has these settings.
	warn := func(key string) {
		warnings = append(warnings, fmt.Sprintf("execution.%s is ignored in prover_mode %q", key, p.mode))
	}
	if uses.shardSize == ignored && p.shardSize > 0 {
		warn("shard_size")
	}
	if uses.nativeRunner == ignored && runnerSet {
		warn("native_runner_bin")
	}
	return warnings, nil
}
