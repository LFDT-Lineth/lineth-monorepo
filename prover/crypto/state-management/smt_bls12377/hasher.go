package smt_bls12377

import (
	"hash"

	. "github.com/consensys/linea-monorepo/prover/utils/types"
)

// Wrapper types for hasher which additionally provides a max value
type Hasher struct {
	hash.Hash            // the underlying hasher
	maxValue  Bls12377Fr // the maximal value obtainable with that hasher
}
