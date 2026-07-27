package backend

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dynamicFeeTxHex is the single EIP-1559 transaction in
// testdata/stateless_input_payload0.json (chain id 59144).
const dynamicFeeTxHex = "02f86882e7088001843b9aca008252089422222222222222222222222222222222222222228080c080a0b09c2773ac7819dcf8cea66d878d5a41ebc04079162d4f78b7e8add87caa81a8a002cb3f20eb686af3b082e97d96a9fd7f68fabb9e927885a29738824b1d2dcd29"

// wantDynamicFeePubKey is the uncompressed SEC1 public key the reference
// encoder recovers for dynamicFeeTxHex (rollup_spec stateless_input.py).
const wantDynamicFeePubKey = "044f355bdcb7cc0af728ef3cceb9615d90684bb5b2ca5f859ab0f0b704075871aa385b6b1b8ead809ca67454d9683fcf2ba03456d6fe2c4abe2b07f0fbdbb2f1c1"

// TestRecoverPublicKey_DynamicFee pins recovery of the golden fixture's
// EIP-1559 transaction to the reference encoder's output.
func TestRecoverPublicKey_DynamicFee(t *testing.T) {
	raw, err := hexToBytes(dynamicFeeTxHex)
	require.NoError(t, err)

	pub, err := recoverPublicKey(raw, big.NewInt(59144))
	require.NoError(t, err)

	want, err := hexToBytes(wantDynamicFeePubKey)
	require.NoError(t, err)
	assert.Equal(t, want, pub)
}

// TestRecoverPublicKey_RoundTrip signs a transaction of each supported type
// with a known key, then asserts recoverPublicKey returns that key's
// uncompressed SEC1 encoding. This exercises the per-type signing hash and the
// v-to-recovery-id normalization.
func TestRecoverPublicKey_RoundTrip(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	wantPub := crypto.FromECDSAPub(&key.PublicKey) // 0x04 || x || y
	require.Len(t, wantPub, 65)

	chainID := big.NewInt(59144)
	to := crypto.PubkeyToAddress(key.PublicKey)

	cases := []struct {
		name   string
		signer types.Signer
		tx     *types.Transaction
	}{
		{
			name:   "LegacyUnprotected",
			signer: types.HomesteadSigner{},
			tx: types.NewTx(&types.LegacyTx{
				Nonce: 0, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(1),
			}),
		},
		{
			name:   "LegacyEIP155",
			signer: types.NewEIP155Signer(chainID),
			tx: types.NewTx(&types.LegacyTx{
				Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(2),
			}),
		},
		{
			name:   "AccessList2930",
			signer: types.LatestSignerForChainID(chainID),
			tx: types.NewTx(&types.AccessListTx{
				ChainID: chainID, Nonce: 2, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(3),
			}),
		},
		{
			name:   "DynamicFee1559",
			signer: types.LatestSignerForChainID(chainID),
			tx: types.NewTx(&types.DynamicFeeTx{
				ChainID: chainID, Nonce: 3, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
				Gas: 21000, To: &to, Value: big.NewInt(4),
			}),
		},
		{
			name:   "Blob4844",
			signer: types.LatestSignerForChainID(chainID),
			tx: types.NewTx(&types.BlobTx{
				ChainID:    uint256.MustFromBig(chainID),
				Nonce:      4,
				GasTipCap:  uint256.NewInt(1),
				GasFeeCap:  uint256.NewInt(2),
				Gas:        21000,
				To:         to,
				Value:      uint256.NewInt(5),
				BlobFeeCap: uint256.NewInt(1),
				BlobHashes: []common.Hash{{0x01}},
			}),
		},
		{
			name:   "SetCode7702",
			signer: types.LatestSignerForChainID(chainID),
			tx: types.NewTx(&types.SetCodeTx{
				ChainID:   uint256.MustFromBig(chainID),
				Nonce:     5,
				GasTipCap: uint256.NewInt(1),
				GasFeeCap: uint256.NewInt(2),
				Gas:       21000,
				To:        to,
				Value:     uint256.NewInt(6),
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signed, err := types.SignTx(tc.tx, tc.signer, key)
			require.NoError(t, err)
			raw, err := signed.MarshalBinary()
			require.NoError(t, err)

			pub, err := recoverPublicKey(raw, chainID)
			require.NoError(t, err)
			assert.Equal(t, wantPub, pub)
		})
	}
}

// TestRecoverPublicKey_InvalidBytes verifies that undecodable transaction bytes
// return an error rather than panicking.
func TestRecoverPublicKey_InvalidBytes(t *testing.T) {
	_, err := recoverPublicKey([]byte{0xff, 0x00, 0x01}, big.NewInt(59144))
	require.Error(t, err)
}

// TestRecoverPublicKey_TypedTxForeignChainID verifies that a typed transaction
// is recovered under its OWN embedded chain id, matching the reference
// (_signature_recovery_parameters does not validate a typed transaction's
// chain id against the payload chain id; signing_hash_1559 uses tx.chain_id).
func TestRecoverPublicKey_TypedTxForeignChainID(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	wantPub := crypto.FromECDSAPub(&key.PublicKey)

	txChainID := big.NewInt(999) // differs from the payload chain id below
	to := crypto.PubkeyToAddress(key.PublicKey)
	signed, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID: txChainID, Nonce: 0, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
		Gas: 21000, To: &to, Value: big.NewInt(1),
	}), types.LatestSignerForChainID(txChainID), key)
	require.NoError(t, err)
	raw, err := signed.MarshalBinary()
	require.NoError(t, err)

	pub, err := recoverPublicKey(raw, big.NewInt(59144))
	require.NoError(t, err)
	assert.Equal(t, wantPub, pub)
}

// TestRecoverPublicKey_TypedTxZeroChainID verifies that a typed transaction
// with an embedded chain id of zero is rejected with an error (recovery needs
// the tx's own chain id for the signing hash; zero is invalid). The tx is
// hand-crafted in RLP because go-ethereum's signing APIs never produce one.
func TestRecoverPublicKey_TypedTxZeroChainID(t *testing.T) {
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	// EIP-2930 payload: [chainId=0, nonce, gasPrice, gas, to, value, data,
	// accessList, yParity, r, s], prefixed with the 0x01 type byte.
	payload, err := rlp.EncodeToBytes([]any{
		big.NewInt(0), uint64(0), big.NewInt(1), uint64(21000), to.Bytes(),
		big.NewInt(1), []byte{}, []any{}, big.NewInt(0),
		big.NewInt(1), big.NewInt(1),
	})
	require.NoError(t, err)

	_, err = recoverPublicKey(append([]byte{0x01}, payload...), big.NewInt(59144))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain id")
}

// TestRecoverPublicKey_LegacyBadV verifies that a protected legacy transaction
// whose v matches neither 27/28 nor 35+2*chainId{,+1} is rejected, matching
// the reference's "bad v" check.
func TestRecoverPublicKey_LegacyBadV(t *testing.T) {
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	// A legacy tx with v=30: protected (not 27/28), but 30-35-2*59144 is not a
	// valid 0/1 recovery id for chain 59144.
	badV, err := types.NewTx(&types.LegacyTx{
		Nonce: 0, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(1),
		V: big.NewInt(30), R: big.NewInt(1), S: big.NewInt(1),
	}).MarshalBinary()
	require.NoError(t, err)

	_, err = recoverPublicKey(badV, big.NewInt(59144))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recovery id")
}

// TestRecoverPublicKey_HighS verifies that a malleable high-s signature is
// rejected, matching the reference's "bad s" check (s must be <= N/2).
func TestRecoverPublicKey_HighS(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainID := big.NewInt(59144)
	signer := types.LatestSignerForChainID(chainID)
	to := crypto.PubkeyToAddress(key.PublicKey)
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID: chainID, Nonce: 0, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
		Gas: 21000, To: &to, Value: big.NewInt(1),
	})
	signed, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	// Flip to the malleable twin: s' = N - s, recovery id inverted. The twin
	// is a valid ECDSA signature for the same key but violates the low-s rule.
	v, r, s := signed.RawSignatureValues()
	n := crypto.S256().Params().N
	sig := make([]byte, 65)
	r.FillBytes(sig[0:32])
	new(big.Int).Sub(n, s).FillBytes(sig[32:64])
	sig[64] = 1 - byte(v.Uint64())
	malleable, err := tx.WithSignature(signer, sig)
	require.NoError(t, err)
	raw, err := malleable.MarshalBinary()
	require.NoError(t, err)

	_, err = recoverPublicKey(raw, chainID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}
