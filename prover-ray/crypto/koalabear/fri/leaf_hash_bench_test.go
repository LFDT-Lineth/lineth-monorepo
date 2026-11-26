package fri

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/bufpool"
)

// BenchmarkHashSizedLeaves hashes wide tables whose columns come from a
// bufpool, as the encoded tables of a commit do: one page-aligned slice per
// column, so the same row of every column maps to the same L1 set.
func BenchmarkHashSizedLeaves(b *testing.B) {
	shapes := []struct{ base, ext, logRows int }{
		{64, 0, 20}, {256, 0, 18}, {512, 0, 17}, {1024, 0, 16}, {0, 64, 18}, {128, 128, 17},
	}
	for _, s := range shapes {
		rows := 1 << s.logRows
		prng := rand.New(utils.NewRandSource(1))
		var basePool bufpool.Pool[field.Element]
		var extPool bufpool.Pool[field.Ext]
		tab := SizedTable{Base: make([][]field.Element, s.base), Ext: make([][]field.Ext, s.ext)}
		for k := range tab.Base {
			tab.Base[k] = basePool.Get(rows)
			copy(tab.Base[k], field.VecPseudoRandBase(prng, rows))
		}
		for k := range tab.Ext {
			tab.Ext[k] = extPool.Get(rows)
			copy(tab.Ext[k], field.VecPseudoRandExt(prng, rows))
		}
		for _, paired := range []bool{false, true} {
			nbLeaves := rows
			if paired {
				nbLeaves /= 2
			}
			out := make([]field.Octuplet, nbLeaves)
			b.Run(fmt.Sprintf("base=%d/ext=%d/rows=2^%d/paired=%v", s.base, s.ext, s.logRows, paired), func(b *testing.B) {
				for b.Loop() {
					hashSizedLeaves(MerkleHashKoalaBear, tab, paired, out)
				}
			})
		}
	}
}
