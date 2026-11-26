package recursion

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// dynLimit is the row limit of the extra permutation row-limit action of the
// dynamic fixture: a size of perm of 16 or more breaks it.
const dynLimit = 9

// dynamicSystem is a compiled system with two dynamic modules and a sized
// one, whose sizes the proofs choose:
//   - dyn (columns a, b, t): b = a², a[i+1] = a[i] + 1 but on the last row, a
//     starting at the cell c (a Lagrange selector at row 0) and b ending at the
//     cell last (at row −1), a ⊆ t;
//   - perm (columns pa, pb): pa a permutation of pb, with an extra row limit of
//     dynLimit on both sides;
//   - small (column s, 4 rows): s ⊆ t, a lookup from a sized module into a
//     dynamic one.
//
// The sizes are absorbed from round sizeRound on.
type dynamicSystem struct {
	sys                *wiop.System
	q                  *wiop.TableRelationQuery
	dyn, perm          *wiop.Module
	a, b, t, pa, pb, s *wiop.Column
	c, last            *wiop.Cell
}

func newDynamicSystem(t *testing.T, sizeRound int) dynamicSystem {
	t.Helper()
	return newDynamicSystemWith(t, sizeRound, compilers.ConfigurePCS(pcs.WithFRINumQueries(2)))
}

// newDynamicSystemWith is newDynamicSystem compiled with opts.
func newDynamicSystemWith(t *testing.T, sizeRound int, opts ...compilers.Option) dynamicSystem {
	t.Helper()
	sys := wiop.NewSystemf("dynamic")
	sys.DynamicSizeRound = sizeRound
	r0, r1 := sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	d := dynamicSystem{sys: sys}
	d.dyn = sys.NewDynamicModule(ctx.Childf("dyn"), wiop.PaddingDirectionRight)
	d.perm = sys.NewDynamicModule(ctx.Childf("perm"), wiop.PaddingDirectionRight)
	small := sys.NewSizedModule(ctx.Childf("small"), 4, wiop.PaddingDirectionNone)
	d.a, d.b, d.t = d.dyn.NewColumn(ctx.Childf("a"), r0), d.dyn.NewColumn(ctx.Childf("b"), r0), d.dyn.NewColumn(ctx.Childf("t"), r0)
	d.pa, d.pb = d.perm.NewColumn(ctx.Childf("pa"), r0), d.perm.NewColumn(ctx.Childf("pb"), r0)
	d.s = small.NewColumn(ctx.Childf("s"), r0)
	d.c, d.last = r0.NewCell(ctx.Childf("c"), false), r0.NewCell(ctx.Childf("last"), false)
	coin := r1.NewCoinField(ctx.Childf("coin"))

	d.dyn.NewVanishing(ctx.Childf("square"), wiop.Sub(d.b.View(), wiop.Mul(d.a.View(), d.a.View())))
	d.dyn.NewVanishingManual(ctx.Childf("step"),
		wiop.Sub(d.a.View().Shift(1), wiop.Add(d.a.View(), wiop.NewConstantField(field.One()))), -1)
	d.dyn.NewLocalConstraint(ctx.Childf("first"), wiop.Mul(coin, wiop.Sub(d.a.View(), d.c)), 0)
	// Local constraints at the end of a dynamic module are not supported:
	// the selector of the last row, in a vanishing.
	d.dyn.NewVanishing(ctx.Childf("end"), wiop.Mul(wiop.NewLagrangeSelector(d.dyn, -1), wiop.Sub(d.b.View(), d.last)))
	sys.NewInclusion(ctx.Childf("lookup"), []wiop.Table{wiop.NewTable(d.a.View())}, []wiop.Table{wiop.NewTable(d.t.View())})
	sys.NewInclusion(ctx.Childf("small_lookup"), []wiop.Table{wiop.NewTable(d.s.View())}, []wiop.Table{wiop.NewTable(d.t.View())})
	d.q = sys.NewPermutation(ctx.Childf("perm"), []wiop.Table{wiop.NewTable(d.pa.View())}, []wiop.Table{wiop.NewTable(d.pb.View())})
	r0.RegisterVerifierAction(&grandproduct.RowLimitAction{Query: d.q, Limit: dynLimit})

	require.NoError(t, compilers.CompileFull(sys, opts...), "the dynamic system should compile")
	return d
}

// prove proves the system with dyn of size n and perm of size np.
func (d dynamicSystem) prove(t *testing.T, n, np int) innerCase {
	t.Helper()
	a, b, tab := make([]uint64, n), make([]uint64, n), make([]uint64, n)
	for i := range n {
		a[i], b[i], tab[i] = uint64(i+1), uint64((i+1)*(i+1)), uint64(n-i)
	}
	pa, pb := make([]uint64, np), make([]uint64, np)
	for i := range np {
		pa[i], pb[i] = uint64(10*(i+1)), uint64(10*(np-i))
	}
	proof, pub := d.sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(d.a, vec(a...))
		rt.AssignColumn(d.b, vec(b...))
		rt.AssignColumn(d.t, vec(tab...))
		rt.AssignColumn(d.pa, vec(pa...))
		rt.AssignColumn(d.pb, vec(pb...))
		rt.AssignColumn(d.s, vec(1, 2, 3, 4))
		rt.AssignCell(d.c, field.ElemFromBase(field.One()))
		rt.AssignCell(d.last, field.ElemFromBase(field.NewElement(b[n-1])))
	})
	return innerCase{fmt.Sprintf("dyn=%d,perm=%d", n, np), d.sys, proof, pub}
}

// TestVerifierDynamic: one machine verifier checks proofs of every size, and
// its coins are the native verifier's, with the sizes absorbed from round 0
// or from round 1.
func TestVerifierDynamic(t *testing.T) {
	for _, sizeRound := range []int{0, 1} {
		d := newDynamicSystem(t, sizeRound)
		o := newOuter(t, d.sys)
		for _, sz := range [][2]int{{4, 4}, {8, 4}, {16, 8}, {4, 8}} {
			c := d.prove(t, sz[0], sz[1])
			t.Run(fmt.Sprintf("sizeRound=%d/%s", sizeRound, c.name), func(t *testing.T) {
				require.NoError(t, c.sys.Verify(c.proof, c.pub), "the honest %s proof should verify natively", c.name)
				r, err := o.run(c.proof, c.pub)
				require.NoError(t, err, "the honest %s proof should verify in the machine", c.name)
				rt := nativeReplay(c.sys, c.proof, c.pub)
				for _, rd := range c.sys.Rounds {
					for _, coin := range rd.Coins {
						if a, ok := o.v.coins[coin.Context.ID]; ok {
							require.Equal(t, rt.GetCoinValue(coin).Ext, CellE6(r.Value(a)), "coin %s", coin.Context.Path())
						}
					}
				}
			})
		}
	}
}

// TestVerifierDynamic_Rejects: every changed cell and every other valid size
// is rejected iff the native verifier's actions (the PCS opening aside)
// reject it; sizes out of bounds are rejected.
func TestVerifierDynamic_Rejects(t *testing.T) {
	d := newDynamicSystem(t, 0)
	o := newOuter(t, d.sys)
	c := d.prove(t, 8, 4)
	dyn, perm := moduleIndex(d.sys, d.dyn), moduleIndex(d.sys, d.perm)

	type tamper struct {
		name string
		f    func(proof *wiop.Proof)
	}
	var tampers []tamper
	for id := range c.proof.Cells {
		tampers = append(tampers, tamper{d.sys.LookupCell(id).Context.Path(), func(p *wiop.Proof) {
			p.Cells[id] = plusOne(p.Cells[id])
		}})
	}
	for _, k := range []int{dyn, perm} {
		for _, n := range []int{4, 16, 32} {
			if n == c.proof.DynamicSizes[k] {
				continue
			}
			tampers = append(tampers, tamper{fmt.Sprintf("module %d of size %d", k, n), func(p *wiop.Proof) {
				p.DynamicSizes[k] = n
			}})
		}
	}
	if testing.Short() && len(tampers) > 12 {
		tampers = tampers[:12]
	}
	rejected := 0
	for _, tp := range tampers {
		proof := cloneProof(c.proof)
		tp.f(&proof)
		native := nativeCheck(c.sys, proof, c.pub)
		_, err := o.run(proof, c.pub)
		require.Equal(t, native != nil, err != nil, "%s: native %v, machine %v", tp.name, native, err)
		if err != nil {
			rejected++
		}
	}
	require.Positive(t, rejected, "some changes should be rejected")
	t.Logf("%d of %d changes rejected", rejected, len(tampers))

	// Sizes System.Verify rejects before any action: below the minimum
	// (shifts of ±1 need 4 rows), above the maximum, not a power of two, or
	// missing. The first two are the machine's to reject.
	for name, f := range map[string]func(p *wiop.Proof){
		"below the minimum":  func(p *wiop.Proof) { p.DynamicSizes[dyn] = 2 },
		"above the maximum":  func(p *wiop.Proof) { p.DynamicSizes[perm] = 2 * wiop.ColumnSizeMaxSupported },
		"not a power of two": func(p *wiop.Proof) { p.DynamicSizes[dyn] = 12 },
		"missing":            func(p *wiop.Proof) { delete(p.DynamicSizes, perm) },
		"a sized module":     func(p *wiop.Proof) { p.DynamicSizes[moduleIndex(d.sys, d.s.Module)] = 4 },
	} {
		proof := cloneProof(c.proof)
		f(&proof)
		require.Error(t, verifyRecovered(d.sys, proof, c.pub), "the native verifier should reject a size %s", name)
		_, err := o.run(proof, c.pub)
		require.Error(t, err, "the machine should reject a size %s", name)
	}
}

// TestVerifierDynamic_RowLimit: a proof whose perm module breaks the extra row
// limit is rejected, by that action alone; a limit that the smallest sizes
// reach is rejected when the verifier is built.
func TestVerifierDynamic_RowLimit(t *testing.T) {
	d := newDynamicSystem(t, 0)
	o := newOuter(t, d.sys)
	c := d.prove(t, 4, 16)
	require.Equal(t, []string{"*grandproduct.RowLimitAction"}, nativeFailures(c.sys, c.proof, c.pub), "only the row limit should reject it natively")
	_, err := o.run(c.proof, c.pub)
	require.Error(t, err, "the machine should reject a proof breaking the row limit")

	// perm has no shift: its smallest size is 1 row, which a limit of 1
	// already rejects.
	d = newDynamicSystem(t, 0)
	r0 := d.sys.Rounds[0]
	r0.VerifierActions = append(r0.VerifierActions, &grandproduct.RowLimitAction{Query: d.q, Limit: 1})
	_, err = newVerifier(&Machine{}, d.sys, VerifierOptions{}, true)
	require.ErrorContains(t, err, "at or above the limit 1", "a limit of 1 row")
}

// TestVerifierDynamic_LagrangeBound: a Lagrange selector at row 4 needs 8
// rows or more, which the shifts do not require (MinRuntimeSize is 1). The
// machine accepts a proof at 8 and 16 rows, and rejects the size 4, as the
// native verifier does. The column is constant and equal to c, so that its
// constraint holds at every size: only the selector's bound rejects 4 rows.
func TestVerifierDynamic_LagrangeBound(t *testing.T) {
	sys := wiop.NewSystemf("lagrange-bound")
	r0, r1 := sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	mod := sys.NewDynamicModule(ctx.Childf("mod"), wiop.PaddingDirectionRight)
	a := mod.NewColumn(ctx.Childf("a"), r0)
	c := r0.NewCell(ctx.Childf("c"), false)
	r1.NewCoinField(ctx.Childf("coin"))
	mod.NewVanishing(ctx.Childf("row4"), wiop.Mul(wiop.NewLagrangeSelector(mod, 4), wiop.Sub(a.View(), c)))
	require.NoError(t, compilers.CompileFull(sys, compilers.ConfigurePCS(pcs.WithFRINumQueries(2))), "the system should compile")
	require.Equal(t, 1, mod.MinRuntimeSize(), "no shift")
	o := newOuter(t, sys)

	for _, n := range []int{8, 16} {
		vals := make([]uint64, n)
		for i := range vals {
			vals[i] = 4
		}
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(a, vec(vals...))
			rt.AssignCell(c, field.ElemFromBase(field.NewElement(4)))
		})
		require.NoError(t, sys.Verify(proof, pub), "the honest proof at %d rows should verify natively", n)
		_, err := o.run(proof, pub)
		require.NoError(t, err, "the honest proof at %d rows should verify in the machine", n)

		if n == 8 {
			small := cloneProof(proof)
			small.DynamicSizes[moduleIndex(sys, mod)] = 4
			require.Error(t, verifyRecovered(sys, small, pub), "the native verifier should reject 4 rows")
			_, err = o.run(small, pub)
			require.Error(t, err, "the machine should reject 4 rows")
		}
	}
}

func moduleIndex(sys *wiop.System, mod *wiop.Module) int {
	for k, m := range sys.Modules {
		if m == mod {
			return k
		}
	}
	panic("no such module")
}

// cloneProof copies the maps of a proof that tests change.
func cloneProof(p wiop.Proof) wiop.Proof {
	res := p
	res.Cells = make(map[wiop.ObjectID]field.Gen, len(p.Cells))
	for k, x := range p.Cells {
		res.Cells[k] = x
	}
	res.DynamicSizes = make(map[int]int, len(p.DynamicSizes))
	for k, n := range p.DynamicSizes {
		res.DynamicSizes[k] = n
	}
	return res
}

// verifyRecovered is System.Verify, a panic returned as an error: Verify
// panics on a proof missing a dynamic size, in the transcript's replay, before
// it checks the sizes.
func verifyRecovered(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return sys.Verify(proof, pub)
}
