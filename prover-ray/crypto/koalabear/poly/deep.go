package poly

import (
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

func LinComb(v []koalabear.Element, alpha koalabear.Element) koalabear.Element {
	var res koalabear.Element
	for _, _v := range v {
		res.Mul(&res, &alpha)
		res.Add(&res, &_v)
	}
	return res
}

// DeepQuotient computes q(X) = (v - p(X)) / (z - X) where p is in Lagrange Normal form
// over domain d and v = p(z) is the claimed evaluation at z outside the domain.
// Returns q in Lagrange Normal form: q[j] = (v - p(ω^j)) / (z - ω^j).
// Panics (division by zero) if z happens to be a domain point.
func DeepQuotient(p Polynomial, v, z koalabear.Element, d *fft.Domain) Polynomial {
	N := len(p)
	q := make(Polynomial, N)
	var omegaJ koalabear.Element
	omegaJ.SetOne()
	omega := d.Generator

	for j := 0; j < N; j++ {
		var num, den koalabear.Element
		num.Sub(&v, &p[j])
		den.Sub(&z, &omegaJ)
		den.Inverse(&den)
		q[j].Mul(&num, &den)
		omegaJ.Mul(&omegaJ, &omega)
	}
	return q
}
