package ethereum

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// commonly used as a dummy "to" address
	TEST_ADDRESS   = common.HexToAddress("0xfeeddeadbeeffeeddeadbeeffeeddead01245678")
	TEST_ADDRESS_A = common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	TEST_HASH_F    = common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	TEST_HASH_A    = common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	CHAIN_ID       = big.NewInt(51)
)

// Test TxData
var testTxDatas = []types.TxData{
	// legacy transaction with dummy value
	&types.LegacyTx{
		Nonce:    2,
		GasPrice: big.NewInt(int64(123543135)),
		Gas:      4531112,
		To:       &TEST_ADDRESS,
		Value:    big.NewInt(int64(845315452)),
		Data:     common.Hex2Bytes("0xdeed8745a20f"),
	},
	// the same legacy transaction but we will
	// sign it with another signer.
	&types.LegacyTx{
		Nonce:    2,
		GasPrice: big.NewInt(int64(123543135)),
		Gas:      4531112,
		To:       &TEST_ADDRESS,
		Value:    big.NewInt(int64(845315452)),
		Data:     common.Hex2Bytes("0xdeed8745a20f"),
	},
	// access-list transaction
	&types.AccessListTx{
		ChainID:  CHAIN_ID,
		Nonce:    2,
		GasPrice: big.NewInt(int64(123543135)),
		Gas:      4531112,
		To:       &TEST_ADDRESS,
		Value:    big.NewInt(int64(845315452)),
		Data:     common.Hex2Bytes("0xdeed8745a20f"),
		AccessList: types.AccessList{
			types.AccessTuple{Address: TEST_ADDRESS_A, StorageKeys: []common.Hash{TEST_HASH_A, TEST_HASH_F}},
		},
	},
	// dynamic-fee transaction
	&types.DynamicFeeTx{
		ChainID:   CHAIN_ID,
		Nonce:     2,
		GasTipCap: big.NewInt(int64(123543135)),
		GasFeeCap: big.NewInt(int64(112121212)),
		Gas:       4531112,
		To:        &TEST_ADDRESS,
		Value:     big.NewInt(int64(845315452)),
		Data:      common.Hex2Bytes("0xdeed8745a20f"),
		AccessList: types.AccessList{
			types.AccessTuple{Address: TEST_ADDRESS_A, StorageKeys: []common.Hash{TEST_HASH_A, TEST_HASH_F}},
		},
	},
}

// Test signers
var testSigners = []types.Signer{
	types.HomesteadSigner{},
	types.NewEIP155Signer(CHAIN_ID),
	types.NewEIP2930Signer(CHAIN_ID),
	types.NewLondonSigner(CHAIN_ID),
}

// Test the consistency with ethereum signatures for unprotected legacy tx.
func TestTransactionSigning(t *testing.T) {

	for i := range testSigners {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {
			// Fetch the right signer from the test vectors
			txData := testTxDatas[i]
			signer := testSigners[i]
			tx := types.NewTx(txData)

			// Sign the transaction
			txHash := signer.Hash(tx)
			privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
			require.NoErrorf(t, err, "for signature #%v", i)
			sig, err := crypto.Sign(txHash[:], privKey)
			require.NoErrorf(t, err, "for signature #%v", i)
			tx, err = tx.WithSignature(signer, sig)
			require.NoErrorf(t, err, "for signature #%v", i)
			from, err := signer.Sender(tx)
			require.NoErrorf(t, err, "for signature #%v", i)

			// Check the transaction hash and that the v value is correct
			jsonSig := GetJsonSignature(tx)
			rlp := EncodeTxForSigning(tx)

			t.Logf("transaction rlp = 0x%x\n", rlp)

			assert.True(t, jsonSig.V == "0x1b" || jsonSig.V == "0x1c", "V should be either 27 or 28 but was %v", jsonSig.V)
			assert.Equal(t, txHash, crypto.Keccak256Hash(rlp), "Mismatch of the tx hash")

			recovered := ecRecover(rlp, jsonSig)
			assert.Equal(t, from.Hex(), recovered.Hex(), "Mismatch of the recovered address")

			// Simulates the decoding of the transaction
			decodedTxData, err := DecodeTxFromBytes(bytes.NewReader(rlp))
			require.NoError(t, err)

			decodedTx := types.NewTx(decodedTxData)

			assert.Equal(t, tx.To(), decodedTx.To())
			assert.Equal(t, tx.Nonce(), decodedTx.Nonce())
			assert.Equal(t, tx.Data(), decodedTx.Data())
			assert.Equal(t, tx.Value(), decodedTx.Value())
			assert.Equal(t, tx.Cost(), decodedTx.Cost())
		})
	}

}

// ecRecovebackend/ethereum/signature.gor the signature from properly encoded transaction
func ecRecover(msg []byte, sig Signature) common.Address {

	// Ethereum signatures are signed using Keccak
	msgHash := crypto.Keccak256Hash(msg)
	pubKey, _, err := RecoverPublicKey(msgHash, sig)
	if err != nil {
		// Can happen if the signature is incorrect. Since we use this function
		// only for testing purpose, it is fine to panic.
		panic(err)
	}

	res := common.Address{}
	res.SetBytes(crypto.Keccak256(pubKey[:])[12:])
	return res
}

// TestRlpDecodeWithSignature verifies that RlpDecodeWithSignature correctly
// decodes a hex-encoded signed RLP transaction and recovers the signature.
func TestRlpDecodeWithSignature(t *testing.T) {

	for i := range testSigners {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {
			// Create and sign a transaction
			txData := testTxDatas[i]
			signer := testSigners[i]
			tx := types.NewTx(txData)

			// Sign the transaction
			privKey, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
			require.NoError(t, err)
			txHash := signer.Hash(tx)
			sig, err := crypto.Sign(txHash[:], privKey)
			require.NoError(t, err)
			signedTx, err := tx.WithSignature(signer, sig)
			require.NoError(t, err)

			// Get the original signature and sender
			origV, origR, origS := signedTx.RawSignatureValues()
			origFrom, err := signer.Sender(signedTx)
			require.NoError(t, err)
			t.Logf("Original: from=%s, V=%s, R=%s, S=%s",
				origFrom.Hex(), origV.String(), origR.String(), origS.String())

			// Encode to hex string using MarshalBinary (includes signature)
			signedRlpBytes, err := signedTx.MarshalBinary()
			require.NoError(t, err)
			hexEncodedTx := "0x" + common.Bytes2Hex(signedRlpBytes)
			t.Logf("Hex encoded signed tx: %s", hexEncodedTx)

			// Decode using RlpDecodeWithSignature
			decodedTx, err := RlpDecodeWithSignature(hexEncodedTx)
			require.NoError(t, err)

			// Verify the decoded transaction has the same signature
			decodedV, decodedR, decodedS := decodedTx.RawSignatureValues()
			assert.Equal(t, origV.String(), decodedV.String(), "V mismatch")
			assert.Equal(t, origR.String(), decodedR.String(), "R mismatch")
			assert.Equal(t, origS.String(), decodedS.String(), "S mismatch")

			// Verify we can recover the same sender
			decodedFrom, err := signer.Sender(decodedTx)
			require.NoError(t, err)
			assert.Equal(t, origFrom, decodedFrom, "Sender address mismatch")

			// Verify transaction fields match
			assert.Equal(t, signedTx.Hash(), decodedTx.Hash(), "Transaction hash mismatch")
			assert.Equal(t, signedTx.Nonce(), decodedTx.Nonce(), "Nonce mismatch")
			assert.Equal(t, signedTx.Gas(), decodedTx.Gas(), "Gas mismatch")
			assert.Equal(t, signedTx.Value(), decodedTx.Value(), "Value mismatch")
			assert.Equal(t, signedTx.Data(), decodedTx.Data(), "Data mismatch")
			assert.Equal(t, signedTx.To(), decodedTx.To(), "To address mismatch")

			// Verify GetJsonSignature works on decoded transaction
			origJsonSig := GetJsonSignature(signedTx)
			decodedJsonSig := GetJsonSignature(decodedTx)
			assert.Equal(t, origJsonSig, decodedJsonSig, "JSON signature mismatch")

			// Verify GetTxHash works on decoded transaction
			origTxHash := GetTxHash(signedTx)
			decodedTxHash := GetTxHash(decodedTx)
			assert.Equal(t, origTxHash, decodedTxHash, "TxHash mismatch")

			t.Logf("✓ RlpDecodeWithSignature correctly decoded tx with signature")
		})
	}
}

// TestRlpDecodeWithSignature_InvalidInput tests error handling for invalid inputs
func TestRlpDecodeWithSignature_InvalidInput(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid hex",
			input:       "not-hex",
			expectError: true,
		},
		{
			name:        "valid hex but invalid RLP",
			input:       "0xdeadbeef",
			expectError: true,
		},
		{
			name:        "unsigned RLP (missing signature)",
			input:       "0xe50284075d1e5f834523a894feeddeadbeeffeeddeadbeeffeeddead012456788432627d7c80",
			expectError: true, // unsigned RLP cannot be decoded as signed tx
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RlpDecodeWithSignature(tc.input)
			if tc.expectError {
				assert.Error(t, err, "expected error for input: %s", tc.input)
				t.Logf("Got expected error: %v", err)
			} else {
				assert.NoError(t, err, "unexpected error for input: %s", tc.input)
			}
		})
	}
}
