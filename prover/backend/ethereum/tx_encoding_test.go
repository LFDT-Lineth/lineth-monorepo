package ethereum

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyTxSigningPayloadRoundTrip(t *testing.T) {
	to := common.HexToAddress("0xada7ae13ed62337000e724b86615c71166bb09b2")
	sign := func(t *testing.T, signer types.Signer) *types.Transaction {
		key, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		require.NoError(t, err)
		signed, err := types.SignNewTx(key, signer, &types.LegacyTx{
			Nonce:    42,
			GasPrice: big.NewInt(1_000_000_000),
			Gas:      21000,
			To:       &to,
			Value:    big.NewInt(12345),
			Data:     []byte{0xde, 0xad, 0xbe, 0xef},
		})
		require.NoError(t, err)
		return signed
	}

	for _, tc := range []struct {
		name      string
		signer    types.Signer
		protected bool
	}{
		{"eip155", types.NewEIP155Signer(big.NewInt(59144)), true},
		{"homestead", types.HomesteadSigner{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := sign(t, tc.signer)
			require.Equal(t, tc.protected, original.Protected(), "unexpected fixture")

			encoded := EncodeTxForSigning(original)
			decoded, err := DecodeTxFromBytes(bytes.NewReader(encoded))
			require.NoError(t, err)

			assert.Equal(t, encoded, EncodeTxForSigning(types.NewTx(decoded)))
		})
	}
}
