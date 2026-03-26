package poly_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"

	"github.com/stretchr/testify/require"
)

func TestEvalUnivariate(t *testing.T) {
	// Just a simple test vector
	testVec := []field.Element{
		field.NewElement(1),
		field.NewElement(2),
		field.NewElement(5),
		field.NewElement(12),
	}

	x := field.NewElement(17)

	y := poly.EvalUnivariate(testVec, x)

	require.Equal(t, y, field.NewElement(60436))

}

func TestMul(t *testing.T) {

	t.Run("same-size", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1)
			b        = vector.ForTest(-1, 1)
			expected = vector.ForTest(-1, 0, 1)
			res      = poly.Mul(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("a-is-smaller", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1)
			b        = vector.ForTest(-1, 0, 1)
			expected = vector.ForTest(-1, -1, 1, 1)
			res      = poly.Mul(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("a-is-larger", func(t *testing.T) {
		var (
			a        = vector.ForTest(-1, 0, 1)
			b        = vector.ForTest(1, 1)
			expected = vector.ForTest(-1, -1, 1, 1)
			res      = poly.Mul(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("a-is-empty", func(t *testing.T) {
		var (
			a        = vector.ForTest()
			b        = vector.ForTest(1, 1)
			expected = vector.ForTest()
			res      = poly.Mul(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("b-is-empty", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1)
			b        = vector.ForTest()
			expected = vector.ForTest()
			res      = poly.Mul(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})
}

func TestAdd(t *testing.T) {

	t.Run("same-size", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1)
			b        = vector.ForTest(-1, 1)
			expected = vector.ForTest(0, 2)
			res      = poly.Add(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("a-is-smaller", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1)
			b        = vector.ForTest(1, 1, 1, 1)
			expected = vector.ForTest(2, 2, 1, 1)
			res      = poly.Add(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("a-is-larger", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1, 1, 1)
			b        = vector.ForTest(1, 1)
			expected = vector.ForTest(2, 2, 1, 1)
			res      = poly.Add(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("a-is-empty", func(t *testing.T) {
		var (
			a        = vector.ForTest()
			b        = vector.ForTest(1, 1)
			expected = vector.ForTest(1, 1)
			res      = poly.Add(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("b-is-empty", func(t *testing.T) {
		var (
			a        = vector.ForTest(1, 1)
			b        = vector.ForTest()
			expected = vector.ForTest(1, 1)
			res      = poly.Add(a, b)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})
}

func TestScalarMul(t *testing.T) {

	t.Run("normal-vec", func(t *testing.T) {
		var (
			vec      = vector.ForTest(1, 2, 3, 4)
			x        = field.NewElement(2)
			expected = vector.ForTest(2, 4, 6, 8)
			res      = poly.ScalarMul(vec, x)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

	t.Run("empty-vec", func(t *testing.T) {
		var (
			vec      = vector.ForTest()
			x        = field.NewElement(2)
			expected = vector.ForTest()
			res      = poly.ScalarMul(vec, x)
		)
		require.Equal(t, vector.Prettify(expected), vector.Prettify(res))
	})

}

func TestEvaluateLagrangeAnyDomain(t *testing.T) {

	t.Run("single-point-domain", func(t *testing.T) {
		var (
			domain   = []field.Element{field.Zero()}
			x        = field.NewElement(42)
			ys       = poly.EvaluateLagrangesAnyDomain(domain, x)
			expected = vector.ForTest(1)
		)
		require.Equal(t, expected, ys)
	})

	t.Run("many-point-domain", func(t *testing.T) {
		var (
			// the first lagrange poly is 1-X and the second one is X
			domain   = vector.ForTest(0, 1)
			x        = field.NewElement(42)
			ys       = poly.EvaluateLagrangesAnyDomain(domain, x)
			expected = vector.ForTest(-41, 42)
		)
		require.Equal(t, expected, ys)
	})

}
