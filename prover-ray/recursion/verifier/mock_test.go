package verifier_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion/verifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// stubSystem is a bare system under key: three public inputs, the middle one
// an extension value, on two rounds flagged as committed, uncompiled, so that
// its Go verification accepts any public inputs with a proof of two
// commitments and nothing else.
func stubSystem(key field.Octuplet) *wiop.System {
	sys := wiop.NewSystemf("stub")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	r0.HasCommitment, r1.HasCommitment = true, true
	ctx := sys.Context.Childf("pub")
	for pos, ext := range []bool{false, true, false} {
		sys.RegisterPublicInputs("Out", r0.NewCell(ctx.Childf("%d", pos), ext), pos)
	}
	sys.PrecomputedCommitment = key
	return sys
}

// TestMock checks that a program verifying one mock proof accepts the cells
// the mock lays out, rejects a public input, a commitment or a verdict
// changed after the fact, and checks nothing for an inactive proof; and that
// Cells fails an unknown key, the Go verification and the shape checks.
func TestMock(t *testing.T) {
	key := field.Octuplet{field.NewElement(7)}
	v, err := verifier.NewMock(stubSystem(key))
	require.NoError(t, err)
	kinds := []int{recursion.KindScalar, recursion.KindE6, recursion.KindScalar}

	// The program: an active flag, a proof, verified under the key as a
	// constant; the public inputs are published.
	m := &recursion.Machine{}
	active := m.InputScalar()
	proof := v.Declare(m)
	pubs := v.Verify(m, active, m.Const(recursion.Cell(key)), proof)
	require.Len(t, pubs, 3)
	for pos, kind := range kinds {
		m.Public(pubs[pos], kind, "Out", pos)
	}
	p, err := m.Compile(wioptest.TestingCompileOptions()...)
	require.NoError(t, err)

	var ext field.Ext
	ext.SetRandom()
	pub := wiop.PublicInput{field.ElemFromBase(field.NewElement(3)), field.ElemFromExt(ext), field.ElemFromBase(field.NewElement(5))}
	wp := wiop.Proof{Commitments: map[int]field.Octuplet{0: {field.NewElement(11)}, 1: {field.NewElement(13)}}}
	cells, err := v.Cells(key, wp, pub)
	require.NoError(t, err)
	require.Len(t, cells, len(proof))

	prove := func(active field.Element, cells []recursion.Cell) error {
		run, err := p.Execute(append([]recursion.Cell{recursion.ScalarCell(active)}, cells...))
		if err != nil {
			return err
		}
		proof, got := p.Prove(run)
		if err := p.Verify(proof, got); err != nil {
			return err
		}
		require.Len(t, got, 3)
		return nil
	}
	require.NoError(t, prove(field.One(), cells), "a laid-out proof should verify")
	require.NoError(t, prove(field.Element{}, make([]recursion.Cell, len(cells))), "an inactive proof should check nothing")

	tampered := append([]recursion.Cell(nil), cells...)
	tampered[0][0].SetUint64(4)
	require.Error(t, prove(field.One(), tampered), "a changed public input should fail")
	tampered = append([]recursion.Cell(nil), cells...)
	tampered[len(tampered)-1][7].SetUint64(1)
	require.Error(t, prove(field.One(), tampered), "a changed verdict should fail")
	tampered = append([]recursion.Cell(nil), cells...)
	tampered[3][2].SetUint64(1)
	require.Error(t, prove(field.One(), tampered), "a changed commitment should fail")

	_, err = v.Cells(field.Octuplet{}, wp, pub)
	require.ErrorContains(t, err, "of no system")
	forged := wp
	forged.DynamicSizes = map[int]int{0: 8}
	_, err = v.Cells(key, forged, pub)
	require.ErrorContains(t, err, "verifying the proof in Go")
	_, err = v.Cells(key, wp, pub[:2])
	require.ErrorContains(t, err, "2 public inputs, want 3")
	_, err = v.Cells(key, wiop.Proof{}, pub)
	require.ErrorContains(t, err, "0 round commitments, want 2")
	extPub := append(wiop.PublicInput(nil), pub...)
	extPub[0] = field.ElemFromExt(ext)
	_, err = v.Cells(key, wp, extPub)
	require.ErrorContains(t, err, "public input 0 is not a base field element")

	other := stubSystem(field.Octuplet{field.NewElement(8)})
	require.NoError(t, v.Accept(other), "a system of the shape should be accepted")
	require.ErrorContains(t, v.Accept(other), "two systems have the key")
	_, err = verifier.NewMock()
	require.ErrorContains(t, err, "no system")
	narrow := wiop.NewSystemf("narrow")
	narrow.RegisterPublicInputs("Out", narrow.NewRound().NewCell(narrow.Context.Childf("x"), false), 0)
	require.ErrorContains(t, v.Accept(narrow), "not the mock's shape")
}
