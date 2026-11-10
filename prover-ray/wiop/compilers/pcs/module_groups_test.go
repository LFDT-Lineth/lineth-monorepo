package pcs

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// moduleGroupsSystem declares modules A, B and Big, in that order, and commits
// their columns in round 0 interleaved across modules: b0, a0, b1, a1 (an
// extension column), big. A and B are dynamic when dynamic is set, else of
// size 4; Big has size 16, so it is always the largest. Every column is opened
// at a coin of round 1.
type moduleGroupsSystem struct {
	sys                *wiop.System
	a0, a1, b0, b1, bg *wiop.Column
	le                 *wiop.LagrangeEval
}

func newModuleGroupsSystem(dynamic bool) moduleGroupsSystem {
	sys := wiop.NewSystemf("pcs-groups")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	module := func(name string) *wiop.Module {
		if dynamic {
			return sys.NewDynamicModule(sys.Context.Childf("%s", name), wiop.PaddingDirectionRight)
		}
		return sys.NewSizedModule(sys.Context.Childf("%s", name), 4, wiop.PaddingDirectionNone)
	}
	a, b := module("A"), module("B")
	big := sys.NewSizedModule(sys.Context.Childf("Big"), 16, wiop.PaddingDirectionNone)

	s := moduleGroupsSystem{sys: sys}
	s.b0 = b.NewColumn(sys.Context.Childf("b0"), r0)
	s.a0 = a.NewColumn(sys.Context.Childf("a0"), r0)
	s.b1 = b.NewColumn(sys.Context.Childf("b1"), r0)
	s.a1 = a.NewExtensionColumn(sys.Context.Childf("a1"), r0)
	s.bg = big.NewColumn(sys.Context.Childf("big"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	views := []*wiop.ColumnView{s.b0.View(), s.a0.View(), s.b1.View(), s.a1.View(), s.bg.View()}
	s.le = sys.NewLagrangeEval(sys.Context.Childf("le"), views, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: s.le})
	return s
}

// varyingVec is a non-constant base vector, so the DEEP quotient does not
// vanish and the opening actually binds the commitment.
func varyingVec(n int, seed uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(seed*31 + uint64(i*i) + 1)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

func varyingExtVec(n int, seed uint64) *wiop.ConcreteVector {
	elems := make([]field.Ext, n)
	for i := range elems {
		elems[i] = field.UintsToExt(seed*17+uint64(i)+2, uint64(i), 3, 0, 0, 0)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromExt(elems)}
}

func (s moduleGroupsSystem) assign(sizeA, sizeB int) func(*wiop.Runtime) {
	return func(rt *wiop.Runtime) {
		rt.AssignColumn(s.a0, varyingVec(sizeA, 1))
		rt.AssignColumn(s.a1, varyingExtVec(sizeA, 2))
		rt.AssignColumn(s.b0, varyingVec(sizeB, 3))
		rt.AssignColumn(s.b1, varyingVec(sizeB, 4))
		rt.AssignColumn(s.bg, varyingVec(16, 5))
	}
}

// TestCommittedLayoutModuleGroups checks the commit order and the shape under
// module groups: within a size, columns are ordered by module declaration
// order, each module is one group, positions follow that order, and the
// largest size is grouped like the others. Without the option, the layout is
// the declaration-order one, with no groups.
func TestCommittedLayoutModuleGroups(t *testing.T) {
	s := newModuleGroupsSystem(false)
	round := s.sys.Rounds[0]
	manifest := allPresentManifest(len(round.Columns))

	layout, shape := getLayoutWith(round, staticSizeOf, manifest, true)
	require.Len(t, shape, 5, "sizes 2^0..2^4")
	require.Equal(t, fri.SizedShape{BaseWidth: 3, ExtWidth: 1, Groups: []fri.GroupShape{
		{BaseWidth: 1, ExtWidth: 1, Ordinal: 0}, // A: a0, a1
		{BaseWidth: 2, Ordinal: 1},              // B: b0, b1
	}}, shape[2], "each group carries its module index as its ordinal")
	require.Equal(t, fri.SizedShape{BaseWidth: 1, Groups: []fri.GroupShape{{BaseWidth: 1, Ordinal: 2}}}, shape[4],
		"the largest size is grouped too")
	require.Equal(t, 0, layout[s.a0.Context.ID].Position, "A's rows come first")
	require.Equal(t, 1, layout[s.b0.Context.ID].Position)
	require.Equal(t, 2, layout[s.b1.Context.ID].Position)
	require.Equal(t, 0, layout[s.a1.Context.ID].Position)

	layout, shape = getLayoutWith(round, staticSizeOf, manifest, false)
	require.Equal(t, fri.SizedShape{BaseWidth: 3, ExtWidth: 1}, shape[2], "no groups without the option")
	require.Equal(t, 0, layout[s.b0.Context.ID].Position, "declaration order without the option")
	require.Equal(t, 1, layout[s.a0.Context.ID].Position)
	require.Equal(t, 2, layout[s.b1.Context.ID].Position)
}

// TestCompileModuleGroupsEndToEnd proves and verifies with module groups, with
// dynamic modules A and B sharing a size or not, below Big's size or at the
// largest size of the round (both, or one of them), at the default and a
// larger RS blowup, checks that a tampered claim is rejected, and that the
// grouping changes the commitment.
func TestCompileModuleGroupsEndToEnd(t *testing.T) {
	for _, blowup := range []int{defaultRSBlowup, 4} {
		testCompileModuleGroupsEndToEnd(t, blowup)
	}
}

func testCompileModuleGroupsEndToEnd(t *testing.T, blowup int) {
	t.Helper()
	for _, sizes := range [][2]int{{4, 4}, {4, 8}, {8, 4}, {32, 32}, {32, 4}, {16, 16}} {
		s := newModuleGroupsSystem(true)
		Compile(s.sys, WithModuleGroups(), WithRSBlowup(blowup))
		proof, pub := s.sys.Prove(s.assign(sizes[0], sizes[1]))
		require.NoError(t, s.sys.Verify(proof, pub), "sizes %v: honest proof must verify", sizes)

		proof.Cells[s.le.EvaluationClaims[0].Context.ID] = field.ElemZero()
		require.Error(t, s.sys.Verify(proof, pub), "sizes %v: tampered claim must be rejected", sizes)

		flat := newModuleGroupsSystem(true)
		Compile(flat.sys, WithRSBlowup(blowup))
		flatProof, _ := flat.sys.Prove(flat.assign(sizes[0], sizes[1]))
		grouped, _ := s.sys.Prove(s.assign(sizes[0], sizes[1]))
		if sizes[0] == sizes[1] {
			require.NotEqual(t, flatProof.Commitments[0], grouped.Commitments[0],
				"sizes %v: modules sharing a size must be committed as separate groups", sizes)
		}
	}
}

// TestModuleGroupsConflictsWithConsumedCompile checks that module groups
// cannot be requested after a compile without them consumed the configuration.
func TestModuleGroupsConflictsWithConsumedCompile(t *testing.T) {
	s := newModuleGroupsSystem(false)
	Compile(s.sys)
	require.ErrorContains(t, Configure(s.sys, WithModuleGroups()), "consumed compile without module groups")

	grouped := newModuleGroupsSystem(false)
	require.NoError(t, Configure(grouped.sys, WithModuleGroups()))
	Compile(grouped.sys)
	require.True(t, configOf(grouped.sys).moduleGroups, "a preconfigured selection must reach Compile")
}

// TestModuleGroupsRejectsSizeDependentOptions checks that module groups cannot
// be combined with column elision or a Merkle cap, whether the options come in
// one call or across a preconfigured selection, while an explicit cap height
// of 0 (no cap) stays accepted.
func TestModuleGroupsRejectsSizeDependentOptions(t *testing.T) {
	const elisionConflict = "conflicts with WithColumnElision"
	for _, tc := range []struct {
		name string
		pre  []Option
		opts []Option
		want string
	}{
		{
			name: "elision",
			opts: []Option{WithModuleGroups(), WithColumnElision()},
			want: elisionConflict,
		},
		{
			name: "preconfigured elision",
			pre:  []Option{WithColumnElision()},
			opts: []Option{WithModuleGroups()},
			want: elisionConflict,
		},
		{
			name: "preconfigured groups",
			pre:  []Option{WithModuleGroups()},
			opts: []Option{WithColumnElision()},
			want: elisionConflict,
		},
		{
			name: "optimal cap",
			opts: []Option{WithModuleGroups(), WithOptimalMerkleCap()},
			want: "Merkle cap optimal",
		},
		{
			name: "cap height",
			opts: []Option{WithModuleGroups(), WithMerkleCapHeight(3)},
			want: "Merkle cap height 3",
		},
		{
			name: "preconfigured cap",
			pre:  []Option{WithMerkleCapHeight(3)},
			opts: []Option{WithModuleGroups()},
			want: "Merkle cap height 3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newModuleGroupsSystem(true)
			if tc.pre != nil {
				require.NoError(t, Configure(s.sys, tc.pre...))
			}
			require.ErrorContains(t, Configure(s.sys, tc.opts...), tc.want)
			require.Panics(t, func() { Compile(s.sys, tc.opts...) }, "Compile must reject the combination")
		})
	}

	s := newModuleGroupsSystem(true)
	Compile(s.sys, WithModuleGroups(), WithMerkleCapHeight(0))
	proof, pub := s.sys.Prove(s.assign(4, 8))
	require.NoError(t, s.sys.Verify(proof, pub), "module groups without a cap must prove and verify")
}
