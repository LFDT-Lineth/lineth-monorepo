package verifier_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursion"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/recursionguests/verifier"
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
// the mock assigns and rejects a proof of another system, a public input, a
// commitment or a verdict changed after the fact; and that Assign fails a
// system of another shape, the Go verification and the shape checks.
func TestMock(t *testing.T) {
	key := field.Octuplet{field.NewElement(7)}
	sys := stubSystem(key)
	v, err := verifier.NewMock(sys)
	require.NoError(t, err)
	kinds := []int{recursion.KindScalar, recursion.KindE6, recursion.KindScalar}

	// The program: a proof, verified under the key as a constant; the public
	// inputs are published.
	m := &recursion.Machine{}
	proof := v.Allocate(m)
	pubs := v.Define(m, m.Const(recursion.Cell(key)), proof)
	require.Len(t, pubs, 3)
	for pos, kind := range kinds {
		m.Public(pubs[pos], kind, "Out", pos)
	}
	p, err := m.Compile(wioptest.TestingCompileOptions()...)
	require.NoError(t, err)

	var ext field.Ext
	_, err = ext.SetRandom()
	require.NoError(t, err)
	pub := wiop.PublicInput{field.ElemFromBase(field.NewElement(3)), field.ElemFromExt(ext), field.ElemFromBase(field.NewElement(5))}
	wp := wiop.Proof{Commitments: map[int]field.Octuplet{0: {field.NewElement(11)}, 1: {field.NewElement(13)}}}
	cells, err := v.Assign(sys, wp, pub)
	require.NoError(t, err)
	require.Len(t, cells, len(proof))

	prove := func(cells []recursion.Cell) error {
		run, err := p.Execute(cells)
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
	require.NoError(t, prove(cells), "an assigned proof should verify")

	other, err := v.Assign(stubSystem(field.Octuplet{field.NewElement(8)}), wp, pub)
	require.NoError(t, err, "a system of the shape should be assigned")
	require.Error(t, prove(other), "a proof of another system should fail")

	tampered := append([]recursion.Cell(nil), cells...)
	tampered[0][0].SetUint64(4)
	require.Error(t, prove(tampered), "a changed public input should fail")
	tampered = append([]recursion.Cell(nil), cells...)
	tampered[len(tampered)-1][7].SetUint64(1)
	require.Error(t, prove(tampered), "a changed verdict should fail")
	tampered = append([]recursion.Cell(nil), cells...)
	tampered[3][2].SetUint64(1)
	require.Error(t, prove(tampered), "a changed commitment should fail")

	forged := wp
	forged.DynamicSizes = map[int]int{0: 8}
	_, err = v.Assign(sys, forged, pub)
	require.ErrorContains(t, err, "verifying the proof in Go")
	_, err = v.Assign(sys, wp, pub[:2])
	require.ErrorContains(t, err, "2 public inputs, want 3")
	_, err = v.Assign(sys, wiop.Proof{}, pub)
	require.ErrorContains(t, err, "0 round commitments, want 2")
	extPub := append(wiop.PublicInput(nil), pub...)
	extPub[0] = field.ElemFromExt(ext)
	_, err = v.Assign(sys, wp, extPub)
	require.ErrorContains(t, err, "public input 0 is not a base field element")
	narrow := wiop.NewSystemf("narrow")
	narrow.RegisterPublicInputs("Out", narrow.NewRound().NewCell(narrow.Context.Childf("x"), false), 0)
	_, err = v.Assign(narrow, wp, pub)
	require.ErrorContains(t, err, "not the verifier's shape")
	_, err = v.Assign(nil, wp, pub)
	require.ErrorContains(t, err, "no system")
	_, err = verifier.NewMock(nil)
	require.ErrorContains(t, err, "no system")
}
