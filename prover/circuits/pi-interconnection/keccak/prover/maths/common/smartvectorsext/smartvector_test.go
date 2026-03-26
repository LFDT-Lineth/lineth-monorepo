//go:build !race

package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/require"
)

func TestWriteInSlice(t *testing.T) {
	for i := 0; i < smartvectors.FuzzIteration; i++ {
		// We reuse the test case generator for linear combinations. We only
		// care about the first vector.
		builder := newTestBuilder(i)
		tcase := builder.NewTestCaseForLinComb()

		success := t.Run(
			fmt.Sprintf("fuzzy-write-in-slice-%v", i),
			func(t *testing.T) {
				v := tcase.svecs[0]

				slice := make([]fext.Element, v.Len())
				v.WriteInSliceExt(slice)

				for j := range slice {
					x := v.GetExt(j)
					require.Equal(t, x.String(), slice[j].String())
				}

				// write in a random place of the slice
				randPos := builder.gen.IntN(v.Len())
				slice[randPos].SetRandom()
				x := v.GetExt(randPos)
				require.NotEqual(t, x.String(), randPos, "forbidden shallow copy")
			},
		)

		require.True(t, success)
	}
}

func TestShiftingTest(t *testing.T) {
	for i := 0; i < smartvectors.FuzzIteration; i++ {
		// We reuse the test case generator for linear combinations. We only
		// care about the first vector.
		builder := newTestBuilder(i)
		tcase := builder.NewTestCaseForLinComb()

		success := t.Run(
			fmt.Sprintf("fuzzy-shifting-%v", i),
			func(t *testing.T) {
				v := tcase.svecs[0]

				offset := builder.gen.IntN(v.Len())

				shifted := v.RotateRight(offset)
				revShifted := v.RotateRight(-offset)

				for i := 0; i < v.Len(); i++ {
					x := v.GetExt(i)
					xR := shifted.GetExt(utils.PositiveMod(i+offset, v.Len()))
					xL := revShifted.GetExt(utils.PositiveMod(i-offset, v.Len()))

					require.Equal(t, x.String(), xL.String())
					require.Equal(t, x.String(), xR.String())
				}
			},
		)

		require.True(t, success)
	}

}

func TestSubvectorFuzzy(t *testing.T) {

	for i := 0; i < smartvectors.FuzzIteration; i++ {

		// We reuse the test case generator for linear combinations. We only
		// care about the first vector.
		builder := newTestBuilder(i)
		tcase := builder.NewTestCaseForLinComb()

		success := t.Run(
			fmt.Sprintf("fuzzy-shifting-%v", i),
			func(t *testing.T) {
				v := tcase.svecs[0]
				length := v.Len()
				// generate the subvector window
				stop := 1 + builder.gen.IntN(length-1)
				start := builder.gen.IntN(stop)

				sub := v.SubVector(start, stop)

				require.Equal(t, stop-start, sub.Len(), "subvector has wrong size")

				for i := 0; i < stop-start; i++ {
					expected := v.GetExt(start + i)
					actual := sub.GetExt(i)
					require.Equal(t, expected.String(), actual.String(), "Start %v, Stop %v, i %v", start, stop, i)
				}

			},
		)

		require.True(t, success)
	}
}
