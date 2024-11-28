package fastpoly

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

/*
Given a polynomial in standard order evaluation form. Return
the evaluations on a coset of a larger domain. If the factor is not a power of two.
The output is a vector of evaluation
*/
func ReEvaluateOnLargerDomainCoset(poly []field.Element, newLen int) []field.Element {

	/*
		Sanity-checks on the sizes of the elements
	*/
	if !utils.IsPowerOfTwo(len(poly)) || !utils.IsPowerOfTwo(newLen) {
		utils.Panic("Both the length %v and the newLen %v should be powers of two", len(poly), newLen)
	}

	if newLen < len(poly) {
		utils.Panic("The newLen %v is smaller than the old one %v", len(poly), newLen)
	}

	small := vector.DeepCopy(poly)
	// memoized
	domainSmall := fft.NewDomain(len(poly))
	domainSmall.FFTInverse(small, fft.DIF)
	fft.BitReverse(small)

	/*
		Small now contains the coefficients of `poly` in normal order
	*/
	large := vector.ZeroPad(small, newLen)
	// memoized
	domainLarge := fft.NewDomain(len(large)).WithCustomCoset(newLen/len(poly), 0)
	domainLarge.FFT(large, fft.DIF, fft.OnCoset())
	fft.BitReverse(large)

	return large
}

/*
Evaluation x^n - 1 for x on a coset of domain of size N

	N is the size of the larger domain
	n Is the size of the smaller domain

The result is (in theory) of size N, but it has a periodicity
of r = N/n. Thus, we only return the "r" first entries

Largely inspired from gnark's
https://github.com/ConsenSys/gnark/blob/8bc13b200cb9aa1ec74e4f4807e2e97fc8d8396f/internal/backend/bls12-377/plonk/prove.go#L734
*/
func EvalXnMinusOneOnACoset(n, N int) []field.Element {
	/*
		Sanity-checks on the sizes of the elements
	*/
	if !utils.IsPowerOfTwo(n) || !utils.IsPowerOfTwo(N) {
		utils.Panic("Both n %v and N %v should be powers of two", n, N)
	}

	if N < n {
		utils.Panic("N %v is smaller than n %v", N, n)
	}

	// memoized
	nBigInt := big.NewInt(int64(n))

	res := make([]field.Element, N/n)
	res[0].SetUint64(field.MultiplicativeGen)
	res[0].Exp(res[0], nBigInt)

	t := fft.GetOmega(N)
	t.Exp(t, nBigInt)

	for i := 1; i < N/n; i++ {
		res[i].Mul(&res[i-1], &t)
	}

	var one fr.Element
	one.SetOne()
	for i := 0; i < N/n; i++ {
		res[i].Sub(&res[i], &one)
	}

	return res
}
