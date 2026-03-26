package poly

import (
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/gnarkutil"
	"github.com/stretchr/testify/require"
)

func TestGnarkEval(t *testing.T) {

	t.Run("normal-poly", func(t *testing.T) {

		def := func(api frontend.API) error {
			var (
				pol      = vector.IntoGnarkAssignment(vector.ForTest(1, 2, 3, 4))
				x        = 2
				expected = 49
				res      = EvaluateUnivariateGnark(api, pol, x)
			)
			api.AssertIsEqual(expected, res)
			return nil
		}

		gnarkutil.AssertCircuitSolved(t, def)
	})

	t.Run("empty-poly", func(t *testing.T) {
		def := func(api frontend.API) error {
			var (
				pol      = vector.IntoGnarkAssignment([]field.Element{})
				x        = 2
				expected = 0
				res      = EvaluateUnivariateGnark(api, pol, x)
			)
			api.AssertIsEqual(expected, res)
			return nil
		}
		gnarkutil.AssertCircuitSolved(t, def)
	})

}

func TestGnarkEvalAnyDomain(t *testing.T) {

	t.Run("single-variable", func(t *testing.T) {

		def := func(api frontend.API) error {
			var (
				domain                     = vector.IntoGnarkAssignment(vector.ForTest(0))
				x        frontend.Variable = 42
				expected                   = vector.IntoGnarkAssignment(vector.ForTest(1))
				res                        = EvaluateLagrangeAnyDomainGnark(api, domain, x)
			)

			require.Len(t, res, len(expected))
			for i := range expected {
				api.AssertIsEqual(expected[i], res[i])
			}

			return nil
		}

		gnarkutil.AssertCircuitSolved(t, def)
	})

	t.Run("multiple-variable", func(t *testing.T) {

		def := func(api frontend.API) error {
			var (
				domain                     = vector.IntoGnarkAssignment(vector.ForTest(0, 1))
				x        frontend.Variable = 42
				expected                   = vector.IntoGnarkAssignment(vector.ForTest(-41, 42))
				res                        = EvaluateLagrangeAnyDomainGnark(api, domain, x)
			)

			require.Len(t, res, len(expected))
			for i := range expected {
				api.AssertIsEqual(expected[i], res[i])
			}

			return nil
		}

		gnarkutil.AssertCircuitSolved(t, def)
	})

}
