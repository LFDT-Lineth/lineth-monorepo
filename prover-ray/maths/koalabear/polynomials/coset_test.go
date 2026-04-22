package polynomials

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

func TestEvalXnMinusOneOnCoset(t *testing.T) {
	const n, N = 4, 32
	ratio := N / n

	res := EvalXnMinusOneOnCoset(n, N)
	if len(res) != ratio {
		t.Fatalf("len: got %d, want %d", len(res), ratio)
	}

	// Reference: for each i in [0, N), compute
	//   ( FrMultiplicativeGen · ωᴺ^i )^n - 1
	// and verify it equals res[i % ratio].
	domainN := fft.NewDomain(uint64(N))
	var one field.Element
	one.SetOne()

	for i := 0; i < N; i++ {
		expected := domainN.Generator
		expected.Exp(expected, big.NewInt(int64(i)))
		expected.Mul(&expected, &domainN.FrMultiplicativeGen)
		expected.Exp(expected, big.NewInt(int64(n)))
		expected.Sub(&expected, &one)

		if expected.String() != res[i%ratio].String() {
			t.Fatalf("i=%d (slot %d): got %s, want %s", i, i%ratio, res[i%ratio].String(), expected.String())
		}
	}
}
