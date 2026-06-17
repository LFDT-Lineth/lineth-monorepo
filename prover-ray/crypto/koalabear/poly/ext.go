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
	"fmt"
	"math/bits"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// ExtPolynomial is a polynomial whose coefficients/evaluations live in the
// Koalabear E4 extension field.
type ExtPolynomial = []ext.E6

var extEvalBufPool sync.Pool

func getExtBuf(n int) []ext.E6 {
	if v := extEvalBufPool.Get(); v != nil {
		if b := v.([]ext.E6); cap(b) >= n {
			return b[:n]
		}
	}
	return make([]ext.E6, n)
}

func putExtBuf(b []ext.E6) {
	extEvalBufPool.Put(b[:cap(b)]) // nolint
}

// AddExt returns the pointwise sum P1 + P2.
func AddExt(P1, P2 ExtPolynomial) (ExtPolynomial, error) {
	if len(P1) != len(P2) {
		return nil, fmt.Errorf("AddExt: length mismatch %d != %d", len(P1), len(P2))
	}
	res := make(ExtPolynomial, len(P1))
	for i := range P1 {
		res[i].Add(&P1[i], &P2[i])
	}
	return res, nil
}

// SubExt returns the pointwise difference P1 - P2.
func SubExt(P1, P2 ExtPolynomial) (ExtPolynomial, error) {
	if len(P1) != len(P2) {
		return nil, fmt.Errorf("SubExt: length mismatch %d != %d", len(P1), len(P2))
	}
	res := make(ExtPolynomial, len(P1))
	for i := range P1 {
		res[i].Sub(&P1[i], &P2[i])
	}
	return res, nil
}

// MulExt returns the pointwise product P1 * P2.
func MulExt(P1, P2 ExtPolynomial) (ExtPolynomial, error) {
	if len(P1) != len(P2) {
		return nil, fmt.Errorf("MulExt: length mismatch %d != %d", len(P1), len(P2))
	}
	res := make(ExtPolynomial, len(P1))
	for i := range P1 {
		res[i].Mul(&P1[i], &P2[i])
	}
	return res, nil
}

// EvaluateAtExt evaluates a base-field polynomial p, stored in Lagrange normal
// form over d, at the extension-field point zeta. Base coefficients are lifted
// during Horner evaluation.
// EvaluateAtExt assumes len(p) is a power of two; behaviour is undefined otherwise.
func EvaluateAtExt(p Polynomial, d *fft.Domain, zeta ext.E6) ext.E6 {
	n := len(p)
	if n == 1 {
		return liftBaseToExt(p[0])
	}
	_p := getBuf(n)
	copy(_p, p)
	nn := uint64(64 - bits.TrailingZeros64(uint64(n)))
	d.FFTInverse(_p, fft.DIF)

	var res ext.E6
	for i := n - 1; i >= 0; i-- {
		iRev := bits.Reverse64(uint64(i)) >> nn
		coeff := liftBaseToExt(_p[iRev])
		res.Mul(&res, &zeta)
		res.Add(&res, &coeff)
	}
	putBuf(_p)
	return res
}

// ExtEvaluateAtExt evaluates an extension-field polynomial p, stored in
// Lagrange normal form over d, at the extension-field point zeta.
func ExtEvaluateAtExt(p ExtPolynomial, d *fft.Domain, zeta ext.E6) ext.E6 {
	n := len(p)
	if n == 1 {
		return p[0]
	}
	_p := getExtBuf(n)
	copy(_p, p)
	nn := uint64(64 - bits.TrailingZeros64(uint64(n)))
	// d.FFTInverseExt(_p, fft.DIF)
	d.FFTInverseExt6(_p, fft.DIF)

	var res ext.E6
	for i := n - 1; i >= 0; i-- {
		iRev := bits.Reverse64(uint64(i)) >> nn
		coeff := _p[iRev]
		res.Mul(&res, &zeta)
		res.Add(&res, &coeff)
	}
	putExtBuf(_p)
	return res
}

// DeepQuotientExt computes q(X) = (v - p(X)) / (z - X) for an extension-field
// polynomial p in Lagrange normal form over d. The domain points remain
// base-field roots of unity and are lifted into E4 for the denominator.
func DeepQuotientExt(p ExtPolynomial, v, z ext.E6, d *fft.Domain) ExtPolynomial {
	N := len(p)
	q := make(ExtPolynomial, N)
	var omegaJ koalabear.Element
	omegaJ.SetOne()
	omega := d.Generator
	for j := 0; j < N; j++ {
		var num, den ext.E6
		omegaJExt := liftBaseToExt(omegaJ)
		num.Sub(&v, &p[j])
		den.Sub(&z, &omegaJExt)
		den.Inverse(&den)
		q[j].Mul(&num, &den)
		omegaJ.Mul(&omegaJ, &omega)
	}
	return q
}

func liftBaseToExt(v koalabear.Element) ext.E6 {
	var res ext.E6
	// res.Lift(&v)
	res.B0.A0.Set(&v)
	return res
}
