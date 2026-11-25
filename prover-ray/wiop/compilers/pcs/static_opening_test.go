package pcs

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// staticTestSystem is a system with two committed rounds, two sizes, a column
// opened at two shifts and opened twice at the same shift.
type staticTestSystem struct {
	sys          *wiop.System
	a, b, c      *wiop.Column
	dupA1, dupA2 *wiop.Cell // the two claims of a at shift 0
}

func newStaticTestSystem(opts ...Option) staticTestSystem {
	sys := wiop.NewSystemf("pcs-static")
	r0, r1, r2 := sys.NewRound(), sys.NewRound(), sys.NewRound()
	big := sys.NewSizedModule(sys.Context.Childf("big"), 8, wiop.PaddingDirectionNone)
	small := sys.NewSizedModule(sys.Context.Childf("small"), 4, wiop.PaddingDirectionNone)
	s := staticTestSystem{sys: sys}
	s.a, s.b = big.NewColumn(sys.Context.Childf("a"), r0), big.NewColumn(sys.Context.Childf("b"), r0)
	s.c = small.NewColumn(sys.Context.Childf("c"), r1)
	zeta := r2.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"),
		[]*wiop.ColumnView{s.a.View(), s.a.View().Shift(1), s.c.View(), s.b.View(), s.a.View()}, zeta)
	s.dupA1, s.dupA2 = le.EvaluationClaims[0], le.EvaluationClaims[4]
	r1.RegisterAction(assignConst{s.c, 7})
	r2.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys, opts...)
	return s
}

// assignConst is a prover action assigning a constant column.
type assignConst struct {
	col *wiop.Column
	val uint64
}

func (a assignConst) Run(rt *wiop.Runtime) {
	rt.AssignColumn(a.col, baseVec(a.col.Module.Size(), a.val))
}

// compareStatic is a prover action, on the opening round, comparing Static's
// claims with those RecoverBatchClaims takes from the runtime: the same cells,
// laid out as the same BatchClaimedValues, at the plan's shifts.
type compareStatic struct {
	t      *testing.T
	action *OpeningVerifierAction
	op     *StaticOpening
}

func (a compareStatic) Run(rt *wiop.Runtime) {
	t, op := a.t, a.op
	shifts, claims, shapes, _, err := RecoverBatchClaims(rt, op.Batches, allPresentManifests(op.Batches))
	require.NoError(t, err, "the runtime's claims")
	require.Len(t, op.Claims, len(claims), "batches")
	for b := range claims {
		require.Len(t, op.Claims[b], len(claims[b]), "batch %d: sizes", b)
		for s, sized := range claims[b] {
			cells := op.Claims[b][s]
			require.Len(t, cells.Base, len(sized.Base), "batch %d size %d: base rows", b, s)
			require.Len(t, cells.Ext, len(sized.Ext), "batch %d size %d: extension rows", b, s)
			for r, vals := range sized.Base {
				require.Len(t, cells.Base[r], len(vals), "batch %d size %d base row %d: claims", b, s, r)
				for k, v := range vals {
					require.Equal(t, v, rt.GetCellValue(cells.Base[r][k]).AsExt(), "batch %d size %d base row %d claim %d", b, s, r, k)
				}
			}
			for r, vals := range sized.Ext {
				require.Len(t, cells.Ext[r], len(vals), "batch %d size %d ext row %d: claims", b, s, r)
				for k, v := range vals {
					require.Equal(t, v, rt.GetCellValue(cells.Ext[r][k]).AsExt(), "batch %d size %d ext row %d claim %d", b, s, r, k)
				}
			}
		}
	}
	plan, err := a.action.c.newPCS().VerifierPlan(shapes, shifts)
	require.NoError(t, err, "the plan of the runtime's openings")
	require.Equal(t, plan, op.Plan, "the plan of the static openings is the runtime's")
}

// TestStaticOpening: the static structure of an opening is the one the
// native prover and verifier derive from a runtime.
func TestStaticOpening(t *testing.T) {
	s := newStaticTestSystem()
	action := openingAction(s.sys)
	op, err := action.Static(s.sys)
	require.NoError(t, err, "the static opening")

	require.Len(t, op.Batches, 2, "two committed rounds")
	require.Equal(t, [][2]*wiop.Cell{{s.dupA1, s.dupA2}}, op.Duplicates, "a's repeated opening at shift 0")
	require.Len(t, op.EvalPoints, 1, "one point per LagrangeEval")
	require.Len(t, op.Manifests, len(s.a.Round().Columns)+len(s.c.Round().Columns), "a manifest per committed column")
	require.Equal(t, 1<<op.Plan.LogCodewordSize, op.QueryRange, "queries over the codeword")
	require.Equal(t, FRINumQueries(s.sys), op.NumQueries, "queries")

	last := s.sys.Rounds[len(s.sys.Rounds)-1]
	last.ProverActions = append([]wiop.ProverAction{compareStatic{t, action, op}}, last.ProverActions...)
	proof, pub := s.sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(s.a, baseVec(8, 3))
		rt.AssignColumn(s.b, baseVec(8, 5))
	})
	require.NoError(t, s.sys.Verify(proof, pub), "the honest proof should verify")
}

// TestStaticOpening_Rejects: Static fails where the opening's structure
// depends on the proof: with column elision, and with a dynamic module.
func TestStaticOpening_Rejects(t *testing.T) {
	s := newStaticTestSystem(WithColumnElision())
	_, err := openingAction(s.sys).Static(s.sys)
	require.ErrorContains(t, err, "elision", "column elision")

	sys := wiop.NewSystemf("pcs-static-dyn")
	r0, r1 := sys.NewRound(), sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys)
	_, err = openingAction(sys).Static(sys)
	require.ErrorContains(t, err, "dynamic", "a dynamic module")
}

func openingAction(sys *wiop.System) *OpeningVerifierAction {
	for _, r := range sys.Rounds {
		for _, a := range r.VerifierActions {
			if x, ok := a.(*OpeningVerifierAction); ok {
				return x
			}
		}
	}
	panic("no opening verifier action")
}
