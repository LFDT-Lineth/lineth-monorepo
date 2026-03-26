package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
)

// An Ethereum account represented with the zkTrie representation
type Account struct {
	// Implicitly we assume that the nonce fits into
	// a int64 but the obtained encoding is consistent
	// with how it would be if it was a big.Int
	Nonce          int64
	Balance        *big.Int
	StorageRoot    KoalaOctuplet
	LineaCodeHash  KoalaOctuplet // Poseidon2 code hash
	KeccakCodeHash FullBytes32
	CodeSize       int64
}

// AccountShomeiTraces is a wrapper for the [Account] and it features the
// encoding of the account as used in the Shomei traces.
type AccountShomeiTraces struct {
	Account
}

func (a Account) WrappedForShomeiTraces() AccountShomeiTraces {
	return AccountShomeiTraces{a}
}

func (a Account) WriteTo(w io.Writer) (int64, error) {
	return a.writeTo(w, false)
}

func (a *Account) ReadFrom(r io.Reader) (int64, error) {
	return a.readFrom(r, false)
}

func (a Account) String() string {
	return fmt.Sprintf(
		"Account{Nonce: %d, Balance: %s, StorageRoot: %s, LineaCodeHash: %s, KeccakCodeHash: %s, CodeSize: %d}",
		a.Nonce, a.Balance, a.StorageRoot.Hex(), a.LineaCodeHash.Hex(), a.KeccakCodeHash.Hex(), a.CodeSize,
	)
}

// Write the account into a writer. The `packed` argument specifies if the
// keccak code hash should be written on 1 or 2 32-bytes words. The first option
// is for serialization and the second option is for hashing with a SNARK-friendly
// hash.
//
// If the account contains a "nil" balance is will be written as zero.
func (a Account) writeTo(w io.Writer, packed bool) (int64, error) {
	n0, _ := WriteInt64On64Bytes(w, a.Nonce)
	// Without this edge-case handling, the function panics if called over
	// Account{}
	balance := a.Balance
	if balance == nil {
		balance = &big.Int{}
	}
	n1, _ := WriteBigIntOn64Bytes(w, balance)
	n2, _ := a.StorageRoot.WriteTo(w)
	n3, _ := a.LineaCodeHash.WriteTo(w)
	var n4 int64
	if packed {
		n4, _ = a.KeccakCodeHash.Write1Word(w)
	} else {
		n4, _ = a.KeccakCodeHash.WriteTo(w)
	}
	n5, _ := WriteInt64On64Bytes(w, a.CodeSize)
	return n0 + n1 + n2 + n3 + n4 + n5, nil
}

// Reads an account from a buffer. The keccak code hash is on a single
// byte.
func (a *Account) readFrom(r io.Reader, packed bool) (int64, error) {

	var err error

	a.Nonce, _, err = ReadInt64On64Bytes(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading nonce : %w", err)
	}
	a.Balance, err = ReadBigIntOn64Bytes(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading balance : %w", err)
	}
	_, err = a.StorageRoot.ReadFrom(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading storage root : %w", err)
	}
	_, err = a.LineaCodeHash.ReadFrom(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading code-hash : %w", err)
	}
	var nK int64
	if packed {
		nK, err = a.KeccakCodeHash.ReadPacked(r)
	} else {
		nK, err = a.KeccakCodeHash.ReadFrom(r)
	}
	if err != nil {
		return 0, fmt.Errorf("reading account : reading keccak codehash : %w", err)
	}
	a.CodeSize, _, err = ReadInt64On64Bytes(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading codesize : %w", err)
	}

	return 256 + nK, nil
}

// writeToShomeiTraces writes the account as in a Shomei trace JSON file.
func (a Account) writeToShomeiTraces(w io.Writer) (int64, error) {

	// Without this edge-case handling, the function panics if called over
	// Account{}
	balance := a.Balance
	if balance == nil {
		balance = &big.Int{}
	}

	n0, e0 := WriteInt64On32Bytes(w, a.Nonce)
	n1, e1 := WriteBigIntOn32Bytes(w, balance)
	n2, e2 := a.StorageRoot.WriteTo(w)
	n3, e3 := a.LineaCodeHash.WriteTo(w)
	n4, e4 := a.KeccakCodeHash.Write1Word(w)
	n5, e5 := WriteInt64On64Bytes(w, a.CodeSize)

	mainErr := errors.Join(e0, e1, e2, e3, e4, e5)
	if mainErr != nil {
		return 0, fmt.Errorf("writing account : %w", mainErr)
	}

	return n0 + n1 + n2 + n3 + n4 + n5, nil
}

// readFromShomeiTraces reads the account as in a Shomei trace JSON file.
func (a *Account) readFromShomeiTraces(r io.Reader) (int64, error) {

	var err error

	a.Nonce, _, err = ReadInt64On32Bytes(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading nonce : %w", err)
	}

	a.Balance, err = ReadBigIntOn32Bytes(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading balance : %w", err)
	}

	_, err = a.StorageRoot.ReadFrom(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading storage root : %w", err)
	}

	_, err = a.LineaCodeHash.ReadFrom(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading code-hash : %w", err)
	}

	_, err = a.KeccakCodeHash.ReadPacked(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading keccak codehash : %w", err)
	}

	a.CodeSize, _, err = ReadInt64On32Bytes(r)
	if err != nil {
		return 0, fmt.Errorf("reading account : reading codesize : %w", err)
	}

	return 192, nil
}

func (a Account) MarshalJSON() ([]byte, error) {
	var buf = &bytes.Buffer{}
	a.writeTo(buf, true)
	marshalled := MarshalHexBytesJSON(buf.Bytes())
	return marshalled, nil
}

func (a *Account) UnmarshalJSON(data []byte) error {
	decoded, err := DecodeQuotedHexString(data)
	if err != nil {
		return fmt.Errorf("could not decode eth account hexstring : %w", err)
	}
	buf := bytes.NewBuffer(decoded)
	_, err = a.readFrom(buf, true)
	if err != nil {
		return fmt.Errorf("unmarshaling JSON account : %w", err)
	}
	return nil
}

func (a AccountShomeiTraces) MarshalJSON() ([]byte, error) {
	var buf = &bytes.Buffer{}
	a.writeToShomeiTraces(buf)
	marshalled := MarshalHexBytesJSON(buf.Bytes())
	return marshalled, nil
}

func (a *AccountShomeiTraces) UnmarshalJSON(data []byte) error {
	decoded, err := DecodeQuotedHexString(data)
	if err != nil {
		return fmt.Errorf("could not decode eth account hexstring : %w", err)
	}
	buf := bytes.NewBuffer(decoded)
	_, err = a.readFromShomeiTraces(buf)
	if err != nil {
		return fmt.Errorf("unmarshaling JSON account : %w", err)
	}
	return nil
}
