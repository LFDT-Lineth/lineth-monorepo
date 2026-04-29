package types

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// KoalaOctuplet is a wrapper around koalabear octuplets which can be used
// for serialization.
type KoalaOctuplet [8]field.Element

// ReadFrom reads a koalabear octuplet from a reader.
func (e *KoalaOctuplet) ReadFrom(r io.Reader) (int64, error) {
	buf := [32]byte{}
	n, err := r.Read(buf[:])
	if err != nil {
		return 0, err
	}
	return int64(n), e.SetBytes(buf[:])
}

// WriteTo writes a koalabear octuplet to a writer.
func (e KoalaOctuplet) WriteTo(w io.Writer) (int64, error) {
	b := e.ToBytes()
	n_, err := w.Write(b)
	return int64(n_), err
}

// ToBytes converts a koalabear octuplet to bytes.
func (e KoalaOctuplet) ToBytes() []byte {
	res := [32]byte{}
	for i := range e {
		x := field.Element(e[i])
		b := x.Bytes()
		copy(res[4*i:], b[:])
	}
	return res[:]
}

// SetBytes sets the receiver to the provided bytes in canonical form.
func (e *KoalaOctuplet) SetBytes(b []byte) error {
	if len(b) != 32 {
		return fmt.Errorf("could not unmarshal koalabear octuplet %x : should have 32 bytes but has %v bytes", b, len(b))
	}

	for i := range *e {
		var x field.Element
		segment := b[4*i : 4*i+4]
		if err := x.SetBytesCanonical(segment); err != nil {
			return fmt.Errorf("could not unmarshal koalabear octuplet %x (%x) : %w", b, segment, err)
		}
		e[i] = x
	}
	return nil
}

// BytesToKoalaOctuplet attempts to convert a byte slice into a koalabear
// element.
func BytesToKoalaOctuplet(b []byte) (KoalaOctuplet, error) {
	var e KoalaOctuplet
	if err := e.SetBytes(b); err != nil {
		return KoalaOctuplet{}, err
	}
	return e, nil
}

// MustBytesToKoalaOctuplet attempts to convert a byte slice into a koalabear
// element.
func MustBytesToKoalaOctuplet(b []byte) KoalaOctuplet {
	e, err := BytesToKoalaOctuplet(b)
	if err != nil {
		panic(err)
	}
	return e
}

// BytesToKoalaOctupletLoose converts a bytestring into an octuplet of koalabear
// element. The function will right-pad with zeroes up to 32 bytes or cut-off
// the input down to 32 bytes and then it will automatically apply the modulo
// reduction for each doubleword overflowing the modulus of koalabear.
func BytesToKoalaOctupletLoose(b []byte) KoalaOctuplet {

	for i := len(b); i < 32; i++ {
		b = append(b, 0)
	}

	if len(b) > 32 {
		b = b[:32]
	}

	if len(b) != 32 {
		panic("normally the input should have been regularized to 32 bytes")
	}

	res := KoalaOctuplet{}
	for i := range res {
		res[i].SetBytes(b[4*i : 4*i+4])
	}

	return res
}

// Hex returns an hex string representing the koalabear octuplet.
func (e KoalaOctuplet) Hex() string {
	return "0x" + hex.EncodeToString(e.ToBytes())
}

// SetHex sets the koalabear element to an hexString
func (e *KoalaOctuplet) SetHex(hexString string) error {
	b, err := utils.HexDecodeString(hexString)
	if err != nil {
		return err
	}
	return e.SetBytes(b)
}

// HexToKoalabearOctuplet converts a hexstring of less than 32 elements into a
// koalabear octuplet.
func HexToKoalabearOctuplet(hexString string) (KoalaOctuplet, error) {
	var e KoalaOctuplet
	if err := e.SetHex(hexString); err != nil {
		return KoalaOctuplet{}, err
	}
	return e, nil
}

// HexToKoalabearOctupletLoose converts a hexstring of less than 32 elements into
// a koalabear octuplet. The string must be a valid hex string (even number of
// element or the function panic).
func HexToKoalabearOctupletLoose(hexString string) KoalaOctuplet {
	b, err := utils.HexDecodeString(hexString)
	if err != nil {
		panic(err)
	}
	return BytesToKoalaOctupletLoose(b)
}

// MustHexToKoalabearOctuplet converts a hexstring of less than 32 elements into a
// koalabear octuplet.
func MustHexToKoalabearOctuplet(hexString string) KoalaOctuplet {
	e, err := HexToKoalabearOctuplet(hexString)
	if err != nil {
		panic(err)
	}
	return e
}

// MarshalJSON implements the Marshaler interface.
func (e KoalaOctuplet) MarshalJSON() ([]byte, error) {
	return MarshalHexBytesJSON(e.ToBytes()), nil
}

// UnmarshalJSON convert a quoted hexstring of less than 32 elements into a
// koalabear octuplet.
func (e *KoalaOctuplet) UnmarshalJSON(b []byte) error {
	decoded, err := DecodeQuotedHexString(b)
	if err != nil {
		return fmt.Errorf(
			"could not decode bytes32 `%v`, expected an hex string of 32 bytes : %w",
			string(b), err,
		)
	}
	return e.SetBytes(decoded)
}

var negativeOne = field.NewFromString("-1")

// IsMaxOctuplet checks if the current octuplet is equal to (-1, -1, -1, -1, -1,
// -1, -1, -1).
func (e KoalaOctuplet) IsMaxOctuplet() bool {
	for i := range e {
		if e[i] != negativeOne {
			return false
		}
	}
	return true
}

// MaxKoalaOctuplet returns (-1, -1, -1, -1, -1, -1, -1, -1)
func MaxKoalaOctuplet() KoalaOctuplet {
	return KoalaOctuplet{
		negativeOne, negativeOne, negativeOne, negativeOne,
		negativeOne, negativeOne, negativeOne, negativeOne,
	}
}

// Cmp compares two koalabear octuplets assuming big-endian order.
func (e KoalaOctuplet) Cmp(other KoalaOctuplet) int {
	for i := range e {
		r := e[i].Cmp(&other[i])
		if r != 0 {
			return r
		}
	}
	return 0
}

// ToOctuplet converts the koalabear octuplet to an octuplet. Calling this
// function may be avoided most of the time but it cannot be always avoided.
func (e KoalaOctuplet) ToOctuplet() field.Octuplet {
	return e
}

// ToBytes32 converts the koalabear octuplet to a bytes32.
func (e KoalaOctuplet) ToBytes32() [32]byte {
	return [32]byte(e.ToBytes())
}

// DummyKoalaOctuplet generates a dummy koalabear octuplet from an integer and
// is useful for test generation.
func DummyKoalaOctuplet(i int) KoalaOctuplet {
	return KoalaOctuplet{
		field.NewElement(uint64(i)),
	}
}
