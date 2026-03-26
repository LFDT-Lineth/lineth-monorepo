package smartvectors

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/stretchr/testify/assert"
)

func TestBatchInvert(t *testing.T) {

	testCases := []SmartVector{
		NewConstant(field.Zero(), 4),
		NewConstant(field.One(), 4),
		ForTest(0, 1, 2, 3, 0, 0, 4, 4),
		ForTest(0, 0, 0, 0),
		ForTest(12, 13, 14, 15),
		NewRotated(Regular(vector.ForTest(0, 0, 2, 2)), 0),
		NewRotated(Regular(vector.ForTest(0, 0, 2, 2)), 1),
		NewRotated(Regular(vector.ForTest(1, 1, 2, 2)), 0),
		NewRotated(Regular(vector.ForTest(3, 3, 2, 2)), 1),
		NewRotated(Regular(vector.ForTest(0, 0, 0, 0)), 0),
		NewRotated(Regular(vector.ForTest(0, 0, 0, 0)), 1),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.Zero(), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.Zero(), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.Zero(), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.NewElement(42), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.NewElement(42), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.NewElement(42), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.Zero(), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.Zero(), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.Zero(), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.NewElement(42), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.NewElement(42), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.NewElement(42), 2, 8),
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			bi := BatchInvert(testCases[i])

			assert.Equal(t, bi.Len(), testCases[i].Len())

			for k := 0; k < bi.Len(); k++ {
				var (
					x = bi.Get(k)
					y = testCases[i].Get(k)
				)

				if y == field.Zero() {
					assert.Equal(t, field.Zero(), x)
					continue
				}

				y.Inverse(&y)
				assert.Equal(t, x, y)
			}
		})
	}
}

func TestIsZero(t *testing.T) {

	testCases := []SmartVector{
		NewConstant(field.Zero(), 4),
		NewConstant(field.One(), 4),
		ForTest(0, 1, 2, 3, 0, 0, 4, 4),
		ForTest(0, 0, 0, 0),
		ForTest(12, 13, 14, 15),
		NewRotated(Regular(vector.ForTest(0, 0, 2, 2)), 0),
		NewRotated(Regular(vector.ForTest(0, 0, 2, 2)), 1),
		NewRotated(Regular(vector.ForTest(1, 1, 2, 2)), 0),
		NewRotated(Regular(vector.ForTest(3, 3, 2, 2)), 1),
		NewRotated(Regular(vector.ForTest(0, 0, 0, 0)), 0),
		NewRotated(Regular(vector.ForTest(0, 0, 0, 0)), 1),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.Zero(), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.Zero(), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.Zero(), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.NewElement(42), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.NewElement(42), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.NewElement(42), 0, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.Zero(), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.Zero(), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.Zero(), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 0, 0), field.NewElement(42), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(0, 0, 1, 1), field.NewElement(42), 2, 8),
		NewPaddedCircularWindow(vector.ForTest(1, 1, 2, 2), field.NewElement(42), 2, 8),
	}

	for i := range testCases {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			iz := IsZero(testCases[i])

			assert.Equal(t, iz.Len(), testCases[i].Len())

			for k := 0; k < iz.Len(); k++ {
				var (
					x = iz.Get(k)
					y = testCases[i].Get(k)
				)

				if y == field.Zero() {
					assert.Equal(t, uint64(1), x.Uint64())
				}

				if y != field.Zero() {
					assert.Equal(t, uint64(0), x.Uint64())
				}

				if t.Failed() {
					t.Fatalf("failed at position %v for testcase %v", k, i)
				}
			}
		})
	}
}
