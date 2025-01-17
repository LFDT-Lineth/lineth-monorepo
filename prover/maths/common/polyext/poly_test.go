package polyext_test

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/polyext"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"

	"github.com/stretchr/testify/require"
)

func TestEvalUnivariate(t *testing.T) {
	// Just a simple test vector
	// (1+a)+(2+2a)X+(5+a)X^2+(12+2a)X^3
	testVec := []fext.Element{
		fext.NewElement(1, 1),
		fext.NewElement(2, 2),
		fext.NewElement(5, 1),
		fext.NewElement(12, 2),
	}

	x := fext.NewElement(3, 4)

	y := polyext.EvalUnivariate(testVec, x)
	// expanded form of the polynomial 128 a^4 + 1072 a^3 + 2056 a^2 + 1494 a + 376
	first := 128*fext.RootPowers[1]*fext.RootPowers[1] + 2056*fext.RootPowers[1] + 376
	second := 1072*fext.RootPowers[1] + 1494
	require.Equal(t, y, *new(fext.Element).SetInt64Pair(int64(first), int64(second)))
}

func TestMul(t *testing.T) {

	t.Run("same-size", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, 2, 3)    // (1+a)+(2+3a)*X
			b        = vectorext.ForTestFromPairs(-1, -1, 1, -2) // (-1-a)+(1-2a)*X
			expected = vectorext.ForTestFromPairs(
				-fext.RootPowers[1]-1,
				-2,
				-5*fext.RootPowers[1]-1,
				-6,
				-6*fext.RootPowers[1]+2,
				-1,
			) // (-6a^2-a+2) X^2 + (-5a^2-6a-1)X - a^2 - 2a - 1
			res = polyext.Mul(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("a-is-smaller", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, -1, -2)        // (1+a)+(-1-2a)*X
			b        = vectorext.ForTestFromPairs(-1, -2, 0, 1, 2, -2) // (-1-2a)+aX+(2-2a)X^2
			expected = vectorext.ForTestFromPairs(
				-2*fext.RootPowers[1]-1,
				-3,
				5*fext.RootPowers[1]+1,
				5,
				-4*fext.RootPowers[1]+2,
				-1,
				4*fext.RootPowers[1]-2,
				-2,
			) // (4a^2-2a-2)X^3+(-4a^2-a+2) X^2+(5a^2+5a+1)X-2a^2-3a-1
			res = polyext.Mul(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("a-is-larger", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(-1, 1, 0, 0, 1, 2) // (-1+a)+(1+2a)Xˆ2
			b        = vectorext.ForTestFromPairs(1, 1, 2, -1)       // (1+a)+(2-a)X
			expected = vectorext.ForTestFromPairs(
				-1+fext.RootPowers[1],
				0,
				-fext.RootPowers[1]-2,
				3,
				2*fext.RootPowers[1]+1,
				3,
				-2*fext.RootPowers[1]+2,
				3,
			) // (2+3a-2a^2)X^3+(1+3a+2a^2)Xˆ2+(-2+3a-aˆ2)X+(-1+a^2)
			res = polyext.Mul(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("a-is-empty", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs()
			b        = vectorext.ForTestFromPairs(1, 1, 2, 2)
			expected = vectorext.ForTestFromPairs()
			res      = polyext.Mul(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("b-is-empty", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, 2, 2)
			b        = vectorext.ForTestFromPairs()
			expected = vectorext.ForTestFromPairs()
			res      = polyext.Mul(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})
}

func TestAdd(t *testing.T) {

	t.Run("same-size", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, 2, 1)
			b        = vectorext.ForTestFromPairs(-1, 1, 3, 3)
			expected = vectorext.ForTestFromPairs(0, 2, 5, 4)
			res      = polyext.Add(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("a-is-smaller", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, 2, 2)
			b        = vectorext.ForTestFromPairs(1, 1, 1, 1, 3, 2, 7, 8)
			expected = vectorext.ForTestFromPairs(2, 2, 3, 3, 3, 2, 7, 8)
			res      = polyext.Add(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("a-is-larger", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, 1, 1, 2, 2, 3, 3)
			b        = vectorext.ForTestFromPairs(1, 1, 4, 4)
			expected = vectorext.ForTestFromPairs(2, 2, 5, 5, 2, 2, 3, 3)
			res      = polyext.Add(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("a-is-empty", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs()
			b        = vectorext.ForTestFromPairs(1, 1, 2, 2)
			expected = vectorext.ForTestFromPairs(1, 1, 2, 2)
			res      = polyext.Add(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("b-is-empty", func(t *testing.T) {
		var (
			a        = vectorext.ForTestFromPairs(1, 1, 2, 2)
			b        = vectorext.ForTestFromPairs()
			expected = vectorext.ForTestFromPairs(1, 1, 2, 2)
			res      = polyext.Add(a, b)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})
}

func TestScalarMul(t *testing.T) {

	t.Run("normal-vec", func(t *testing.T) {
		var (
			vec      = vectorext.ForTestFromPairs(1, 2, 3, 4, 2, 1)
			x        = fext.NewElement(2, 1)
			expected = vectorext.ForTestFromPairs(
				2+2*fext.RootPowers[1],
				5,
				6+4*fext.RootPowers[1],
				11,
				4+fext.RootPowers[1],
				4,
			)
			res = polyext.ScalarMul(vec, x)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

	t.Run("empty-vec", func(t *testing.T) {
		var (
			vec      = vectorext.ForTestFromPairs()
			x        = fext.NewElement(2, 2)
			expected = vectorext.ForTestFromPairs()
			res      = polyext.ScalarMul(vec, x)
		)
		require.Equal(t, vectorext.Prettify(expected), vectorext.Prettify(res))
	})

}

func TestEvaluateLagrangeAnyDomain(t *testing.T) {

	t.Run("single-point-domain", func(t *testing.T) {
		var (
			domain   = []fext.Element{{field.Zero(), field.Zero()}}
			x        = fext.NewElement(42, 42)
			ys       = polyext.EvaluateLagrangesAnyDomain(domain, x)
			expected = vectorext.ForTestFromPairs(1, 0)
		)
		require.Equal(t, expected, ys)
	})

	t.Run("many-point-domain", func(t *testing.T) {
		var (
			// the first lagrange poly is 1-X and the second one is X
			domain   = vectorext.ForTestFromPairs(0, 0, 1, 0)
			x        = fext.NewElement(42, 0)
			ys       = polyext.EvaluateLagrangesAnyDomain(domain, x)
			expected = vectorext.ForTestFromPairs(-41, 0, 42, 0)
		)
		require.Equal(t, expected, ys)
	})

	t.Run("many-point-domain-random", func(t *testing.T) {
		// #nosec G404 -- we don't need a cryptographic PRNG for testing purposes
		rng := rand.New(utils.NewRandSource((43)))
		rand := []fext.Element{fext.PseudoRand(rng), fext.PseudoRand(rng), fext.PseudoRand(rng)}
		randX := fext.PseudoRand(rng)
		expectedYs := []fext.Element{}
		for i, coord := range rand {
			var term1, term2, aux, y fext.Element
			// compute the first term
			term1.Sub(&randX, &rand[(i+3+1)%3])
			aux.Sub(&coord, &rand[(i+3+1)%3])
			term1.Div(&term1, &aux)
			// compute the second term
			term2.Sub(&randX, &rand[(i+3-1)%3])
			aux.Sub(&coord, &rand[(i+3-1)%3])
			term2.Div(&term2, &aux)
			// add the expected y term to expectedYs
			y.Mul(&term1, &term2)
			expectedYs = append(expectedYs, y)
		}
		var (
			domain   = rand
			x        = randX
			ys       = polyext.EvaluateLagrangesAnyDomain(domain, x)
			expected = expectedYs
		)
		require.Equal(t, expected, ys)
	})

}
