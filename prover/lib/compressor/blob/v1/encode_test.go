package v1_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/backend/blobdecompression"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDictPath = "../../compressor_dict.bin"

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
			Name: "contract-deployment-legacy-0x0",
			Tx: &types.LegacyTx{
				Nonce:    3,
				GasPrice: big.NewInt(10002),
				Gas:      7000007,
				To:       &common.Address{},
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
			Name: "contract-deployment-legacy",
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
			Name: "contract-tx-legacy",
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
			Name: "payment-legacy",
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
	}

	for _, tc := range testTx {

		t.Run(tc.Name, func(t *testing.T) {

			var (
				tx   = types.MustSignNewTx(privKey, signer, tc.Tx)
				buf  = &bytes.Buffer{}
				addr = &common.Address{}
			)

			if err := v1.EncodeTxForCompression(tx, buf); err != nil {
				t.Fatalf("could not encode the transaction")
			}

			var (
				data = buf.Bytes()
				r    = bytes.NewReader(data)
			)

			txData2, err := v1.DecodeTxFromUncompressed(r, addr)

			if err != nil {
				t.Fatalf("could not deserialize the transaction err=%v", err.Error())
			}

			tx2 := types.NewTx(txData2)

			assert.Equal(t, tx.To(), tx2.To(), "field `to` mismatches")

		})

	}

}

func TestEncodeDecodeFromResponse(t *testing.T) {

	var (
		testDir        = "../testdata/v1/prover-responses"
		testFiles, err = os.ReadDir(testDir)
	)

	if err != nil {
		t.Fatalf("can't read test files: %v", err)
	}

	for _, testFile := range testFiles {

		if testFile.IsDir() {
			continue
		}

		fName := testFile.Name()

		if fName == ".DS_Store" {
			continue
		}

		t.Run(fName, func(t *testing.T) {

			var (
				filePath = path.Join(testDir, fName)
				f, err   = os.Open(filePath)
			)

			require.NoErrorf(t, err, "could not open the test file path=%v err=%v", filePath, err)

			var response = &blobdecompression.Response{}
			if err = json.NewDecoder(f).Decode(response); err != nil {
				t.Fatalf("could not deserialize testfile path=%v err=%v", filePath, err)
			}

			data, err := base64.StdEncoding.DecodeString(response.CompressedData)
			if err != nil {
				t.Fatalf("could not deserialize the bsae64 decompression data err=%v", err)
			}

			batches, err := decompressBlob(data)
			if err != nil {
				t.Fatalf("could not decompress the blob: err=%v", err)
			}

			for batchI := range batches {
				for blockI := range batches[batchI] {

					r := bytes.NewReader(batches[batchI][blockI])
					blockData, err := v1.DecodeBlockFromUncompressed(r)

					if err != nil {
						t.Fatalf("could not decode block: %v", err)
					}

					for _, txData := range blockData.Txs {
						tx := types.NewTx(txData)
						if tx.To() == (&common.Address{}) {
							t.Fatalf("transaction's 'to' decoded as the zero address instead of nil")
						}
					}

				}
			}
		})

	}

}

func decompressBlob(b []byte) ([][][]byte, error) {

	// we should be able to hash the blob with MiMC with no errors;
	// this is a good indicator that the blob is valid.
	if len(b)%fr.Bytes != 0 {
		return nil, errors.New("invalid blob length; not a multiple of 32")
	}

	dict, err := os.ReadFile(testDictPath)
	if err != nil {
		return nil, fmt.Errorf("can't read dict: %w", err)
	}
	dictStore, err := dictionary.SingletonStore(dict, 1)
	if err != nil {
		return nil, err
	}
	r, err := v1.DecompressBlob(b, dictStore)
	if err != nil {
		return nil, fmt.Errorf("can't decompress blob: %w", err)
	}

	batches := make([][][]byte, len(r.Header.BatchSizes))
	for i, batchNbBytes := range r.Header.BatchSizes {
		batches[i] = make([][]byte, 0)
		batchLenYet := 0
		for batchLenYet < batchNbBytes {
			batches[i] = append(batches[i], r.Blocks[0])
			batchLenYet += len(r.Blocks[0])
			r.Blocks = r.Blocks[1:]
		}
		if batchLenYet != batchNbBytes {
			return nil, errors.New("invalid batch size")
		}
	}
	if len(r.Blocks) != 0 {
		return nil, errors.New("not all blocks were consumed")
	}

	return batches, nil
}
