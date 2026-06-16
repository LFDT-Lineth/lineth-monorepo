package reedsolomon

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poly"
)

func e6FromU64(a0, a1, b0, b1 uint64, b2 ...uint64) ext.E6 {
	var z ext.E6
	z.B0.A0.SetUint64(a0)
	z.B0.A1.SetUint64(a1)
	z.B1.A0.SetUint64(b0)
	z.B1.A1.SetUint64(b1)
	if len(b2) > 0 {
		z.B2.A0.SetUint64(b2[0])
	}
	if len(b2) > 1 {
		z.B2.A1.SetUint64(b2[1])
	}
	return z
}

func liftE6(v koalabear.Element) ext.E6 {
	var z ext.E6
	z.B0.A0.Set(&v)
	return z
}

func canonicalEvalExt(coeffs []ext.E6, z ext.E6) ext.E6 {
	if len(coeffs) == 0 {
		return ext.E6{}
	}
	y := coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		y.Mul(&y, &z)
		y.Add(&y, &coeffs[i])
	}
	return y
}

func TestEncodeExt(t *testing.T) {
	coeffs := poly.ExtPolynomial{
		e6FromU64(1, 2, 3, 4),
		e6FromU64(5, 6, 7, 8),
		e6FromU64(9, 10, 11, 12),
		e6FromU64(13, 14, 15, 16),
	}

	domainD := fft.NewDomain(uint64(len(coeffs)))
	p := make(poly.ExtPolynomial, len(coeffs))
	copy(p, coeffs)
	domainD.FFTExt6(p, fft.DIF)
	utils.BitReverse(p)

	encoder := NewEncoder(8)
	encoded := encoder.EncodeExt(p, domainD)

	domainN := fft.NewDomain(uint64(len(encoded)))
	omega := domainN.Generator
	var omegaJ koalabear.Element
	omegaJ.SetOne()
	for j := range encoded {
		x := liftE6(omegaJ)
		want := canonicalEvalExt(coeffs, x)
		if !encoded[j].Equal(&want) {
			t.Fatalf("encoded[%d] = %s, want %s", j, encoded[j].String(), want.String())
		}
		omegaJ.Mul(&omegaJ, &omega)
	}
}
