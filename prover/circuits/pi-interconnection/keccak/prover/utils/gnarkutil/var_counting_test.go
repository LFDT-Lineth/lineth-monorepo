package gnarkutil

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark/frontend"
)

func TestCountVariables(t *testing.T) {

	testcases := []struct {
		Circ         any
		NbPub, NbSec int
	}{
		{
			Circ: struct {
				A frontend.Variable `gnark:",public"`
				B frontend.Variable `gnark:",secret"`
			}{},
			NbPub: 1,
			NbSec: 1,
		},
		{
			Circ: struct {
				A frontend.Variable `gnark:",secret"`
				B frontend.Variable `gnark:",public"`
			}{},
			NbPub: 1,
			NbSec: 1,
		},
		{
			Circ: struct {
				A [3]frontend.Variable `gnark:",secret"`
				B frontend.Variable    `gnark:",public"`
			}{},
			NbPub: 1,
			NbSec: 3,
		},
		{
			Circ: struct {
				A frontend.Variable    `gnark:",secret"`
				B [3]frontend.Variable `gnark:",public"`
			}{},
			NbPub: 3,
			NbSec: 1,
		},
	}

	for i := range testcases {

		testcaseName := fmt.Sprintf("case-%v", i)

		t.Run(testcaseName, func(t *testing.T) {

			nbPub, nbSec := CountVariables(testcases[i].Circ)
			if nbPub != testcases[i].NbPub {
				t.Errorf("nbPub: %d != %d", nbPub, testcases[i].NbPub)
			}
			if nbSec != testcases[i].NbSec {
				t.Errorf("nbSec: %d != %d", nbSec, testcases[i].NbSec)
			}

		})

	}

}
