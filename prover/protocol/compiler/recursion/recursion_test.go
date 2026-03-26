package recursion

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitextension"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {

	// logrus.SetLevel(logrus.FatalLevel)

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
			splitextension.CompileSplitExtToBase,
			vortex.Compile(
				2,
				false,
				vortex.ForceNumOpenedColumns(4),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.PremarkAsSelfRecursed(),
				vortex.WithOptionalSISHashingThreshold(64),
			),
		},
		{
			cleanup.CleanUp,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1 << 13),
			),
			vortex.Compile(
				8,
				false,
				vortex.ForceNumOpenedColumns(32),
				vortex.WithSISParams(&ringsis.StdParams),
				vortex.WithOptionalSISHashingThreshold(64),
			),
			selfrecursion.SelfRecurse,
			cleanup.CleanUp,
			poseidon2.CompilePoseidon2,
			compiler.Arcane(
				compiler.WithTargetColSize(1 << 13),
			),
			vortex.Compile(
				8,
				false,
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
					MaxNumProof: 1,
				})
			}

			comp2 := wizard.Compile(define2, dummy.CompileAtProverLvl())

			proverRuntime := wizard.RunProverUntilRound(comp1, prove1, recCtx.GetStoppingRound()+1, false)
			witness1 := ExtractWitness(proverRuntime)

			prove2 := func(run *wizard.ProverRuntime) {
				recCtx.Assign(run, []Witness{witness1}, nil)
			}

			proof2 := wizard.Prove(comp2, prove2)
			assert.NoErrorf(t, wizard.Verify(comp2, proof2), "invalid proof")
		})
	}
}

// TestWithMaxNumProofNotFilled tests recursion when MaxNumProof > 1 but
// fewer proofs are actually provided. This tests the padding/dummy proof logic.
func TestWithMaxNumProofNotFilled(t *testing.T) {

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

	suite := []func(*wizard.CompiledIOP){
		logderivativesum.CompileLookups,
		localcs.Compile,
		globalcs.Compile,
		univariates.Naturalize,
		mpts.Compile(),
		splitextension.CompileSplitExtToBase,
		vortex.Compile(
			2,
			false,
			vortex.ForceNumOpenedColumns(4),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.PremarkAsSelfRecursed(),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	}

	comp1 := wizard.Compile(define1, suite...)

	var recCtx *Recursion

	// MaxNumProof = 2, but we will only provide 1 witness
	define2 := func(build2 *wizard.Builder) {
		recCtx = DefineRecursionOf(build2.CompiledIOP, comp1, Parameters{
			Name:        "test",
			MaxNumProof: 2,
		})
	}

	comp2 := wizard.Compile(define2, dummy.CompileAtProverLvl())

	proverRuntime := wizard.RunProverUntilRound(comp1, prove1, recCtx.GetStoppingRound()+1, false)
	witness1 := ExtractWitness(proverRuntime)

	// Only provide 1 witness even though MaxNumProof = 2
	prove2 := func(run *wizard.ProverRuntime) {
		recCtx.Assign(run, []Witness{witness1}, nil)
	}

	proof2 := wizard.Prove(comp2, prove2)
	assert.NoErrorf(t, wizard.Verify(comp2, proof2), "invalid proof")
}

// TestWithMaxNumProofFilled tests recursion when MaxNumProof > 1 and
// all proof slots are filled.
func TestWithMaxNumProofFilled(t *testing.T) {

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

	suite := []func(*wizard.CompiledIOP){
		logderivativesum.CompileLookups,
		localcs.Compile,
		globalcs.Compile,
		univariates.Naturalize,
		mpts.Compile(),
		splitextension.CompileSplitExtToBase,
		vortex.Compile(
			2,
			false,
			vortex.ForceNumOpenedColumns(4),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.PremarkAsSelfRecursed(),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	}

	comp1 := wizard.Compile(define1, suite...)

	var recCtx *Recursion

	// MaxNumProof = 2, and we will provide 2 witnesses
	define2 := func(build2 *wizard.Builder) {
		recCtx = DefineRecursionOf(build2.CompiledIOP, comp1, Parameters{
			Name:        "test",
			MaxNumProof: 2,
		})
	}

	comp2 := wizard.Compile(define2, dummy.CompileAtProverLvl())

	// Generate two witnesses
	proverRuntime1 := wizard.RunProverUntilRound(comp1, prove1, recCtx.GetStoppingRound()+1, false)
	witness1 := ExtractWitness(proverRuntime1)

	proverRuntime2 := wizard.RunProverUntilRound(comp1, prove1, recCtx.GetStoppingRound()+1, false)
	witness2 := ExtractWitness(proverRuntime2)

	// Provide both witnesses
	prove2 := func(run *wizard.ProverRuntime) {
		recCtx.Assign(run, []Witness{witness1, witness2}, nil)
	}

	proof2 := wizard.Prove(comp2, prove2)
	assert.NoErrorf(t, wizard.Verify(comp2, proof2), "invalid proof")
}

// declarePiColumn declares a column of size 1 and links it to a public input.
// This follows the pattern used in the distributed package.
func declarePiColumn(comp *wizard.CompiledIOP, name string) {
	col := comp.InsertProof(0, ifaces.ColID(name+"_PI_COLUMN"), 1, true)
	comp.InsertPublicInput(name, accessors.NewFromPublicColumn(col, 0))
}

// assignPiColumn assigns the column backing a public input.
func assignPiColumn(run *wizard.ProverRuntime, name string, val field.Element) {
	run.AssignColumn(
		ifaces.ColID(name+"_PI_COLUMN"),
		smartvectors.NewRegular([]field.Element{val}),
	)
}

// TestWithPublicInputs tests recursion with non-empty public inputs.
func TestWithPublicInputs(t *testing.T) {

	pubValue1 := field.NewElement(12345)
	pubValue2 := field.NewElement(67890)

	define1 := func(bui *wizard.Builder) {
		var (
			a = bui.RegisterCommit("A", 1024)
			b = bui.RegisterCommit("B", 1024)
		)

		// Register public inputs with column-backed accessors (following distributed package pattern)
		declarePiColumn(bui.CompiledIOP, "PUB1")
		declarePiColumn(bui.CompiledIOP, "PUB2")

		bui.Inclusion("Q", []ifaces.Column{a}, []ifaces.Column{b})
	}

	prove1 := func(run *wizard.ProverRuntime) {
		run.AssignColumn("A", smartvectors.NewConstant(field.Zero(), 1024))
		run.AssignColumn("B", smartvectors.NewConstant(field.Zero(), 1024))

		// Assign public input columns
		assignPiColumn(run, "PUB1", pubValue1)
		assignPiColumn(run, "PUB2", pubValue2)
	}

	suite := []func(*wizard.CompiledIOP){
		logderivativesum.CompileLookups,
		localcs.Compile,
		globalcs.Compile,
		univariates.Naturalize,
		mpts.Compile(),
		splitextension.CompileSplitExtToBase,
		vortex.Compile(
			2,
			false,
			vortex.ForceNumOpenedColumns(4),
			vortex.WithSISParams(&ringsis.StdParams),
			vortex.PremarkAsSelfRecursed(),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	}

	comp1 := wizard.Compile(define1, suite...)

	// Verify public inputs are registered correctly
	assert.Equal(t, 2, len(comp1.PublicInputs), "expected 2 public inputs")

	var recCtx *Recursion

	define2 := func(build2 *wizard.Builder) {
		recCtx = DefineRecursionOf(build2.CompiledIOP, comp1, Parameters{
			Name:        "test",
			MaxNumProof: 1,
		})
	}

	comp2 := wizard.Compile(define2, dummy.CompileAtProverLvl())

	proverRuntime := wizard.RunProverUntilRound(comp1, prove1, recCtx.GetStoppingRound()+1, false)
	witness1 := ExtractWitness(proverRuntime)

	prove2 := func(run *wizard.ProverRuntime) {
		recCtx.Assign(run, []Witness{witness1}, nil)
	}

	proof2 := wizard.Prove(comp2, prove2)
	assert.NoErrorf(t, wizard.Verify(comp2, proof2), "invalid proof")
}
