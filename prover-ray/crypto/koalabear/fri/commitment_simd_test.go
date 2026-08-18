package fri

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// TestHashSizedLeavesMatchesScalarReference cross-checks hashSizedLeaves'
// batched AVX-512 path against hashLeafScalar, the independent scalar
// reference, for every leaf. 40 rows gives 40 unpaired leaves (32 SIMD + 8
// scalar tail) and 20 paired leaves (16 SIMD + 4 scalar tail), so both modes
// exercise a full SIMD group whose digest the scalar tail loop never computes,
// plus the tail itself.
func TestHashSizedLeavesMatchesScalarReference(t *testing.T) {
	const rows = 40
	var ctr uint64
	t2 := tableOfSize(rows, &ctr)

	for _, paired := range []bool{false, true} {
		nbLeaves := rows
		if paired {
			nbLeaves = rows / 2
		}

		got := make([]field.Octuplet, nbLeaves)
		hashSizedLeaves(t2, paired, got)

		l := newLeafLayout(t2, paired)
		hasher := poseidon2.NewMDHasher()
		for j := 0; j < nbLeaves; j++ {
			want := l.hashLeafScalar(hasher, t2, j)
			if got[j] != want {
				t.Fatalf("paired=%v leaf %d: batched digest %v != scalar reference %v", paired, j, got[j], want)
			}
		}
	}
}
