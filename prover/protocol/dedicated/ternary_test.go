package dedicated

import (
	"fmt"
	"testing"

	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

// TestTernary checks [Ternary] over constant, full and padded-window inputs.
// Full vectors and end-anchored windows cover the case where the active range
// reaches the last row, which previously read past the end of the column.
func TestTernary(t *testing.T) {

	const size = 8

	boolShapes := []sv.SmartVector{
		sv.NewConstant(field.Zero(), size),
		sv.NewConstant(field.One(), size),
		sv.ForTest(0, 1, 0, 1, 0, 1, 0, 1),
		sv.ForTest(1, 1, 1, 1, 1, 1, 1, 1),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 0, 1, 0), field.Zero(), 0, size),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 0, 1, 0), field.Zero(), 2, size),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 0, 1, 0), field.Zero(), 4, size),
	}

	valShapes := []sv.SmartVector{
		sv.NewConstant(field.NewElement(7), size),
		sv.ForTest(10, 11, 12, 13, 14, 15, 16, 17),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 2, 3, 4), field.NewElement(42), 0, size),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 2, 3, 4), field.NewElement(42), 4, size),
	}

	runCase := func(t *testing.T, cond, ifTrue, ifFalse sv.SmartVector) {

		var ctx *TernaryCtx

		define := func(b *wizard.Builder) {
			c := b.RegisterCommit("COND", size)
			tCol := b.RegisterCommit("IF_TRUE", size)
			fCol := b.RegisterCommit("IF_FALSE", size)
			ctx = Ternary(b.CompiledIOP, c, tCol, fCol)
		}

		prover := func(run *wizard.ProverRuntime) {
			run.AssignColumn("COND", cond)
			run.AssignColumn("IF_TRUE", ifTrue)
			run.AssignColumn("IF_FALSE", ifFalse)

			ctx.Run(run)

			res := ctx.Result.GetColAssignment(run)
			for k := 0; k < size; k++ {
				want := ifFalse.Get(k)
				c := cond.Get(k)
				if !c.IsZero() {
					want = ifTrue.Get(k)
				}
				got := res.Get(k)
				require.Truef(t, want.Equal(&got), "row #%v: want %v got %v", k, want.String(), got.String())
			}
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prover)
		if err := wizard.Verify(comp, proof); err != nil {
			t.Fatalf("verifier did not accept: %v", err.Error())
		}
	}

	for ci, cond := range boolShapes {
		for ti, ifTrue := range valShapes {
			for fi, ifFalse := range valShapes {
				t.Run(fmt.Sprintf("cond-%v-true-%v-false-%v", ci, ti, fi), func(t *testing.T) {
					runCase(t, cond, ifTrue, ifFalse)
				})
			}
		}
	}
}
