package mpts

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

func TestLdeOf(t *testing.T) {

	gen_8, err := fft.Generator(8)
	if err != nil {
		panic(err)
	}

	gen_4, err1 := fft.Generator(4)
	if err1 != nil {
		panic(err1)
	}
	testcases := []struct {
		Name string
		Poly []fext.Element
		LDE  []fext.Element
	}{
		{
			Name: "constant-poly",
			Poly: vectorext.Repeat(fext.NewFromUint(23, 0, 0, 0), 8),
			LDE:  vectorext.Repeat(fext.NewFromUint(23, 0, 0, 0), 32),
		},
		{
			Name: "x-poly",
			Poly: vectorext.ForTest(1, -1),
			LDE:  vectorext.PowerVec(fext.Lift(gen_8), 8),
		},
		{
			Name: "x-poly-2",
			Poly: vectorext.PowerVec(fext.Lift(gen_4), 4),
			LDE:  vectorext.PowerVec(fext.Lift(gen_8), 8),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {

			sizeBig := len(tc.LDE)
			res := make([]fext.Element, sizeBig)
			copy(res, tc.Poly)
			_ldeOfExt(res, len(tc.Poly), sizeBig)

			for i := range tc.LDE {
				if !tc.LDE[i].Equal(&res[i]) {
					t.Errorf("mismatch res=%v, expected=%v", vectorext.Prettify(res), vectorext.Prettify(tc.LDE))
					return
				}
			}
		})
	}

}
