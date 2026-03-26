package symbolic

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/assert"
)

func TestRemoveZeroes(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		c = NewDummyVar("c")
	)

	testCases := []struct {
		InputVars  []*Expression
		InputMagn  []int
		OutputVars []*Expression
		OutputMagn []int
	}{
		{
			InputVars:  []*Expression{a, a, b, c, b},
			InputMagn:  []int{1, 0, 2, 0, 1},
			OutputVars: []*Expression{a, b, b},
			OutputMagn: []int{1, 2, 1},
		},
	}

	for i := range testCases {

		oMagn, oVar := removeZeroCoeffs(testCases[i].InputMagn, testCases[i].InputVars)
		assert.Equal(t, testCases[i].OutputVars, oVar)
		assert.Equal(t, testCases[i].OutputMagn, oMagn)
	}
}

func TestExpandTerms(t *testing.T) {

	var (
		a = NewDummyVar("a")
		b = NewDummyVar("b")
		// c = NewDummyVar("c")
	)

	testCases := []struct {
		InputVars  []*Expression
		InputMagn  []int
		OutputVars []*Expression
		OutputMagn []int
		Op         Operator
	}{
		{
			InputVars:  []*Expression{Add(a, b), Mul(a, b)},
			InputMagn:  []int{2, 2},
			OutputVars: []*Expression{a, b, Mul(a, b)},
			OutputMagn: []int{2, 2, 2},
			Op:         &LinComb{},
		},
		{
			InputVars:  []*Expression{Add(a, b), Mul(a, b)},
			InputMagn:  []int{2, 2},
			OutputVars: []*Expression{Add(a, b), a, b},
			OutputMagn: []int{2, 2, 2},
			Op:         &Product{},
		},
		{
			InputVars:  []*Expression{Add(a, b), Mul(a, b)},
			InputMagn:  []int{2, 2},
			OutputVars: []*Expression{a, b, Mul(a, b)},
			OutputMagn: []int{2, 2, 2},
			Op:         LinComb{},
		},
		{
			InputVars:  []*Expression{Add(a, b), Mul(a, b)},
			InputMagn:  []int{2, 2},
			OutputVars: []*Expression{Add(a, b), a, b},
			OutputMagn: []int{2, 2, 2},
			Op:         Product{},
		},
	}

	for _, tc := range testCases {
		oMagn, oVars := expandTerms(tc.Op, tc.InputMagn, tc.InputVars)
		assert.Equal(t, tc.OutputVars, oVars)
		assert.Equal(t, tc.OutputMagn, oMagn)
	}
}

func TestRegroupTerms(t *testing.T) {

	var (
		a    = NewDummyVar("a")
		b    = NewDummyVar("b")
		c    = NewDummyVar("c")
		zero = NewConstant(0)
	)

	testCases := []struct {
		InputVars  []*Expression
		InputMagn  []int
		OutputVars []*Expression
		OutputMagn []int
		ConstVars  []field.Element
		ConstMagn  []int
		Op         Operator
	}{
		{
			InputVars:  []*Expression{a, b, c, b, a, c, zero},
			InputMagn:  []int{1, 1, 1, 1, 1, 1, 1},
			OutputVars: []*Expression{a, b, c},
			OutputMagn: []int{2, 2, 2},
			ConstVars:  []field.Element{field.Zero()},
			ConstMagn:  []int{1},
		},
	}

	for _, tc := range testCases {
		oMagn, oVars, cMagn, cVars := regroupTerms(tc.InputMagn, tc.InputVars)
		assert.Equal(t, tc.OutputVars, oVars)
		assert.Equal(t, tc.OutputMagn, oMagn)
		assert.Equal(t, tc.ConstVars, cVars)
		assert.Equal(t, tc.ConstMagn, cMagn)
	}
}
