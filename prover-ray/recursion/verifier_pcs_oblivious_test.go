package recursion

import (
	"fmt"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// obliviousSystem is a size-oblivious system (pcs.WithSizeOblivious) whose
// opening has the shapes the machine routes as data:
//   - dyn (columns a, d; a dynamic module): a[i+1] = a[i] + 1 but on the last
//     row (shifts 0 and 1), d = a² committed in the second round;
//   - flags (columns f, g; dynamic): f boolean, g = 3·f an extension column;
//   - table (columns p, c; 8 rows): p precomputed, c = p².
//
// Its batches are the two rounds and the precomputed one; the modules'
// sizes order them in the trees, and decide the rounds they join FRI at.
type obliviousSystem struct {
	sys            *wiop.System
	a, d, f, g, c  *wiop.Column
	r0, r1         *wiop.Round
	dynMod, flagMd *wiop.Module
}

func newObliviousSystem(t *testing.T) obliviousSystem {
	t.Helper()
	sys := wiop.NewSystemf("oblivious")
	r0, r1 := sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	s := obliviousSystem{sys: sys, r0: r0, r1: r1}
	s.dynMod = sys.NewDynamicModule(ctx.Childf("dyn"), wiop.PaddingDirectionRight)
	s.flagMd = sys.NewDynamicModule(ctx.Childf("flags"), wiop.PaddingDirectionRight)
	table := sys.NewSizedModule(ctx.Childf("table"), 8, wiop.PaddingDirectionNone)
	s.a = s.dynMod.NewColumn(ctx.Childf("a"), r0)
	s.f = s.flagMd.NewColumn(ctx.Childf("f"), r0)
	s.g = s.flagMd.NewExtensionColumn(ctx.Childf("g"), r0)
	p := table.NewPrecomputedColumn(ctx.Childf("p"), vec(1, 2, 3, 4, 5, 6, 7, 8))
	s.c = table.NewColumn(ctx.Childf("c"), r0)
	s.d = s.dynMod.NewColumn(ctx.Childf("d"), r1)

	s.dynMod.NewVanishingManual(ctx.Childf("step"),
		wiop.Sub(s.a.View().Shift(1), wiop.Add(s.a.View(), wiop.NewConstantField(field.One()))), -1)
	s.dynMod.NewVanishing(ctx.Childf("square"), wiop.Sub(s.d.View(), wiop.Mul(s.a.View(), s.a.View())))
	s.flagMd.NewVanishing(ctx.Childf("bool"), wiop.Sub(s.f.View(), wiop.Mul(s.f.View(), s.f.View())))
	s.flagMd.NewVanishing(ctx.Childf("triple"),
		wiop.Sub(s.g.View(), wiop.Mul(wiop.NewConstantField(field.NewElement(3)), s.f.View())))
	table.NewVanishing(ctx.Childf("table"), wiop.Sub(s.c.View(), wiop.Mul(p.View(), p.View())))
	r1.RegisterAction(assignColumn{s.d, func(rt *wiop.Runtime) *wiop.ConcreteVector {
		n := s.dynMod.RuntimeSize(rt)
		vals := make([]uint64, n)
		for i := range vals {
			vals[i] = uint64((i + 1) * (i + 1))
		}
		return vec(vals...)
	}})
	require.NoError(t, compilers.CompileFull(sys,
		compilers.ConfigurePCS(pcs.WithSizeOblivious(), pcs.WithFRINumQueries(2))), "the oblivious system should compile")
	return s
}

// outer builds the machine verifier of the system's proofs, PCS opening
// included.
func (s obliviousSystem) outer(t *testing.T) outer {
	t.Helper()
	return pcsOuter(t, innerCase{s.sys.Context.Path(), s.sys, wiop.Proof{}, nil})
}

// prove proves the system with dyn of size na and flags of size nf. With lie
// set to a column's name ("a", "f" or "c"), the prover commits to the
// column, then replaces it, with values for which the constraints still
// hold: the claims are the replacement's, which only the PCS opening checks.
func (s obliviousSystem) prove(t *testing.T, na, nf int, lie string) innerCase {
	t.Helper()
	a, f, g := make([]uint64, na), make([]uint64, nf), make([]field.Ext, nf)
	for i := range a {
		a[i] = uint64(i + 1)
	}
	for i := range f {
		f[i] = uint64(i % 2)
		g[i] = field.Lift(field.NewElement(3 * f[i]))
	}
	last := s.sys.Rounds[len(s.sys.Rounds)-1]
	saved := [3][]wiop.ProverAction{s.r0.ProverActions, s.r1.ProverActions, last.ProverActions}
	defer func() {
		s.r0.ProverActions, s.r1.ProverActions, last.ProverActions = saved[0], saved[1], saved[2]
	}()
	replace := func(r *wiop.Round, col *wiop.Column, val func(rt *wiop.Runtime) *wiop.ConcreteVector) {
		r.ProverActions = append(slices.Clone(r.ProverActions), assignColumn{col, val})
	}
	switch lie {
	case "a":
		// a + 5 steps by one as a does, and d follows it
		replace(s.r0, s.a, func(*wiop.Runtime) *wiop.ConcreteVector {
			b := make([]uint64, na)
			for i := range b {
				b[i] = a[i] + 5
			}
			return vec(b...)
		})
		replace(s.r1, s.d, func(*wiop.Runtime) *wiop.ConcreteVector {
			d := make([]uint64, na)
			for i := range d {
				d[i] = (a[i] + 5) * (a[i] + 5)
			}
			return vec(d...)
		})
	case "f":
		replace(s.r0, s.f, func(*wiop.Runtime) *wiop.ConcreteVector {
			b := slices.Clone(f)
			b[0] = 1 - b[0]
			return vec(b...)
		})
		replace(s.r0, s.g, func(*wiop.Runtime) *wiop.ConcreteVector {
			h := slices.Clone(g)
			h[0] = field.Lift(field.NewElement(3 * (1 - f[0])))
			return &wiop.ConcreteVector{Plain: field.VecFromExt(h)}
		})
	case "c":
		replace(s.r0, s.c, func(*wiop.Runtime) *wiop.ConcreteVector { return vec(1, 4, 9, 16, 25, 36, 49, 64) })
	}
	if lie != "" {
		cheatOpening(t, s.sys)
	}
	proof, pub := s.sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(s.a, vec(a...))
		rt.AssignColumn(s.f, vec(f...))
		rt.AssignColumn(s.g, &wiop.ConcreteVector{Plain: field.VecFromExt(g)})
		c := []uint64{1, 4, 9, 16, 25, 36, 49, 64}
		if lie == "c" {
			c[0] = 2 // committed, then replaced by the honest values
		}
		rt.AssignColumn(s.c, vec(c...))
	})
	name := fmt.Sprintf("dyn=%d,flags=%d", na, nf)
	if lie == "" {
		require.NoError(t, s.sys.Verify(proof, pub), "the honest %s proof should verify natively", name)
	}
	return innerCase{name, s.sys, proof, pub}
}

// obliviousSizes are the size combinations of the tests: the modules of
// different sizes, of the same one, and their order swapped.
var obliviousSizes = [][2]int{{4, 2}, {8, 8}, {4, 16}, {32, 4}}

// TestVerifierOblivious: one machine verifier checks the proofs of a
// size-oblivious system at every size, PCS opening included; and the proof of
// a machine program compiled size-oblivious, all of whose modules are sized.
func TestVerifierOblivious(t *testing.T) {
	s := newObliviousSystem(t)
	o := s.outer(t)
	for _, sz := range obliviousSizes {
		c := s.prove(t, sz[0], sz[1], "")
		t.Run(c.name, func(t *testing.T) {
			_, err := o.run(c.proof, c.pub)
			require.NoError(t, err, "the honest %s proof should verify in the machine", c.name)
		})
	}

	t.Run("machine-oblivious", func(t *testing.T) {
		m, _ := publicProgram()
		p, err := m.Compile(append(slices.Clone(testPCSOpts), compilers.ConfigurePCS(pcs.WithSizeOblivious()))...)
		require.NoError(t, err, "the program should compile size-oblivious")
		r, err := p.Execute(publicInputs())
		require.NoError(t, err, "the program's execution")
		proof, pub := p.Prove(r)
		require.NoError(t, p.Verify(proof, pub), "the program's proof should verify natively")
		c := innerCase{"machine-oblivious", p.Sys, proof, pub}
		_, err = pcsOuter(t, c).run(c.proof, c.pub)
		require.NoError(t, err, "the program's proof should verify in the machine")
	})
}

// TestVerifierOblivious_Rejects: every input of an honest proof, changed in
// turn, is rejected, but the siblings of dynamic paths above their trees'
// heights, which no check reads: changing those is accepted.
func TestVerifierOblivious_Rejects(t *testing.T) {
	s := newObliviousSystem(t)
	o := s.outer(t)
	c := s.prove(t, 8, 4, "")
	in, err := o.v.Inputs(c.proof, c.pub)
	require.NoError(t, err, "the honest proof's inputs")
	ob := o.v.pcs.(*obliviousPCS)
	lay, err := ob.prepare(c.proof)
	require.NoError(t, err, "the honest proof's layout")
	free := 0
	for k := range in {
		if testing.Short() && k%8 != 0 {
			continue
		}
		bad := slices.Clone(in)
		bad[k][0].Add(&bad[k][0], new(field.Element).SetOne())
		if s, ok := ob.siblings[k]; ok {
			if q, b, h := s[0], s[1], s[2]; h > len(c.proof.PCSOpeningProof.InputQueries[q][lay.ob.treeOf[b]].Siblings) {
				require.NoError(t, o.runInputs(bad), "input %d, a sibling above its tree, is not read", k)
				free++
				continue
			}
		}
		require.Error(t, o.runInputs(bad), "input %d changed should be rejected", k)
	}
	require.Positive(t, free, "siblings above a tree")
}

// TestVerifierOblivious_RejectsForged: proofs whose claims are not the
// committed columns', which only the PCS opening rejects natively: of each
// committed module (two dynamic ones, in two batches, and a sized one), at
// sizes that put them at different rounds.
func TestVerifierOblivious_RejectsForged(t *testing.T) {
	s := newObliviousSystem(t)
	o := s.outer(t)
	for _, lie := range []string{"a", "f", "c"} {
		for _, sz := range [][2]int{{4, 16}, {32, 4}} {
			c := s.prove(t, sz[0], sz[1], lie)
			t.Run(lie+"/"+c.name, func(t *testing.T) {
				require.Empty(t, nativeFailures(c.sys, c.proof, c.pub), "the other actions accept")
				require.ErrorContains(t, c.sys.Verify(c.proof, c.pub), "pcs.Verify", "the PCS opening rejects")
				in, err := o.v.Inputs(c.proof, c.pub)
				require.NoError(t, err, "the forged proof's inputs")
				r, err := o.p.Execute(in)
				require.NoError(t, err, "the Merkle paths hold")
				require.Error(t, proveAndVerify(o.p, r), "the forged claims of %s should be rejected", lie)
			})
		}
	}
}

// TestVerifierOblivious_RejectsOrder: the modules' order π is checked: two
// modules of the same round swapped give the same sums per round, which only
// the order check rejects.
func TestVerifierOblivious_RejectsOrder(t *testing.T) {
	s := newObliviousSystem(t)
	o := s.outer(t)
	c := s.prove(t, 8, 8, "") // dyn, flags and table: one round
	in, err := o.v.Inputs(c.proof, c.pub)
	require.NoError(t, err, "the honest proof's inputs")
	ob := o.v.pcs.(*obliviousPCS)
	require.Len(t, ob.orderIn, 3, "three committed modules")
	bad := slices.Clone(in)
	bad[ob.orderIn[0]], bad[ob.orderIn[1]] = bad[ob.orderIn[1]], bad[ob.orderIn[0]]
	require.Error(t, o.runInputs(bad), "the modules swapped in π should be rejected")
}
