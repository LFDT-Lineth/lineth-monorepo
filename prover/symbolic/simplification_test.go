package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimplifyLinComb(t *testing.T) {
	var (
		a    = NewDummyVar("a")
		b    = NewDummyVar("b")
		c    = NewDummyVar("c")
		zero = NewConstant(0)
		one  = NewConstant(1)
	)

	testCases := []struct {
		Name       string
		InputVars  []*Expression
		InputMagn  []int
		OutputVars []*Expression
		OutputMagn []int
	}{
		{
			Name:       "Remove Zeroes",
			InputVars:  []*Expression{a, a, b, c, b},
			InputMagn:  []int{1, 0, 2, 0, 1},
			OutputVars: []*Expression{a, b},
			OutputMagn: []int{1, 3},
		},
		{
			Name:       "Expand LinComb",
			InputVars:  []*Expression{Add(a, b), Mul(a, b)},
			InputMagn:  []int{2, 2},
			OutputVars: []*Expression{a, b, Mul(a, b)},
			OutputMagn: []int{2, 2, 2},
		},
		{
			Name:       "Regroup Terms",
			InputVars:  []*Expression{a, b, c, b, a, c, zero},
			InputMagn:  []int{1, 1, 1, 1, 1, 1, 1},
			OutputVars: []*Expression{a, b, c},
			OutputMagn: []int{2, 2, 2},
		},
		{
			Name:       "Constants",
			InputVars:  []*Expression{a, one, NewConstant(2)},
			InputMagn:  []int{1, 2, 3}, // a + 2*1 + 3*2 = a + 8
			OutputVars: []*Expression{a, NewConstant(8)},
			OutputMagn: []int{1, 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			oVars, oMagn := simplifyLinComb(tc.InputVars, tc.InputMagn)

			assert.Equal(t, len(tc.OutputVars), len(oVars))
			assert.Equal(t, len(tc.OutputMagn), len(oMagn))

			for i := range oVars {
				// Compare ESHash for equality
				assert.Equal(t, tc.OutputVars[i].ESHash, oVars[i].ESHash)
				assert.Equal(t, tc.OutputMagn[i], oMagn[i])
			}
		})
	}
}

func TestSimplifyProduct(t *testing.T) {
	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		// c = NewDummyVar("c")
	)

	testCases := []struct {
		Name       string
		InputVars  []*Expression
		InputMagn  []int
		OutputVars []*Expression
		OutputMagn []int
	}{
		{
			Name:       "Expand Product",
			InputVars:  []*Expression{Mul(a, b), a},
			InputMagn:  []int{2, 1}, // (a*b)^2 * a^1 = a^2 * b^2 * a^1 = a^3 * b^2
			OutputVars: []*Expression{a, b},
			OutputMagn: []int{3, 2},
		},
		{
			Name:       "Constants",
			InputVars:  []*Expression{a, NewConstant(2)},
			InputMagn:  []int{1, 2}, // a^1 * 2^2 = 4a
			OutputVars: []*Expression{a, NewConstant(4)},
			OutputMagn: []int{1, 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			oVars, oMagn := simplifyProduct(tc.InputVars, tc.InputMagn)

			assert.Equal(t, len(tc.OutputVars), len(oVars))
			assert.Equal(t, len(tc.OutputMagn), len(oMagn))

			for i := range oVars {
				assert.Equal(t, tc.OutputVars[i].ESHash, oVars[i].ESHash)
				assert.Equal(t, tc.OutputMagn[i], oMagn[i])
			}
		})
	}
}
