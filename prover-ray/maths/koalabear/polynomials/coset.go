package polynomials

import (
	"math/big"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// EvalXnMinusOneOnCoset evaluates xⁿ - 1 for x ranging over a coset of the
// size-N multiplicative subgroup.
//
// The coset is { MultiplicativeGen · ωᴺ^i : i = 0, …, N-1 } where ωᴺ is the
// primitive N-th root of unity. Because xⁿ - 1 is constant on every coset of
// the size-(N/n) subgroup, only the first r = N/n distinct values are returned.
//
// Both n and N must be powers of two, and N ≥ n.
func EvalXnMinusOneOnCoset(n, N int) []field.Element {
	if !utils.IsPowerOfTwo(n) || !utils.IsPowerOfTwo(N) {
		utils.Panic("both n (%v) and N (%v) must be powers of two", n, N)
	}
	if N < n {
		utils.Panic("N (%v) must be ≥ n (%v)", N, n)
	}

	r := N / n
	nBig := big.NewInt(int64(n))

	// res[0] = MultiplicativeGen^n  (the coset shift raised to the n-th power)
	res := make([]field.Element, r)
	res[0].SetUint64(field.MultiplicativeGen)
	res[0].Exp(res[0], nBig)

	// t = (primitive N-th root of unity)^n  — step between consecutive values
	t, err := fft.Generator(uint64(N))
	if err != nil {
		panic(err)
	}
	t.Exp(t, nBig)

	for i := 1; i < r; i++ {
		res[i].Mul(&res[i-1], &t)
	}

	// Subtract 1 from every entry
	var one field.Element
	one.SetOne()
	for i := 0; i < r; i++ {
		res[i].Sub(&res[i], &one)
	}

	return res
}
