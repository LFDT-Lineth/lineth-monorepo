package gkrtable

import (
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/stretchr/testify/require"
)

const (
	// n is the log size of the tables of the tests, and nbRows the number of compressions that they assign:
	// the table has padding rows.
	n      = 3
	nbRows = 5
	// out is the first column of the output of the compression.
	out = 2 * blockLen
)

// randomRows returns random compressions.
func randomRows(nb int) [][NbColumns]field.Element {
	rng := rand.New(rand.NewPCG(uint64(nb), 7))
	compressor := poseidon2.NewCompressor(poseidon2.NewDefaultPermutation())
	rows := make([][NbColumns]field.Element, nb)
	for i := range rows {
		var l, r, o [blockLen]field.Element
		for k := range blockLen {
			l[k].SetUint64(uint64(rng.Uint32()))
			r[k].SetUint64(uint64(rng.Uint32()))
		}
		compressor.CompressField(&o, &l, &r)
		copy(rows[i][:blockLen], l[:])
		copy(rows[i][blockLen:], r[:])
		copy(rows[i][out:], o[:])
	}
	return rows
}

// newTable defines a table of 2ⁿ rows in a system.
func newTable(t *testing.T) (*wiop.System, *Table) {
	t.Helper()
	sys := wiop.NewSystemf("gkrtable")
	table, err := Define(sys, sys.NewRound(), n)
	require.NoError(t, err)
	return sys, table
}

// prove compiles the system and proves it, with the table assigned to the random compressions, then
// calls tamper, if not nil, which can override the assignment.
func prove(t *testing.T, sys *wiop.System, table *Table, tamper func(rt *wiop.Runtime)) (wiop.Proof, wiop.PublicInput) {
	t.Helper()
	require.NoError(t, compilers.CompileFull(sys))
	return sys.Prove(func(rt *wiop.Runtime) {
		require.NoError(t, table.Assign(rt, randomRows(nbRows)))
		if tamper != nil {
			tamper(rt)
		}
	})
}

func addOne(e field.Ext) field.Ext {
	one := field.OneExt()
	e.Add(&e, &one)
	return e
}

// TestTable checks that a table of random compressions, with padding, proves and verifies, and that
// a prover that changes an output of a row of T or two rows of E is rejected, as is a proof with a changed
// element of the GKR proof, coordinate of r* or value y.
func TestTable(t *testing.T) {
	sys, table := newTable(t)
	proof, pub := prove(t, sys, table, nil)
	require.NoError(t, sys.Verify(proof, pub))

	changeCell := func(cell *wiop.Cell) wiop.Proof {
		changed := proof
		changed.Cells = maps.Clone(proof.Cells)
		changed.Cells[cell.Context.ID] = field.ElemFromExt(addOne(proof.Cells[cell.Context.ID].AsExt()))
		return changed
	}
	changeProofElement := func(i int) wiop.Proof {
		changed := proof
		elements := slices.Clone(proof.ExtVectors[table.Proof.Context.ID])
		elements[i] = addOne(elements[i])
		changed.ExtVectors = map[wiop.ObjectID][]field.Ext{table.Proof.Context.ID: elements}
		return changed
	}
	nbElements := len(proof.ExtVectors[table.Proof.Context.ID])

	for _, c := range []struct {
		name    string
		changed wiop.Proof
		message string
	}{
		{"a coordinate of r*", changeCell(table.Point[0]), "is not r*"},
		{"a value y", changeCell(table.Values[0]), "is not y"},
		{"an element of the proof", changeProofElement(nbElements / 2), "disagrees with gate eval"},
	} {
		t.Run("changed "+c.name, func(t *testing.T) {
			require.ErrorContains(t, sys.Verify(c.changed, pub), c.message)
		})
	}

	overrideRow := func(rt *wiop.Runtime, col *wiop.Column, row int) {
		values := slices.Clone(rt.GetColumnAssignment(col).Plain.AsBase())
		one := field.One()
		values[row].Add(&values[row], &one)
		rt.OverrideColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(values)})
	}
	for _, c := range []struct {
		name    string
		tamper  func(table *Table) func(*wiop.Runtime)
		message string
	}{
		{"an output of a padding row", func(table *Table) func(*wiop.Runtime) {
			return func(rt *wiop.Runtime) { overrideRow(rt, table.Columns[out+4], nbRows+1) }
		}, "do not have the evaluations y at r*"},
		// Two padding rows of E, after its action assigns it: they hold the same compression, so one more
		// in one and one less in the other leaves the sum unchanged.
		{"two rows of E", func(table *Table) func(*wiop.Runtime) {
			eq := table.eq
			eq.Round().RegisterAction(wioptest.ActionFunc(func(rt *wiop.Runtime) {
				values := slices.Clone(rt.GetColumnAssignment(eq).Plain.AsExt())
				one := field.OneExt()
				values[nbRows].Add(&values[nbRows], &one)
				values[nbRows+1].Sub(&values[nbRows+1], &one)
				rt.OverrideColumn(eq, &wiop.ConcreteVector{Plain: field.VecFromExt(values)})
			}))
			return nil
		}, "global quotient check failed"},
	} {
		t.Run("changed "+c.name, func(t *testing.T) {
			sys, table := newTable(t)
			proof, pub := prove(t, sys, table, c.tamper(table))
			require.ErrorContains(t, sys.Verify(proof, pub), c.message)
		})
	}
}
