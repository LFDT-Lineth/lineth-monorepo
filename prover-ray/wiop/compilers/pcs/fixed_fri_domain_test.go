package pcs

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// TestCompileFixedFRIDomainEndToEnd proves and verifies with a fixed FRI
// domain, with and without module groups, for dynamic modules A and B of
// several sizes: every proof folds the same number of rounds, that of the
// largest committable column, while a default proof folds as many as its
// largest column needs. A tampered claim is rejected, and a verifier must
// use the prover's domain.
func TestCompileFixedFRIDomainEndToEnd(t *testing.T) {
	for _, groups := range []bool{false, true} {
		for _, sizes := range [][2]int{{4, 4}, {32, 4}, {64, 128}} {
			opts := []Option{WithFixedFRIDomain(), WithFRINumQueries(8)}
			if groups {
				opts = append(opts, WithModuleGroups())
			}
			s := newModuleGroupsSystem(true)
			Compile(s.sys, opts...)
			require.True(t, FRIFixedDomain(s.sys))
			proof, pub := s.sys.Prove(s.assign(sizes[0], sizes[1]))
			require.NoError(t, s.sys.Verify(proof, pub), "groups %v, sizes %v: honest proof must verify", groups, sizes)
			require.Len(t, proof.PCSOpeningProof.FRIProof.RoundRoots, int(FRIMaxCommittableSizeLog2(s.sys))-1,
				"groups %v, sizes %v: a fixed domain folds every round", groups, sizes)

			tampered, _ := s.sys.Prove(s.assign(sizes[0], sizes[1]))
			tampered.Cells[s.le.EvaluationClaims[0].Context.ID] = field.ElemZero()
			require.Error(t, s.sys.Verify(tampered, pub), "groups %v, sizes %v: tampered claim must be rejected", groups, sizes)

			restricted := newModuleGroupsSystem(true)
			Compile(restricted.sys, WithFRINumQueries(8))
			require.False(t, FRIFixedDomain(restricted.sys))
			restrictedProof, _ := restricted.sys.Prove(restricted.assign(sizes[0], sizes[1]))
			largest := max(sizes[0], sizes[1], 16)
			require.Len(t, restrictedProof.PCSOpeningProof.FRIProof.RoundRoots, log2(largest)-1,
				"sizes %v: a default proof folds as many rounds as its largest column needs", sizes)
		}
	}
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

func log2(n int) int {
	l := 0
	for 1<<l < n {
		l++
	}
	return l
}
