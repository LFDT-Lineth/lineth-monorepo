package types

import (
	"fmt"
	"io"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/utils"
)

// Full bytes32 is a wrapper around bytes32 used to specifically represent
// a bytes32 that may not fit on a single field element and require 2 words
// to be hashed with MiMC.
type FullBytes32 [32]byte

func (f FullBytes32) WriteTo(w io.Writer) (n int64, err error) {
	padded := LeftPadded(f[:])
	w.Write(padded[:])
	return 64, nil
}

func (f FullBytes32) Write1Word(w io.Writer) (n int64, err error) {
	n_, err := w.Write(f[:])
	return int64(n_), err
}

func (f *FullBytes32) ReadFrom(r io.Reader) (n int64, err error) {
	buf := [64]byte{}
	r.Read(buf[:])
	copy((*f)[:], RemovePadding(buf[:]))
	return 64, nil
}

func (f *FullBytes32) ReadPacked(r io.Reader) (n int64, err error) {
	r.Read((*f)[:])
	return 32, nil
}

func (f FullBytes32) Hex() string {
	return utils.HexEncodeToString(f[:])
}

func AsFullBytes32(b []byte) FullBytes32 {
	return FullBytes32([32]byte(b))
}

// Creates a bytes32 from an hexstring. Panic if it fails. Mostly useful for
// testing.
func FullBytes32FromHex(s string) FullBytes32 {
	b, err := utils.HexDecodeString(s)
	if err != nil {
		utils.Panic("not an hexadecimal %v", s)
	}
	var res FullBytes32
	copy(res[:], b)
	return res
}

// Marshal "e" into JSON format
func (f FullBytes32) MarshalJSON() ([]byte, error) {
	marshalled := MarshalHexBytesJSON(f[:])
	return marshalled, nil
}

// Unmarshal an ethereum address from JSON format. The expected format is an hex
// string.
func (f *FullBytes32) UnmarshalJSON(b []byte) error {
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

// Constructs a dummy full bytes from an integer
func DummyFullByte(i int) (f FullBytes32) {
	f[0] = byte(i)
	return f
}

// Converts a slice of [Bytes32] or [FullBytes32] into a slice of [32]byte
func AsByteArrSlice[T ~[32]byte](s []T) [][32]byte {
	res := make([][32]byte, len(s))
	for i := range s {
		res[i] = [32]byte(s[i])
	}
	return res
}

// LeftPadToFullBytes32 pads a bytes32 element into a Bytes32 by adding zeroes to
// the left until the slice has 32 bytes
func LeftPadToFullBytes32(b []byte) FullBytes32 {
	if len(b) > 32 {
		utils.Panic("Passed a string of %v element but the max is 32", len(b))
	}
	c := append(make([]byte, 32-len(b)), b...)
	return FullBytes32(c)
}

// IsZero checks if the FullBytes32 is all zeros
func (f FullBytes32) IsZero() bool {
	for i := range f {
		if f[i] != 0 {
			return false
		}
	}
	return true
}

// Uint64BigEndian interprets f as a big-endian unsigned integer.
// If the value does not fit in uint64, returns (0, false). Callers must check ok.
func (f FullBytes32) Uint64BigEndian() (uint64, bool) {
	bi := new(big.Int).SetBytes(f[:])
	if !bi.IsUint64() {
		return 0, false
	}
	return bi.Uint64(), true
}
