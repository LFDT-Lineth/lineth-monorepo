package pcs

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// TestCompileFixedFRIDomainEndToEnd proves and verifies with a fixed FRI
// domain, with and without module groups, on one compiled System per setting,
// for dynamic modules A and B of several sizes: every proof folds the rounds of
// the largest committable column, while a default proof folds as many as its
// largest column needs. A tampered claim is rejected, and neither verifier
// accepts the other's proofs.
func TestCompileFixedFRIDomainEndToEnd(t *testing.T) {
	for _, groups := range []bool{false, true} {
		opts := []Option{WithFRINumQueries(8)}
		if groups {
			opts = append(opts, WithModuleGroups())
		}
		fixed := newModuleGroupsSystem(true)
		Compile(fixed.sys, append(opts, WithFixedFRIDomain())...)
		require.True(t, FRIFixedDomain(fixed.sys))
		restricted := newModuleGroupsSystem(true)
		Compile(restricted.sys, opts...)
		require.False(t, FRIFixedDomain(restricted.sys))

		for _, sizes := range [][2]int{{2, 4}, {4, 4}, {32, 4}, {64, 128}} {
			proof, pub := fixed.sys.Prove(fixed.assign(sizes[0], sizes[1]))
			require.NoError(t, fixed.sys.Verify(proof, pub), "groups %v, sizes %v: honest proof must verify", groups, sizes)
			require.Len(t, proof.PCSOpeningProof.FRIProof.RoundRoots, int(FRIMaxCommittableSizeLog2(fixed.sys))-1,
				"groups %v, sizes %v: a fixed domain folds every round", groups, sizes)

			tampered, _ := fixed.sys.Prove(fixed.assign(sizes[0], sizes[1]))
			tampered.Cells[fixed.le.EvaluationClaims[0].Context.ID] = field.ElemZero()
			require.Error(t, fixed.sys.Verify(tampered, pub), "groups %v, sizes %v: tampered claim must be rejected", groups, sizes)

			restrictedProof, restrictedPub := restricted.sys.Prove(restricted.assign(sizes[0], sizes[1]))
			require.NoError(t, restricted.sys.Verify(restrictedProof, restrictedPub))
			require.Len(t, restrictedProof.PCSOpeningProof.FRIProof.RoundRoots, utils.Log2Ceil(max(sizes[0], sizes[1], 16))-1,
				"groups %v, sizes %v: a default proof folds as many rounds as its largest column needs", groups, sizes)
			require.Error(t, restricted.sys.Verify(proof, pub), "a restricted verifier must reject a fixed-domain proof")
			require.Error(t, fixed.sys.Verify(restrictedProof, restrictedPub), "a fixed-domain verifier must reject a restricted proof")
		}
	}
}

// TestFixedFRIDomainRejectsUnfoldedDynamicSize: with a fixed FRI domain, a
// dynamic module of size 1 would join FRI at the boundary round, unfolded,
// while it is folded at every larger size. Proving panics, and the verifier
// rejects a proof whose dynamic size is changed to 1; a restricted System
// accepts the size.
func TestFixedFRIDomainRejectsUnfoldedDynamicSize(t *testing.T) {
	fixed := newModuleGroupsSystem(true)
	Compile(fixed.sys, WithSizeOblivious(), WithFRINumQueries(8))
	require.PanicsWithError(t,
		`pcs: open: pcs: dynamic column "pcs-groups/a0" has size 2^0, below the 2^1 a fixed FRI domain folds`,
		func() { fixed.sys.Prove(fixed.assign(1, 4)) })

	proof, pub := fixed.sys.Prove(fixed.assign(2, 4))
	require.NoError(t, fixed.sys.Verify(proof, pub))
	for idx, m := range fixed.sys.Modules {
		if m.Context.Path() == "pcs-groups/A" {
			proof.DynamicSizes[idx] = 1
		}
	}
	require.ErrorContains(t, fixed.sys.Verify(proof, pub), "below the 2^1 a fixed FRI domain folds")

	restricted := newModuleGroupsSystem(true)
	Compile(restricted.sys, WithModuleGroups(), WithFRINumQueries(8))
	proof, pub = restricted.sys.Prove(restricted.assign(1, 4))
	require.NoError(t, restricted.sys.Verify(proof, pub), "a restricted System accepts a size-1 dynamic module")
}

// TestFixedFRIDomainConflictsWithConsumedCompile checks that a fixed FRI domain
// cannot be requested after a compile without it consumed the configuration,
// and that a preconfigured selection reaches Compile.
func TestFixedFRIDomainConflictsWithConsumedCompile(t *testing.T) {
	s := newModuleGroupsSystem(false)
	Compile(s.sys)
	require.ErrorContains(t, Configure(s.sys, WithFixedFRIDomain()), "consumed compile without a fixed FRI domain")

	fixed := newModuleGroupsSystem(false)
	require.NoError(t, Configure(fixed.sys, WithFixedFRIDomain()))
	Compile(fixed.sys)
	require.True(t, FRIFixedDomain(fixed.sys), "a preconfigured selection must reach Compile")
}

// TestSizeOblivious checks that WithSizeOblivious selects module groups and a
// fixed FRI domain, and rejects what module groups reject.
func TestSizeOblivious(t *testing.T) {
	s := newModuleGroupsSystem(false)
	Compile(s.sys, WithSizeOblivious())
	require.True(t, FRIFixedDomain(s.sys))
	require.True(t, configOf(s.sys).moduleGroups)

	other := newModuleGroupsSystem(false)
	require.ErrorContains(t, Configure(other.sys, WithSizeOblivious(), WithColumnElision()), "conflicts with WithColumnElision")
}
