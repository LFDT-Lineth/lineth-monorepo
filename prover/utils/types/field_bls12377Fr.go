package types

import (
	"fmt"
	"io"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Bls12377Fr represents an arbtrary bytes string of 32 bytes. It is used to
// represent the output of non-field based hash function such as keccak256 or
// sha256.
type Bls12377Fr [32]byte

// Marshal "e" into JSON format
func (f Bls12377Fr) MarshalJSON() ([]byte, error) {
	marshalled := MarshalHexBytesJSON(f[:])
	return marshalled, nil
}

// Unmarshal an ethereum address from JSON format. The expected format is an hex
// string.
func (f *Bls12377Fr) UnmarshalJSON(b []byte) error {

	decoded, err := DecodeQuotedHexString(b)
	if err != nil {
		return fmt.Errorf(
			"could not decode bytes32 `%v`, expected an hex string of 32 bytes : %w",
			string(b), err,
		)
	}

	if len(decoded) != 32 {
		return fmt.Errorf(
			"could not unmarshal bytes32 %x : should have 32 bytes but has %v bytes",
			decoded, len(decoded),
		)
	}

	copy((*f)[:], decoded)
	return nil
}

// Writes the bytes32 into the given write.
func (b Bls12377Fr) WriteTo(w io.Writer) (int64, error) {
	_, err := w.Write(b[:])
	if err != nil {
		panic(err) // hard forbid any error
	}
	return 32, nil
}

// Reads a bytes32 from the given reader
func (b *Bls12377Fr) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read((*b)[:])
	return int64(n), err
}

// Returns an hexstring representation of the Bls12377Fr
func (d Bls12377Fr) Hex() string {
	return fmt.Sprintf("0x%x", [32]byte(d))
}

// Create a bytes32 from a slice
func AsBls12377Fr(b []byte) (d Bls12377Fr) {
	// Sanity-check the length of the digest
	if len(b) != len(Bls12377Fr{}) {
		utils.Panic("Passed a string of %v bytes but expected %v", len(b), 32)
	}
	copy(d[:], b)
	return d
}

// MustGetFrElement checks that the bytes32 is a valid field element
func (b Bls12377Fr) MustGetFrElement() fr.Element {
	var f fr.Element
	if err := f.SetBytesCanonical(b[:]); err != nil {
		utils.Panic("Invalid field element %v", err.Error())
	}
	return f
}
