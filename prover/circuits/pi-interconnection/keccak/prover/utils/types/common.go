package types

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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

func WriteInt64On32Bytes(w io.Writer, x int64) (int64, error) {
	res := [32]byte{}
	binary.BigEndian.PutUint64(res[24:], uint64(x))
	n, err := w.Write(res[:])
	if err != nil {
		return int64(n), fmt.Errorf("could not write 32 bytes into Writer : %w", err)
	}
	return int64(n), nil
}

func ReadInt64On32Bytes(r io.Reader) (x, n_ int64, err error) {
	// Read the first 24 bytes into the garbage
	padding := [24]byte{}
	n, err := r.Read(padding[:])
	if err != nil {
		return 0, int64(n), fmt.Errorf("could not read 32 bytes from buffer: %v", err)
	}
	// Then reads the following 8 bytes. We reuse the padding buffer to save an
	// allocation.
	n, err = r.Read(padding[:8])
	if err != nil {
		return 0, int64(n), fmt.Errorf("could not read 32 bytes from buffer: %v", err)
	}
	xU64 := binary.BigEndian.Uint64(padding[:8])
	if n < 0 {
		panic("we are only reading 8 bits so this should not overflow")
	}
	xU64 &= 0x7fffffffffffffff  // TODO delete this if negative numbers are allowed
	return int64(xU64), 32, err // #nosec G115 -- above line precludes overflowing
}

// Big int are assumed to fit on 32 bytes and are written as a single
// block of 32 bytes in bigendian form (i.e, zero-padded on the left)
func WriteBigIntOn32Bytes(w io.Writer, b *big.Int) (int64, error) {
	balanceBig := b.FillBytes(make([]byte, 32))
	n, err := w.Write(balanceBig)
	return int64(n), err
}

// Read a bigint from a reader assuming it takes 32 bytes
func ReadBigIntOn32Bytes(r io.Reader) (*big.Int, error) {
	res := [32]byte{}
	_, err := r.Read(res[:])
	if err != nil {
		return nil, fmt.Errorf("reading big int, could not 32 bytes from reader: %v", err)
	}
	bi := new(big.Int).SetBytes(res[:])
	if bi.IsUint64() && bi.Uint64() == 0 {
		// there is an edge-case here that breaks the test. Namely, giving 32
		// zero bytes to a big.Int or calling NewInt with `0` do not produce the
		// same instances although they are both equivalent. This case handling
		// aims at preventing this.
		bi = big.NewInt(0)
	}

	return bi, nil
}
