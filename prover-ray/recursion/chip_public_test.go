package recursion

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// publicProgram: a scalar input, an E6 input and their product, each exposed
// as a public input.
func publicProgram() (*Machine, [3]int) {
	m := &Machine{}
	s, x := m.InputScalar(), m.InputE6()
	y := m.Mul(s, x)
	m.Public(s, KindScalar, "S", 0)
	m.Public(x, KindE6, "X", 0)
	m.Public(y, KindE6, "Y", 0)
	return m, [3]int{s, x, y}
}

// publicInputs are the inputs of publicProgram.
func publicInputs() []Cell {
	var x Cell // an E6 value: lanes 0..5
	for i := range KindE6 {
		x[i].SetUint64(uint64(i) + 11)
	}
	return []Cell{ScalarCell(field.NewElement(4)), x}
}

// TestPublic checks that a scalar is exposed as a base public input and an E6
// value as one extension public input, at the positions of their tags, and
// that the proof verifies.
func TestPublic(t *testing.T) {
	m, addrs := publicProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(publicInputs())
	require.NoError(t, err)
	proof, pub := p.Prove(r)
	require.Len(t, pub, 3, "one scalar and two E6 values, one public input each")

	for k, tag := range []wiop.PublicInputTag{"S", "X", "Y"} {
		_, pos := p.Sys.LookupPublicInputByTag(tag, 0)
		require.GreaterOrEqual(t, pos, 0, "%s_0 should be registered", tag)
		cell := r.Value(addrs[k])
		if tag == "S" {
			require.True(t, pub[pos].IsBase(), "a scalar should be a base public input")
			require.Equal(t, cell[0], pub[pos].AsBase(), "S should be lane 0 of the cell")
			continue
		}
		require.False(t, pub[pos].IsBase(), "%s should be an extension public input", tag)
		want, got := CellE6(cell), pub[pos].AsExt()
		require.True(t, want.Equal(&got), "%s should be the E6 value of the cell", tag)
	}
	require.NoError(t, p.Verify(proof, pub))
}

// TestPublic_Rejects checks that a prover whose public input differs from the
// cell, with a consistent transcript, or whose public chip lane differs from
// the bus, fails verification. A public input is forged through its cell's
// assigner: tampering with the public-input vector after proving would fail
// on the transcript instead, whatever binds the value.
func TestPublic_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		forge  wiop.PublicInputTag // the public input to forge, if any
		tamper func(p *Program, r *Run)
	}{
		{name: "forged scalar public input", forge: "S"}, // the lane-0 opening
		{name: "forged E6 public input", forge: "Y"},     // E6Check only
		{name: "tampered public chip lane", tamper: func(p *Program, r *Run) {
			col := r.Witness(p, publicMod, laneName(publicV, 2))
			col[1].Add(&col[1], new(field.Element).SetOne())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := publicProgram()
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			if tc.forge != "" {
				cell, _ := p.Sys.LookupPublicInputByTag(tc.forge, 0)
				honest := cell.Assigner
				cell.Assigner = func(rt *wiop.Runtime) field.Gen { return honest(rt).Add(field.ElemOne()) }
			}
			r, err := p.Execute(publicInputs())
			require.NoError(t, err)
			if tc.tamper != nil {
				tc.tamper(p, r)
			}
			proof, pub := p.Prove(r)
			require.Error(t, p.Verify(proof, pub), "a public input or lane that differs from the cell should fail")
		})
	}
}

// TestPublic_KindPanics checks that Public rejects a kind without a
// public-input shape: a digest or a lane count that is not a cell kind.
func TestPublic_KindPanics(t *testing.T) {
	m := &Machine{}
	a := m.InputDigest()
	for _, kind := range []int{3, KindDigest} {
		require.Panics(t, func() { m.Public(a, kind, "A", 0) }, "kind %d", kind)
	}
}
