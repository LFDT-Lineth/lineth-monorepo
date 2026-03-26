package fastpoly_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/common/fastpoly"
	"github.com/stretchr/testify/require"
)

func TestXMinusOneOnACoset(t *testing.T) {

	n := 4
	N := 32
	ratio := N / n

	res := fastpoly.EvalXnMinusOneOnACoset(n, N)
	require.Equal(t, ratio, len(res), "Bad length")

	/*
		Compute (w_N)^{in} - 1
	*/
	one := koalabear.One()
	for i := 0; i < N; i++ {
		domainN := fft.NewDomain(uint64(N))
		expected := domainN.Generator
		expected.Exp(expected, big.NewInt(int64(i)))
		expected.Mul(&expected, &domainN.FrMultiplicativeGen)
		expected.Exp(expected, big.NewInt(int64(n)))
		expected.Sub(&expected, &one)

		require.Equal(t, expected.String(), res[i%ratio].String())
	}

}
