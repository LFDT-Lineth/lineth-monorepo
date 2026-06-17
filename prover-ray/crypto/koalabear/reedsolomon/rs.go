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

package reedsolomon

import (
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poly"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

func NewEncoder(N uint64) Encoder {
	return NewEncoderWithDomainCache(N, nil)
}

// NewEncoderWithDomainCache constructs an encoder using cache for its FFT
// domain.
func NewEncoderWithDomainCache(N uint64, cache *poly.DomainCache) Encoder {
	return Encoder{Domain: cache.Get(N)}
}

type Encoder struct {
	Domain *fft.Domain
}

// scatterBitReversedCoeffs expands n-bit-reversed coefficients into the
// matching N-bit-reversed zero-padded slots, in place.
func scatterBitReversedCoeffs[T any](p []T, n, N int) {
	if n <= 1 {
		return
	}
	shift := bits.TrailingZeros64(uint64(N)) - bits.TrailingZeros64(uint64(n))
	stride := 1 << shift
	for i := n - 1; i >= 0; i-- {
		p[i<<shift] = p[i]
	}
	if stride == 1 {
		return
	}
	var zero T
	for i := 1; i < n; i++ {
		if i&(stride-1) != 0 {
			p[i] = zero
		}
	}
}

// RSEncode evalutes p on the N-th roots of unity (N must be > len(p))
// p is in Lagrange form
// it returns a copy of p
// Optional fftOpts are forwarded to both internal FFTs (e.g. to cap inner
// parallelism with fft.WithNbTasks when Encode is itself called inside a
// parallel.Execute loop).
func (encoder *Encoder) Encode(p poly.Polynomial, d *fft.Domain, fftOpts ...fft.Option) poly.Polynomial {

	// get the size of p
	n := len(p)

	// create _p, a copy of p of size N (zero-padded)
	N := encoder.Domain.Cardinality
	_p := make(poly.Polynomial, N)
	copy(_p, p)

	// Lagrange normal to canonical bit-reversed (w.r.t. n). We place those
	// coefficients directly in N-bit-reversed order and use a DIT FFT, avoiding
	// the two explicit BitReverse passes previously needed for normal order.
	d.FFTInverse(_p[:n], fft.DIF, fftOpts...)
	scatterBitReversedCoeffs(_p, n, int(N))
	encoder.Domain.FFT(_p, fft.DIT, fftOpts...)

	// return _p
	return _p
}

// EncodeExt evaluates an extension-field polynomial on the encoder domain.
// The input p is in Lagrange normal form over d; the output is a fresh
// extension polynomial in Lagrange normal form over encoder.Domain.
func (encoder *Encoder) EncodeExt(p poly.ExtPolynomial, d *fft.Domain, fftOpts ...fft.Option) poly.ExtPolynomial {
	n := len(p)

	N := encoder.Domain.Cardinality
	_p := make(poly.ExtPolynomial, N)
	copy(_p, p)

	d.FFTInverseExt6(_p[:n], fft.DIF, fftOpts...)
	scatterBitReversedCoeffs(_p, n, int(N))
	encoder.Domain.FFTExt6(_p, fft.DIT, fftOpts...)

	return _p
}
