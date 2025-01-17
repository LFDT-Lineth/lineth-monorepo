package fastpolyext_test

import (
	"fmt"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/fft/fastpolyext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext/gnarkfext"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
)

func TestGnarkInterpolate(t *testing.T) {

	testCases := [][]fext.Element{
		vectorext.ForTest(0, 0, 0, 0),
		vectorext.ForTest(1, 0, 1, 0),
		vectorext.ForTest(1, 1, 1, 1),
	}

	for i := range testCases {

		t.Run(fmt.Sprintf("test-cases-%v", i), func(t *testing.T) {

			def := func(api frontend.API) error {
				outerApi := gnarkfext.API{Inner: api}
				var (
					x         = fext.NewElement(42, 0)
					vec       = vectorext.IntoGnarkAssignment(testCases[i])
					expectedY = fastpolyext.Interpolate(testCases[i], x)
					computedY = fastpolyext.InterpolateGnark(outerApi, vec, gnarkfext.ExtToVariable(x))
				)
				outerApi.AssertIsEqualToField(expectedY, computedY)
				return nil
			}

			gnarkutil.AssertCircuitSolved(t, def)
		})
	}

}
