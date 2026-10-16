package recursion

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// selectProgram selects between two 8-lane inputs a and c, by the inputs one
// and zero. With check, it asserts the E6 lanes of each output (lanes 0..5
// of the first, 2..7 of the second) equal those of the selected input; with
// out, the outputs are only copied, so that nothing but the select chip
// checks them. zero is the address of the input zero.
func selectProgram(check bool) (m *Machine, in []Cell, outs [2]int, zero int) {
	m = &Machine{}
	cp := withCopy(m)
	var a, c Cell
	for i := range CellWidth {
		a[i].SetUint64(uint64(10 + i))
		c[i].SetUint64(uint64(20 + i))
	}
	in = []Cell{a, c, ScalarCell(field.One()), {}}
	ca, cc, one := m.InputDigest(), m.InputDigest(), m.InputScalar()
	zero = m.InputScalar()
	outs = [2]int{m.Select(one, ca, cc), m.Select(zero, ca, cc)}
	if check {
		m.AssertEqE6(m.Lanes(outs[0], 0, KindE6), m.Lanes(ca, 0, KindE6))
		m.AssertEqE6(m.Lanes(outs[1], 2, KindE6), m.Lanes(cc, 2, KindE6))
	} else {
		cp.Copy(outs[0])
		cp.Copy(outs[1])
	}
	return m, in, outs, zero
}

func TestSelectChip(t *testing.T) {
	m, in, outs, _ := selectProgram(true)
	p, err := m.Compile()
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.Equal(t, in[0], r.Value(outs[0]), "bit 1 selects a")
	require.Equal(t, in[1], r.Value(outs[1]), "bit 0 selects c")
	require.NoError(t, proveAndVerify(p, r))

	in[3] = ScalarCell(field.NewElement(2))
	_, err = p.Execute(in)
	require.Error(t, err, "bit 2 executed")
}

// TestSelectChipRejects: a wrong selection, claimed consistently on the bus
// (the cells' values changed, then traced), is rejected.
func TestSelectChipRejects(t *testing.T) {
	two := field.NewElement(2)
	for name, cheat := range map[string]func(r *Run, in []Cell, outs [2]int, zero int){
		// lane 7 of the first output, beyond the E6 lanes
		"lane 7": func(r *Run, _ []Cell, outs [2]int, _ int) { r.values[outs[0]][7].SetUint64(99) },
		// the zero bit claimed 1, the output still c
		"bit": func(r *Run, _ []Cell, _ [2]int, zero int) { r.values[zero][0].SetOne() },
		// the zero bit claimed 2, the output c + 2·(a − c) on every lane
		"non-boolean bit": func(r *Run, in []Cell, outs [2]int, zero int) {
			r.values[zero][0] = two
			for i := range CellWidth {
				var d field.Element
				d.Sub(&in[0][i], &in[1][i])
				d.Mul(&d, &two)
				r.values[outs[1]][i].Add(&in[1][i], &d)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, in, outs, zero := selectProgram(false)
			p, err := m.Compile()
			require.NoError(t, err)
			r, err := p.Execute(in)
			require.NoError(t, err)
			cheat(r, in, outs, zero)
			require.NoError(t, p.trace(r))
			require.Error(t, proveAndVerify(p, r), "wrong selection accepted")
		})
	}
}
