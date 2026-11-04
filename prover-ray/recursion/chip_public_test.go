package recursion

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// publicProgram: a scalar input, an E6 input and their product, each exposed
// as public inputs.
func publicProgram() (*Machine, [3]int) {
	m := &Machine{}
	s, x := m.InputScalar(), m.InputE6()
	y := m.Mul(s, x)
	m.Public(s, KindScalar, "S", 0)
	m.Public(x, KindE6, "X", 0)
	m.Public(y, KindE6, "Y", 0)
	return m, [3]int{s, x, y}
}

// publicInputs are the inputs of publicProgram from a seed.
func publicInputs(seed uint64) []Cell {
	var x Cell // an E6 value: lanes 0..5
	for i := range KindE6 {
		x[i].SetUint64(seed*10 + uint64(i) + 1)
	}
	return []Cell{ScalarCell(field.NewElement(seed + 3)), x}
}

// TestPublic checks that the exposed lanes are the proof's public inputs, at
// the positions of their tags, and that the proof verifies.
func TestPublic(t *testing.T) {
	m, addrs := publicProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)

	for _, seed := range []uint64{1, 2} {
		r, err := p.Execute(publicInputs(seed))
		require.NoError(t, err)
		proof, pub := p.Prove(r)
		require.Len(t, pub, 1+6+6, "one scalar and two E6 values")

		want := map[wiop.PublicInputTag][]int{"S": {0}, "X": {0, 1, 2, 3, 4, 5}, "Y": {0, 1, 2, 3, 4, 5}}
		for k, tag := range []wiop.PublicInputTag{"S", "X", "Y"} {
			cell := r.Value(addrs[k])
			for _, l := range want[tag] {
				_, pos := p.Sys.LookupPublicInputByTag(tag, l)
				require.GreaterOrEqual(t, pos, 0, "%s_%d should be registered", tag, l)
				diff := pub[pos].Sub(field.ElemFromBase(cell[l]))
				require.True(t, diff.IsZero(), "%s_%d should be lane %d of the cell", tag, l, l)
			}
		}
		require.NoError(t, p.Verify(proof, pub))
	}
}

// TestPublic_Rejects checks that a public input that differs from the cell,
// or a public chip lane that differs from the bus, fails verification.
func TestPublic_Rejects(t *testing.T) {
	m, _ := publicProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)

	cases := []struct {
		name    string
		witness func(r *Run)           // tampers the run before proving
		pub     func(wiop.PublicInput) // tampers the public inputs after proving
	}{
		{name: "tampered scalar public input", pub: func(pub wiop.PublicInput) {
			_, pos := p.Sys.LookupPublicInputByTag("S", 0)
			pub[pos] = pub[pos].Add(field.ElemOne())
		}},
		{name: "tampered E6 lane public input", pub: func(pub wiop.PublicInput) {
			_, pos := p.Sys.LookupPublicInputByTag("Y", 3)
			pub[pos] = pub[pos].Add(field.ElemOne())
		}},
		{name: "tampered public chip lane", witness: func(r *Run) {
			col := r.Witness(p, publicMod, laneName(publicV, 2))
			col[1].Add(&col[1], new(field.Element).SetOne())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := p.Execute(publicInputs(1))
			require.NoError(t, err)
			if tc.witness != nil {
				tc.witness(r)
			}
			proof, pub := p.Prove(r)
			if tc.pub != nil {
				tc.pub(pub)
			}
			require.Error(t, p.Verify(proof, pub), "a public input or lane that differs from the cell should fail")
		})
	}

	t.Run("wrong length", func(t *testing.T) {
		r, err := p.Execute(publicInputs(1))
		require.NoError(t, err)
		proof, pub := p.Prove(r)
		require.Error(t, p.Verify(proof, pub[:len(pub)-1]))
	})
}

// TestPublic_KindPanics checks that Public rejects a lane count that is not a
// cell kind.
func TestPublic_KindPanics(t *testing.T) {
	m := &Machine{}
	a := m.InputScalar()
	require.Panics(t, func() { m.Public(a, 3, "A", 0) })
}
