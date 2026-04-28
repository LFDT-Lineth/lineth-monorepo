package zkcdriver_test

import (
	"testing"

	"github.com/consensys/go-corset/pkg/ir/mir"
	"github.com/consensys/linea-monorepo/prover-ray/utils/files"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/zkcdriver"
)

func TestZkEVMDefinition(t *testing.T) {
	// @alex: I checked that it worked with Ranges and Interleavings. I will
	// leave it as skipped for now but we will need to eventually.
	t.Skipf("you will need to relax the non-support for range-checks in ray")

	sys := wiop.NewSystemf("zkevm-test")
	sys.NewRound()
	_ = zkcdriver.NewZkCDriver(
		sys,
		zkcdriver.Settings{OptimisationLevel: &mir.DEFAULT_OPTIMISATION_LEVEL},
		files.MustRead("./testdata/zkevm.bin"),
	)
}
