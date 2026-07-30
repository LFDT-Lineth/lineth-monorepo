package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/stretchr/testify/require"
)

// piVec builds a base-field ConcreteVector from the given values.
func piVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// piGen builds a base-field scalar value.
func piGen(v uint64) field.Gen {
	var e field.Element
	e.SetUint64(v)
	return field.ElemFromBase(e)
}

// TestPublicInput exercises the public-input flow: a public value is exposed by
// opening a column position into a cell (the opening also registers a local
// constraint binding cell == col[pos]), the cell is the sole registered public
// input, and the system is compiled so the binding constraint becomes a
// verifier check. The statement is an ordered []field.Gen aligned to
// registration order.
func TestPublicInput(t *testing.T) {
	const (
		n   = 4
		pos = 2 // col[2] == 30 below
	)

	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("pi-cells")
		r0 := sys.NewRound()
		m := sys.NewSizedModule(sys.Context.Childf("m"), n, wiop.PaddingDirectionNone)
		col := m.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		// Open col[pos] into a cell; Open registers the local constraint
		// cell == col[pos], which soundly binds the public input into the proof.
		cell := col.At(pos).Open(sys.Context.Childf("my-public-input"))
		layout := &wiop.PublicInputLayout{GuestPublicOutputs: []*wiop.Cell{cell}}
		layout.Register(sys)
		localvanishing.Compile(sys)
		global.Compile(sys)
		return sys, col, cell
	}

	sys, col, cell := build()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, piVec(10, 20, 30, 40))
	})

	// The public value lives only in the statement (in registration order),
	// never in the proof.
	require.Len(t, pub, 1)
	require.Equal(t, piGen(30), pub[0], "opened cell must equal col[pos]")
	require.NotContains(t, proof.Cells, cell.Context.ID, "public-input cell must not be in the proof")

	// Honest statement verifies.
	require.NoError(t, sys.Verify(proof, pub))

	// A statement of the wrong length (missing / extra) is rejected.
	require.Error(t, sys.Verify(proof, wiop.PublicInput{}))
	require.Error(t, sys.Verify(proof, wiop.PublicInput{pub[0], piGen(0)}))

	// A wrong public value breaks the cell == col[pos] binding and is rejected.
	require.Error(t, sys.Verify(proof, wiop.PublicInput{piGen(99)}), "tampered public input must be rejected")

	// A proof that smuggles the public-input cell back in is rejected.
	sys2, col2, cell2 := build()
	proof2, pub2 := sys2.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col2, piVec(10, 20, 30, 40))
	})
	proof2.Cells[cell2.Context.ID] = pub2[0]
	require.Error(t, sys2.Verify(proof2, pub2), "public-input cell must not appear in the proof")
}

// TestPublicInputDynamicColumn is the same flow as [TestPublicInput] but the
// opened column lives on a dynamic-size module: its domain size is inferred from
// the assignment at prove time and round-trips through Proof.DynamicSizes, while
// the public input is still exposed as the opened cell.
func TestPublicInputDynamicColumn(t *testing.T) {
	const pos = 0 // col[0] == 30 below

	build := func() (*wiop.System, *wiop.Column, *wiop.Cell) {
		sys := wiop.NewSystemf("pi-dyn")
		r0 := sys.NewRound()
		m := sys.NewDynamicModule(sys.Context.Childf("m"), wiop.PaddingDirectionRight)
		col := m.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		cell := col.At(pos).Open(sys.Context.Childf("open"))
		(&wiop.PublicInputLayout{GuestPublicOutputs: []*wiop.Cell{cell}}).Register(sys)
		localvanishing.Compile(sys)
		global.Compile(sys)
		return sys, col, cell
	}

	sys, col, cell := build()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		// Length 4 (a power of two): the dynamic module's size is inferred as 4.
		rt.AssignColumn(col, piVec(30, 20, 10, 40))
	})

	// The dynamic module's runtime size round-trips in the proof.
	require.Equal(t, 4, proof.DynamicSizes[col.Context.ID.Slot()], "dynamic module size must round-trip")

	// The public value lives only in the statement, never in the proof.
	require.Len(t, pub, 1)
	require.Equal(t, piGen(30), pub[0], "opened cell must equal col[0]")
	require.NotContains(t, proof.Cells, cell.Context.ID)

	// Honest statement verifies; a wrong public value breaks the cell == col[0]
	// binding and is rejected.
	require.NoError(t, sys.Verify(proof, pub))
	require.Error(t, sys.Verify(proof, wiop.PublicInput{piGen(99)}),
		"tampered public input must be rejected")
}

// TestRegisterDuplicatedPublicInput checks the registration guards:
// registering the same cell twice is rejected (dedup).
func TestRegisterDuplicatedPublicInputs(t *testing.T) {
	sys := wiop.NewSystemf("pi-guards")
	r0 := sys.NewRound()
	m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := m.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := col.At(0).Open(sys.Context.Childf("open"))

	// A layout containing the same cell twice is rejected.
	require.Panics(t, func() {
		(&wiop.PublicInputLayout{GuestPublicOutputs: []*wiop.Cell{cell, cell}}).Register(sys)
	}, "duplicate cell must be rejected")
}
