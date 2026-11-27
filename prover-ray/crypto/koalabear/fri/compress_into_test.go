package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randOctuplets(prng *rand.Rand, n int) []field.Octuplet {
	raw := field.VecPseudoRandBase(prng, 8*n)
	res := make([]field.Octuplet, n)
	for i := range res {
		copy(res[i][:], raw[8*i:8*i+8])
	}
	return res
}

// TestCompressInto checks compressInto against the scalar compression for
// both hashes, at lengths below, at and around the 16-lane batch width, so
// that both the batched groups and the scalar tail are covered.
func TestCompressInto(t *testing.T) {
	for _, hash := range []MerkleHash{MerkleHashKoalaBear, MerkleHashBNPacked} {
		for _, n := range []int{0, 1, 15, 16, 17, 48, 1000} {
			t.Run(fmt.Sprintf("%v/n=%d", hash, n), func(t *testing.T) {
				prng := rand.New(utils.NewRandSource(int64(n)))
				dst := randOctuplets(prng, n)
				right := randOctuplets(prng, n)
				want := make([]field.Octuplet, n)
				for j := range want {
					want[j] = hash.compress(dst[j], right[j])
				}
				compressInto(hash, dst, right)
				for j := range want {
					assert.Equal(t, want[j], dst[j], "dst[%d] should be C(dst[%d], right[%d])", j, j, j)
				}
			})
		}
	}
}

func TestCompressInto_LengthMismatch(t *testing.T) {
	require.PanicsWithValue(t, "fri: compressInto: length mismatch", func() {
		compressInto(MerkleHashKoalaBear, make([]field.Octuplet, 16), make([]field.Octuplet, 17))
	}, "slices of different lengths should be rejected")
}

// pairLeavesGroups returns numGroups tables of rows rows, group g holding
// g%3+1 base columns and g%2 extension columns.
func pairLeavesGroups(prng *rand.Rand, numGroups, rows int) []SizedTable {
	groups := make([]SizedTable, numGroups)
	for g := range groups {
		groups[g].Base = make([][]field.Element, g%3+1)
		for k := range groups[g].Base {
			groups[g].Base[k] = field.VecPseudoRandBase(prng, rows)
		}
		groups[g].Ext = make([][]field.Ext, g%2)
		for k := range groups[g].Ext {
			groups[g].Ext[k] = field.VecPseudoRandExt(prng, rows)
		}
	}
	return groups
}

// TestHashPairLeaves_BatchedChain checks hashPairLeaves on enough leaves to
// take the batched chaining path (64 leaves, and 40 so that a scalar tail
// remains) against a fully scalar reference: each group's leaf digest hashed
// one leaf at a time, chained with the scalar compression.
func TestHashPairLeaves_BatchedChain(t *testing.T) {
	for _, hash := range []MerkleHash{MerkleHashKoalaBear, MerkleHashBNPacked} {
		for _, numLeaves := range []int{40, 64} {
			t.Run(fmt.Sprintf("%v/leaves=%d", hash, numLeaves), func(t *testing.T) {
				groups := pairLeavesGroups(rand.New(utils.NewRandSource(int64(numLeaves))), 4, 2*numLeaves)

				got := make([]field.Octuplet, numLeaves)
				hashPairLeaves(hash, groups, got)

				layouts := make([]leafLayout, len(groups))
				for g := range groups {
					layouts[g] = newLeafLayout(groups[g], true)
				}
				hasher := hash.newLeafHasher()
				for j := range got {
					digests := make([]field.Octuplet, len(groups))
					for g := range groups {
						digests[g] = layouts[g].hashLeafScalar(hasher, groups[g], j)
					}
					assert.Equal(t, chainDigests(hash, digests), got[j], "leaf %d should chain the groups' scalar digests", j)
				}
			})
		}
	}
}

// BenchmarkHashPairLeaves hashes the pair leaves of a bottom table split into
// module groups: one batched leaf hash per group, and len(groups)-1 chaining
// compressions per leaf.
func BenchmarkHashPairLeaves(b *testing.B) {
	const logRows = 16
	for _, shape := range []struct{ groups, base int }{{8, 16}, {32, 4}, {4, 64}} {
		prng := rand.New(utils.NewRandSource(1))
		groups := make([]SizedTable, shape.groups)
		for g := range groups {
			groups[g].Base = make([][]field.Element, shape.base)
			for k := range groups[g].Base {
				groups[g].Base[k] = field.VecPseudoRandBase(prng, 1<<logRows)
			}
		}
		leaves := make([]field.Octuplet, 1<<(logRows-1))
		b.Run(fmt.Sprintf("groups=%d/base=%d/rows=2^%d", shape.groups, shape.base, logRows), func(b *testing.B) {
			for b.Loop() {
				hashPairLeaves(MerkleHashKoalaBear, groups, leaves)
			}
		})
	}
}
