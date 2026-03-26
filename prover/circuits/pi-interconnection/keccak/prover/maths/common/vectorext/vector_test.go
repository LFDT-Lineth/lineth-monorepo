package vectorext_test

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectors(t *testing.T) {

	var (
		// a, b and x are very common vectors in all the tests
		a = vectorext.ForTestFromPairs(1, 2, 3, 4, 5, 6)
		b = vectorext.ForTestFromPairs(3, 4, 5, 6, 7, 8)
		x = fext.NewElement(2, 0)

		// aBAndXMustNotChange asserts that a and b did not change as this is
		// a very common check in all the sub-tests.
		aBAndXMustNotChange = func(t *testing.T) {
			require.Equal(t, vectorext.ForTestFromPairs(1, 2, 3, 4, 5, 6), a, "a must not change")
			require.Equal(t, vectorext.ForTestFromPairs(3, 4, 5, 6, 7, 8), b, "b must not change")
			require.Equal(t, "2+0*u", x.String(), "x must not change")
		}
	)

	t.Run("DeepCopy", func(t *testing.T) {
		c := vectorext.DeepCopy(a)
		assert.Equal(t, a, c, "the deep copied vector must be equal")
		c[0] = fext.NewElement(40, 0)
		aBAndXMustNotChange(t)
	})

	t.Run("ScalarMul", func(t *testing.T) {
		c := vectorext.DeepCopy(a)
		vectorext.ScalarMul(c, b, x)
		assert.Equal(t, vectorext.ForTestFromPairs(6, 8, 10, 12, 14, 16), c, "c must be equal to 2*b")
		vectorext.ScalarMul(c, c, x)
		assert.Equal(t, vectorext.ForTestFromPairs(12, 16, 20, 24, 28, 32), c, "c must be equal to 4*b")
		aBAndXMustNotChange(t)
	})

	t.Run("ScalarProd", func(t *testing.T) {
		c := vectorext.ScalarProd(a, b)
		assert.Equal(t, fmt.Sprintf("%d+%d*u", 80*fext.RootPowers[1]+53, 130), c.String())
		aBAndXMustNotChange(t)
	})

	t.Run("Rand", func(t *testing.T) {
		c, d := vectorext.Rand(5), vectorext.Rand(5)
		assert.NotEqual(t, c, d, "Rand should not return twice the same value")
	})

	t.Run("MulElementWise", func(t *testing.T) {
		c := vectorext.DeepCopy(b)
		vectorext.MulElementWise(c, b, a)
		assert.Equal(t, vectorext.ForTestFromPairs(
			3+8*fext.RootPowers[1],
			10,
			24*fext.RootPowers[1]+15,
			38,
			35+48*fext.RootPowers[1],
			82,
		), c)

		c = vectorext.DeepCopy(b)
		vectorext.MulElementWise(c, c, a)
		assert.Equal(t, vectorext.ForTestFromPairs(
			3+8*fext.RootPowers[1],
			10,
			24*fext.RootPowers[1]+15,
			38,
			35+48*fext.RootPowers[1],
			82,
		), c)

		aBAndXMustNotChange(t)
	})

	t.Run("Prettify", func(t *testing.T) {
		assert.Equal(t, "[1+2*u, 3+4*u, 5+6*u]", vectorext.Prettify(a))
		aBAndXMustNotChange(t)
	})

	t.Run("Reverse", func(t *testing.T) {
		c := vectorext.DeepCopy(a)
		vectorext.Reverse(c)
		// we invert the order of the pairs, but not the order inside the pairs as that
		// would lead to different field extensions
		assert.Equal(t, vectorext.ForTestFromPairs(5, 6, 3, 4, 1, 2), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Repeat", func(t *testing.T) {
		y := fext.NewElement(1, 2)
		c := vectorext.Repeat(y, 4)
		assert.Equal(t, vectorext.ForTestFromPairs(1, 2, 1, 2, 1, 2, 1, 2), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Add", func(t *testing.T) {
		c := vectorext.DeepCopy(a)
		vectorext.Add(c, a, b)
		assert.Equal(t, vectorext.ForTestFromPairs(4, 6, 8, 10, 12, 14), c)

		c = vectorext.DeepCopy(a)
		vectorext.Add(c, c, b)
		assert.Equal(t, vectorext.ForTestFromPairs(4, 6, 8, 10, 12, 14), c)

		c = vectorext.DeepCopy(a)
		vectorext.Add(c, a, b, a)
		assert.Equal(t, vectorext.ForTestFromPairs(5, 8, 11, 14, 17, 20), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Sub", func(t *testing.T) {
		c := vectorext.DeepCopy(a)
		vectorext.Sub(c, b, a)
		assert.Equal(t, vectorext.ForTestFromPairs(2, 2, 2, 2, 2, 2), c)

		c = vectorext.DeepCopy(a)
		vectorext.Sub(c, b, c)
		assert.Equal(t, vectorext.ForTestFromPairs(2, 2, 2, 2, 2, 2), c)

		c = vectorext.DeepCopy(b)
		vectorext.Sub(c, c, a)
		assert.Equal(t, vectorext.ForTestFromPairs(2, 2, 2, 2, 2, 2), c)
		aBAndXMustNotChange(t)
	})

	t.Run("ZeroPad", func(t *testing.T) {
		c := vectorext.ZeroPad(a, 5)
		assert.Equal(t, vectorext.ForTestFromPairs(1, 2, 3, 4, 5, 6, 0, 0, 0, 0), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Interleave", func(t *testing.T) {
		c := vectorext.Interleave(a, b)
		assert.Equal(t, vectorext.ForTestFromPairs(1, 2, 3, 4, 3, 4, 5, 6, 5, 6, 7, 8), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Fill", func(t *testing.T) {
		c := vectorext.DeepCopy(a)
		vectorext.Fill(c, x)
		assert.Equal(t, vectorext.ForTestFromPairs(2, 0, 2, 0, 2, 0), c)
		aBAndXMustNotChange(t)
	})

	t.Run("PowerVec", func(t *testing.T) {
		c := vectorext.PowerVec(x, 5)
		assert.Equal(t, vectorext.ForTestFromPairs(1, 0, 2, 0, 4, 0, 8, 0, 16, 0), c)
	})

	t.Run("IntoGnarkAssignment", func(t *testing.T) {
		c := vectorext.IntoGnarkAssignment(a)
		for i := range c {
			first := c[i].A0.(field.Element)
			second := c[i].A1.(field.Element)
			assert.Equal(t, a[i].A0.String(), first.String())
			assert.Equal(t, a[i].A1.String(), second.String())
		}
		aBAndXMustNotChange(t)
	})

}

func TestReverse(t *testing.T) {
	vec := []fext.Element{
		fext.NewElement(0, 0),
		fext.NewElement(1, 0),
		fext.NewElement(2, 0),
	}
	vectorext.Reverse(vec)
	require.Equal(t, vec[0], fext.NewElement(2, 0))
	require.Equal(t, vec[1], fext.NewElement(1, 0))
	require.Equal(t, vec[2], fext.NewElement(0, 0))
}

func TestScalarProd(t *testing.T) {
	require.Equal(t,
		vectorext.ScalarProd(
			vectorext.ForTest(1, 2, 3, 4),
			vectorext.ForTest(1, 2, 3, 4),
		),
		fext.NewElement(30, 0),
	)
}

func TestForTest(t *testing.T) {
	testcase := vectorext.ForTest(-1, 0, 1)
	require.Equal(t, testcase[0].String(), "-1+0*u")
	require.Equal(t, testcase[1].String(), "0+0*u")
	require.Equal(t, testcase[2].String(), "1+0*u")
}
