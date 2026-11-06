package gkrtable

import (
	"maps"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/stretchr/testify/require"
)

const (
	logSize = 3
	// nbRows is the number of compressions that the tests assign: the table has padding rows.
	nbRows = 5
)

// randomRows returns random compressions.
func randomRows(nb int) [][nbColumns]field.Element {
	rng := rand.New(rand.NewPCG(uint64(nb), 7))
	compressor := poseidon2.NewCompressor(poseidon2.NewDefaultPermutation())
	rows := make([][nbColumns]field.Element, nb)
	for i := range rows {
		var left, right, out [blockLen]field.Element
		for k := range blockLen {
			left[k].SetUint64(uint64(rng.Uint32()))
			right[k].SetUint64(uint64(rng.Uint32()))
		}
		compressor.CompressField(&out, &left, &right)
		copy(rows[i][:blockLen], left[:])
		copy(rows[i][blockLen:], right[:])
		copy(rows[i][2*blockLen:], out[:])
	}
	return rows
}

// actionFunc is a test-only prover action.
type actionFunc func(rt *wiop.Runtime)

func (f actionFunc) Run(rt *wiop.Runtime) { f(rt) }

// newTable defines a table of 2^logSize rows in a system.
func newTable(t *testing.T) (*wiop.System, *Table) {
	t.Helper()
	sys := wiop.NewSystemf("gkrtable")
	table, err := Define(sys, sys.NewRound(), logSize)
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
// a prover that changes an input or an output of a row of T, a value y, or a row of E is rejected,
// as is a proof with a changed element of the GKR proof, coordinate of r* or value y.
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
		elements := append([]field.Ext(nil), proof.ExtVectors[table.Proof.Context.ID]...)
		elements[i] = addOne(elements[i])
		changed.ExtVectors = map[wiop.ObjectID][]field.Ext{table.Proof.Context.ID: elements}
		return changed
	}
	nbElements := len(proof.ExtVectors[table.Proof.Context.ID])

	for name, changed := range map[string]wiop.Proof{
		"the first coordinate of r*":  changeCell(table.Point[0]),
		"the last coordinate of r*":   changeCell(table.Point[logSize-1]),
		"the first value":             changeCell(table.Values[0]),
		"the last value":              changeCell(table.Values[nbColumns-1]),
		"the first element of proof":  changeProofElement(0),
		"the middle element of proof": changeProofElement(nbElements / 2),
		"the last element of proof":   changeProofElement(nbElements - 1),
	} {
		t.Run("changed "+name, func(t *testing.T) {
			require.ErrorContains(t, sys.Verify(changed, pub), "gkrtable: GKR proof")
		})
	}

	overrideRow := func(rt *wiop.Runtime, col *wiop.Column, row int) {
		values := append([]field.Element(nil), rt.GetColumnAssignment(col).Plain.AsBase()...)
		one := field.One()
		values[row].Add(&values[row], &one)
		rt.OverrideColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(values)})
	}
	for name, tamper := range map[string]func(table *Table) func(*wiop.Runtime){
		"an input of a row": func(table *Table) func(*wiop.Runtime) {
			return func(rt *wiop.Runtime) { overrideRow(rt, table.Columns[5], 1) }
		},
		"an output of a padding row": func(table *Table) func(*wiop.Runtime) {
			return func(rt *wiop.Runtime) { overrideRow(rt, table.Columns[20], nbRows+1) }
		},
		// a value y, after the GKR proof assigns it and before the round is absorbed
		"the first value y": func(table *Table) func(*wiop.Runtime) {
			table.Sigma.Round().RegisterAction(actionFunc(func(rt *wiop.Runtime) {
				rt.OverrideCell(table.Values[0], field.ElemFromExt(addOne(rt.GetCellValue(table.Values[0]).AsExt())))
			}))
			return nil
		},
		"the last value y": func(table *Table) func(*wiop.Runtime) {
			last := table.Values[nbColumns-1]
			table.Sigma.Round().RegisterAction(actionFunc(func(rt *wiop.Runtime) {
				rt.OverrideCell(last, field.ElemFromExt(addOne(rt.GetCellValue(last).AsExt())))
			}))
			return nil
		},
		// a row of E, after its action assigns it
		"a row of E": func(table *Table) func(*wiop.Runtime) {
			eq := table.Module.Columns[nbColumns]
			require.True(t, eq.IsExtension)
			eq.Round().RegisterAction(actionFunc(func(rt *wiop.Runtime) {
				values := append([]field.Ext(nil), rt.GetColumnAssignment(eq).Plain.AsExt()...)
				values[2] = addOne(values[2])
				rt.OverrideColumn(eq, &wiop.ConcreteVector{Plain: field.VecFromExt(values)})
			}))
			return nil
		},
	} {
		t.Run("changed "+name, func(t *testing.T) {
			sys, table := newTable(t)
			proof, pub := prove(t, sys, table, tamper(table))
			require.Error(t, sys.Verify(proof, pub))
		})
	}
}

// TestAssignTooManyRows checks that Assign refuses more rows than the table has.
func TestAssignTooManyRows(t *testing.T) {
	_, table := newTable(t)
	require.ErrorContains(t, table.Assign(nil, randomRows(1<<logSize+1)), "9 rows in a table of 8")
}
