package recursion

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

var bitsValues = []uint64{0, 1, 5, 1<<24 - 1, 1 << 24, 0x7f000000 /* p − 1 */, 0x12345678, 1<<30 + 3}

const bitsN = 10

// bitsProgram decomposes one input per value of bitsValues; with check, two
// outputs of each decomposition are copied, so that their writes are read on
// the bus.
func bitsProgram(check bool) (*Machine, []Bits) {
	m := &Machine{}
	cp := withCopy(m)
	outs := make([]Bits, 0, len(bitsValues))
	for range bitsValues {
		out := m.Bits(m.InputScalar(), bitsN)
		if check {
			cp.Copy(out.Shr[3])
			cp.Copy(out.Bit[2])
		}
		outs = append(outs, out)
	}
	return m, outs
}

func bitsInputs() []Cell {
	in := make([]Cell, len(bitsValues))
	for i, v := range bitsValues {
		in[i] = ScalarCell(field.NewElement(v))
	}
	return in
}

// TestBitsChip: the decompositions of bitsValues (p − 1 included) hold their
// shifted values and bits, and prove.
func TestBitsChip(t *testing.T) {
	m, outs := bitsProgram(true)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(bitsInputs())
	require.NoError(t, err)
	for k, v := range bitsValues {
		low := v & (1<<bitsN - 1)
		for i := range bitsN {
			require.Equal(t, ScalarCell(field.NewElement(low>>i)), r.Value(outs[k].Shr[i]), "value %d: Shr[%d]", v, i)
			require.Equal(t, ScalarCell(field.NewElement(v>>i&1)), r.Value(outs[k].Bit[i]), "value %d: Bit[%d]", v, i)
		}
	}
	require.NoError(t, proveAndVerify(p, r))
}

// TestBitsChipRejects: a bit flipped, a copied output changed, or the
// decomposition of v + p (the same field element, but not its canonical
// bits) is rejected.
func TestBitsChipRejects(t *testing.T) {
	for name, tamper := range map[string]struct {
		check bool
		f     func(r *Run, p *Program)
	}{
		"bit": {false, func(r *Run, p *Program) { flip(&r.Witness(p, bitsMod, bitsB)[2*BitsWidth+1]) }},
		// Shr[3] of the second value, read by its copy
		"shifted value": {true, func(r *Run, p *Program) {
			r.Witness(p, bitsMod, bitsR)[BitsWidth+3].SetUint64(7)
		}},
		// v = 5: every column of the decomposition of 5 + p is consistent,
		// and only the canonical check fails
		"non canonical": {false, func(r *Run, p *Program) {
			const k, prime = 2, 0x7f000001
			x := bitsValues[k] + prime
			b, w, rr := r.Witness(p, bitsMod, bitsB), r.Witness(p, bitsMod, bitsW), r.Witness(p, bitsMod, bitsR)
			inv := r.Witness(p, bitsMod, bitsInv)
			for i := range BitsWidth {
				row := k*BitsWidth + i
				b[row].SetUint64(x >> i & 1)
				w[row].SetUint64(x & (1<<i - 1))
				if i < bitsN {
					rr[row].SetUint64((x & (1<<bitsN - 1)) >> i)
				}
				if i == 24 {
					inv[row].SetZero() // z = 127 − 127 = 0
				}
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := bitsProgram(tamper.check)
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(bitsInputs())
			require.NoError(t, err)
			tamper.f(r, p)
			require.Error(t, proveAndVerify(p, r), "tampered trace accepted")
		})
	}
}

func flip(e *field.Element) {
	if e.IsZero() {
		e.SetOne()
	} else {
		e.SetZero()
	}
}
