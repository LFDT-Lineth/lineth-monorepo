package hashtypes

import (
	"hash"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/bls12377/field"
	. "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Wrapper types for hasher which additionally provides a max value
type Hasher struct {
	hash.Hash            // the underlying hasher
	maxValue  Bls12377Fr // the maximal value obtainable with that hasher
}

// Immutable accessor for the max value of the hasher
func (h Hasher) MaxBls12377Fr() Bls12377Fr {
	return h.maxValue
}

// Create a Keccak hasher
func Keccak() Hasher {
	return Hasher{
		Hash: crypto.NewKeccakState(),
		maxValue: Bls12377Fr{
			255, 255, 255, 255,
			255, 255, 255, 255,
			255, 255, 255, 255,
			255, 255, 255, 255,
			255, 255, 255, 255,
			255, 255, 255, 255,
			255, 255, 255, 255,
			255, 255, 255, 255,
		},
	}
}

// Create a new MiMC hasher
func MiMC() Hasher {
	maxVal := field.NewFromString("-1")
	return Hasher{
		Hash:     mimc.NewMiMC(),
		maxValue: maxVal.Bytes(),
	}
}
