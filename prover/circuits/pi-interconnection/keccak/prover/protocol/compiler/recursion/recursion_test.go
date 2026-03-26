package recursion

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	define1 := func(bui *wizard.Builder) {

		var (
			a = bui.RegisterCommit("A", 1024)
			b = bui.RegisterCommit("B", 1024)
		)

		bui.Inclusion("Q", []ifaces.Column{a}, []ifaces.Column{b})
	}

	prove1 := func(run *wizard.ProverRuntime) {
		run.AssignColumn("A", smartvectors.NewConstant(field.Zero(), 1024))
		run.AssignColumn("B", smartvectors.NewConstant(field.Zero(), 1024))
	}

	suites := [][]func(*wizard.CompiledIOP){
		{
			logderivativesum.CompileLookups,
			localcs.Compile,
			globalcs.Compile,
			univariates.Naturalize,
			mpts.Compile(),
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(4),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.PremarkAsSelfRecursed(),
				vortex.WithOptionalSISHashingThreshold(64),
			),
		},
		{
			cleanup.CleanUp,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1 << 13),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(32),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.WithOptionalSISHashingThreshold(64),
			),
			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			mimc.CompileMiMC,
			compiler.Arcane(
				compiler.WithTargetColSize(1 << 13),
			),
			vortex.Compile(
				8,
				vortex.ForceNumOpenedColumns(32),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.WithOptionalSISHashingThreshold(64),
				vortex.PremarkAsSelfRecursed(),
			),
		},
	}

	for i, s := range suites {

		t.Run(fmt.Sprintf("case-%v", i), func(t *testing.T) {

			comp1 := wizard.Compile(define1, s...)
			var recCtx *Recursion

			define2 := func(build2 *wizard.Builder) {
				recCtx = DefineRecursionOf(build2.CompiledIOP, comp1, Parameters{
					Name:        "test",
					WithoutGkr:  true,
					MaxNumProof: 1,
				})
			}

			comp2 := wizard.Compile(define2, dummy.CompileAtProverLvl())

			proverRuntime := wizard.RunProverUntilRound(comp1, prove1, recCtx.GetStoppingRound()+1)
			witness1 := ExtractWitness(proverRuntime)

			prove2 := func(run *wizard.ProverRuntime) {
				recCtx.Assign(run, []Witness{witness1}, nil)
			}

			proof2 := wizard.Prove(comp2, prove2)
			assert.NoErrorf(t, wizard.Verify(comp2, proof2), "invalid proof")
		})
	}
}
