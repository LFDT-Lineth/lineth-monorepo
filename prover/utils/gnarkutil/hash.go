package gnarkutil

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
)

// SumMerkleDamgardDynamicLength computes the Merkle-Damgård hash of the input data, truncated at the given length.
// Parameters:
//   - api: constraint builder
//   - f: 2-1 hash compression (one-way) function
//   - initialState: the initialization vector (IV) in the Merkle-Damgård chain. It must be a value whose preimage is not known.
//   - length: length of the prefix of data to be hashed. The verifier will not accept a value outside the range {0, 1, ..., len(data)}.
//     The gnark prover will refuse to attempt to generate such an unsuccessful proof.
//   - data: the values a prefix of which is to be hashed.
func SumMerkleDamgardDynamicLength(api frontend.API, f hash.Compressor, initialState frontend.Variable, length frontend.Variable, data []frontend.Variable) frontend.Variable {
	resT := logderivlookup.New(api)
	state := initialState

	resT.Insert(state)
	for _, v := range data {
		state = f.Compress(state, v)
		resT.Insert(state)
	}

	return resT.Lookup(length)[0]
}
