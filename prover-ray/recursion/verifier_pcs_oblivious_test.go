package recursion

import (
	"math/bits"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// oblOpts compiles a system size-oblivious, with the tests' query count.
var oblOpts = []compilers.Option{compilers.ConfigurePCS(pcs.WithFRINumQueries(2), pcs.WithSizeOblivious())}

// oblMachineInner is a proof of a machine program compiled size-oblivious.
func oblMachineInner(t *testing.T) innerCase {
	t.Helper()
	m, _ := publicProgram()
	p, err := m.Compile(oblOpts...)
	require.NoError(t, err)
	r, err := p.Execute(publicInputs())
	require.NoError(t, err)
	proof, pub := p.Prove(r)
	require.NoError(t, p.Verify(proof, pub))
	return innerCase{"machine", p.Sys, proof, pub}
}

// TestOblivious_Machine: the machine verifies a proof of a machine program
// compiled size-oblivious, its sizes constants of the verifier.
func TestOblivious_Machine(t *testing.T) {
	c := oblMachineInner(t)
	_, err := pcsOuter(t, c).run(c.proof, c.pub)
	require.NoError(t, err, "the honest proof should verify in the machine")
}

// TestOblivious_Dynamic: one machine verifier checks the proofs of every size
// of a system with dynamic modules compiled size-oblivious, PCS opening
// included.
func TestOblivious_Dynamic(t *testing.T) {
	d := newDynamicSystemWith(t, 0, oblOpts...)
	m := &Machine{}
	v, err := NewVerifier(m, d.sys)
	require.NoError(t, err, "the verifier should build")
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	o := outer{v, p}
	for _, sz := range [][2]int{{4, 4}, {8, 4}, {16, 8}, {4, 8}, {64, 4}} {
		c := d.prove(t, sz[0], sz[1])
		t.Run(c.name, func(t *testing.T) {
			require.NoError(t, c.sys.Verify(c.proof, c.pub), "the honest proof should verify natively")
			_, err := o.run(c.proof, c.pub)
			require.NoError(t, err, "the honest proof should verify in the machine")
		})
	}
}

// requireInputsBound checks that every input of an honest proof, changed in
// turn at lane 0, is rejected, but the siblings above a tree's height, which
// the proof leaves free and whose change must be accepted. It checks one
// input out of step.
func requireInputsBound(t *testing.T, o outer, c innerCase, step int) {
	t.Helper()
	in, err := o.v.Inputs(c.proof, c.pub)
	require.NoError(t, err, "the honest proof's inputs")
	oa, err := o.v.obl.assign(c.sys, c.proof)
	require.NoError(t, err)
	free, bound := 0, 0
	for k := 0; k < len(in); k += step {
		bad := slices.Clone(in)
		bad[k][0].Add(&bad[k][0], new(field.Element).SetOne())
		err := o.runInputs(bad)
		if f := o.v.inputs[k].free; f != nil && f(oa) {
			require.NoError(t, err, "input %d is a sibling above its tree: free", k)
			free++
			continue
		}
		require.Error(t, err, "input %d changed should be rejected", k)
		bound++
	}
	t.Logf("%d inputs: %d bound, %d free (checked 1 in %d)", len(in), bound, free, step)
}

// TestOblivious_Rejects: every input of an honest proof is bound.
func TestOblivious_Rejects(t *testing.T) {
	t.Run("machine", func(t *testing.T) {
		c := oblMachineInner(t)
		requireInputsBound(t, pcsOuter(t, c), c, 1)
	})
	t.Run("dynamic", func(t *testing.T) {
		d := newDynamicSystemWith(t, 0, oblOpts...)
		c := d.prove(t, 8, 4)
		requireInputsBound(t, pcsOuter(t, c), c, 1)
	})
}

// oblShapesProof is shapesProof compiled size-oblivious, without the module
// of one row: two sized modules of different sizes, in different rounds. With
// lie set to a column's name ("a" or "s"), the prover commits to the column,
// then replaces it with another witness: only the PCS opening rejects it.
func oblShapesProof(t *testing.T, lie string) innerCase {
	t.Helper()
	sys := wiop.NewSystemf("shapes")
	r0, r1 := sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	big := sys.NewSizedModule(ctx.Childf("big"), 16, wiop.PaddingDirectionNone)
	small := sys.NewSizedModule(ctx.Childf("small"), 4, wiop.PaddingDirectionNone)
	a := big.NewColumn(ctx.Childf("a"), r0)
	s := small.NewColumn(ctx.Childf("s"), r1)
	big.NewVanishingManual(ctx.Childf("step"),
		wiop.Sub(a.View().Shift(1), wiop.Add(a.View(), wiop.NewConstantField(field.One()))), -1)
	small.NewVanishing(ctx.Childf("bool"), wiop.Sub(s.View(), wiop.Mul(s.View(), s.View())))

	r1.RegisterAction(assignColumn{s, func(*wiop.Runtime) *wiop.ConcreteVector { return vec(0, 0, 0, 0) }})
	require.NoError(t, compilers.CompileFull(sys, oblOpts...), "the inner system should compile")
	switch lie {
	case "a":
		r0.RegisterAction(assignColumn{a, func(*wiop.Runtime) *wiop.ConcreteVector {
			return vec(6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21)
		}})
	case "s":
		r1.RegisterAction(assignColumn{s, func(*wiop.Runtime) *wiop.ConcreteVector { return vec(1, 0, 1, 1) }})
	}
	if lie != "" {
		cheatOpening(t, sys)
	}
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(a, vec(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16))
	})
	if lie == "" {
		require.NoError(t, sys.Verify(proof, pub), "the honest shapes proof should verify natively")
	}
	return innerCase{"shapes", sys, proof, pub}
}

// TestOblivious_RejectsForged: proofs whose claims are not the committed
// columns', which only the PCS opening rejects natively: every Merkle path
// holds, and the folds do not.
func TestOblivious_RejectsForged(t *testing.T) {
	c := oblShapesProof(t, "")
	_, err := pcsOuter(t, c).run(c.proof, c.pub)
	require.NoError(t, err, "the honest shapes proof should verify in the machine")
	for _, col := range []string{"a", "s"} {
		t.Run(col, func(t *testing.T) {
			c := oblShapesProof(t, col)
			require.Empty(t, nativeFailures(c.sys, c.proof, c.pub), "the other actions accept")
			require.ErrorContains(t, c.sys.Verify(c.proof, c.pub), "pcs.Verify")
			o := pcsOuter(t, c)
			in, err := o.v.Inputs(c.proof, c.pub)
			require.NoError(t, err, "the forged proof's inputs")
			r, err := o.p.Execute(in)
			require.NoError(t, err, "the Merkle paths hold")
			require.Error(t, proveAndVerify(o.p, r), "the forged claims of %s should be rejected", col)
		})
	}
}

// sizedPublicProgram is publicProgram with extra more multiplications of
// extra more inputs: the same chips, columns and public inputs, at other
// heights. It returns its inputs too.
func sizedPublicProgram(extra int) (*Machine, []Cell) {
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
	return m, in
}

// keyed is a proof of a program, with what a verifier taking the key and the
// heights as data reads of its system.
type keyed struct {
	sys   *wiop.System
	proof wiop.Proof
	pub   wiop.PublicInput
}

func proveSized(t *testing.T, extra int) keyed {
	t.Helper()
	m, in := sizedPublicProgram(extra)
	p, err := m.Compile(oblOpts...)
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	proof, pub := p.Prove(r)
	require.NoError(t, p.Verify(proof, pub))
	return keyed{p.Sys, proof, pub}
}

// logHeights returns the log sizes of a system's modules.
func logHeights(sys *wiop.System) []int {
	res := make([]int, len(sys.Modules))
	for k, mod := range sys.Modules {
		res[k] = bits.TrailingZeros(uint(mod.Size()))
	}
	return res
}

// TestOblivious_Heights: one verifier, built from a program, with the key
// and the modules' heights as cells, verifies the proofs of programs with the
// same chips at other heights, under their keys and heights, and rejects a
// proof under another program's key or heights.
func TestOblivious_Heights(t *testing.T) {
	a, b, c := proveSized(t, 0), proveSized(t, 10), proveSized(t, 40)
	require.NotEqual(t, logHeights(a.sys), logHeights(b.sys), "the programs should differ in a height")
	require.NotEqual(t, a.sys.PrecomputedCommitment, b.sys.PrecomputedCommitment, "the programs have their own keys")

	m := &Machine{}
	key := m.InputDigest()
	heights := make([]int, len(a.sys.Modules))
	for k := range heights {
		heights[k] = m.InputScalar()
	}
	v, err := NewVerifierWith(m, a.sys, VerifierOptions{Key: key, HasKey: true, Heights: heights})
	require.NoError(t, err, "the verifier should build")
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	o := outer{v, p}

	// inputs lays out a proof of x under the key and heights of under.
	inputs := func(x, under keyed) []Cell {
		in := []Cell{Cell(under.sys.PrecomputedCommitment)}
		for _, l := range logHeights(under.sys) {
			in = append(in, ScalarCell(field.NewElement(uint64(l))))
		}
		rest, err := v.InputsFor(x.sys, x.proof, x.pub)
		require.NoError(t, err)
		return append(in, rest...)
	}
	for name, x := range map[string]keyed{"a": a, "b": b, "c": c} {
		t.Logf("program %s: heights %v", name, logHeights(x.sys))
		require.NoError(t, o.runInputs(inputs(x, x)), "the proof of %s should verify under its key and heights", name)
	}

	// b's proof under a's key, with b's heights
	in := inputs(b, b)
	in[0] = Cell(a.sys.PrecomputedCommitment)
	require.Error(t, o.runInputs(in), "a proof under another program's key should be rejected")
	// b's proof under its key, with one height of another program in turn
	for k := range heights {
		in := inputs(b, b)
		if in[1+k] == ScalarCell(field.NewElement(uint64(logHeights(c.sys)[k]))) {
			continue
		}
		in[1+k] = ScalarCell(field.NewElement(uint64(logHeights(c.sys)[k])))
		require.Error(t, o.runInputs(in), "a proof under another height of module %d should be rejected", k)
	}
}
