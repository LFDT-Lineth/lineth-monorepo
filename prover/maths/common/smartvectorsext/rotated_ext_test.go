package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/utils"
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
	original := vectorext.Rand(size)
	myVec := NewRegularExt(original)

	for offset := 0; offset < size; offset++ {
		rotated := NewRotatedExt(*myVec, offset)
		written := make([]fext.Element, size)
		rotated.WriteInSliceExt(written)

		for i := range written {
			require.Equal(t, written[i].String(), original[utils.PositiveMod(i+offset, size)].String())
		}
	}
}

func TestRotatedOffsetOverflow(t *testing.T) {
	v := []fext.Element{fext.NewFromString("1"),
		fext.NewFromString("2"),
		fext.NewFromString("3"),
		fext.NewFromString("4"),
		fext.NewFromString("5")}
	myVec := NewRegularExt(v)
	// First we check that the absolute value for negative offsets
	// are not allowed to be larger than the length
	negOffset := -7
	expectedPanicMessage := fmt.Sprintf("len %v is less than, offset %v", 5, negOffset)
	require.PanicsWithValue(t, expectedPanicMessage, func() { NewRotatedExt(*myVec, negOffset) },
		"NewRotated should panic with 'got negative offset' message")
	// Next we check offset overflow
	offset := 1
	r := NewRotatedExt(*myVec, offset)
	rotateOffset := 1 << 41
	// The function should panic as rotateOffset is too large
	require.PanicsWithValue(t, "offset is too large", func() { r.RotateRight(rotateOffset) },
		"RotateRight should panic with 'offset is too large' message")
}

func TestRotateRightSimple(t *testing.T) {
	v := []fext.Element{fext.NewFromString("0"),
		fext.NewFromString("1"),
		fext.NewFromString("2"),
		fext.NewFromString("3"),
		fext.NewFromString("4")}
	myVec := NewRegularExt(v)
	offset := 1
	rotated := NewRotatedExt(*myVec, offset)
	// Next we rotate the vector
	rotated_ := rotated.RotateRight(2)
	m := make([]fext.Element, 0, len(v))
	for i := range v {
		m = append(m, rotated_.GetExt(i))
	}
	v_shifted := []fext.Element{fext.NewFromString("3"),
		fext.NewFromString("4"),
		fext.NewFromString("0"),
		fext.NewFromString("1"),
		fext.NewFromString("2")}
	require.Equal(t, m, v_shifted)
}
