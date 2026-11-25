package fri

import (
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// planFixture is an opening of two batches, of several sizes, base and
// extension rows, and a row opened at two shifts: its PCS, the inputs Verify
// takes, and an honest proof.
type planFixture struct {
	pcs   *PCS
	input VerifyInputs
	proof OpeningProof
}

func newPlanFixture(t *testing.T) planFixture {
	t.Helper()
	params, err := NewParams(5, 3, 2)
	require.NoError(t, err, "the FRI parameters")
	pcs, err := NewPCS(params, makeEncoders(int(params.numRounds()+1), 4))
	require.NoError(t, err, "the PCS")

	prng := rand.New(utils.NewRandSource(20261120))
	b0 := make(Batch, 4)
	b0[3] = SizedTable{
		Base: [][]field.Element{field.VecPseudoRandBase(prng, 8), field.VecPseudoRandBase(prng, 8), field.VecPseudoRandBase(prng, 8)},
		Ext:  [][]field.Ext{field.VecPseudoRandExt(prng, 8)},
	}
	b0[1] = SizedTable{Base: [][]field.Element{field.VecPseudoRandBase(prng, 2), field.VecPseudoRandBase(prng, 2)}}
	b1 := make(Batch, 3)
	b1[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4), field.VecPseudoRandExt(prng, 4)}}
	witnesses := []Batch{b0, b1}

	s0 := make(BatchShifts, 4)
	s0[3] = SizedShifts{Base: [][]int{{0}, {0, 1}, {0}}, Ext: [][]int{{7}}}
	s0[1] = SizedShifts{Base: [][]int{{0}, {0}}}
	s1 := make(BatchShifts, 3)
	s1[2] = SizedShifts{Ext: [][]int{{0, 1}, {0}}}
	shifts := []BatchShifts{s0, s1}

	committed := []CommitterState{pcs.Commit(b0), pcs.Commit(b1)}
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	challenges := Challenges{
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0), field.UintsToExt(31, 0, 1, 0, 0, 0), field.UintsToExt(37, 0, 0, 1, 0, 0),
		},
		QueryPositions: []int{5, 26},
	}
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses: witnesses, Committed: committed, Shifts: shifts, Zeta: zeta, Challenges: challenges,
	})
	input := VerifyInputs{
		Roots:         []field.Octuplet{committed[0].Tree.Root(), committed[1].Tree.Root()},
		Shapes:        utils.Map(Batch.Shape, witnesses),
		Shifts:        shifts,
		ClaimedValues: claimed,
		Zeta:          zeta,
		Challenges:    challenges,
	}
	require.NoError(t, pcs.Verify(input, proof), "the honest opening should verify")
	return planFixture{pcs, input, proof}
}

// TestVerifierPlan: the plan describes the opening Verify checks: its
// schedule, its levels (sizes, rounds, columns, powers, claim points), and
// its input trees, in the order and of the heights of the proof's openings.
func TestVerifierPlan(t *testing.T) {
	fx := newPlanFixture(t)
	plan, err := fx.pcs.VerifierPlan(fx.input.Shapes, fx.input.Shifts)
	require.NoError(t, err, "the plan of the fixture")

	require.Equal(t, 3, plan.NumRounds, "rounds")
	require.Equal(t, 5, plan.LogCodewordSize, "codeword size")
	require.Equal(t, 2, plan.RateLog, "rate")
	require.Len(t, fx.proof.FRIProof.RoundRoots, plan.NumRounds-1, "one root per intermediate round")

	// Generators[j] generates the round-j domain, of size 2^(5 − j).
	require.Len(t, plan.Generators, plan.NumRounds+1, "generators")
	for j, g := range plan.Generators {
		var x field.Element
		x.Exp(g, bigPow2(plan.LogCodewordSize-j-1))
		require.False(t, x.IsOne(), "generator %d: order below 2^%d", j, plan.LogCodewordSize-j)
		x.Square(&x)
		require.True(t, x.IsOne(), "generator %d: order above 2^%d", j, plan.LogCodewordSize-j)
	}

	// Levels: the sizes 3, 2, 1, by decreasing size, joining at round 3 − size.
	require.Len(t, plan.Levels, 3, "levels")
	wantColumns := []int{4, 2, 2}
	for i, l := range plan.Levels {
		require.Equal(t, 3-i, l.SizeLog2, "level %d size", i)
		require.Equal(t, i, l.Round, "level %d round", i)
		require.Equal(t, len(l.Entries), l.RunningPower, "level %d running power", i)
		require.Len(t, l.Entries, wantColumns[i], "level %d columns", i)
		powers := map[int]bool{}
		for _, e := range l.Entries {
			powers[e.Power] = true
			sized := fx.input.Shifts[e.Batch][l.SizeLog2]
			want := sized.Base
			if e.IsExt {
				want = sized.Ext
			}
			require.Equal(t, want[e.Row], e.Shifts, "level %d: the shifts of batch %d, row %d", i, e.Batch, e.Row)
			for k, s := range e.Shifts {
				point, err := fx.pcs.shiftedPoint(uint8(l.SizeLog2), s, fx.input.Zeta)
				require.NoError(t, err, "claim point")
				var rotated field.Ext
				rotated.MulByElement(&fx.input.Zeta, &e.Rotations[k])
				require.Equal(t, point, rotated, "level %d: claim point of shift %d", i, s)
			}
		}
		require.Len(t, powers, len(l.Entries), "level %d: distinct powers", i)
	}

	// Trees: one per batch, in the proof's order, of the proof's heights, a
	// pair at the level of each of its sizes.
	for _, q := range fx.proof.InputQueries {
		require.Len(t, q, len(plan.Trees), "trees")
		for i, tree := range plan.Trees {
			o := q[i]
			require.Equal(t, fx.input.Shapes[tree.Batch], tree.Shape, "tree %d shape", i)
			require.Len(t, o.Leaves, tree.Height, "tree %d: one row level per height", i)
			require.Len(t, o.Siblings, tree.Height-1, "tree %d: siblings above the bottom pair", i)
			for s, sized := range tree.Shape {
				if sized.BaseWidth+sized.ExtWidth == 0 {
					continue
				}
				level := plan.RateLog + s - 1
				if s == len(tree.Shape)-1 {
					level = tree.Height - 1
				}
				pair := o.Leaves[level]
				require.NotNil(t, pair, "tree %d: the pair of size %d at level %d", i, s, level)
				require.Len(t, pair[0].Base, sized.BaseWidth, "tree %d size %d: base row", i, s)
				require.Len(t, pair[0].Ext, sized.ExtWidth, "tree %d size %d: extension row", i, s)
			}
		}
	}
}

// TestVerifierPlan_Rejects: the plan fails where Verify fails before reading
// the proof.
func TestVerifierPlan_Rejects(t *testing.T) {
	fx := newPlanFixture(t)

	badShifts := []BatchShifts{fx.input.Shifts[0], {}}
	_, err := fx.pcs.VerifierPlan(fx.input.Shapes, badShifts)
	require.Error(t, err, "shifts that do not follow the shapes")

	// Without any fold, only one column can be checked.
	noFold, err := NewParams(2, 0, 2)
	require.NoError(t, err, "parameters without fold")
	pcs, err := NewPCS(noFold, makeEncoders(1, 4))
	require.NoError(t, err, "the PCS without fold")
	shapes := []Shape{{{BaseWidth: 2}}}
	shifts := []BatchShifts{{{Base: [][]int{{0}, {0}}}}}
	_, err = pcs.VerifierPlan(shapes, shifts)
	require.ErrorContains(t, err, "no FRI fold", "two columns without fold")

	// A size below the final polynomial's joins after the last fold.
	final, err := NewParams(5, 3, 2, LogFinalPolySize(2))
	require.NoError(t, err, "parameters with a final polynomial of 4 coefficients")
	pcs, err = NewPCS(final, makeEncoders(4, 4))
	require.NoError(t, err, "the PCS with a large final polynomial")
	shapes = []Shape{{{BaseWidth: 1}, {}, {}, {BaseWidth: 1}}}
	shifts = []BatchShifts{{{Base: [][]int{{0}}}, {}, {}, {Base: [][]int{{0}}}}}
	_, err = pcs.VerifierPlan(shapes, shifts)
	require.ErrorContains(t, err, "after the last", "a level after the last fold")
}

func bigPow2(n int) *big.Int { return new(big.Int).Lsh(big.NewInt(1), uint(n)) }
