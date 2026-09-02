package gnarkutil

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
)

// AsWitness converts a slice of field elements to a slice of witness variables
// of the same length with only public inputs.
func AsWitnessPublic(v []frontend.Variable) witness.Witness {

	var (
		wit, _  = witness.New(ecc.BLS12_377.ScalarField())
		witChan = make(chan any, len(v))
	)

	for _, w := range v {
		witChan <- w
	}

	close(witChan)

	if err := wit.Fill(len(v), 0, witChan); err != nil {
		panic(err)
	}

	return wit
}
