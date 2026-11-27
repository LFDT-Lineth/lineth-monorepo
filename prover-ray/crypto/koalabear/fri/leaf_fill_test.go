package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/assert"
)

// leafFillTable returns a table of rows rows with nbBase base and nbExt
// extension columns.
func leafFillTable(prng *rand.Rand, nbBase, nbExt, rows int) SizedTable {
	t := SizedTable{Base: make([][]field.Element, nbBase), Ext: make([][]field.Ext, nbExt)}
	for k := range t.Base {
		t.Base[k] = field.VecPseudoRandBase(prng, rows)
	}
	for k := range t.Ext {
		t.Ext[k] = field.VecPseudoRandExt(prng, rows)
	}
	return t
}

// assertLeavesMatchScalar checks every leaf of out against the scalar MDHasher
// path.
func assertLeavesMatchScalar(t *testing.T, tab SizedTable, paired bool, out []field.Octuplet) {
	t.Helper()
	l := newLeafLayout(tab, paired)
	hasher := MerkleHashKoalaBear.newLeafHasher()
	for j := range out {
		assert.Equal(t, l.hashLeafScalar(hasher, tab, j), out[j], "leaf %d should match the scalar MDHasher digest", j)
	}
}

// TestHashSizedLeaves_MatchesScalar checks the batched leaf hash against the
// scalar path. The shapes include extension columns, and (5 base, 3 ext) puts
// the final block's pad gap inside an extension element in both layouts; 40
// leaves leave a scalar tail after two full groups.
func TestHashSizedLeaves_MatchesScalar(t *testing.T) {
	shapes := []struct{ base, ext int }{{1, 0}, {8, 0}, {5, 3}, {0, 7}, {37, 2}}
	for _, s := range shapes {
		for _, paired := range []bool{false, true} {
			for _, nbLeaves := range []int{16, 40, 128} {
				name := fmt.Sprintf("base=%d/ext=%d/paired=%v/leaves=%d", s.base, s.ext, paired, nbLeaves)
				t.Run(name, func(t *testing.T) {
					rows := nbLeaves
					if paired {
						rows *= 2
					}
					tab := leafFillTable(rand.New(utils.NewRandSource(int64(rows))), s.base, s.ext, rows)
					out := make([]field.Octuplet, nbLeaves)
					hashSizedLeaves(MerkleHashKoalaBear, tab, paired, out)
					assertLeavesMatchScalar(t, tab, paired, out)
				})
			}
		}
	}
}
