package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
)

func TestEvaluateOnExtendedDomainRootMatchesEncode(t *testing.T) {

	var (
		p = []field.Element{
			field.NewElement(1),
			field.NewElement(5),
			field.NewElement(9),
			field.NewElement(13),
		}
		codec   = NewEncoder(8, len(p))
		encoded = codec.Encode(p)
		prng    = rand.New(utils.NewRandSource(0))
	)

	for j := range encoded {

		var (
			x    = field.PseudoRandElemExt(prng)
			got  = polynomials.EvalLagrange(field.VecFromBase(encoded), x)
			want = polynomials.EvalLagrange(field.VecFromBase(p), x)
		)

		if !got.Equal(&want.Ext) {
			t.Fatalf("evaluation[%d] = %s, want %s", j, got.String(), encoded[j].String())
		}
	}
}

func TestExtEvaluateOnExtendedDomainRootMatchesEncodeExt(t *testing.T) {

	var (
		p = []field.Ext{
			field.IntsToExt(1, 2, 3, 4, 0, 0),
			field.IntsToExt(5, 6, 7, 8, 0, 0),
			field.IntsToExt(9, 10, 11, 12, 0, 0),
			field.IntsToExt(13, 14, 15, 16, 0, 0),
		}

		codec   = NewEncoder(8, len(p))
		encoded = codec.EncodeExt(p)
		prng    = rand.New(utils.NewRandSource(0))
	)

	for j := range encoded {

		var (
			x    = field.PseudoRandElemExt(prng)
			got  = polynomials.EvalLagrange(field.VecFromExt(encoded), x)
			want = polynomials.EvalLagrange(field.VecFromExt(p), x)
		)

		if !got.Equal(&want.Ext) {
			t.Fatalf("evaluation[%d] = %s, want %s", j, got.String(), encoded[j].String())
		}
	}
}

func TestEncodeExt(t *testing.T) {

	var (
		coeffs = []field.Ext{
			field.IntsToExt(1, 2, 3, 4, 0, 0),
			field.IntsToExt(5, 6, 7, 8, 0, 0),
			field.IntsToExt(9, 10, 11, 12, 0, 0),
			field.IntsToExt(13, 14, 15, 16, 0, 0),
		}
		domainD = fft.NewDomain(uint64(len(coeffs)))
		p       = make([]field.Ext, len(coeffs))
	)

	copy(p, coeffs)
	domainD.FFTExt6(p, fft.DIF)
	gutils.BitReverse(p)

	var (
		codec   = NewEncoder(8, len(coeffs))
		encoded = codec.EncodeExt(p)
		domainN = fft.NewDomain(uint64(len(encoded)))
		omega   = domainN.Generator
		omegaJ  = field.One()
	)

	for j := range encoded {
		x := field.Lift(omegaJ)
		want := polynomials.EvalCanonicalExt(coeffs, x)
		if !encoded[j].Equal(&want) {
			t.Fatalf("encoded[%d] = %s, want %s", j, encoded[j].String(), want.String())
		}
		omegaJ.Mul(&omegaJ, &omega)
	}
}
