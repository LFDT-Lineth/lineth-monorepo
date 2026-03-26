package vector_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectors(t *testing.T) {

	var (
		// a, b and x are very common vectors in all the tests
		a = vector.ForTest(1, 2, 3, 4, 5)
		b = vector.ForTest(3, 4, 5, 6, 7)
		x = field.NewElement(2)

		// aBAndXMustNotChange asserts that a and b did not change as this is
		// a very common check in all the sub-tests.
		aBAndXMustNotChange = func(t *testing.T) {
			require.Equal(t, vector.ForTest(1, 2, 3, 4, 5), a, "a must not change")
			require.Equal(t, vector.ForTest(3, 4, 5, 6, 7), b, "b must not change")
			require.Equal(t, "2", x.String(), "x must not change")
		}
	)

	t.Run("DeepCopy", func(t *testing.T) {
		c := vector.DeepCopy(a)
		assert.Equal(t, a, c, "the deep copied vector must be equal")
		c[0] = field.NewElement(40)
		aBAndXMustNotChange(t)
	})

	t.Run("ScalarMul", func(t *testing.T) {
		c := vector.DeepCopy(a)
		vector.ScalarMul(c, b, x)
		assert.Equal(t, vector.ForTest(6, 8, 10, 12, 14), c, "c must be equal to 2*b")
		vector.ScalarMul(c, c, x)
		assert.Equal(t, vector.ForTest(12, 16, 20, 24, 28), c, "c must be equal to 2*b")
		aBAndXMustNotChange(t)
	})

	t.Run("ScalarProd", func(t *testing.T) {
		c := vector.ScalarProd(a, b)
		assert.Equal(t, "85", c.String())
		aBAndXMustNotChange(t)
	})

	t.Run("Rand", func(t *testing.T) {
		c, d := vector.Rand(5), vector.Rand(5)
		assert.NotEqual(t, c, d, "Rand should not return twice the same value")
	})

	t.Run("MulElementWise", func(t *testing.T) {
		c := vector.DeepCopy(b)
		vector.MulElementWise(c, b, a)
		assert.Equal(t, vector.ForTest(3, 8, 15, 24, 35), c)

		c = vector.DeepCopy(b)
		vector.MulElementWise(c, c, a)
		assert.Equal(t, vector.ForTest(3, 8, 15, 24, 35), c)

		aBAndXMustNotChange(t)
	})

	t.Run("Prettify", func(t *testing.T) {
		assert.Equal(t, "[1, 2, 3, 4, 5]", vector.Prettify(a))
		aBAndXMustNotChange(t)
	})

	t.Run("Reverse", func(t *testing.T) {
		c := vector.DeepCopy(a)
		vector.Reverse(c)
		assert.Equal(t, vector.ForTest(5, 4, 3, 2, 1), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Repeat", func(t *testing.T) {
		c := vector.Repeat(x, 5)
		assert.Equal(t, vector.ForTest(2, 2, 2, 2, 2), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Add", func(t *testing.T) {
		c := vector.DeepCopy(a)
		vector.Add(c, a, b)
		assert.Equal(t, vector.ForTest(4, 6, 8, 10, 12), c)

		c = vector.DeepCopy(a)
		vector.Add(c, c, b)
		assert.Equal(t, vector.ForTest(4, 6, 8, 10, 12), c)

		c = vector.DeepCopy(a)
		vector.Add(c, a, b, a)
		assert.Equal(t, vector.ForTest(5, 8, 11, 14, 17), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Sub", func(t *testing.T) {
		c := vector.DeepCopy(a)
		vector.Sub(c, b, a)
		assert.Equal(t, vector.ForTest(2, 2, 2, 2, 2), c)

		c = vector.DeepCopy(a)
		vector.Sub(c, b, c)
		assert.Equal(t, vector.ForTest(2, 2, 2, 2, 2), c)

		c = vector.DeepCopy(b)
		vector.Sub(c, c, a)
		assert.Equal(t, vector.ForTest(2, 2, 2, 2, 2), c)
		aBAndXMustNotChange(t)
	})

	t.Run("ZeroPad", func(t *testing.T) {
		c := vector.ZeroPad(a, 7)
		assert.Equal(t, vector.ForTest(1, 2, 3, 4, 5, 0, 0), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Interleave", func(t *testing.T) {
		c := vector.Interleave(a, b)
		assert.Equal(t, vector.ForTest(1, 3, 2, 4, 3, 5, 4, 6, 5, 7), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Fill", func(t *testing.T) {
		c := vector.DeepCopy(a)
		vector.Fill(c, x)
		assert.Equal(t, vector.ForTest(2, 2, 2, 2, 2), c)
		aBAndXMustNotChange(t)
	})

	t.Run("PowerVec", func(t *testing.T) {
		c := vector.PowerVec(x, 5)
		assert.Equal(t, vector.ForTest(1, 2, 4, 8, 16), c)
	})

	t.Run("IntoGnarkAssignment", func(t *testing.T) {
		c := vector.IntoGnarkAssignment(a)
		for i := range c {
			ci := c[i].(field.Element)
			assert.Equal(t, a[i].String(), ci.String())
		}
		aBAndXMustNotChange(t)
	})

}

func TestReverse(t *testing.T) {
	vec := []field.Element{field.NewElement(0), field.NewElement(1), field.NewElement(2)}
	vector.Reverse(vec)
	require.Equal(t, vec[0], field.NewElement(2))
	require.Equal(t, vec[1], field.NewElement(1))
	require.Equal(t, vec[2], field.NewElement(0))
}

func TestScalarProd(t *testing.T) {
	require.Equal(t,
		vector.ScalarProd(
			vector.ForTest(1, 2, 3, 4),
			vector.ForTest(1, 2, 3, 4),
		),
		field.NewElement(30),
	)
}

func TestForTest(t *testing.T) {
	testcase := vector.ForTest(-1, 0, 1)
	require.Equal(t, testcase[0].String(), "-1")
	require.Equal(t, testcase[1].String(), "0")
	require.Equal(t, testcase[2].String(), "1")
}
