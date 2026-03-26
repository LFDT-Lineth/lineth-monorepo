package types

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/utils"
)

// EthAddress represents an ethereum address. It consists of 20bytes and is the
// obtained by hashing the corresponding ECDSA-secp256k1 public key and keeping
// the 20 first bytes.
type EthAddress [20]byte

// Marshal "e" into JSON format
func (e EthAddress) MarshalJSON() ([]byte, error) {
	hexAddress := utils.HexEncodeToString(e[:])
	marshalled, err := json.Marshal(hexAddress)
	if err != nil {
		return nil, fmt.Errorf("could not marshal eth address : %w", err)
	}
	return marshalled, nil
}

// Unmarshal an ethereum address from JSON format. The expected format is an hex
// string of 20 bytes. If the input string is larger than 20 bytes, truncate the
// leftmost bytes of the input address. If it is too small zero pad it on the
// left.
func (e *EthAddress) UnmarshalJSON(b []byte) error {
	decoded, err := DecodeQuotedHexString(b)
	if err != nil {
		return fmt.Errorf("could not decode eth address %v : %w", string(b), err)
	}

	switch {
	case len(decoded) < 20:
		copy((*e)[20-len(decoded):], decoded)

	case len(decoded) >= 20:
		copy((*e)[:], decoded[len(decoded)-20:])
	}

	return nil
}

// Writes the padded 40 bytes address into the given write.
func (e EthAddress) WriteTo(w io.Writer) (int64, error) {
	padded := LeftPadded(e[:])
	n, err := w.Write(padded)
	if err != nil {
		panic(err) // hard forbid any error
	}
	return int64(n), nil
}

// Reads a padded 40 bytes address from the given reader
func (e *EthAddress) ReadFrom(r io.Reader) (int64, error) {

	var buf [40]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return 0, err
	}
	unpadded := RemovePadding(buf[:])
	copy((*e)[:], unpadded)
	return int64(n), nil
}

func (e EthAddress) Hex() string {
	return utils.HexEncodeToString(e[:])
}

func AddressFromHex(h string) (EthAddress, error) {
	byt, err := utils.HexDecodeString(h)
	if err != nil {
		return EthAddress{}, fmt.Errorf("reading hex address %s : %w", h, err)
	}
	if len(byt) != 20 {
		return EthAddress{}, fmt.Errorf("reading hex address %s : has %d bytes", h, len(byt))
	}
	var res EthAddress
	copy(res[:], byt)
	return res, nil
}

func MustAddressFromHex(h string) EthAddress {
	a, err := AddressFromHex(h)
	if err != nil {
		panic(err)
	}
	return a
}

// Construct a dummy address from an integer
func DummyAddress(i int) (a EthAddress) {
	a[0] = byte(i)
	return a
}
