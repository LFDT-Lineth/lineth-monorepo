package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// TestZeroLayerTree checks the zero layer tree against the tree buildTreeExt
// builds over the zero codeword: same root, caps and branches.
func TestZeroLayerTree(t *testing.T) {
	for height := 1; height <= 6; height++ {
		want := buildTreeExt(make([]field.Ext, 1<<height))
		got := zeroLayerTree{height: height}
		require.Equal(t, want.Root(), got.Root(), "height %d", height)
		for depth := 0; depth <= height; depth++ {
			require.Equal(t, want.OpenCap(depth), got.OpenCap(depth), "height %d depth %d", height, depth)
			for _, idx := range []int{0, 1<<height - 1, 1 << (height - 1)} {
				require.Equal(t, want.OpenBranchToDepth(idx, depth), got.OpenBranchToDepth(idx, depth),
					"height %d depth %d idx %d", height, depth, idx)
			}
		}
	}
}

// TestFixedDomain opens levels below the top of the Params domain. A
// FixedDomain PCS folds from the top whatever the sizes opened: its proofs
// have the round count of Params, the rounds before the largest size opened
// fold the zero codeword, and they verify. A restricted PCS folds fewer
// rounds, and neither verifies the other's proofs.
func TestFixedDomain(t *testing.T) {
	const logN, logD = 7, 6
	for _, sizes := range [][]uint8{{6, 2}, {4, 2}, {3}, {2, 1}, {0}} {
		t.Run(fmt.Sprintf("sizes=%v", sizes), func(t *testing.T) {
			prng := rand.New(utils.NewRandSource(20261027))
			newFixture := func(fixed bool, capHeight uint8) *ldtFixture {
				fx := newLDTFixture(t, logN, logD, 4, LogFinalPolySize(0))
				fx.pcs.FixedDomain = fixed
				fx.pcs.Params.MerkleCapHeight = capHeight
				coeffs := rand.New(utils.NewRandSource(7))
				for _, size := range sizes {
					fx.addLevel(t, size, field.VecPseudoRandExt(coeffs, 1<<size))
				}
				return fx
			}
			foldAlphas := make([]field.Ext, logD)
			for i := range foldAlphas {
				foldAlphas[i] = field.PseudoRandExt(prng)
			}
			positions := []int{3, 77, 100, 127}

			for _, capHeight := range []uint8{0, 2} {
				fixed := newFixture(true, capHeight)
				proof := fixed.open(t, foldAlphas, positions)
				require.NoError(t, fixed.verify(foldAlphas, positions, proof), "cap %d", capHeight)
				require.Len(t, proof.FRIProof.RoundRoots, logD-1, "a fixed domain folds every round")
				for j := uint8(1); j < logD-sizes[0]; j++ {
					require.Equal(t, zeroLayerTree{height: int(logN - j)}.Root(), proof.FRIProof.RoundRoots[j-1],
						"round %d folds the zero codeword", j)
				}

				if sizes[0] == 0 {
					// a restricted PCS has no fold (D = 1), a separate case
					continue
				}
				restricted := newFixture(false, capHeight)
				restrictedPositions := make([]int, len(positions))
				for k, s := range positions {
					restrictedPositions[k] = s >> (logD - sizes[0])
				}
				restrictedProof := restricted.open(t, foldAlphas, restrictedPositions)
				require.NoError(t, restricted.verify(foldAlphas, restrictedPositions, restrictedProof))
				require.Len(t, restrictedProof.FRIProof.RoundRoots, int(sizes[0])-1)
				if sizes[0] < logD {
					require.Error(t, restricted.verify(foldAlphas, positions, proof), "a restricted verifier must reject a fixed-domain proof")
					require.Error(t, fixed.verify(foldAlphas, restrictedPositions, restrictedProof), "a fixed-domain verifier must reject a restricted proof")
				}

				// a zero layer is committed like any other: tampering with
				// one of its opened values is rejected
				if sizes[0] < logD-1 {
					tampered := proof
					tampered.FRIProof.RunningQueries = append([]RunningQuery(nil), proof.FRIProof.RunningQueries...)
					tampered.FRIProof.RunningQueries[0] = append(RunningQuery(nil), proof.FRIProof.RunningQueries[0]...)
					tampered.FRIProof.RunningQueries[0][0].Leaf[0] = field.NewElement(1)
					require.Error(t, fixed.verify(foldAlphas, positions, tampered), "a zero layer's opening must bind")
				}
			}
		})
	}
}

// TestFixedDomainRejectsLargerSize: a size above the fixed domain cannot be
// opened.
func TestFixedDomainRejectsLargerSize(t *testing.T) {
	fx := newLDTFixture(t, 3, 2, 1)
	fx.pcs.FixedDomain = true
	big, err := fx.pcs.restrictTo(3)
	require.Nil(t, big)
	require.ErrorContains(t, err, "exceeds the fixed FRI domain")
}
