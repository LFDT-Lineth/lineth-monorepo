// Copyright 2020 ConsenSys AG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fext

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
)

// Mul sets z to the Element-product of x,y, returns z
func (z *Element) Mul(x, y *Element) *Element {
	var a, b, c fr.Element
	a.Add(&x.A0, &x.A1)
	b.Add(&y.A0, &y.A1)
	a.Mul(&a, &b)
	b.Mul(&x.A0, &y.A0)
	c.Mul(&x.A1, &y.A1)
	z.A1.Sub(&a, &b).Sub(&z.A1, &c)
	MulByQnr(&c)
	z.A0.Sub(&b, &c)
	return z
}

// Square sets z to the Element-product of x,x returns z
func (z *Element) Square(x *Element) *Element {
	//algo 22 https://eprint.iacr.org/2010/354.pdf
	z.Mul(x, x)
	return z
}

// MulByNonResidue multiplies a Element by (0,1)
func (z *Element) MulByNonResidue(x *Element) *Element {
	a := x.A0
	b := x.A1 // fetching x.A1 in the function below is slower
	MulByQnr(&b)
	z.A0.Neg(&b)
	z.A1 = a
	return z
}

// MulByNonResidueInv multiplies a Element by (0,1)^{-1}
func (z *Element) MulByNonResidueInv(x *Element) *Element {
	//z.A1.MulByNonResidueInv(&x.A0)
	a := x.A1
	qnr := new(fr.Element).SetInt64(noQNR)
	var qnrInv fr.Element
	qnrInv.Inverse(qnr)
	z.A1.Mul(&x.A0, &qnrInv).Neg(&z.A1)
	z.A0 = a
	return z
}

// Inverse sets z to the Element-inverse of x, returns z
func (z *Element) Inverse(x *Element) *Element {
	// Algorithm 8 from https://eprint.iacr.org/2010/354.pdf
	//var a, b, t0, t1, tmp fr.Element
	var t0, t1, tmp fr.Element
	a := &x.A0 // creating the buffers a, b is faster than querying &x.A0, &x.A1 in the functions call below
	b := &x.A1
	t0.Square(a)
	t1.Square(b)
	tmp.Set(&t1)
	MulByQnr(&tmp)
	t0.Add(&t0, &tmp)
	t1.Inverse(&t0)
	z.A0.Mul(a, &t1)
	z.A1.Mul(b, &t1).Neg(&z.A1)

	return z
}

// norm sets x to the norm of z
func (z *Element) norm(x *fr.Element) {
	var tmp fr.Element
	x.Square(&z.A1)
	tmp.Set(x)
	MulByQnr(&tmp)
	x.Square(&z.A0).Add(x, &tmp)
	// A0^2+A1^2*QNR
}
func MulByQnr(x *fr.Element) {
	old := new(fr.Element).Set(x)
	for i := 0; i < noQNR-1; i++ {
		x.Add(x, old)
	}
}

/*
// MulBybTwistCurveCoeff multiplies by 1/(0,1)
func (z *Element) MulBybTwistCurveCoeff(x *Element) *Element {

	var res Element
	res.A0.Set(&x.A1)
	res.A1.MulByNonResidueInv(&x.A0)
	z.Set(&res)

	return z
}
*/
