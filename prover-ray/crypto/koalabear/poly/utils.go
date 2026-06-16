// Copyright Consensys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
// specific language governing permissions and limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package poly

import (
	"math/big"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// isPowerOfTwo checks if n is a power of two
func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// NextPowerOfTwo returns the next power of two greater than or equal to n
func NextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	if isPowerOfTwo(n) {
		return n
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n + 1
}

func LagrangeAtZeta(zeta koalabear.Element, N, i int) koalabear.Element {
	var zetan, omegai, one, Nk koalabear.Element
	Nk.SetUint64(uint64(N))
	omegai, err := koalabear.Generator(uint64(N))
	if err != nil {
		panic(err)
	}
	omegai.Exp(omegai, big.NewInt(int64(i)))
	zetan.Exp(zeta, big.NewInt(int64(N)))
	one.SetOne()
	zetan.Sub(&zetan, &one)
	zeta.Sub(&zeta, &omegai)
	omegai.Mul(&omegai, &zetan)
	zeta.Mul(&zeta, &Nk)
	omegai.Div(&omegai, &zeta)
	return omegai
}

func LagrangeAtZetaExt(zeta ext.E6, N, i int) ext.E6 {
	var zetaN, numerator, denominator, one ext.E6
	zetaN.Exp(zeta, big.NewInt(int64(N)))
	one.SetOne()
	zetaN.Sub(&zetaN, &one)

	omegaI, err := koalabear.Generator(uint64(N))
	if err != nil {
		panic(err)
	}
	omegaI.Exp(omegaI, big.NewInt(int64(i)))
	numerator.MulByElement(&zetaN, &omegaI)

	var omegaIExt ext.E6
	// omegaIExt.Lift(&omegaI)
	omegaIExt.B0.A0.Set(&omegaI)
	denominator.Sub(&zeta, &omegaIExt)

	var Nk koalabear.Element
	Nk.SetUint64(uint64(N))
	denominator.MulByElement(&denominator, &Nk)
	denominator.Inverse(&denominator)

	numerator.Mul(&numerator, &denominator)
	return numerator
}

// LagrangeWeightsOnExtendedDomainRoot returns the evaluations of the Lagrange
// basis over d at bigD.Generator^rootIndex. It supports the common case where
// d is a subgroup domain of bigD, but does not rely on that relationship.
func LagrangeWeightsOnExtendedDomainRoot(d *fft.Domain, bigD *fft.Domain, rootIndex int) []koalabear.Element {
	N := int(d.Cardinality)
	weights := make([]koalabear.Element, N)
	if N == 1 {
		weights[0].SetOne()
		return weights
	}

	bigN := int(bigD.Cardinality)
	rootIndex %= bigN
	if rootIndex < 0 {
		rootIndex += bigN
	}

	var x koalabear.Element
	if rootIndex == 0 {
		x.SetOne()
	} else {
		x.Exp(bigD.Generator, big.NewInt(int64(rootIndex)))
	}

	denominators := make([]koalabear.Element, N)
	var h koalabear.Element
	h.SetOne()
	for i := 0; i < N; i++ {
		denominators[i].Sub(&x, &h)
		if denominators[i].IsZero() {
			weights[i].SetOne()
			return weights
		}
		h.Mul(&h, &d.Generator)
	}

	invDenominators := koalabear.BatchInvert(denominators)

	var xN, one, common, nInv, nElt koalabear.Element
	xN.Exp(x, big.NewInt(int64(N)))
	one.SetOne()
	common.Sub(&xN, &one)
	nElt.SetUint64(uint64(N))
	nInv.Inverse(&nElt)
	common.Mul(&common, &nInv)

	h.SetOne()
	for i := 0; i < N; i++ {
		weights[i].Mul(&common, &h)
		weights[i].Mul(&weights[i], &invDenominators[i])
		h.Mul(&h, &d.Generator)
	}
	return weights
}

// EvaluateLagrangeWithWeights evaluates a base-field polynomial in Lagrange
// form using precomputed Lagrange basis weights for the same domain.
func EvaluateLagrangeWithWeights(p Polynomial, weights []koalabear.Element) koalabear.Element {
	if len(p) != len(weights) {
		panic("EvaluateLagrangeWithWeights: length mismatch")
	}
	pv := koalabear.Vector(p)
	return (&pv).InnerProduct(koalabear.Vector(weights))
}

// ExtEvaluateLagrangeWithWeights is the extension-field counterpart of
// EvaluateLagrangeWithWeights. The Lagrange weights live in the base koalabear.
func ExtEvaluateLagrangeWithWeights(p ExtPolynomial, weights []koalabear.Element) ext.E6 {
	if len(p) != len(weights) {
		panic("ExtEvaluateLagrangeWithWeights: length mismatch")
	}
	a := ext.VectorE6(p)
	var res, tmp ext.E6
	for i := 0; i < len(a); i++ {
		tmp.MulByElement(&a[i], &weights[i])
		res.Add(&res, &tmp)
	}
	return res
}

// EvaluateOnExtendedDomainRoot evaluates p, given in Lagrange form over d, at
// bigD.Generator^rootIndex.
func EvaluateOnExtendedDomainRoot(p Polynomial, d *fft.Domain, bigD *fft.Domain, rootIndex int) koalabear.Element {
	weights := LagrangeWeightsOnExtendedDomainRoot(d, bigD, rootIndex)
	return EvaluateLagrangeWithWeights(p, weights)
}

// ExtEvaluateOnExtendedDomainRoot is the extension-field counterpart of
// EvaluateOnExtendedDomainRoot.
func ExtEvaluateOnExtendedDomainRoot(p ExtPolynomial, d *fft.Domain, bigD *fft.Domain, rootIndex int) ext.E6 {
	weights := LagrangeWeightsOnExtendedDomainRoot(d, bigD, rootIndex)
	return ExtEvaluateLagrangeWithWeights(p, weights)
}

// PowUint64 returns base^exp in koalabear via binary exponentiation.
// Used to seed per-chunk accumulators when parallelizing loops that
// otherwise carry a serial multiplicative dependency (a_{i+1} = a_i · g).
func PowUint64(base koalabear.Element, exp uint64) koalabear.Element {
	var res koalabear.Element
	res.SetOne()
	b := base
	for exp != 0 {
		if exp&1 == 1 {
			res.Mul(&res, &b)
		}
		b.Mul(&b, &b)
		exp >>= 1
	}
	return res
}

func EvalXnMinusOneOnCoset(n, N int) []koalabear.Element {
	// if !utils.IsPowerOfTwo(n) || !utils.IsPowerOfTwo(N) {
	// 	utils.Panic("both n (%v) and N (%v) must be powers of two", n, N)
	// }
	// if N < n {
	// 	utils.Panic("N (%v) must be ≥ n (%v)", N, n)
	// }

	r := N / n
	nBig := big.NewInt(int64(n))

	// res[0] = MultiplicativeGen^n  (the coset shift raised to the n-th power)
	res := make([]koalabear.Element, r)
	res[0] = fft.GeneratorFullMultiplicativeGroup()
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
	var one koalabear.Element
	one.SetOne()
	for i := 0; i < r; i++ {
		res[i].Sub(&res[i], &one)
	}

	return res
}
