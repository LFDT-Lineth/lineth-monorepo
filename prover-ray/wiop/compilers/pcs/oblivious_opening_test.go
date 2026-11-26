package pcs

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// obliviousTestSystem is a size-oblivious system with two dynamic modules (a,
// b) and a sized one (big), columns in two committed rounds, base and
// extension columns, shifts ±1, and a column opened twice at a shift.
type obliviousTestSystem struct {
	sys                *wiop.System
	a, b               *wiop.Module
	a0, a1, a2, b0, b1 *wiop.Column
	big                *wiop.Column
	dupA0, dupA1       *wiop.Cell // the two claims of a0 at shift −1
}

func newObliviousTestSystem(t *testing.T) obliviousTestSystem {
	t.Helper()
	sys := wiop.NewSystemf("pcs-oblivious")
	r0, r1, r2 := sys.NewRound(), sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	s := obliviousTestSystem{sys: sys}
	s.a = sys.NewDynamicModule(ctx.Childf("a"), wiop.PaddingDirectionRight)
	s.b = sys.NewDynamicModule(ctx.Childf("b"), wiop.PaddingDirectionRight)
	bigMod := sys.NewSizedModule(ctx.Childf("big"), 16, wiop.PaddingDirectionNone)
	s.b0 = s.b.NewColumn(ctx.Childf("b0"), r0)
	s.a0 = s.a.NewColumn(ctx.Childf("a0"), r0)
	s.big = bigMod.NewColumn(ctx.Childf("big"), r0)
	s.a1 = s.a.NewExtensionColumn(ctx.Childf("a1"), r0)
	s.b1 = s.b.NewColumn(ctx.Childf("b1"), r0)
	s.a2 = s.a.NewColumn(ctx.Childf("a2"), r1)
	zeta := r2.NewCoinField(ctx.Childf("zeta"))
	le := sys.NewLagrangeEval(ctx.Childf("le"), []*wiop.ColumnView{
		s.a0.View().Shift(-1), s.b0.View(), s.a1.View().Shift(1), s.big.View(), s.a0.View(),
		s.b1.View().Shift(1), s.a2.View(), s.a0.View().Shift(-1), s.big.View().Shift(-1),
	}, zeta)
	s.dupA0, s.dupA1 = le.EvaluationClaims[0], le.EvaluationClaims[7]
	r1.RegisterAction(assignDyn{s.a2, s.a0})
	r2.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys, WithSizeOblivious(), WithFRINumQueries(2))
	return s
}

// assignDyn assigns col with the size of ref's module, a dynamic module's.
type assignDyn struct{ col, ref *wiop.Column }

func (a assignDyn) Run(rt *wiop.Runtime) {
	rt.AssignColumn(a.col, varyingVec(a.ref.Module.RuntimeSize(rt), 9))
}

func (s obliviousTestSystem) prove(t *testing.T, check wiop.ProverAction, na, nb int) {
	t.Helper()
	last := s.sys.Rounds[len(s.sys.Rounds)-1]
	saved := last.ProverActions
	last.ProverActions = append([]wiop.ProverAction{check}, saved...)
	defer func() { last.ProverActions = saved }()
	proof, pub := s.sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(s.a0, varyingVec(na, 1))
		rt.AssignColumn(s.a1, varyingExtVec(na, 2))
		rt.AssignColumn(s.b0, varyingVec(nb, 3))
		rt.AssignColumn(s.b1, varyingVec(nb, 4))
		rt.AssignColumn(s.big, varyingVec(16, 5))
	})
	require.NoError(t, s.sys.Verify(proof, pub), "the honest proof at sizes %d, %d should verify", na, nb)
}

// compareOblivious is a prover action, on the opening round, comparing the
// oblivious structure, laid out at the runtime's sizes, with what the prover
// commits and opens: the shapes, the shifts and claims RecoverBatchClaims
// takes from the runtime, and the powers of the plan of those.
type compareOblivious struct {
	t  *testing.T
	c  *compiled
	op *ObliviousOpening
}

func (a compareOblivious) Run(rt *wiop.Runtime) {
	t, op := a.t, a.op
	wantShifts, claims, wantShapes, _, err := RecoverBatchClaims(rt, op.Batches, allPresentManifests(op.Batches))
	require.NoError(t, err, "the runtime's claims")
	shapes, shifts := op.layout(func(m *wiop.Module) int { return m.RuntimeSize(rt) })
	require.Equal(t, wantShapes, shapes, "the shapes")
	require.Equal(t, wantShifts, shifts, "the shifts")

	plan, err := a.c.newPCS().VerifierPlan(shapes, shifts)
	require.NoError(t, err, "the plan at the runtime's sizes")
	require.Equal(t, op.NumRounds, plan.NumRounds, "rounds")
	require.Equal(t, op.LogCodewordSize, plan.LogCodewordSize, "codeword size")
	require.Equal(t, op.Generators, plan.Generators, "generators")
	powers := map[[3]int]int{} // (batch, size, row of the size, ext as 1) → power
	for _, l := range plan.Levels {
		require.Equal(t, op.RunningPower, l.RunningPower, "the running power of size %d", l.SizeLog2)
		require.Equal(t, op.MaxSizeLog2-l.SizeLog2, l.Round, "the round of size %d", l.SizeLog2)
		for _, e := range l.Entries {
			k := e.Row
			if e.IsExt {
				k = -1 - e.Row
			}
			powers[[3]int{e.Batch, l.SizeLog2, k}] = e.Power
		}
	}

	for b, groups := range op.Groups {
		next := map[int]fri.SizedShape{} // rows of a size placed so far
		for _, g := range groups {
			s := log2(g.Module.RuntimeSize(rt))
			placed := next[s]
			for ext, cols := range [][]OpenedColumn{g.Base, g.Ext} {
				for i, col := range cols {
					row, k := placed.BaseWidth+i, placed.BaseWidth+i
					vals := claims[b][s].Base[row]
					power := g.Power + i
					if ext == 1 {
						row, k = placed.ExtWidth+i, -1-placed.ExtWidth-i
						vals = claims[b][s].Ext[row]
						power += len(g.Base)
					}
					require.Equal(t, power, powers[[3]int{b, s, k}], "the power of %s", col.Column.Context.Path())
					require.Len(t, col.Claims, len(vals), "the claims of %s", col.Column.Context.Path())
					for j, c := range col.Claims {
						require.Equal(t, vals[j], rt.GetCellValue(c).AsExt(), "claim %d of %s", j, col.Column.Context.Path())
					}
				}
			}
			placed.BaseWidth += len(g.Base)
			placed.ExtWidth += len(g.Ext)
			next[s] = placed
		}
	}
}

func log2(n int) int {
	k := 0
	for 1<<k < n {
		k++
	}
	return k
}

// TestObliviousOpening: the oblivious structure, laid out at the sizes of a
// proof, is the layout the prover commits and opens, whatever the sizes.
func TestObliviousOpening(t *testing.T) {
	s := newObliviousTestSystem(t)
	action := openingAction(s.sys)
	op, err := action.Oblivious(s.sys)
	require.NoError(t, err, "the oblivious opening")

	require.Len(t, op.Batches, 2, "two committed rounds")
	require.Equal(t, [][2]*wiop.Cell{{s.dupA0, s.dupA1}}, op.Duplicates, "a0's repeated opening at shift −1")
	require.Len(t, op.Groups[0], 3, "a, b and big in round 0")
	require.Len(t, op.Groups[1], 1, "a in round 1")
	require.Equal(t, 6, op.RunningPower, "six committed columns")
	require.Equal(t, []int{-1, 0}, op.Groups[0][0].Base[0].Shifts, "a0's shifts, as declared")
	require.Equal(t, 1, op.MinSizeLog2, "the smallest folded size")
	require.Equal(t, int(action.c.maxSizeLog2), op.MaxSizeLog2, "the fixed domain")

	for _, sz := range [][2]int{{4, 4}, {8, 4}, {4, 32}, {16, 16}} {
		t.Run(fmt.Sprintf("a=%d,b=%d", sz[0], sz[1]), func(t *testing.T) {
			s.prove(t, compareOblivious{t, action.c, op}, sz[0], sz[1])
		})
	}
}

// TestObliviousOpening_Rejects: Oblivious fails on an opening that is not
// size-oblivious, and on a sized module whose rule differs from a dynamic
// one's.
func TestObliviousOpening_Rejects(t *testing.T) {
	s := newStaticTestSystem()
	_, err := openingAction(s.sys).Oblivious(s.sys)
	require.ErrorContains(t, err, "not size-oblivious", "the default configuration")

	for name, size := range map[string]int{"unfolded": 1, "too small for its shifts": 2} {
		sys := wiop.NewSystemf("pcs-oblivious-small")
		r0, r1 := sys.NewRound(), sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), r0)
		zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
		le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View(), col.View().Shift(1)}, zeta)
		r1.RegisterAction(&selfAssignLagrange{le: le})
		Compile(sys, WithSizeOblivious())
		_, err = openingAction(sys).Oblivious(sys)
		require.ErrorContains(t, err, "below the", "a sized module %s", name)
	}
}
