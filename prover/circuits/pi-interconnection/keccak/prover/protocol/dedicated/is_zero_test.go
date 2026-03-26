package dedicated

import (
	"fmt"
	"testing"

	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/require"
)

func TestIsZero(t *testing.T) {

	inputVectors := []sv.SmartVector{
		sv.NewConstant(field.Zero(), 8),
		sv.NewConstant(field.One(), 8),
		sv.ForTest(0, 1, 2, 3, 0, 0, 4, 4),
		sv.ForTest(0, 0, 0, 0, 0, 0, 0, 0),
		sv.ForTest(12, 13, 14, 15, 12, 13, 14, 15),
		sv.NewRotated(sv.Regular(vector.ForTest(0, 0, 2, 2, 0, 0, 2, 2)), 0),
		sv.NewRotated(sv.Regular(vector.ForTest(0, 0, 2, 2, 0, 0, 2, 2)), 1),
		sv.NewRotated(sv.Regular(vector.ForTest(1, 1, 2, 2, 1, 1, 2, 2)), 0),
		sv.NewRotated(sv.Regular(vector.ForTest(3, 3, 2, 2, 3, 3, 2, 2)), 1),
		sv.NewRotated(sv.Regular(vector.ForTest(0, 0, 0, 0, 0, 0, 0, 0)), 0),
		sv.NewRotated(sv.Regular(vector.ForTest(0, 0, 0, 0, 0, 0, 0, 0)), 1),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.Zero(), 0, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.Zero(), 0, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.Zero(), 0, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.NewElement(42), 0, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.NewElement(42), 0, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.NewElement(42), 0, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.Zero(), 2, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.Zero(), 2, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.Zero(), 2, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.NewElement(42), 2, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.NewElement(42), 2, 8),
		sv.NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.NewElement(42), 2, 8),
	}

	masks := []sv.SmartVector{
		nil, // nil = no mask
		sv.NewConstant(field.Zero(), 8),
		sv.NewConstant(field.One(), 8),
		sv.ForTest(0, 0, 0, 0, 1, 1, 1, 1),
		sv.ForTest(0, 1, 0, 1, 0, 1, 0, 1),
	}

	runIsZeroTest := func(t *testing.T, inpVec, mask sv.SmartVector, withExpr bool) {

		var (
			izc *IsZeroCtx
		)

		define := func(b *wizard.Builder) {
			var (
				c any = b.RegisterCommit("C", 8)
				m ifaces.Column
			)

			if mask != nil {
				m = b.RegisterPrecomputed("M", mask)
			}

			if withExpr {
				c = sym.Add(c, sym.Mul(c, c))
			}

			if mask == nil {
				izc = IsZero(b.CompiledIOP, c)
			}

			if mask != nil {
				izc = IsZeroMask(b.CompiledIOP, c, m)
			}
		}

		prover := func(run *wizard.ProverRuntime) {
			run.AssignColumn("C", inpVec)

			izc.Run(run)

			// Sanity-check that IsZero is properly assigned
			iszero := izc.IsZero.GetColAssignment(run)

			for k := 0; k < izc.IsZero.Size(); k++ {
				var (
					z = iszero.Get(k)
					c = inpVec.Get(k)
					m = true
				)

				if mask != nil {
					m = mask.Get(k) == field.One()
				}

				switch {
				case !m || !c.IsZero():
					require.Equalf(t, uint64(0), z.Uint64(), "row #%v", k)
				case c.IsZero() && m:
					require.Equalf(t, uint64(1), z.Uint64(), "row #%v", k)
				}
			}
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prover)
		if err := wizard.Verify(comp, proof); err != nil {
			t.Fatalf("verifier did not accept: %v", err.Error())
		}
	}

	for ivID, inpVec := range inputVectors {
		for mID, mask := range masks {

			t.Run(fmt.Sprintf("testcase-%v-mask-%v", ivID, mID), func(t *testing.T) {
				runIsZeroTest(t, inpVec, mask, false)
			})

			t.Run(fmt.Sprintf("testcase-%v-mask-%v-with-expression", ivID, mID), func(t *testing.T) {
				runIsZeroTest(t, inpVec, mask, false)
			})
		}
	}
}
