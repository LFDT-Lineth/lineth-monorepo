package v0_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"testing"

	v0 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeTx(t *testing.T) {

	var (
		privKey, _ = ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		chainID    = big.NewInt(51)
		signer     = types.NewLondonSigner(chainID)
	)

	testTx := []struct {
		Name string
		Tx   types.TxData
	}{
		{
			Name: "contract-deployment-legacy",
			Tx: &types.LegacyTx{
				Nonce:    3,
				GasPrice: big.NewInt(10002),
				Gas:      7000007,
				To:       nil,
				Value:    big.NewInt(66666666),
				Data:     hexutil.MustDecode("0xdeadbeafbeefbeef12345689"),
			},
		},
		{
			Name: "contract-tx-legacy",
			Tx: &types.LegacyTx{
				Nonce:    3,
				GasPrice: big.NewInt(10002),
				Gas:      7000007,
				To:       &common.Address{12, 24},
				Value:    big.NewInt(66666666),
				Data:     hexutil.MustDecode("0xdeadbeafbeefbeef12345689"),
			},
		},
		{
			Name: "payment-legacy",
			Tx: &types.LegacyTx{
				Nonce:    3,
				GasPrice: big.NewInt(10002),
				Gas:      7000007,
				To:       &common.Address{12, 24},
				Value:    big.NewInt(66666666),
				Data:     nil,
			},
		},
		{
			Name: "payment-0x0-legacy",
			Tx: &types.LegacyTx{
				Nonce:    3,
				GasPrice: big.NewInt(10002),
				Gas:      7000007,
				To:       &common.Address{},
				Value:    big.NewInt(66666666),
				Data:     nil,
			},
		},
		{
			Name: "contract-deployment-dyn-fee",
			Tx: &types.DynamicFeeTx{
				Nonce:     3,
				GasTipCap: big.NewInt(10002),
				GasFeeCap: big.NewInt(33333),
				Gas:       7000007,
				To:        nil,
				Value:     big.NewInt(66666666),
				Data:      hexutil.MustDecode("0xdeadbeafbeefbeef12345689"),
				ChainID:   chainID,
			},
		},
		{
			Name: "contract-tx-dyn-fee",
			Tx: &types.DynamicFeeTx{
				Nonce:     3,
				GasTipCap: big.NewInt(10002),
				GasFeeCap: big.NewInt(33333),
				Gas:       7000007,
				To:        &common.Address{12, 24},
				Value:     big.NewInt(66666666),
				Data:      hexutil.MustDecode("0xdeadbeafbeefbeef12345689"),
				ChainID:   chainID,
			},
		},
		{
			Name: "payment-dyn-fee",
			Tx: &types.DynamicFeeTx{
				Nonce:     3,
				GasTipCap: big.NewInt(10002),
				GasFeeCap: big.NewInt(33333),
				Gas:       7000007,
				To:        &common.Address{12, 24},
				Value:     big.NewInt(66666666),
				Data:      nil,
				ChainID:   chainID,
			},
		},
		{
			Name: "payment-0x0-dyn-fee",
			Tx: &types.DynamicFeeTx{
				Nonce:     3,
				GasTipCap: big.NewInt(10002),
				GasFeeCap: big.NewInt(33333),
				Gas:       7000007,
				To:        &common.Address{},
				Value:     big.NewInt(66666666),
				Data:      nil,
				ChainID:   chainID,
			},
		},
	}

	for _, tc := range testTx {

		t.Run(tc.Name, func(t *testing.T) {

			var (
				tx   = types.MustSignNewTx(privKey, signer, tc.Tx)
				buf  = &bytes.Buffer{}
				addr = &common.Address{}
			)

			if err := v0.EncodeTxForCompression(tx, buf); err != nil {
				t.Fatalf("could not encode the transaction")
			}

			var (
				data = buf.Bytes()
				r    = bytes.NewReader(data)
			)

			txData2, err := v0.DecodeTxFromUncompressed(r, addr)

			if err != nil {
				t.Fatalf("could not deserialize the transaction err=%v", err.Error())
			}

			tx2 := types.NewTx(txData2)

			assert.Equal(t, tx.To(), tx2.To(), "field `to` mismatches")

		})

	}

}
