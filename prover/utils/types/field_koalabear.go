package types

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/bls12377/field"
)

// KoalaFr is a wrapper for gnark's koalabear elements that is suitable
// for serialization.
type KoalaFr field.Element

// ToBytes converts a koalabear element to bytes.
func (e KoalaFr) ToBytes() []byte {
	x := field.Element(e)
	b := x.Bytes()
	return b[:]
}

// FromBytes sets the receiver to the provided bytes in canonical form
func (e *KoalaFr) SetBytes(b []byte) error {
	var x field.Element
	if len(b) > 4 {
		return fmt.Errorf(
			"could not unmarshal bytes32 %x : should have 32 bytes but has %v bytes",
			b, len(b),
		)
	}

	if len(b) < 4 {
		b = append(make([]byte, 4-len(b)), b...)
	}

	if err := x.SetBytesCanonical(b); err != nil {
		return fmt.Errorf("could not unmarshal bytes32 %x : %w", b, err)
	}

	*e = KoalaFr(x)
	return nil
}

// MarshalJSON converts the koalabear element to json using hex encoding and
// prefixing with 0x.
func (e KoalaFr) MarshalJSON() ([]byte, error) {
	return MarshalHexBytesJSON(e.ToBytes()), nil
}

// UnmarshalJSON converts the koalabear element from json using hex decoding.
// The decoder tolerates the absence of an 0x prefix but will error if the
// provided string does not fit in 4 bytes.
func (e *KoalaFr) UnmarshalJSON(b []byte) error {
	decoded, err := DecodeQuotedHexString(b)
	if err != nil {
		return fmt.Errorf(
			"could not decode bytes32 `%v`, expected an hex string of 32 bytes : %w",
			string(b), err,
		)
	}
	return e.SetBytes(decoded)
}

// ToGnark returns the underlying gnark koalabear element
func (e KoalaFr) ToGnark() field.Element {
	return field.Element(e)
}
