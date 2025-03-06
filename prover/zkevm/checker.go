package zkevm

import (
	"sync"

	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
)

var (
	checkerZkEvm     *ZkEvm
	onceCheckerZkEvm = sync.Once{}

	checkerCompilationSuite = compilationSuite{
		// The dummy compiler returns the witness as the proof and manually
		// checks it as the verifier. It is essentially the trivial proof
		// system.
		dummy.Compile,
	}
)

// The checker is not meant to generate proofs, it is meant to be used to check
// that the provided prover inputs are correct. It typically is used to audit
// the traces of the arithmetization. Currently, it does not include the keccaks
// nor does it include the state-management checks.
func CheckerZkEvm(tl *config.TracesLimits, cfg *config.Config) *ZkEvm {
	onceCheckerZkEvm.Do(func() {
		settings := Settings{
			Arithmetization: arithmetization.Settings{
				Limits:                   tl,
				OptimisationLevel:        &mir.DEFAULT_OPTIMISATION_LEVEL,
				IgnoreCompatibilityCheck: &cfg.Execution.IgnoreCompatibilityCheck,
			},
			CompilationSuite: checkerCompilationSuite,
			Metadata: wizard.VersionMetadata{
				Title:   "linea/evm-execution/checker",
				Version: "beta-v1",
			},
		}
		checkerZkEvm = NewZkEVM(settings)
	})
	return checkerZkEvm
}
