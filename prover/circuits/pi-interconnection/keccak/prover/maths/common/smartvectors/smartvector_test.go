//go:build !race

package smartvectors

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/require"
)

func TestWriteInSlice(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
		// We reuse the test case generator for linear combinations. We only
		// care about the first vector.
		builder := newTestBuilder(i)
		tcase := builder.NewTestCaseForLinComb()

		success := t.Run(
			fmt.Sprintf("fuzzy-write-in-slice-%v", i),
			func(t *testing.T) {
				v := tcase.svecs[0]

				slice := make([]field.Element, v.Len())
				v.WriteInSlice(slice)

				for j := range slice {
					x := v.Get(j)
					require.Equal(t, x.String(), slice[j].String())
				}

				// write in a random place of the slice
				randPos := builder.gen.IntN(v.Len())
				slice[randPos].SetRandom()
				x := v.Get(randPos)
				require.NotEqual(t, x.String(), randPos, "forbidden shallow copy")
			},
		)

		require.True(t, success)
	}
}

func TestShiftingTest(t *testing.T) {
	for i := 0; i < FuzzIteration; i++ {
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
					x := v.Get(i)
					xR := shifted.Get(utils.PositiveMod(i+offset, v.Len()))
					xL := revShifted.Get(utils.PositiveMod(i-offset, v.Len()))

					require.Equal(t, x.String(), xL.String())
					require.Equal(t, x.String(), xR.String())
				}
			},
		)

		require.True(t, success)
	}

}

func TestSubvectorFuzzy(t *testing.T) {

	for i := 0; i < FuzzIteration; i++ {

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
					expected := v.Get(start + i)
					actual := sub.Get(i)
					require.Equal(t, expected.String(), actual.String(), "Start %v, Stop %v, i %v", start, stop, i)
				}

			},
		)

		require.True(t, success)
	}
}

func TestTryReduceSizeRight(t *testing.T) {

	testVectors := []SmartVector{
		ForTest(2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
		ForTest(6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
		ForTest(6, 6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
		ForTest(2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6),
		ForTest(6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6),
		ForTest(6, 6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6),
		ForTest(1),
	}

	expectedN := []int{
		16,
		15,
		14,
		0,
		0,
		0,
		0,
	}

	for i, v := range testVectors {

		v_, n := TryReduceSizeRight(v)

		if v.Len() != v_.Len() {
			t.Fatalf("the lengths mismatch %v != %v", v.Len(), v_.Len())
		}

		if n != expectedN[i] {
			t.Errorf("the lengths mismatch %v != %v", n, expectedN[i])
		}

		for i := range v.IntoRegVecSaveAlloc() {

			var (
				vi  = v.Get(i)
				vi_ = v_.Get(i)
			)

			if !vi.Equal(&vi_) {
				t.Errorf("the values mismatch %v != %v", vi.String(), vi_.String())
			}
		}
	}
}

func TestTryReduceSizeLeft(t *testing.T) {

	testVectors := []SmartVector{
		ForTest(2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
		ForTest(2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6),
		ForTest(2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6, 6),
		ForTest(6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
		ForTest(6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6),
		ForTest(6, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 6, 6),
		ForTest(1),
	}

	expectedN := []int{
		16,
		15,
		14,
		0,
		0,
		0,
		0,
	}

	for i, v := range testVectors {

		v_, n := TryReduceSizeLeft(v)

		if v.Len() != v_.Len() {
			t.Fatalf("the lengths mismatch %v != %v", v.Len(), v_.Len())
		}

		if n != expectedN[i] {
			t.Errorf("the lengths mismatch %v != %v", n, expectedN[i])
		}

		for i := range v.IntoRegVecSaveAlloc() {

			var (
				vi  = v.Get(i)
				vi_ = v_.Get(i)
			)

			if !vi.Equal(&vi_) {
				t.Errorf("the values mismatch %v != %v", vi.String(), vi_.String())
			}
		}
	}
}
