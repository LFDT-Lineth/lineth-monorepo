package utils

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/crypto/sha3"
)

func FmtInt32Bytes(v int) [32]byte {
	var res [32]byte
	b := big.NewInt(int64(v)).Bytes()
	copy(res[32-len(b):], b)
	return res
}

func FmtUint32Bytes(v uint) [32]byte {
	var res [32]byte
	var i big.Int
	i.SetUint64(uint64(v))
	b := i.Bytes()
	copy(res[32-len(b):], b)
	return res
}

// Format an integer as a 32 bytes hex string
func FmtIntHex32Bytes(v int) string {
	bytes := FmtInt32Bytes(v)
	return hexutil.Encode(bytes[:])
}

func HexDecodeString(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
}

func HexMustDecodeString(s string) []byte {
	b, err := HexDecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func HexEncodeToString(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}

// Compute the keccak of a stream of bytes. Returns the hex string.
func KeccakHash(stream []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(stream)
	return h.Sum(nil)
}

// Format an integer as a big-endian uint256
func AsBigEndian32Bytes(x int) (res [32]byte) {
	new(big.Int).SetInt64(int64(x)).FillBytes(res[:])
	return res
}
