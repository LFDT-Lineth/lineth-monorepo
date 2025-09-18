package ethereum

import (
	"bytes"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// Size of an ECDSA ethereum signature
	SIGNATURE_SIZE_BYTES = 65
)

// The value of a signature
type Signature struct {
	R string `json:"r"`
	S string `json:"s"`
	V string `json:"v"`
}

// Returns the sender of a transaction
func GetFrom(tx *ethtypes.Transaction) types.EthAddress {
	from, err := GetSigner(tx).Sender(tx)
	if err != nil {
		v, r, s := tx.RawSignatureValues()
		utils.Panic(
			"sender address recovery failed: tx.chainID=%v, tx.signature=[v=%v, r=%v, s=%v], tx.details=%++v, error=%v",
			tx.ChainId(), v.String(), r.String(), s.String(), tx, err.Error(),
		)
	}
	return types.EthAddress(from)
}

// Returns the signature in json and the sender of the transaction
// Signature in JSONable format and from as an hex string
func GetJsonSignature(tx *ethtypes.Transaction) Signature {

	// Depending on the type of transaction and the chainID, we may need
	// to update V in a specific way.
	v, r, s := tx.RawSignatureValues()
	V := new(big.Int)
	V.Set(v)

	switch tx.Type() {
	case ethtypes.LegacyTxType:
		// Otherwise, it's just a homestead and we just return V
		if tx.Protected() { // use directly, the tx's chainID
			chainIdMul := new(big.Int).Mul(tx.ChainId(), big.NewInt(2))
			V.Sub(V, chainIdMul)
			V.Sub(V, big.NewInt(8))
		}
	case ethtypes.AccessListTxType, ethtypes.DynamicFeeTxType, ethtypes.SetCodeTxType:
		// AL txs are defined to use 0 and 1 as their recovery
		// id, add 27 to become equivalent to unprotected Homestead signatures.
		V.Add(V, big.NewInt(27))
	default:
		utils.Panic("Unknown transaction type")
	}

	// If the above is correct, then we have the following guarantee with is a
	// sinequanone solution for the ecrecover to work.
	if V.Uint64() != 27 && V.Uint64() != 28 {
		utils.Panic("V should be `27` or `28` for it to work. Was %v", V.Uint64())
	}

	// Set the result in json
	return Signature{
		V: hexutil.Encode(V.Bytes()),
		R: hexutil.Encode(common.LeftPadBytes(r.Bytes(), 32)),
		S: hexutil.Encode(common.LeftPadBytes(s.Bytes(), 32)),
	}
}

// RecoverPublicKey returns the public key from the signature and the msg hash
// the signature should be "cleaned" before calling this function.
func RecoverPublicKey(msgHash [32]byte, sig Signature) (pubKey [64]byte, encodedSig [65]byte, err error) {

	/*
		For ecrecover, we must encode the signature as follows

			bytes 0..32 must be R
			bytes 32..64 must be S
			bytes 0..1 must be V (27|28)
	*/
	r := hexutil.MustDecode(sig.R)
	s := hexutil.MustDecode(sig.S)
	v := hexutil.MustDecode(sig.V)

	// Sanity-check the value of `v`
	if len(v) != 1 || (v[0] != 27 && v[0] != 28) {
		utils.Panic("v should be either 27 or 28 as single byte. Found %v", v)
	}

	// Sanity-check the dimensions of r, s
	if len(r) > 32 || len(s) > 32 {
		utils.Panic("r and s should have size 32 but their respective sizes are %v %v", len(r), len(s))
	}

	// It can happen that the signature bytes use less than 32 bytes
	r = common.LeftPadBytes(r, 32)
	s = common.LeftPadBytes(s, 32)

	var w bytes.Buffer
	w.Write(r[:32])
	w.Write(s[:32])
	w.WriteByte(v[0] - 27) // Write v as a single byte
	copy(encodedSig[:], w.Bytes())
	pubkey_, err := crypto.Ecrecover(msgHash[:], encodedSig[:])
	copy(pubKey[:], pubkey_[1:])
	return pubKey, encodedSig, err
}
