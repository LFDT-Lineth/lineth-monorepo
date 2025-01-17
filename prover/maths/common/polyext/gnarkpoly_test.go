package polyext

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext/gnarkfext"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/stretchr/testify/require"
)

func TestGnarkEval(t *testing.T) {

	t.Run("normal-poly", func(t *testing.T) {

		def := func(api frontend.API) error {
			outerAPI := gnarkfext.API{Inner: api}
			var (
				pol      = vectorext.IntoGnarkAssignment(vectorext.ForTestFromPairs(1, 2, 3, 4, -1, -2))
				x        = gnarkfext.Variable{2, 1}
				expected = gnarkfext.Variable{
					-5*fext.RootPowers[1] + 3,
					-2*fext.RootPowers[1] + 1,
				}
				res = EvaluateUnivariateGnark(api, pol, x)
			)
			outerAPI.AssertIsEqual(expected, res)
			return nil
		}

		gnarkutil.AssertCircuitSolved(t, def)
	})

	t.Run("empty-poly", func(t *testing.T) {
		def := func(api frontend.API) error {
			outerAPI := gnarkfext.API{Inner: api}
			var (
				pol      = vectorext.IntoGnarkAssignment([]fext.Element{})
				x        = gnarkfext.Variable{2, 3}
				expected = gnarkfext.NewZero()
				res      = EvaluateUnivariateGnark(api, pol, x)
			)
			outerAPI.AssertIsEqual(expected, res)
			return nil
		}
		gnarkutil.AssertCircuitSolved(t, def)
	})

}

func TestGnarkEvalAnyDomain(t *testing.T) {

	t.Run("single-variable", func(t *testing.T) {

		def := func(api frontend.API) error {
			outerAPI := gnarkfext.API{Inner: api}
			var (
				domain   = vectorext.IntoGnarkAssignment(vectorext.ForTestFromPairs(0, 0))
				x        = gnarkfext.Variable{42, 0}
				expected = vectorext.IntoGnarkAssignment(vectorext.ForTestFromPairs(1, 0))
				res      = EvaluateLagrangeAnyDomainGnark(api, domain, x)
			)

			require.Len(t, res, len(expected))
			for i := range expected {
				outerAPI.AssertIsEqual(expected[i], res[i])
			}

			return nil
		}

		gnarkutil.AssertCircuitSolved(t, def)
	})

	t.Run("multiple-variable", func(t *testing.T) {

		def := func(api frontend.API) error {
			outerAPI := gnarkfext.API{Inner: api}
			var (
				domain   = vectorext.IntoGnarkAssignment(vectorext.ForTestFromPairs(0, 0, 1, 0))
				x        = gnarkfext.Variable{42, 0}
				expected = vectorext.IntoGnarkAssignment(vectorext.ForTestFromPairs(-41, 0, 42, 0))
				res      = EvaluateLagrangeAnyDomainGnark(api, domain, x)
			)

			require.Len(t, res, len(expected))
			for i := range expected {
				outerAPI.AssertIsEqual(expected[i], res[i])
			}

			return nil
		}

		gnarkutil.AssertCircuitSolved(t, def)
	})

}
