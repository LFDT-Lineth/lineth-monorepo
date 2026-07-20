package fri

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateOnExtendedDomainRootMatchesEncode(t *testing.T) {

	var (
		p = []field.Element{
			field.NewElement(1),
			field.NewElement(5),
			field.NewElement(9),
			field.NewElement(13),
		}
	)

	codec := NewEncoder(8, len(p))
	encoded := codec.Encode(p)
	domain := domainLight{cardinality: codec.Domain.Cardinality, generator: codec.Domain.Generator}
	logN := utils.Log2Ceil(len(encoded))
	rate := codec.InverseRate()

	for pos := range encoded {
		naturalIndex := bitReverseIdx(pos, logN)
		var want field.Element
		if naturalIndex%rate == 0 {
			want = p[naturalIndex/rate]
		} else {
			want = polynomials.EvalLagrange(
				field.VecFromBase(p),
				field.ElemFromBase(domainPoint(domain, pos)),
			).AsBase()
		}
		assert.Equal(t, want, encoded[pos], "encoded[%d]", pos)
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
		domain  = domainLight{cardinality: codec.Domain.Cardinality, generator: codec.Domain.Generator}
		logN    = utils.Log2Ceil(len(encoded))
		rate    = codec.InverseRate()
	)

	for pos := range encoded {
		naturalIndex := bitReverseIdx(pos, logN)
		var want field.Ext
		if naturalIndex%rate == 0 {
			want = p[naturalIndex/rate]
		} else {
			want = polynomials.EvalLagrange(
				field.VecFromExt(p),
				field.ElemFromExt(domainPointExt(domain, pos)),
			).AsExt()
		}
		assert.Equal(t, want, encoded[pos], "encoded[%d]", pos)
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
		domain  = domainLight{cardinality: codec.Domain.Cardinality, generator: codec.Domain.Generator}
	)

	for j := range encoded {
		want := polynomials.EvalCanonicalExt(coeffs, domainPointExt(domain, j))
		assert.Equal(t, want, encoded[j], "encoded[%d]", j)
	}
}
