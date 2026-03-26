package types

import (
	"fmt"
	"io"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
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

// Constructs a dummy Bls12377Fr from an integer
func DummyBls12377Fr(i int) (d Bls12377Fr) {
	d[31] = byte(i)
	return d
}

// LeftPadToBls12377Fr pads a bytes32 element into a Bls12377Fr by adding zeroes to
// the left until the slice has 32 bytes
func LeftPadToBls12377Fr(b []byte) Bls12377Fr {
	if len(b) > 32 {
		utils.Panic("Passed a string of %v element but the max is 32", len(b))
	}
	c := append(make([]byte, 32-len(b)), b...)
	return AsBls12377Fr(c)
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

// Creates a bytes32 from an hexstring. Panic if it fails. Mostly useful for testing.
// the string s is left padded with zeroes if less than 32 characters are provided
// if more than 32 characters are provided, the function will panic
// function expects an even number of chars
// Ox prefix is optional
func Bls12377FrFromHex(s string) Bls12377Fr {
	b, err := utils.HexDecodeString(s)
	if err != nil {
		utils.Panic("not an hexadecimal %v", s)
	}
	if len(b) > 32 {
		utils.Panic("String passed should have even length <= 32 bytes")
	}

	var res Bls12377Fr
	copy(res[32-len(b):], b)

	var f fr.Element
	if err := f.SetBytesCanonical(res[:]); err != nil {
		utils.Panic("Invalid field element %v", err.Error())
	}

	return res
}

// MustGetFrElement checks that the bytes32 is a valid field element
func (b Bls12377Fr) MustGetFrElement() fr.Element {
	var f fr.Element
	if err := f.SetBytesCanonical(b[:]); err != nil {
		utils.Panic("Invalid field element %v", err.Error())
	}
	return f
}

// Bls12381ScalarToBls12377Scalars interprets its input as a BLS12-381 scalar, with a modular reduction if necessary, returning two BLS12-377 scalars
// r[1] is the lower 16 bytes and r[0] is the higher ones.
// useful in circuit "assign" functions
func (f Bls12381Fr) ToBls12377Fr() (r Bls12377Fr, err error) {

	var (
		x fr381.Element
		y fr.Element
	)

	if err = x.SetBytesCanonical(f[:]); err != nil {
		return
	}

	if err = y.SetBytesCanonical(f[:]); err != nil {
		return
	}

	copy(r[:], f[:])
	return r, nil
}

// Returns a dummy digest
func DummyDigest(i int) (d Bls12377Fr) {
	d[31] = byte(i) // on the last one to not create overflows
	return d
}
