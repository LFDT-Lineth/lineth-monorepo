package recursion

import (
	"fmt"
	"math/bits"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// sizedProgram is a proof of publicProgram with extra more multiplications of
// extra more inputs, compiled size-oblivious: for every extra, the same chips,
// columns and public inputs, at other heights and under another key.
func sizedProgram(t *testing.T, extra int) innerCase {
	t.Helper()
	m, _ := publicProgram()
	in := publicInputs()
	for i := range extra {
		z := m.InputE6()
		m.Mul(z, z)
		var c Cell
		c[0].SetUint64(uint64(100 + i))
		c[3].SetUint64(uint64(7 * i))
		in = append(in, c)
	}
	p, err := m.Compile(append(slices.Clone(testPCSOpts), compilers.ConfigurePCS(pcs.WithSizeOblivious()))...)
	require.NoError(t, err, "the program should compile size-oblivious")
	r, err := p.Execute(in)
	require.NoError(t, err, "the program's execution")
	proof, pub := p.Prove(r)
	require.NoError(t, p.Verify(proof, pub), "the program's proof should verify natively")
	return innerCase{"sized", p.Sys, proof, pub}
}

// logHeights returns the log sizes of a system's modules, all sized.
func logHeights(sys *wiop.System) []int {
	res := make([]int, len(sys.Modules))
	for k, mod := range sys.Modules {
		res[k] = bits.TrailingZeros(uint(mod.Size()))
	}
	return res
}

// heightsOuter is a machine verifying, under a key and heights it takes as
// its first inputs, the proofs of the systems of sys's structure.
type heightsOuter struct {
	outer
	numHeights int
}

func newHeightsOuter(t *testing.T, sys *wiop.System) heightsOuter {
	t.Helper()
	m := &Machine{}
	key := m.InputDigest()
	heights := make([]int, len(sys.Modules))
	for k := range heights {
		heights[k] = m.InputScalar()
	}
	v, err := NewVerifierWith(m, sys, VerifierOptions{Key: key, HasKey: true, Heights: heights})
	require.NoError(t, err, "the verifier should build")
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err, "the verifier's program should compile")
	return heightsOuter{outer{v, p}, len(heights)}
}

// inputs lays out the proof c under the key and the heights of under.
func (o heightsOuter) inputs(t *testing.T, c innerCase, under *wiop.System) []Cell {
	t.Helper()
	in := []Cell{Cell(under.PrecomputedCommitment)}
	for _, l := range logHeights(under) {
		in = append(in, ScalarCell(field.NewElement(uint64(l))))
	}
	rest, err := o.v.InputsFor(c.sys, c.proof, c.pub)
	require.NoError(t, err, "the proof's inputs")
	return append(in, rest...)
}

// TestVerifierHeights: one verifier, built from a program, with the key and
// the modules' heights as cells, verifies the proofs of programs with the
// same chips at other heights, each under its own key and heights.
func TestVerifierHeights(t *testing.T) {
	progs := []innerCase{sizedProgram(t, 0), sizedProgram(t, 10), sizedProgram(t, 40)}
	require.NotEqual(t, logHeights(progs[0].sys), logHeights(progs[1].sys), "the programs should differ in a height")
	require.NotEqual(t, logHeights(progs[1].sys), logHeights(progs[2].sys), "the programs should differ in a height")
	require.NotEqual(t, progs[0].sys.PrecomputedCommitment, progs[1].sys.PrecomputedCommitment, "the programs should have their own keys")

	o := newHeightsOuter(t, progs[0].sys)
	for _, c := range progs {
		require.NoError(t, o.runInputs(o.inputs(t, c, c.sys)),
			"the proof of the program of heights %v should verify under its key and heights", logHeights(c.sys))
	}
}

// TestVerifierHeights_Rejects: a proof is rejected under another program's
// key, under another height of any of its modules, and under a height below
// the one the module's openings need.
func TestVerifierHeights_Rejects(t *testing.T) {
	a, b, c := sizedProgram(t, 0), sizedProgram(t, 10), sizedProgram(t, 40)
	o := newHeightsOuter(t, a.sys)
	require.NoError(t, o.runInputs(o.inputs(t, b, b.sys)), "the honest proof should verify")

	t.Run("another key", func(t *testing.T) {
		in := o.inputs(t, b, b.sys)
		in[0] = Cell(c.sys.PrecomputedCommitment)
		require.Error(t, o.runInputs(in), "a proof under another program's key should be rejected")
	})
	for k := range o.numHeights {
		for name, other := range map[string]int{"another program's": logHeights(c.sys)[k], "one more": logHeights(b.sys)[k] + 1, "one": 1} {
			if other == logHeights(b.sys)[k] {
				continue
			}
			t.Run(fmt.Sprintf("module %d under %s height", k, name), func(t *testing.T) {
				in := o.inputs(t, b, b.sys)
				in[1+k] = ScalarCell(field.NewElement(uint64(other)))
				require.Error(t, o.runInputs(in), "module %d of height 2^%d should be rejected under 2^%d",
					k, logHeights(b.sys)[k], other)
			})
		}
	}
}

// TestVerifierHeights_InvalidArguments: the verifier fails closed on height
// cells it cannot use, and Inputs on a system it does not verify.
func TestVerifierHeights_InvalidArguments(t *testing.T) {
	a, b := sizedProgram(t, 0), sizedProgram(t, 10)

	t.Run("wrong number of heights", func(t *testing.T) {
		m := &Machine{}
		_, err := NewVerifierWith(m, a.sys, VerifierOptions{Heights: []int{m.InputScalar()}})
		require.ErrorContains(t, err, "height cells", "one height cell for several modules should be rejected")
	})
	t.Run("static opening", func(t *testing.T) {
		c := machineInner(t) // the default PCS configuration
		m := &Machine{}
		heights := make([]int, len(c.sys.Modules))
		for k := range heights {
			heights[k] = m.InputScalar()
		}
		_, err := NewVerifierWith(m, c.sys, VerifierOptions{Heights: heights})
		require.ErrorContains(t, err, "size-oblivious", "heights as cells need a size-oblivious opening")
	})
	t.Run("another system without heights", func(t *testing.T) {
		v, err := NewVerifier(&Machine{}, a.sys)
		require.NoError(t, err, "the verifier should build")
		_, err = v.InputsFor(b.sys, b.proof, b.pub)
		require.ErrorContains(t, err, "at its sizes", "a verifier with constant sizes verifies its system only")
	})
	t.Run("another structure", func(t *testing.T) {
		o := newHeightsOuter(t, a.sys)
		s := newObliviousSystem(t)
		c := s.prove(t, 8, 4, "")
		_, err := o.v.InputsFor(c.sys, c.proof, c.pub)
		require.ErrorContains(t, err, "want", "a system of another structure should be rejected")
	})
}
