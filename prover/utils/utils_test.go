package utils_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/go-playground/assert/v2"
	"github.com/stretchr/testify/require"
)

func TestDivCeil(t *testing.T) {

	require.Equal(t, 1, utils.DivCeil(1, 10))
	require.Equal(t, 1, utils.DivCeil(2, 10))
	require.Equal(t, 1, utils.DivCeil(3, 10))
	require.Equal(t, 1, utils.DivCeil(4, 10))
	require.Equal(t, 1, utils.DivCeil(5, 10))
	require.Equal(t, 1, utils.DivCeil(6, 10))
	require.Equal(t, 1, utils.DivCeil(7, 10))
	require.Equal(t, 1, utils.DivCeil(8, 10))
	require.Equal(t, 1, utils.DivCeil(9, 10))
	require.Equal(t, 1, utils.DivCeil(10, 10))
	require.Equal(t, 2, utils.DivCeil(11, 10))
	require.Equal(t, 2, utils.DivCeil(19, 10))
	require.Equal(t, 2, utils.DivCeil(20, 10))
	require.Equal(t, 3, utils.DivCeil(21, 10))
}

func TestIsPowerOfTwo(t *testing.T) {
	require.Equal(t, true, utils.IsPowerOfTwo(4))
	// The below negative number is the only one giving true if the constraint in
	// IsPowerOfTwo were n != 0 instead of n > 0 (found by zkSecurity audit)
	require.Equal(t, false, utils.IsPowerOfTwo(-9223372036854775808))
}

func TestNextPowerOfTwo(t *testing.T) {
	require.Equal(t, 4, utils.NextPowerOfTwo(3))
	// To Test the method with a power of two input,
	// the output should be equal to the input
	powTwoInp := 1 << 32
	require.Equal(t, powTwoInp, utils.NextPowerOfTwo(powTwoInp))
	// 2 ** 62 is the largest output of the method NextPowerOfTwo()
	num := 1 << 61
	num++
	require.Equal(t, 1<<62, utils.NextPowerOfTwo(num))
	// To check if the code panics if large values (e.g. val > 2 ** 62) are sent as input
	largeNum := 1 << 62
	largeNum++
	require.PanicsWithValue(t, "input out of range", func() { utils.NextPowerOfTwo(largeNum) },
		"NextPowerOfTwo should panic with 'Input is too large' message")

}

func TestNextPowerOfTwoExample(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{1, 1},
		{2, 2},
		{5, 8},
		{12, 16},
		{20, 32},
		{33, 64},
		{100, 128},
		{255, 256},
		{500, 512},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("NextPowerOfTwo(%d)", test.input), func(t *testing.T) {
			result := utils.NextPowerOfTwo(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}
