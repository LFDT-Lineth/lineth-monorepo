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

// /!\ In this package, every inputs polynomials must be in lagrange basis (the inputs come from columns of a trace).

package poly

import (
	"math/bits"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// Polynomial is a wrapper around EPolynomial that includes additional metadata such as shift.
type Polynomial = []koalabear.Element

// evalBufPool pools []koalabear.Element slices used as temporary buffers inside
// BuildGrandProduct and BuildGrandSum. koalabear.Element contains no pointers,
// so pooled slices do not prevent GC of other objects.
var evalBufPool sync.Pool

func getBuf(n int) []koalabear.Element {
	if v := evalBufPool.Get(); v != nil {
		if b := v.([]koalabear.Element); cap(b) >= n {
			return b[:n]
		}
	}
	return make([]koalabear.Element, n)
}

func putBuf(b []koalabear.Element) {
	evalBufPool.Put(b[:cap(b)]) // nolint
}

// Evaluate evaluates a polynomial p in Lagrange form at zeta
// the domain d is assumed to be correctly formed
func Evaluate(p Polynomial, d *fft.Domain, zeta koalabear.Element) koalabear.Element {
	n := len(p)
	_p := getBuf(n)
	copy(_p, p)
	nn := uint64(64 - bits.TrailingZeros64(uint64(n)))
	d.FFTInverse(_p, fft.DIF)
	var res koalabear.Element
	for i := n - 1; i >= 0; i-- {
		iRev := bits.Reverse64(uint64(i)) >> nn
		res.Mul(&res, &zeta)
		res.Add(&res, &_p[iRev])
	}
	putBuf(_p)
	return res
}
