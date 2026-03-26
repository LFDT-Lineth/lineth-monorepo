package types

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/utils"
)

func DecodeQuotedHexString(b []byte) ([]byte, error) {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return nil, fmt.Errorf(
			"could not unmarshal hex string : expected a quoted string but got `%v`, error : %w",
			string(b), err,
		)
	}
	decoded, err := utils.HexDecodeString(unquoted)
	if err != nil {
		return nil, fmt.Errorf(
			"could not unmarshal hex string : expected an hex string but got `%v`, error : %w",
			unquoted, err,
		)
	}
	return decoded, nil
}

func MarshalHexBytesJSON(b []byte) []byte {
	hexstring := utils.HexEncodeToString(b)
	return []byte(strconv.Quote(hexstring))
}

// LeftPadded copies b into a new byte slice of size 2*len(b).
// Every 2 bytes from b are copied into res, left padded by 2 zero-bytes.
func LeftPadded(b []byte) []byte {

	if len(b)%2 != 0 {
		panic("input length must be even")
	}

	res := make([]byte, len(b)*2)

	// We copy every 2 bytes from tmp into res, left started by 2 zero-bytes.
	for i := 0; i < len(b)/2; i++ {
		copy(res[4*i+2:4*i+4], b[2*i:2*i+2])
	}
	return res
}

// LeftPadded48Zeros copies b into a new byte slice, left padded by 48 zero-bytes.
func LeftPadded48Zeros(b []byte) []byte {

	if len(b) != 16 {
		panic("input length must be 16")
	}

	res := make([]byte, 64)
	copy(res[48:], b[:])
	return res
}

// RemovePadding removes the zero padding from the byte slice.
// Every 4 bytes from b are copied into res, removing the first 2 zero-bytes.
func RemovePadding(b []byte) []byte {

	if len(b)%4 != 0 {
		panic("input length must be a multiple of 4")
	}

	data := make([]byte, len(b)/2)
	for i := 0; i < len(b)/4; i++ {
		copy(data[2*i:2*i+2], b[4*i+2:4*i+4])
	}
	return data
}

// Remove48Padding removes the 48 zero padding from the byte slice.
func Remove48Padding(b []byte) []byte {

	if len(b) != 64 {
		panic("input length must be 64")
	}

	data := make([]byte, 16)
	copy(data[:], b[48:])
	return data
}

// WriteInt64On64Bytes writes an int64 in big-endian form on 64 bytes using the
// koalabear formatting.
func WriteInt64On64Bytes(w io.Writer, x int64) (int64, error) {
	xBytes := [8]byte{}

	// Convert the int64 to its 8-byte representation
	binary.BigEndian.PutUint64(xBytes[:], uint64(x))

	// We copy every 2 bytes from tmp into res, left padded by 2 zero-bytes.
	res := LeftPadded48Zeros(LeftPadded(xBytes[:]))

	n, err := w.Write(res[:])
	if err != nil {
		return int64(n), fmt.Errorf("could not write 64 bytes into Writer : %w", err)
	}
	return int64(n), nil
}

// / WriteInt64On32Bytes writes an int64 in big-endian form on 32 bytes using the
// koalabear formatting.
func WriteInt64On32Bytes(w io.Writer, x int64) (int64, error) {
	xBytes := [8]byte{}
	// Convert the int64 to its 8-byte representation
	binary.BigEndian.PutUint64(xBytes[:], uint64(x))

	w.Write(make([]byte, 32-len(xBytes)))

	n, err := w.Write(xBytes[:])
	if err != nil {
		return int64(n), fmt.Errorf("could not write 32 bytes into Writer : %w", err)
	}
	return int64(n), nil
}

func ReadInt64On64Bytes(r io.Reader) (x, n_ int64, err error) {
	var buf [64]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return 0, int64(n), fmt.Errorf("could not read 64 bytes: %w", err)
	}

	if n != 64 {
		return 0, int64(n), fmt.Errorf("could not read 64 bytes: read %v", n)
	}

	// De-interleave the data from the 64-byte buffer into an 8-byte buffer
	data := RemovePadding(Remove48Padding(buf[:]))

	// Convert the 8 data bytes back to a uint64
	xU64 := binary.BigEndian.Uint64(data[:])

	if n < 0 {
		panic("invalid n, should never be negative")
	}
	xU64 &= 0x7fffffffffffffff  // TODO delete this if negative numbers are allowed
	return int64(xU64), 64, err // #nosec G115 -- above line precludes overflowing
}

func ReadInt64On32Bytes(r io.Reader) (x, n_ int64, err error) {
	var buf [32]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return 0, int64(n), fmt.Errorf("could not read 32 bytes: %w", err)
	}

	var bi big.Int
	bi.SetBytes(buf[:])

	if !bi.IsInt64() {
		return 0, int64(n), fmt.Errorf("could not read 32 bytes, had %v: %w", bi, err)
	}

	return bi.Int64(), 32, nil
}

// Big int are assumed to fit on 64 bytes and are written as a single
// block of 64 bytes in bigendian form (i.e, zero-padded on the left)
func WriteBigIntOn64Bytes(w io.Writer, b *big.Int) (int64, error) {
	balanceBig := b.FillBytes(make([]byte, 32))
	balanceBigPadded := LeftPadded(balanceBig)
	n, err := w.Write(balanceBigPadded)
	return int64(n), err
}

// WriteBigIntOn32Bytes writes a big-integer in normal big-endian form and fits
// it on 32 bytes.
func WriteBigIntOn32Bytes(w io.Writer, b *big.Int) (int64, error) {
	balanceBig := b.FillBytes(make([]byte, 32))
	n, err := w.Write(balanceBig)
	return int64(n), err
}

// ReadBigIntOn32Bytes reads a big-integer in normal big-endian form and fits
// it on 32 bytes.
func ReadBigIntOn32Bytes(r io.Reader) (*big.Int, error) {
	buf := [32]byte{}
	_, err := r.Read(buf[:])
	if err != nil {
		return nil, fmt.Errorf("reading big int, could not 32 bytes from reader: %v", err)
	}
	return new(big.Int).SetBytes(buf[:]), nil
}

// Read a bigint from a reader assuming it takes 64 bytes
func ReadBigIntOn64Bytes(r io.Reader) (*big.Int, error) {
	buf := [64]byte{}
	_, err := r.Read(buf[:])
	if err != nil {
		return nil, fmt.Errorf("reading big int, could not 64 bytes from reader: %v", err)
	}

	res := RemovePadding(buf[:])

	bi := new(big.Int).SetBytes(res[:])
	if bi.IsUint64() && bi.Uint64() == 0 {
		// there is an edge-case here that breaks the test. Namely, giving 64
		// zero bytes to a big.Int or calling NewInt with `0` do not produce the
		// same instances although they are both equivalent. This case handling
		// aims at preventing this.
		bi = big.NewInt(0)
	}

	return bi, nil
}
