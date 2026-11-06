package gkrtable

import (
	"fmt"
	"math/bits"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrverifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/stretchr/testify/require"
)

// layer is what a system that verifies the GKR proof of a [Table] is given of it.
type layer struct {
	table *Table
	// the seed σ and the coin λ, the point r*, the values y and the result R of the sum, and the GKR
	// proof
	sigma, lambda, result field.Ext
	point                 []field.Ext
	values                [nbColumns]field.Ext
	proof                 []field.Ext
}

// proveLayer proves and verifies a table of random compressions.
func proveLayer(t *testing.T) *layer {
	t.Helper()
	sys, table := newTable(t)
	l := &layer{table: table}
	table.Lambda.Round().RegisterAction(actionFunc(func(rt *wiop.Runtime) {
		l.sigma = rt.GetCoinValue(table.Sigma).AsExt()
		l.lambda = rt.GetCoinValue(table.Lambda).AsExt()
	}))
	proof, pub := prove(t, sys, table, nil)
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
// compressions that the verifier needs. It returns the result of the verification. A prover that holds
// a value that the bus of the verifier does not have fails while it builds the lookup, which is reported
// as an error too; any other panic fails the test.
func proveNext(t *testing.T, l *layer) (err error) {
	t.Helper()
	sys := wiop.NewSystemf("gkrtable-next")
	round := sys.NewRound()

	values := make(map[*wiop.Cell]field.Element)
	cells := func(name string, e field.Ext) (c [6]*wiop.Cell) {
		for k, coordinate := range e.AppendBase(nil) {
			c[k] = round.NewCell(sys.Context.Childf("%s-%d", name, k), false)
			values[c[k]] = coordinate
		}
		return c
	}
	seed, lambda, result := cells("sigma", l.sigma), cells("lambda", l.lambda), cells("result", l.result)
	point := new(gkrverifier.Point)
	for j, r := range l.point {
		*point = append(*point, cells(fmt.Sprintf("point-%d", j), r))
	}
	var y [nbColumns][6]*wiop.Cell
	claims := make(map[int][]gkrverifier.Claim)
	for c := range y {
		y[c] = cells(fmt.Sprintf("y-%d", c), l.values[c])
		claims[l.table.wire(c)] = []gkrverifier.Claim{{Point: point, Value: y[c]}}
	}

	circuit, schedule, n := l.table.circuit.Circuit, l.table.circuit.Schedule, l.table.logSize
	nbCompressions, err := gkrverifier.NbCompressions(circuit, schedule, n)
	require.NoError(t, err)
	next, err := Define(sys, round, bits.TrailingZeros(uint(utils.NextPowerOfTwo(nbCompressions))))
	require.NoError(t, err)
	verifier, err := gkrverifier.Define(sys, round, circuit, schedule, n,
		gkrverifier.Inputs{Seed: seed, Claims: claims}, next.Columns)
	require.NoError(t, err)
	ConstrainSum(next.Module, lambda, result, y)

	rows, err := verifier.Compressions([6]field.Element(l.sigma.AppendBase(nil)), l.proof)
	require.NoError(t, err)
	round.RegisterAction(actionFunc(func(rt *wiop.Runtime) {
		require.NoError(t, verifier.Assign(rt, l.proof))
	}))

	require.NoError(t, compilers.CompileFull(sys))
	defer func() {
		if r := recover(); r != nil {
			require.Contains(t, fmt.Sprint(r), "has no match in the lookup table")
			err = fmt.Errorf("the prover has a row that is not in the table: %v", r)
		}
	}()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		for cell, value := range values {
			rt.AssignCell(cell, field.ElemFromBase(value))
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
