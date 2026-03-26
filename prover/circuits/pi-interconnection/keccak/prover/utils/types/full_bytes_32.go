package types

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Full bytes32 is a wrapper around bytes32 used to specifically represent
// a bytes32 that may not fit on a single field element and require 2 words
// to be hashed with MiMC.
type FullBytes32 Bytes32

func (f FullBytes32) WriteTo(w io.Writer) (n int64, err error) {
	buf := [32]byte{}
	copy(buf[16:], f[16:])
	w.Write(buf[:])
	copy(buf[16:], f[:16])
	w.Write(buf[:])
	return 64, nil
}

func (f FullBytes32) Write1Word(w io.Writer) (n int64, err error) {
	return Bytes32(f).WriteTo(w)
}

func (f *FullBytes32) ReadFrom(r io.Reader) (n int64, err error) {
	buf0, buf1 := [32]byte{}, [32]byte{}
	r.Read(buf0[:])
	r.Read(buf1[:])
	copy((*f)[16:], buf0[16:])
	copy((*f)[:16], buf1[16:])
	return 64, nil
}

func (f *FullBytes32) ReadPacked(r io.Reader) (n int64, err error) {
	r.Read((*f)[:])
	return 32, nil
}

func (f FullBytes32) Hex() string {
	return Bytes32(f).Hex()
}

func AsFullBytes32(b []byte) FullBytes32 {
	return FullBytes32(AsBytes32(b))
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
