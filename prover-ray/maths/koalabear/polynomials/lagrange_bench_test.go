package polynomials

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

const benchSize = 1 << 14

// BenchmarkEvalLagrangeExtExt evaluates a Lagrange-basis extension polynomial
// at an extension point — this is the hottest path that depends on extension
// multiplication and batch inversion together.
func BenchmarkEvalLagrangeExtExt(b *testing.B) {
	rng := newRng()
	evals := make([]field.Ext, benchSize)
	for i := range evals {
		evals[i] = randExt(rng)
	}
	z := field.ElemFromExt(randExt(rng))
	poly := field.VecFromExt(evals)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EvalLagrange(poly, z)
	}
}

// BenchmarkComputeLagrangeAtZExt builds the entire Lᵢ(z) vector for z ∈ F_{p^k}.
func BenchmarkComputeLagrangeAtZExt(b *testing.B) {
	rng := newRng()
	z := field.ElemFromExt(randExt(rng))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeLagrangeAtZ(z, uint64(benchSize))
	}
}
