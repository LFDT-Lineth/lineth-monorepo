package gnarkutil

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
)

/*
Allocate a slice of field element
*/
func AllocateSlice(n int) []frontend.Variable {
	return make([]frontend.Variable, n)
}

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

// GetSlicePosition returns a position in a slice of variables. Or zero if no match
// is found.
func GetSlicePosition(api frontend.API, v []frontend.Variable, pos frontend.Variable) (res, isMatched frontend.Variable) {

	res, isMatched = frontend.Variable(0), frontend.Variable(0)

	for i, v := range v {
		iIsMatch := api.IsZero(api.Sub(pos, frontend.Variable(i)))
		res = api.Select(iIsMatch, v, res)
		// No need to use "Or" is since all the "i" are distinct, pos cannot be
		// equal to more than one "i".
		isMatched = api.Add(isMatched, iIsMatch)
	}

	return res, isMatched
}
