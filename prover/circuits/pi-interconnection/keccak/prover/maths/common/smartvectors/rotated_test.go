package smartvectors_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/require"
)

func TestRotatedSubVector(t *testing.T) {

	size := 16
	original := vector.Rand(size)
	myVec := smartvectors.NewRegular(original)

	for offset := 0; offset < size; offset++ {
		rotated := smartvectors.NewRotated(*myVec, offset)
		for start := 0; start < size; start++ {
			for stop := start + 1; stop <= size; stop++ {

				sub := rotated.SubVector(start, stop)
				recovered := sub.Get(0)
				require.Equal(t, recovered.String(), original[utils.PositiveMod(start+offset, size)].String())

			}
		}
	}

}

func TestRotatedWriteInSlice(t *testing.T) {

	size := 16
	original := vector.Rand(size)
	myVec := smartvectors.NewRegular(original)

	for offset := 0; offset < size; offset++ {
		rotated := smartvectors.NewRotated(*myVec, offset)
		written := make([]field.Element, size)
		rotated.WriteInSlice(written)

		for i := range written {
			require.Equal(t, written[i].String(), original[utils.PositiveMod(i+offset, size)].String())
		}
	}
}

func TestRotatedOffsetOverflow(t *testing.T) {
	v := []field.Element{field.NewFromString("1"),
		field.NewFromString("2"),
		field.NewFromString("3"),
		field.NewFromString("4"),
		field.NewFromString("5")}
	myVec := smartvectors.NewRegular(v)
	// First we check that the absolute value for negative offsets
	// are not allowed to be larger than the length
	negOffset := -7
	expectedPanicMessage := fmt.Sprintf("len %v is less than, offset %v", 5, negOffset)
	require.PanicsWithValue(t, expectedPanicMessage, func() { smartvectors.NewRotated(*myVec, negOffset) },
		"NewRotated should panic with 'got negative offset' message")
	// Next we check offset overflow
	offset := 1
	r := smartvectors.NewRotated(*myVec, offset)
	rotateOffset := 1 << 41
	// The function should panic as rotateOffset is too large
	require.PanicsWithValue(t, "offset is too large", func() { r.RotateRight(rotateOffset) },
		"RotateRight should panic with 'offset is too large' message")
}

func TestRotateRightSimple(t *testing.T) {
	v := []field.Element{field.NewFromString("0"),
		field.NewFromString("1"),
		field.NewFromString("2"),
		field.NewFromString("3"),
		field.NewFromString("4")}
	myVec := smartvectors.NewRegular(v)
	offset := 1
	rotated := smartvectors.NewRotated(*myVec, offset)
	// Next we rotate the vector
	rotated_ := rotated.RotateRight(2)
	m := make([]field.Element, 0, len(v))
	for i := range v {
		m = append(m, rotated_.Get(i))
	}
	v_shifted := []field.Element{field.NewFromString("3"),
		field.NewFromString("4"),
		field.NewFromString("0"),
		field.NewFromString("1"),
		field.NewFromString("2")}
	require.Equal(t, m, v_shifted)
}
