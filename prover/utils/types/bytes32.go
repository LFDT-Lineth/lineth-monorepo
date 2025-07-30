package types

import (
	"fmt"
	"io"
	"math/big"
	"reflect"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Bytes32 represents an arbtrary bytes string of 32 bytes. It is used to
// represent the output of non-field based hash function such as keccak256 or
// sha256.
type Bytes32 [32]byte

// Marshal "e" into JSON format
func (f Bytes32) MarshalJSON() ([]byte, error) {
	marshalled := MarshalHexBytesJSON(f[:])
	return marshalled, nil
}

// Unmarshal an ethereum address from JSON format. The expected format is an hex
// string.
func (f *Bytes32) UnmarshalJSON(b []byte) error {

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

// Returns true if the receiver is a valid field element
func (f *Bytes32) IsBn254Fr() bool {
	var (
		x            field.Element
		reserialized [32]byte
	)
	x.SetBytes(f[:])
	reserialized = x.Bytes()
	return reflect.DeepEqual([32]byte(*f), reserialized)
}

// Writes the bytes32 into the given write.
func (b Bytes32) WriteTo(w io.Writer) (int64, error) {
	_, err := w.Write(b[:])
	if err != nil {
		panic(err) // hard forbid any error
	}
	return 32, nil
}

// Reads a bytes32 from the given reader
func (b *Bytes32) ReadFrom(r io.Reader) (int64, error) {
	n, err := r.Read((*b)[:])
	return int64(n), err
}

/*
Cmp two Bytes32s. The Bytes32 are interpreted as big-endians big integers and
then are compared. Returns:
  - a < b : -1
  - a == b : 0
  - a > b : 1
*/
func Bytes32Cmp(a, b Bytes32) int {
	var bigA, bigB big.Int
	bigA.SetBytes(a[:])
	bigB.SetBytes(b[:])
	return bigA.Cmp(&bigB)
}

// Returns an hexstring representation of the Bytes32
func (d Bytes32) Hex() string {
	return fmt.Sprintf("0x%x", [32]byte(d))
}

// Constructs a dummy Bytes32 from an integer
func DummyBytes32(i int) (d Bytes32) {
	d[31] = byte(i)
	return d
}

// LeftPadToBytes32 pads a bytes32 element into a Bytes32 by adding zeroes to
// the left until the slice has 32 bytes
func LeftPadToBytes32(b []byte) Bytes32 {
	if len(b) > 32 {
		utils.Panic("Passed a string of %v element but the max is 32", len(b))
	}
	c := append(make([]byte, 32-len(b)), b...)
	return AsBytes32(c)
}

// Create a bytes32 from a slice
func AsBytes32(b []byte) (d Bytes32) {
	// Sanity-check the length of the digest
	if len(b) != len(Bytes32{}) {
		utils.Panic("Passed a string of %v bytes but expected %v", len(b), 32)
	}
	copy(d[:], b)
	return d
}

// Creates a bytes32 from an hexstring. Panic if it fails. Mostly useful for testing.
// the string s is left padded with zeroes if less than 64 characters are provided
// if more than 64 characters are provided, the function will panic
// function expects an even number of chars
// Ox prefix is optional
func Bytes32FromHex(s string) Bytes32 {
	b, err := utils.HexDecodeString(s)
	if err != nil {
		utils.Panic("not an hexadecimal %v", s)
	}
	if len(b) > 32 {
		utils.Panic("String passed should have even length <= 32 bytes")
	}

	var res Bytes32
	copy(res[32-len(b):], b)
	var f field.Element
	if err := f.SetBytesCanonical(res[:]); err != nil {
		utils.Panic("Invalid field element %v", err.Error())
	}
	return res
}

// Returns a dummy digest
func DummyDigest(i int) (d Bytes32) {
	d[31] = byte(i) // on the last one to not create overflows
	return d
}

// SetField sets the bytes32 from a field.Element
func (b *Bytes32) SetField(f field.Element) {
	*b = Bytes32(f.Bytes())
}

// ToField returns the bytes32 as a field.Element
func (b Bytes32) ToField() field.Element {
	var f field.Element
	if err := f.SetBytesCanonical(b[:]); err != nil {
		panic(err)
	}
	return f
}
