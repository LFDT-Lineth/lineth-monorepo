package reedsolomon

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poly"
)

func TestEvaluateOnExtendedDomainRootMatchesEncode(t *testing.T) {
	p := poly.Polynomial{
		rsBaseElement(1),
		rsBaseElement(2),
		rsBaseElement(3),
		rsBaseElement(4),
	}

	domainD := fft.NewDomain(uint64(len(p)))
	encoder := NewEncoder(8)
	encoded := encoder.Encode(p, domainD)

	for j := range encoded {
		got := poly.EvaluateOnExtendedDomainRoot(p, domainD, encoder.Domain, j)
		if !got.Equal(&encoded[j]) {
			t.Fatalf("evaluation[%d] = %s, want %s", j, got.String(), encoded[j].String())
		}
	}
}

func TestExtEvaluateOnExtendedDomainRootMatchesEncodeExt(t *testing.T) {
	p := poly.ExtPolynomial{
		e6FromU64(1, 2, 3, 4),
		e6FromU64(5, 6, 7, 8),
		e6FromU64(9, 10, 11, 12),
		e6FromU64(13, 14, 15, 16),
	}

	domainD := fft.NewDomain(uint64(len(p)))
	encoder := NewEncoder(8)
	encoded := encoder.EncodeExt(p, domainD)

	for j := range encoded {
		got := poly.ExtEvaluateOnExtendedDomainRoot(p, domainD, encoder.Domain, j)
		if !got.Equal(&encoded[j]) {
			t.Fatalf("evaluation[%d] = %s, want %s", j, got.String(), encoded[j].String())
		}
	}
}

func rsBaseElement(v uint64) koalabear.Element {
	var e koalabear.Element
	e.SetUint64(v)
	return e
}
