package types

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Bls12381Fr represents an arbtrary bytes string of 32 bytes. It is used to
// represent the output of non-field based hash function such as keccak256 or
// sha256.
type Bls12381Fr [32]byte

// Bytes returns the bytes representation of the Bls12381Fr
func (d Bls12381Fr) Bytes() []byte {
	return d[:]
}

// Create a bytes32 from a slice
func AsBls12381Fr(b []byte) (d Bls12381Fr) {
	// Sanity-check the length of the digest
	if len(b) != len(Bls12381Fr{}) {
		utils.Panic("Passed a string of %v bytes but expected %v", len(b), 32)
	}
	copy(d[:], b)
	return d
}

// Creates a bytes32 from an hexstring. Panic if it fails. Mostly useful for testing.
// the string s is left padded with zeroes if less than 32 characters are provided
// if more than 32 characters are provided, the function will panic
// function expects an even number of chars
// Ox prefix is optional
func Bls12381FrFromHex(s string) Bls12381Fr {
	b, err := utils.HexDecodeString(s)
	if err != nil {
		utils.Panic("can't decode hex=%v, err: %v", s, err)
	}
	if len(b) > 32 {
		utils.Panic("String passed should have even length <= 32 bytes; len(b)=%v", len(b))
	}

	var res Bls12381Fr
	copy(res[32-len(b):], b)

	var f fr.Element
	if err := f.SetBytesCanonical(res[:]); err != nil {
		utils.Panic("Invalid field element %v", err.Error())
	}

	return res
}
