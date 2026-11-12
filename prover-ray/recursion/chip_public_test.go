package recursion

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// publicProgram: a scalar input, an E6 input, their product and a digest
// input, each exposed as public inputs.
func publicProgram() (*Machine, [4]int) {
	m := &Machine{}
	s, x, d := m.InputScalar(), m.InputE6(), m.InputDigest()
	y := m.Mul(s, x)
	m.Public(s, KindScalar, "S", 0)
	m.Public(x, KindE6, "X", 0)
	m.Public(y, KindE6, "Y", 0)
	m.Public(d, KindDigest, "D", 0)
	return m, [4]int{s, x, y, d}
}

// publicInputs are the inputs of publicProgram.
func publicInputs() []Cell {
	var x, d Cell
	for i := range KindE6 {
		x[i].SetUint64(uint64(i) + 11)
	}
	for i := range d {
		d[i].SetUint64(uint64(i) + 21)
	}
	return []Cell{ScalarCell(field.NewElement(4)), x, d}
}

// TestPublic checks that a scalar is exposed as a base public input, an E6
// value as one extension public input and a digest as eight base public
// inputs, at the positions of their tags, and that the proof verifies.
func TestPublic(t *testing.T) {
	m, addrs := publicProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(publicInputs())
	require.NoError(t, err)
	proof, pub := p.Prove(r)
	require.Len(t, pub, 1+1+1+KindDigest, "a scalar, two E6 values and a digest")

	for k, tag := range []wiop.PublicInputTag{"S", "X", "Y", "D"} {
		cell := r.Value(addrs[k])
		switch tag {
		case "S", "D":
			kind := map[wiop.PublicInputTag]int{"S": KindScalar, "D": KindDigest}[tag]
			for l := range kind {
				_, pos := p.Sys.LookupPublicInputByTag(tag, l)
				require.GreaterOrEqual(t, pos, 0, "%s_%d should be registered", tag, l)
				require.True(t, pub[pos].IsBase(), "%s_%d should be a base public input", tag, l)
				require.Equal(t, cell[l], pub[pos].AsBase(), "%s_%d should be lane %d of the cell", tag, l, l)
			}
		default:
			_, pos := p.Sys.LookupPublicInputByTag(tag, 0)
			require.GreaterOrEqual(t, pos, 0, "%s_0 should be registered", tag)
			require.False(t, pub[pos].IsBase(), "%s should be an extension public input", tag)
			want, got := CellE6(cell), pub[pos].AsExt()
			require.True(t, want.Equal(&got), "%s should be the E6 value of the cell", tag)
		}
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
		index  int
		tamper func(p *Program, r *Run)
	}{
		{name: "forged scalar public input", forge: "S"},           // the lane-0 opening
		{name: "forged E6 public input", forge: "Y"},               // E6Check only
		{name: "forged digest public input", forge: "D", index: 7}, // the lane-7 opening
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
				cell, _ := p.Sys.LookupPublicInputByTag(tc.forge, tc.index)
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

// TestPublic_KindPanics checks that Public rejects a lane count that is not a
// cell kind.
func TestPublic_KindPanics(t *testing.T) {
	m := &Machine{}
	a := m.InputDigest()
	for _, kind := range []int{0, 3, 9} {
		require.Panics(t, func() { m.Public(a, kind, "A", 0) }, "kind %d", kind)
	}
}
