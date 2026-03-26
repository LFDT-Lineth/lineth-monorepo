package utils

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"

	bls12377fr "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// Encode the uint64 into an hexstring representing it as a u256 in bigendian form
func HexHashUint64(v ...uint64) string {
	buffer := bytes.Buffer{}
	var I big.Int
	for i := range v {
		I.SetUint64(v[i])
		bytes := I.Bytes()
		bytes = append(make([]byte, 32-len(bytes)), bytes...)
		buffer.Write(bytes)
	}
	res := crypto.Keccak256(buffer.Bytes())
	return hexutil.Encode(res)
}

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

// Apply the modulus of the BLS12-377 scalar field
func ApplyModulusBn254(b string) string {
	var f bn254fr.Element
	f.SetString(b)
	fbytes := f.Bytes()
	return hexutil.Encode(fbytes[:])
}

// Apply the modulus
func ApplyModulusBls12377(b string) string {
	var f bls12377fr.Element
	f.SetString(b)
	fbytes := f.Bytes()
	return hexutil.Encode(fbytes[:])
}

func HexDecodeString(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
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

// Parse one or more hex string into a byte array, hash it and return the result
// as an hexstring. If several hex string are passed, what is hashed is the
// concatenation of the strings and the hasher is implictly updated only once.
// The hash function is Keccak.
func HexHashHex(v ...string) string {
	buffer := bytes.Buffer{}
	for i := range v {
		decoded, err := hexutil.Decode(v[i])
		if err != nil {
			logrus.Errorf("could not decode `%v` from list `%v`, because `%v`. This can happen when"+
				" the state-manager option is activated but no zk-merkleProof were found", v[i], v, err)
		}
		buffer.Write(decoded)
	}
	res := crypto.Keccak256(buffer.Bytes())
	return hexutil.Encode(res)
}

// Concatenate hex strings
func HexConcat(v ...string) string {
	buffer := bytes.Buffer{}
	for i := range v {
		decoded := hexutil.MustDecode(v[i])
		buffer.Write(decoded)
	}
	return hexutil.Encode(buffer.Bytes())
}

// Format an integer as a big-endian uint256
func AsBigEndian32Bytes(x int) (res [32]byte) {
	new(big.Int).SetInt64(int64(x)).FillBytes(res[:])
	return res
}
