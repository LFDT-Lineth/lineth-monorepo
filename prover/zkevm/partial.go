package zkevm

import (
	"sync"

	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
)

const (
	// Number of columns from the arithmetization that are kept to instantiate
	// light prover.
	numColLimitLight = 10
)

var (
	partialZkEvm     *ZkEvm
	oncePartialZkEvm = sync.Once{}

	partialCompilationSuite = compilationSuite{
		compiler.Arcane(1<<16, 1<<17, true),
		vortex.Compile(2, vortex.WithDryThreshold(16)),
	}
)

// Returns the zkEVM objects corresponding to the light zkEVM prover. Namely,
// it will generate a proof checking only a small portion of the requested
// computation it is meant primarily for testing and integration testing
// purpose. When called for the first time, it will compile the corresponding
// light zkEVM using the config option. The next times it is called, it will
// ignore the configuration options and directly return the previously compiled
// object. It therefore means that it should not be called twice with different
// config options.
func PartialZkEvm(tl *config.TracesLimits, cfg *config.Config) *ZkEvm {

	// This is hidden behind a once, because the compilation time can take a
	// significant amount of time and we want it to be only triggered when we
	// need it and only once (for instance not when we are using the full mode
	// prover).
	oncePartialZkEvm.Do(func() {

		// The light-prover does not support other features than the
		// arithmetization itself, i.e. it currently does not instantiate the
		// modules to verify keccak or the state-manager traces.
		settings := Settings{
			Arithmetization: arithmetization.Settings{
				Limits:                   tl,
				OptimisationLevel:        &mir.DEFAULT_OPTIMISATION_LEVEL,
				IgnoreCompatibilityCheck: &cfg.Execution.IgnoreCompatibilityCheck,
			},
			CompilationSuite: partialCompilationSuite,
			Metadata: wizard.VersionMetadata{
				Title:   "linea/evm-execution/partial",
				Version: "beta-v1",
			},
		}
		partialZkEvm = NewZkEVM(settings)
	})

	return partialZkEvm
}
