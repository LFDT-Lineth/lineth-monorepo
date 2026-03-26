package vectorext_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectors(t *testing.T) {

	var (
		// a, b and x are very common vectors in all the tests
		list_a = []int{1, 2, 3, 4, 5, 6, 7, 8}
		list_b = []int{3, 4, 5, 6, 7, 8, 9, 10}

		a = vectorext.ForTestFromQuads(list_a...)
		b = vectorext.ForTestFromQuads(list_b...)
		x = fext.NewFromUint(2, 0, 0, 0)

		// aBAndXMustNotChange asserts that a and b did not change as this is
		// a very common check in all the sub-tests.
		aBAndXMustNotChange = func(t *testing.T) {
			require.Equal(t, vectorext.ForTestFromQuads(list_a...), a, "a must not change")
			require.Equal(t, vectorext.ForTestFromQuads(list_b...), b, "b must not change")
			require.Equal(t, "2+0*u+(0+0*u)*v", x.String(), "x must not change")
		}
	)

	t.Run("Rand", func(t *testing.T) {
		c, d := vectorext.Rand(5), vectorext.Rand(5)
		assert.NotEqual(t, c, d, "Rand should not return twice the same value")
	})

	t.Run("Prettify", func(t *testing.T) {
		assert.Equal(t, "[1+2*u+(3+4*u)*v, 5+6*u+(7+8*u)*v]", vectorext.Prettify(a))
		aBAndXMustNotChange(t)
	})

	t.Run("Constant", func(t *testing.T) {
		y := fext.NewFromUint(1, 2, 3, 4)
		c := vectorext.Repeat(y, 2)
		assert.Equal(t, vectorext.ForTestFromQuads(1, 2, 3, 4, 1, 2, 3, 4), c)
		aBAndXMustNotChange(t)
	})

	t.Run("ZeroPad", func(t *testing.T) {
		c := vectorext.ZeroPad(a, 3)
		assert.Equal(t, vectorext.ForTestFromQuads(1, 2, 3, 4, 5, 6, 7, 8, 0, 0, 0, 0), c)
		aBAndXMustNotChange(t)
	})

	t.Run("Interleave", func(t *testing.T) {
		c := vectorext.Interleave(a, b)
		assert.Equal(t, vectorext.ForTestFromQuads(1, 2, 3, 4, 3, 4, 5, 6, 5, 6, 7, 8, 7, 8, 9, 10), c)
		aBAndXMustNotChange(t)
	})

	t.Run("PowerVec", func(t *testing.T) {
		c := vectorext.PowerVec(x, 3)
		assert.Equal(t, vectorext.ForTestFromQuads(1, 0, 0, 0, 2, 0, 0, 0, 4, 0, 0, 0), c)
	})

	// t.Run("IntoGnarkAssignment", func(t *testing.T) {
	// 	c := vectorext.IntoGnarkAssignment(a)
	// 	var tmp fext.Element
	// 	for i := range c {
	// 		tmp.B0.A0 = c[i].B0.A0.(field.Element)
	// 		tmp.B0.A1 = c[i].B0.A1.(field.Element)
	// 		tmp.B1.A0 = c[i].B1.A0.(field.Element)
	// 		tmp.B1.A1 = c[i].B1.A1.(field.Element)
	// 		assert.Equal(t, a[i].B0.A0.String(), tmp.B0.A0.String())
	// 		assert.Equal(t, a[i].B0.A1.String(), tmp.B0.A1.String())
	// 		assert.Equal(t, a[i].B1.A0.String(), tmp.B1.A0.String())
	// 		assert.Equal(t, a[i].B1.A1.String(), tmp.B1.A1.String())
	// 	}
	// 	aBAndXMustNotChange(t)
	// })

}

func TestForTest(t *testing.T) {
	testcase := vectorext.ForTest(-1, 0, 1)
	require.Equal(t, testcase[0].String(), "-1+0*u+(0+0*u)*v")
	require.Equal(t, testcase[1].String(), "0+0*u+(0+0*u)*v")
	require.Equal(t, testcase[2].String(), "1+0*u+(0+0*u)*v")
}
