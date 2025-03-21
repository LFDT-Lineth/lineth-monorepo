package globalcs_test

import (
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/lookup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/specialqueries"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitter"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitter/sticker"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
)

// BenchmarkGlobalConstraint benchmarks the global constraints compiler against the
// actual zk-evm constraint system.
func BenchmarkGlobalConstraintWoArtefacts(b *testing.B) {

	// partialSuite corresponds to the actual compilation suite of
	// the full zk-evm down to the point where the global constraints
	// are compiled.
	partialSuite := []func(*wizard.CompiledIOP){
		mimc.CompileMiMC,
		specialqueries.RangeProof,
		specialqueries.CompileFixedPermutations,
		permutation.CompileGrandProduct,
		lookup.CompileLogDerivative,
		innerproduct.Compile,
		sticker.Sticker(1<<10, 1<<19),
		splitter.SplitColumns(1 << 19),
		cleanup.CleanUp,
		localcs.Compile,
		globalcs.Compile,
	}

	// In order to load the config we need to position ourselves in the root
	// folder.
	_ = os.Chdir("../../..")
	defer os.Chdir("protocol/compiler/globalcs")

	// config corresponds to the config we use on sepolia
	cfg, cfgErr := config.NewConfigFromFile("./config/config-sepolia-full.toml")
	if cfgErr != nil {
		b.Fatalf("could not find the config: %v", cfgErr)
	}

	// Shut the logger to not overwhelm the benchmark output
	logrus.SetLevel(logrus.PanicLevel)

	b.ResetTimer()

	for c_ := 0; c_ < b.N; c_++ {

		b.StopTimer()

		// Removes the artefacts to
		if err := os.RemoveAll("/tmp/prover-artefacts"); err != nil {
			b.Fatalf("could not remove the artefacts: %v", err)
		}

		b.StartTimer()

		_ = zkevm.FullZKEVMWithSuite(&cfg.TracesLimits, partialSuite, cfg)

	}

}
