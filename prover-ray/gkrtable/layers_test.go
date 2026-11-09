package gkrtable_test

import (
	"fmt"
	"math/bits"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrtable"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrverifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

const (
	// n is the log size of the table that a layer proves, and nbRows the number of compressions that it
	// assigns: the table has padding rows.
	n      = 3
	nbRows = 5
)

// layer is what a system that verifies the GKR proof of a [gkrtable.Table] is given of it.
type layer struct {
	table *gkrtable.Table
	// the seed σ and the coin λ, the point r*, the values y and the result R of the sum, and the GKR
	// proof
	seed, lambda, result field.Ext
	point                []field.Ext
	values               [gkrtable.NbColumns]field.Ext
	proof                []field.Ext
}

// proveLayer proves and verifies a table of random compressions.
func proveLayer(t *testing.T) *layer {
	t.Helper()
	sys := wiop.NewSystemf("gkrtable")
	table, err := gkrtable.Define(sys, sys.NewRound(), n)
	require.NoError(t, err)
	l := &layer{table: table}
	table.Lambda.Round().RegisterAction(wioptest.ActionFunc(func(rt *wiop.Runtime) {
		l.seed = rt.GetCoinValue(table.Seed).AsExt()
		l.lambda = rt.GetCoinValue(table.Lambda).AsExt()
	}))
	require.NoError(t, compilers.CompileFull(sys))
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		require.NoError(t, table.Assign(rt, gkrtable.RandomRows(nbRows)))
	})
	require.NoError(t, sys.Verify(proof, pub))

	cell := func(c *wiop.Cell) field.Ext { return proof.Cells[c.Context.ID].AsExt() }
	for _, c := range table.Point {
		l.point = append(l.point, cell(c))
	}
	for c := range l.values {
		l.values[c] = cell(table.Values[c])
	}
	l.result = cell(table.Result)
	l.proof = proof.ExtVectors[table.Proof.Context.ID]
	return l
}

// proveNext proves and verifies a system, which holds the layer's σ, r*, y, λ and R in cells, verifies
// the layer's GKR proof with [gkrverifier], constrains R = Σ_c λᶜ y_c, and has a table of the
// compressions that the verifier needs. It returns the result of the verification, or
// [wioptest.ErrMissingRow].
func proveNext(t *testing.T, l *layer) (err error) {
	t.Helper()
	sys := wiop.NewSystemf("gkrtable-next")
	round := sys.NewRound()

	var assignments []func(rt *wiop.Runtime)
	cells := func(name string, e field.Ext) [6]*wiop.Cell {
		c := wioptest.NewE6Cells(sys.Context, round, name)
		assignments = append(assignments, func(rt *wiop.Runtime) { wioptest.AssignE6(rt, c, e) })
		return c
	}
	seed, lambda, result := cells("seed", l.seed), cells("lambda", l.lambda), cells("result", l.result)
	point := new(gkrverifier.Point)
	for j, r := range l.point {
		*point = append(*point, cells(fmt.Sprintf("point-%d", j), r))
	}
	var y [gkrtable.NbColumns][6]*wiop.Cell
	for c := range y {
		y[c] = cells(fmt.Sprintf("y-%d", c), l.values[c])
	}

	nbCompressions, err := gkrverifier.NbCompressions(l.table.Circuit, l.table.Schedule, len(l.table.Point))
	require.NoError(t, err)
	next, err := gkrtable.Define(sys, round, bits.TrailingZeros(uint(utils.NextPowerOfTwo(nbCompressions))))
	require.NoError(t, err)
	verifier, err := gkrverifier.Define(sys, round, l.table.Circuit, l.table.Schedule, len(l.table.Point),
		gkrverifier.Inputs{Seed: seed, Claims: l.table.Claims(point, y)}, next.Columns)
	require.NoError(t, err)
	gkrtable.ConstrainSum(next.Module, lambda, result, y)

	rows, err := verifier.Compressions([6]field.Element(l.seed.AppendBase(nil)), l.proof)
	require.NoError(t, err)
	round.RegisterAction(wioptest.ActionFunc(func(rt *wiop.Runtime) {
		require.NoError(t, verifier.Assign(rt, l.proof))
	}))

	require.NoError(t, compilers.CompileFull(sys))
	defer wioptest.RecoverMissingRow(t, &err)
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		for _, assign := range assignments {
			assign(rt)
		}
		require.NoError(t, next.Assign(rt, rows))
	})
	return sys.Verify(proof, pub)
}

// TestNextLayer checks that a system proves and verifies the GKR proof of a table that another system
// proved: the verifier of the proof and the constraints of the sum are in the second system, whose
// own table holds the compressions of the verifier. A changed element of the GKR proof, or cell of r*,
// y or R, is rejected.
func TestNextLayer(t *testing.T) {
	l := proveLayer(t)
	require.NoError(t, proveNext(t, l))

	one := field.OneExt()
	for name, change := range map[string]func(l *layer){
		"an element of the GKR proof": func(l *layer) {
			l.proof = slices.Clone(l.proof)
			l.proof[len(l.proof)/2].Add(&l.proof[len(l.proof)/2], &one)
		},
		"a coordinate of r*": func(l *layer) { l.point = slices.Clone(l.point); l.point[0].Add(&l.point[0], &one) },
		"a cell of y":        func(l *layer) { l.values[3].Add(&l.values[3], &one) },
		"the result R":       func(l *layer) { l.result.Add(&l.result, &one) },
	} {
		t.Run("changed "+name, func(t *testing.T) {
			changed := *l
			change(&changed)
			require.Error(t, proveNext(t, &changed))
		})
	}
}
