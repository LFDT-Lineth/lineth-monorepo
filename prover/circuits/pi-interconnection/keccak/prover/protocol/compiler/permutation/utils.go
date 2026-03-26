package permutation

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizardutils"
)

const permutationStr = "PERMUTATION"

// deriveName constructs a name for the permutation context
func deriveName[R ~string](q query.Permutation, ss ...any) R {
	ss = append([]any{permutationStr, q}, ss...)
	return wizardutils.DeriveName[R](ss...)
}

// deriveName constructs a name for the permutation context
func deriveNameGen[R ~string](ss ...any) R {
	ss = append([]any{permutationStr}, ss...)
	return wizardutils.DeriveName[R](ss...)
}
