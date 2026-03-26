package fiatshamir

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/gnarkutil"
)

func TestGnarkSafeGuardUpdate(t *testing.T) {

	f := func(api frontend.API) error {
		fs := NewGnarkFiatShamir(api, nil)
		a := fs.RandomField()
		b := fs.RandomField()
		api.AssertIsDifferent(a, b)
		return nil
	}

	gnarkutil.AssertCircuitSolved(t, f)
}

func TestGnarkRandomVec(t *testing.T) {

	for _, testCase := range randomIntVecTestCases {
		testName := fmt.Sprintf("%v-integers-of-%v-bits", testCase.NumIntegers, testCase.IntegerBitSize)
		t.Run(testName, func(t *testing.T) {

			f := func(api frontend.API) error {

				gnarkFs := NewGnarkFiatShamir(api, nil)
				fs := NewMiMCFiatShamir()

				fs.Update(field.NewElement(2))
				gnarkFs.Update(field.NewElement(2))

				a := fs.RandomManyIntegers(testCase.NumIntegers, 1<<testCase.IntegerBitSize)
				aGnark := gnarkFs.RandomManyIntegers(testCase.NumIntegers, 1<<testCase.IntegerBitSize)

				for i := range a {
					api.AssertIsEqual(a[i], aGnark[i])
				}

				return nil
			}

			gnarkutil.AssertCircuitSolved(t, f)
		})
	}
}

func TestGnarkFiatShamirEmpty(t *testing.T) {

	f := func(api frontend.API) error {
		Y := NewMiMCFiatShamir().RandomField()
		fs := NewGnarkFiatShamir(api, nil)
		y := fs.RandomField()
		api.AssertIsEqual(Y, y)
		return nil
	}

	gnarkutil.AssertCircuitSolved(t, f)
}

func TestGnarkUpdateVec(t *testing.T) {

	f := func(api frontend.API) error {
		fs := NewMiMCFiatShamir()
		fs.UpdateVec(vector.ForTest(2, 2, 1, 2))
		y1 := fs.RandomField()

		fs2 := NewGnarkFiatShamir(api, nil)
		fs2.UpdateVec([]frontend.Variable{2, 2, 1, 2})
		y2 := fs2.RandomField()

		api.AssertIsEqual(y1, y2)
		return nil
	}

	gnarkutil.AssertCircuitSolved(t, f)
}
