package fastpoly

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/gnarkutil"
)

func TestGnarkInterpolate(t *testing.T) {

	testCases := [][]field.Element{
		vector.ForTest(0, 0, 0, 0),
		vector.ForTest(1, 0, 1, 0),
		vector.ForTest(1, 1, 1, 1),
	}

	for i := range testCases {

		t.Run(fmt.Sprintf("test-cases-%v", i), func(t *testing.T) {

			def := func(api frontend.API) error {
				var (
					x         = field.NewElement(42)
					vec       = vector.IntoGnarkAssignment(testCases[i])
					expectedY = Interpolate(testCases[i], x)
					computedY = InterpolateGnark(api, vec, x)
				)
				api.AssertIsEqual(expectedY, computedY)
				return nil
			}

			gnarkutil.AssertCircuitSolved(t, def)
		})
	}

}
