package global_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// requireSamePrograms asserts that got holds exactly the programs of want,
// the same objects in the same order.
func requireSamePrograms(t *testing.T, want, got []any) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		require.Same(t, want[i], got[i], "program %d was rebuilt", i)
	}
}

// The quotient program of a bucket is built by the first proof and reused,
// unchanged, by the next one: the second proof must only bind its tables and
// scalars to it, and come out identical to the first.
func TestQuotientPrograms_ReusedAcrossProofs(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)

			proof1, pub1 := sc.Sys.Prove(sc.AssignHonest)
			built := global.BuiltPrograms(sc.Sys)
			require.NotEmpty(t, built)

			proof2, pub2 := sc.Sys.Prove(sc.AssignHonest)
			requireSamePrograms(t, built, global.BuiltPrograms(sc.Sys))

			require.Equal(t, proof1, proof2, "a proof on a reused program must be identical")
			require.Equal(t, pub1, pub2)
			require.NoError(t, sc.Sys.Verify(proof2, pub2))
		})
	}
}

// A dynamic module's bucket gets one program per runtime size: proving at a
// new size builds one more, proving again at a size already seen reuses it.
func TestQuotientPrograms_OnePerDynamicSize(t *testing.T) {
	sys := wiop.NewSystemf("dyn-fib-sizes")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	mod.NewVanishing(sys.Context.Childf("fib"),
		wiop.Sub(wiop.Sub(col.View(), col.View().Shift(-1)), col.View().Shift(-2)))
	global.Compile(sys)

	prove := func(n int) {
		t.Helper()
		fib := make([]field.Element, n)
		fib[0].SetOne()
		fib[1].SetOne()
		for i := 2; i < n; i++ {
			fib[i].Add(&fib[i-1], &fib[i-2])
		}
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(fib)})
		})
		require.NoError(t, sys.Verify(proof, pub), "size %d", n)
	}

	prove(8)
	at8 := global.BuiltPrograms(sys)
	require.Len(t, at8, 1)

	prove(16)
	both := global.BuiltPrograms(sys)
	require.Len(t, both, 2, "a new size builds its own program")
	require.Same(t, at8[0], both[0])

	prove(8)
	requireSamePrograms(t, both, global.BuiltPrograms(sys))
}

// The quotient programs built by one proof must not keep anything of it: a
// second proof of a different witness on the same system -- other columns,
// so other coins -- must equal that witness's proof on a fresh system, and an
// invalid witness must still be rejected. The vanishing reads its column at a
// shift (a cancelled row, so a cancellation table) and through a Lagrange
// selector.
func TestQuotientPrograms_BindEachProofsData(t *testing.T) {
	const size, pos = 8, 2
	build := func() (*wiop.System, *wiop.Column, *wiop.Column) {
		sys := wiop.NewSystemf("gl-program-rebind")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
		x := mod.NewColumn(sys.Context.Childf("x"), r0)
		y := mod.NewColumn(sys.Context.Childf("y"), r0)
		// y[i] = x[i]·x[i-1] on every row but the first, and x[pos] = 0.
		mod.NewVanishing(sys.Context.Childf("prod"),
			wiop.Sub(y.View(), wiop.Mul(x.View(), x.View().Shift(-1))))
		mod.NewVanishingManual(sys.Context.Childf("zero"),
			wiop.Mul(x.View(), wiop.NewLagrangeSelector(mod, pos)))
		global.Compile(sys)
		pcs.Compile(sys)
		return sys, x, y
	}
	witness := func(seed uint64, corrupt bool) func(*wiop.Column, *wiop.Column) func(*wiop.Runtime) {
		return func(x, y *wiop.Column) func(*wiop.Runtime) {
			xs, ys := make([]field.Element, size), make([]field.Element, size)
			for i := range xs {
				if i != pos {
					xs[i].SetUint64(seed + uint64(i))
				}
			}
			for i := 1; i < size; i++ {
				ys[i].Mul(&xs[i], &xs[i-1])
			}
			if corrupt {
				ys[5].SetUint64(1)
			}
			return func(rt *wiop.Runtime) {
				rt.AssignColumn(x, &wiop.ConcreteVector{Plain: field.VecFromBase(xs)})
				rt.AssignColumn(y, &wiop.ConcreteVector{Plain: field.VecFromBase(ys)})
			}
		}
	}

	sys, x, y := build()
	proofA, pubA := sys.Prove(witness(3, false)(x, y))
	require.NoError(t, sys.Verify(proofA, pubA))
	built := global.BuiltPrograms(sys)

	proofB, pubB := sys.Prove(witness(11, false)(x, y))
	requireSamePrograms(t, built, global.BuiltPrograms(sys))
	require.NoError(t, sys.Verify(proofB, pubB), "a second witness on reused programs must verify")
	require.NotEqual(t, proofA, proofB)

	fresh, fx, fy := build()
	want, wantPub := fresh.Prove(witness(11, false)(fx, fy))
	require.Equal(t, want, proofB, "a proof on reused programs must equal a fresh system's")
	require.Equal(t, wantPub, pubB)

	bad, badPub := sys.Prove(witness(11, true)(x, y))
	require.Error(t, sys.Verify(bad, badPub), "an invalid witness on reused programs must be rejected")
}
