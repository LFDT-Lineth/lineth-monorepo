package fastpoly_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/require"
)

func TestReEvalOnCoset(t *testing.T) {

	// With the constant polynomial
	smaller := vector.ForTest(1, 1, 1, 1)
	larger := fastpoly.ReEvaluateOnLargerDomainCoset(smaller, 8)
	require.Equal(t, append(smaller, smaller...), larger)

	// With the identity polynomial
	smaller = vector.ForTest(0, 1, 0, 0)
	expectedLarger := vector.ZeroPad(smaller, 8)

	fft.NewDomain(4).WithCoset().FFT(smaller, fft.DIF)
	fft.BitReverse(smaller)
	fft.NewDomain(8).WithCoset().FFT(expectedLarger, fft.DIF, fft.OnCoset())
	fft.BitReverse(expectedLarger)

	larger = fastpoly.ReEvaluateOnLargerDomainCoset(smaller, 8)
	require.Equal(t, expectedLarger, larger)

}

func TestXMinusOneOnACoset(t *testing.T) {

	n := 4
	N := 32
	ratio := N / n

	res := fastpoly.EvalXnMinusOneOnACoset(n, N)
	require.Equal(t, ratio, len(res), "Bad length")

	/*
		Compute (w_N)^{in} - 1
	*/
	one := field.One()
	for i := 0; i < N; i++ {
		domainN := fft.NewDomain(N)
		expected := domainN.Generator
		expected.Exp(expected, big.NewInt(int64(i)))
		expected.Mul(&expected, &domainN.FrMultiplicativeGen)
		expected.Exp(expected, big.NewInt(int64(n)))
		expected.Sub(&expected, &one)

		require.Equal(t, expected.String(), res[i%ratio].String())
	}

}

func BenchmarkReEvalOnCoset(b *testing.B) {

	// logRatio = 2 means that we want to reevaluate on a coset that is
	// 4 time larger
	lowPow := 20
	bigPow := 22

	for _, logRatio := range []int{1, 2, 3, 4} {
		for logSize := lowPow; logSize <= bigPow; logSize++ {
			// With the constant polynomial
			smaller := vector.Rand(1 << logSize)
			// Dummy run to ensure, the domain is precomputed
			_ = fastpoly.ReEvaluateOnLargerDomainCoset(smaller, 1<<(logSize+logRatio))

			b.Run(fmt.Sprintf("Domain of size %v - ratio %v", 1<<logSize, 1<<logRatio), func(b *testing.B) {
				for k := 0; k < b.N; k++ {
					_ = fastpoly.ReEvaluateOnLargerDomainCoset(smaller, 1<<(logSize+logRatio))
				}
			})
		}
	}

}
