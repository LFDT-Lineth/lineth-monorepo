//go:build !fuzzlight

package fullrecursion_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/fullrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/lookup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/specialqueries"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitter"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitter/sticker"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
)

func TestLookup(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	define := func(bui *wizard.Builder) {

		var (
			a = bui.RegisterCommit("A", 8)
			b = bui.RegisterCommit("B", 8)
		)

		bui.Inclusion("Q", []ifaces.Column{a}, []ifaces.Column{b})
	}

	prove := func(run *wizard.ProverRuntime) {
		run.AssignColumn("A", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8))
		run.AssignColumn("B", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8))
	}

	suites := [][]func(*wizard.CompiledIOP){
		{
			lookup.CompileLogDerivative,
			localcs.Compile,
			globalcs.Compile,
			univariates.CompileLocalOpening,
			univariates.Naturalize,
			univariates.MultiPointToSinglePoint(8),
			vortex.Compile(2, vortex.ForceNumOpenedColumns(4), vortex.WithSISParams(&ringsis.StdParams)),
			fullrecursion.FullRecursion(true),
			dummy.CompileAtProverLvl,
		},
		{
			lookup.CompileLogDerivative,
			localcs.Compile,
			globalcs.Compile,
			univariates.CompileLocalOpening,
			univariates.Naturalize,
			univariates.MultiPointToSinglePoint(8),
			vortex.Compile(2, vortex.ForceNumOpenedColumns(4), vortex.WithSISParams(&ringsis.StdParams)),
			fullrecursion.FullRecursion(true),
			mimc.CompileMiMC,
			specialqueries.RangeProof,
			lookup.CompileLogDerivative,
			specialqueries.CompileFixedPermutations,
			permutation.CompileGrandProduct,
			innerproduct.Compile,
			sticker.Sticker(1<<8, 1<<16),
			splitter.SplitColumns(1 << 16),
			localcs.Compile,
			globalcs.Compile,
			univariates.CompileLocalOpening,
			univariates.Naturalize,
			univariates.MultiPointToSinglePoint(1 << 16),
			vortex.Compile(2, vortex.ForceNumOpenedColumns(4), vortex.WithSISParams(&ringsis.StdParams)),
			fullrecursion.FullRecursion(true),
			dummy.CompileAtProverLvl,
		},
	}

	for i, s := range suites {

		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {

			comp := wizard.Compile(
				define,
				s...,
			)

			proof := wizard.Prove(comp, prove)

			if err := wizard.Verify(comp, proof); err != nil {
				t.Fatalf("verifier failed: %v", err)
			}
		})

	}
}
