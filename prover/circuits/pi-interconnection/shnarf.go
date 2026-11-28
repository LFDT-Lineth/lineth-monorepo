package pi_interconnection

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
)

type ShnarfIteration struct {
	BlobDataSnarkHash                          [32]frontend.Variable
	NewStateRootHash                           [32]frontend.Variable
	EvaluationPointBytes, EvaluationClaimBytes [32]frontend.Variable
}

// RangeCheck asserts that every byte of the iteration's preimage components
// is in [0, 256), as required by the keccak gadget (ComputeShnarfs), which
// does not range-check its inputs itself.
func (i *ShnarfIteration) RangeCheck(api frontend.API) {
	rc := rangecheck.New(api)
	for _, arr := range [][32]frontend.Variable{
		i.BlobDataSnarkHash, i.NewStateRootHash, i.EvaluationPointBytes, i.EvaluationClaimBytes,
	} {
		for _, b := range arr {
			rc.Check(b, 8)
		}
	}
}

// ComputeShnarfs DOES NOT check nbShnarfs ≤ len(s.iterations)
func ComputeShnarfs(h keccak.BlockHasher, parent [32]frontend.Variable, iterations []ShnarfIteration) (result [][32]frontend.Variable) {
	result = make([][32]frontend.Variable, len(iterations))
	prevShnarf := parent

	for i, t := range iterations {
		result[i] = h.Sum(nil, prevShnarf, t.BlobDataSnarkHash, t.NewStateRootHash, t.EvaluationPointBytes, t.EvaluationClaimBytes)
		prevShnarf = result[i]
	}

	return
}

func (i *ShnarfIteration) SetZero() {
	for j := range i.EvaluationClaimBytes {
		i.NewStateRootHash[j] = 0
		i.EvaluationPointBytes[j] = 0
		i.EvaluationClaimBytes[j] = 0
		i.BlobDataSnarkHash[j] = 0
	}
}
