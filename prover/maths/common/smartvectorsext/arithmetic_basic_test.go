package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBatchInvert(t *testing.T) {

	testCases := []smartvectors.SmartVector{
		NewConstantExt(fext.Zero(), 4),
		NewConstantExt(fext.One(), 4),
		ForTestExt(0, 1, 2, 3, 0, 0, 4, 4),
		ForTestExt(0, 0, 0, 0),
		ForTestExt(12, 13, 14, 15),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 2, 2)), 0),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 2, 2)), 1),
		NewRotatedExt(RegularExt(vectorext.ForTest(1, 1, 2, 2)), 0),
		NewRotatedExt(RegularExt(vectorext.ForTest(3, 3, 2, 2)), 1),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 0, 0)), 0),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 0, 0)), 1),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.Zero(), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.Zero(), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.Zero(), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.NewElement(42, 43), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.NewElement(42, 43), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.NewElement(42, 43), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.Zero(), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.Zero(), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.Zero(), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.NewElement(42, 43), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.NewElement(42, 43), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.NewElement(42, 43), 2, 8),
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			bi := BatchInvert(testCases[i])

			assert.Equal(t, bi.Len(), testCases[i].Len())

			for k := 0; k < bi.Len(); k++ {
				var (
					x = bi.GetExt(k)
					y = testCases[i].GetExt(k)
				)

				if y == fext.Zero() {
					assert.Equal(t, fext.Zero(), x)
					continue
				}

				y.Inverse(&y)
				assert.Equal(t, x, y)
			}
		})
	}
}

func TestIsZero(t *testing.T) {

	testCases := []smartvectors.SmartVector{
		NewConstantExt(fext.Zero(), 4),
		NewConstantExt(fext.One(), 4),
		ForTestExt(0, 1, 2, 3, 0, 0, 4, 4),
		ForTestExt(0, 0, 0, 0),
		ForTestExt(12, 13, 14, 15),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 2, 2)), 0),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 2, 2)), 1),
		NewRotatedExt(RegularExt(vectorext.ForTest(1, 1, 2, 2)), 0),
		NewRotatedExt(RegularExt(vectorext.ForTest(3, 3, 2, 2)), 1),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 0, 0)), 0),
		NewRotatedExt(RegularExt(vectorext.ForTest(0, 0, 0, 0)), 1),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.Zero(), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.Zero(), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.Zero(), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.NewElement(42, 43), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.NewElement(42, 43), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.NewElement(42, 43), 0, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.Zero(), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.Zero(), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.Zero(), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 0, 0), fext.NewElement(42, 43), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(0, 0, 1, 1), fext.NewElement(42, 43), 2, 8),
		NewPaddedCircularWindowExt(vectorext.ForTest(1, 1, 2, 2), fext.NewElement(42, 43), 2, 8),
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			iz := IsZero(testCases[i])

			assert.Equal(t, iz.Len(), testCases[i].Len())

			for k := 0; k < iz.Len(); k++ {
				var (
					x = iz.GetExt(k)
					y = testCases[i].GetExt(k)
				)

				if y == fext.Zero() {
					a, b := x.Uint64()
					assert.Equal(t, uint64(1), a)
					assert.Equal(t, uint64(0), b)
				}

				if y != fext.Zero() {
					a, b := x.Uint64()
					assert.Equal(t, uint64(0), a)
					assert.Equal(t, uint64(0), b)
				}

				if t.Failed() {
					t.Fatalf("failed at position %v for testcase %v", k, i)
				}
			}
		})
	}
}
