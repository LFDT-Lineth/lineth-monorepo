package gkrtable

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/stretchr/testify/require"
)

// TestConstrainSum checks that a system with cells holding λ, y and R = Σ_c λᶜ y_c proves and
// verifies, and that it does not if a y_c or R is changed.
func TestConstrainSum(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	random := func() (e field.Ext) {
		var coordinates [6]field.Element
		for k := range coordinates {
			coordinates[k].SetUint64(uint64(rng.Uint32()))
		}
		e.SetBase(coordinates[:])
		return
	}
	lambda := random()
	var y [nbColumns]field.Ext
	for c := range y {
		y[c] = random()
	}
	var result field.Ext
	for c := nbColumns - 1; c >= 0; c-- {
		result.Mul(&result, &lambda)
		result.Add(&result, &y[c])
	}

	// prove returns the result of the verification of a system whose cells hold the values.
	prove := func(y [nbColumns]field.Ext, result field.Ext) error {
		sys := wiop.NewSystemf("gkrtable-sum")
		round := sys.NewRound()
		module := sys.NewSizedModule(sys.Context.Childf("module"), 2, wiop.PaddingDirectionNone)

		cells := func(name string) (c [6]*wiop.Cell) {
			for k := range c {
				c[k] = round.NewCell(sys.Context.Childf("%s-%d", name, k), false)
			}
			return
		}
		lambdaCells, resultCells := cells("lambda"), cells("result")
		var yCells [nbColumns][6]*wiop.Cell
		for c := range yCells {
			yCells[c] = cells("y")
		}
		ConstrainSum(module, lambdaCells, resultCells, yCells)

		require.NoError(t, compilers.CompileFull(sys))
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			assign := func(cells [6]*wiop.Cell, e field.Ext) {
				for k, coordinate := range e.AppendBase(nil) {
					rt.AssignCell(cells[k], field.ElemFromBase(coordinate))
				}
			}
			assign(lambdaCells, lambda)
			assign(resultCells, result)
			for c := range yCells {
				assign(yCells[c], y[c])
			}
		})
		return sys.Verify(proof, pub)
	}

	require.NoError(t, prove(y, result))

	one := field.OneExt()
	changedY := y
	changedY[7].Add(&changedY[7], &one)
	require.Error(t, prove(changedY, result), "a changed y_c was accepted")

	changedResult := result
	changedResult.Add(&changedResult, &one)
	require.Error(t, prove(y, changedResult), "a changed R was accepted")
}
