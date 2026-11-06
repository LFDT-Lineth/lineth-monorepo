package compilers_test

import (
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// newConcurrentProveSystem returns a fully compiled system over modules of n
// rows -- large enough for the parallel and pooled code paths -- that
// reaches the global quotient (a cubic vanishing, so with a ratio above
// one), the permutation and range-check reductions and the PCS opening, and
// the witness for proof number seed.
func newConcurrentProveSystem(t *testing.T, n int) (*wiop.System, func(seed int) func(rt *wiop.Runtime)) {
	t.Helper()
	sys := wiop.NewSystemf("concurrent-prove")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	cube := mod.NewColumn(sys.Context.Childf("cube"), r0)
	rev := mod.NewColumn(sys.Context.Childf("rev"), r0)
	small := mod.NewColumn(sys.Context.Childf("small"), r0)
	mod.NewVanishing(sys.Context.Childf("cubic"),
		wiop.Sub(wiop.Mul(wiop.Mul(a.View(), a.View()), a.View()), cube.View()))
	sys.NewPermutation(sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(a.View())},
		[]wiop.Table{wiop.NewTable(rev.View())})
	mod.NewRangeCheck(sys.Context.Childf("rc"), small, 256)
	require.NoError(t, compilers.CompileFull(sys, wioptest.TestingCompileOptions()...))

	witness := func(seed int) func(rt *wiop.Runtime) {
		return func(rt *wiop.Runtime) {
			va := make([]field.Element, n)
			vCube := make([]field.Element, n)
			vRev := make([]field.Element, n)
			vSmall := make([]field.Element, n)
			for i := range n {
				va[i].SetUint64(uint64(i*(2*seed+3) + seed + 1))
				vCube[i].Square(&va[i]).Mul(&vCube[i], &va[i])
				vSmall[i].SetUint64(uint64((i + 31*seed) % 256))
			}
			for i := range n {
				vRev[i] = va[n-1-i]
			}
			rt.AssignColumn(a, &wiop.ConcreteVector{Plain: field.VecFromBase(va)})
			rt.AssignColumn(cube, &wiop.ConcreteVector{Plain: field.VecFromBase(vCube)})
			rt.AssignColumn(rev, &wiop.ConcreteVector{Plain: field.VecFromBase(vRev)})
			rt.AssignColumn(small, &wiop.ConcreteVector{Plain: field.VecFromBase(vSmall)})
		}
	}
	return sys, witness
}

// Concurrent Prove calls on one System share its lazily built state and the
// process-wide buffer pools. Each must produce exactly the proof a serial
// call produces for the same witness, both on a cold System (first proofs,
// caches built under contention) and on a warm one (pooled buffers handed
// from one proof to another); run under -race this also checks that sharing
// is properly synchronised.
func TestConcurrentProveMatchesSerial(t *testing.T) {
	const (
		n      = 1 << 12
		nProof = 4
	)
	field.DrainPools()
	sys, witness := newConcurrentProveSystem(t, n)

	type result struct {
		proof wiop.Proof
		pub   wiop.PublicInput
	}
	proveConcurrently := func() [nProof]result {
		var (
			wg  sync.WaitGroup
			out [nProof]result
		)
		for i := range nProof {
			wg.Go(func() {
				out[i].proof, out[i].pub = sys.Prove(witness(i))
			})
		}
		wg.Wait()
		return out
	}

	cold := proveConcurrently()
	var serial [nProof]result
	for i := range nProof {
		serial[i].proof, serial[i].pub = sys.Prove(witness(i))
		require.NoError(t, sys.Verify(serial[i].proof, serial[i].pub), "serial proof %d must verify", i)
	}
	warm := proveConcurrently()

	for i := range nProof {
		require.Equal(t, serial[i], cold[i], "concurrent proof %d on a cold System must equal the serial one", i)
		require.Equal(t, serial[i], warm[i], "concurrent proof %d on a warm System must equal the serial one", i)
	}
	require.NotNil(t, serial[0].proof.PCSOpeningProof, "the proof must reach the PCS opening")
	require.NotEqual(t, serial[0].proof, serial[1].proof, "distinct witnesses must give distinct proofs")
}
