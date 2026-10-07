package global_test

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
)

// BenchmarkProveQuotient measures the global-compiler prover on a module with
// nCols columns and nCols-1 quadratic vanishing constraints (ratio 2), which
// stresses the per-column re-evaluation on the large coset.
func BenchmarkProveQuotient(b *testing.B) {
	for _, tc := range []struct{ logN, nCols int }{{16, 64}, {20, 16}, {20, 64}} {
		b.Run(fmt.Sprintf("n=2^%d/cols=%d", tc.logN, tc.nCols), func(b *testing.B) {
			size := 1 << tc.logN
			sys := wiop.NewSystemf("gl-bench")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
			cols := make([]*wiop.Column, tc.nCols)
			for i := range cols {
				cols[i] = mod.NewColumn(sys.Context.Childf("c%d", i), r0)
			}
			for i := 0; i+1 < tc.nCols; i++ {
				mod.NewVanishingManual(sys.Context.Childf("v%d", i),
					wiop.Sub(wiop.Mul(cols[i].View(), cols[i+1].View()), cols[i].View()))
			}
			global.Compile(sys)

			vecs := make([]*wiop.ConcreteVector, tc.nCols)
			for i := range vecs {
				e := make([]field.Element, size)
				for j := range e {
					e[j].SetUint64(uint64(i*size + j + 1))
				}
				vecs[i] = &wiop.ConcreteVector{Plain: field.VecFromBase(e)}
			}
			assign := func(rt *wiop.Runtime) {
				for i, c := range cols {
					rt.AssignColumn(c, vecs[i])
				}
			}
			sys.Prove(assign) // warm-up
			b.ResetTimer()
			for b.Loop() {
				sys.Prove(assign)
			}
		})
	}
}
