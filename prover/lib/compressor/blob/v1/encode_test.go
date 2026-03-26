package v1_test

import (
	"bytes"
	"compress/zlib"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/backend/dataavailability"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDictPath = "../../compressor_dict.bin"

func TestEncodeDecodeTx(t *testing.T) {

	var (
		privKey, _ = ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		chainID    = big.NewInt(51)
		ldnSigner  = types.NewLondonSigner(chainID)
		prgSigner  = types.NewPragueSigner(chainID)
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
				AccessList: types.AccessList{
					{
						Address:     common.Address{1},
						StorageKeys: nil,
					},
				},
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
		{
			Name: "set-code",
			Tx: &types.SetCodeTx{
				ChainID:   uint256.MustFromBig(chainID),
				GasTipCap: uint256.NewInt(10002),
				GasFeeCap: uint256.NewInt(33333),
				Gas:       7000007,
				To:        common.Address{12, 24},
				Value:     uint256.NewInt(66666666),
				Nonce:     3,
				Data:      hexutil.MustDecode("0xdeadbeafbeefbeef12345689"),
				AuthList: []types.SetCodeAuthorization{{
					ChainID: *uint256.MustFromBig(chainID),
					Address: common.Address{1, 2},
					Nonce:   3,
					V:       4,
					R:       uint256.Int{5},
					S:       uint256.Int{6},
				}},
			},
		},
	}

	for _, tc := range testTx {

		t.Run(tc.Name, func(t *testing.T) {

			var signer types.Signer

			switch tc.Tx.(type) {
			case *types.SetCodeTx:
				signer = prgSigner
			default:
				signer = ldnSigner

			}
			tx := types.MustSignNewTx(privKey, signer, tc.Tx)
			buf := new(bytes.Buffer)
			addr := new(common.Address)

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

			var response = &dataavailability.Response{}
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

func TestEncodeBlockWithType4Tx(t *testing.T) {
	// Create a blob with a single block, containing an EIP-7702, Set Code transaction.

	// blockRlpBase64 from https://explorer.devnet.linea.build/block/9128081
	// compressed using zlib
	const (
		blockRlpCompressedBase64 = "eNr6yXz6J1PjgiXaYeZsl2a0dIqk9Zwp6VrjxnNbat9dhon/VNM++EbdZ1gge8b3xb3jsVWrW7embztzReqykKv0lK4S4Q8Lnf46XJnsPoUBC1jAbGSeHnZvvu2FBxqfjvi5+DxSz79VunX6ItfHYWdOsa93XmA7/zk7R00l966idw8Fzk/LdXT/0X3HQcGh1b1G4tjE+bMXMHanRh7yiFh3emvmHE822asLZq41yan3537kvs9p8aTt3DsZGUY6aGju9pjYUm46haGZUaemJWPdi+0LGBkYSg0YfLwdGBhjN2OLGTnXf6EOMmvTK8REzj7Jiz9sL2W0yafbX+Bz6SZOD/EqhQ6YSvYFYS/kxaXPhC773+z6bNKBH3nRHg+kZ+asPcCYpL/1cfIWxYaGBYScuODxhiNOM/7IiMz6/eXEzPydKurrHJ+kzJ7ss2TqTOmKoB2hPxmddlxl+XGp6TkzX6vIl2wWBgjZzL5QYcrhFQ+353xYH7M3s3GBySvpOUk2+3waWho1/8gc+BH3I6bpOfMUD+8H2ZGy1afKRFwe+Jmuua82q6KYn3HBDHnL/RwZM74uMvvGu9rKN2yh96ENzW+CLt87qtyr13NQbIHkdktLzs5bnWv2il1+Pbki+f6L5BL/hwLSUy9XOWpfXsbGOD9a5kyqk7RFw5pNRQ4lG5fnvJ587Xl1yG2lH+s3TmRacmxB9L6D+eyi9jVPamdufSsgpyXF3Xfr1AQF5/kyWx9PPW3z/Edmc+fr6y2sYcwWTUEcU5Ra0j4dFz39hvXe6l8HHM6ZSV4/z53S0Mx4TmtB7WSNe24fL7yWklz5333uK/fY0vMn1M7kyO/5tub2Spe/Kgvkbz569PU596zK8zud7N5N1vPQ2LIroWBF9hrO6Mrmf3OXHTgACAAA///o8zd2"
		dictPath                 = "../../dict/25-04-21.bin"
	)
	var block types.Block
	blockBytes := zlibDecompressBase64EncodedBytes(t, blockRlpCompressedBase64)
	require.NoError(t, rlp.DecodeBytes(blockBytes, &block))

	bm, err := v1.NewBlobMaker(127000, dictPath)
	require.NoError(t, err)

	ok, err := bm.Write(blockBytes, false)
	require.NoError(t, err)
	require.True(t, ok)

	blobBytes := bm.Bytes()

	decompressR, err := v1.DecompressBlob(blobBytes, dictionary.NewStore(dictPath))
	require.NoError(t, err)

	require.Equal(t, 1, len(decompressR.Blocks))

	decodedBlock, err := v1.DecodeBlockFromUncompressed(bytes.NewReader(decompressR.Blocks[0]))
	require.NoError(t, err)

	blockBack := decodedBlock.ToStd()
	require.Equal(t, 2, len(blockBack.Transactions()))

	tx := block.Transactions()[0]
	txBack := blockBack.Transactions()[0]
	require.Equal(t, types.SetCodeTxType, int(tx.Type()))
	require.Equal(t, types.SetCodeTxType, int(txBack.Type()))
	require.Equal(t, tx.To(), txBack.To())
	require.Equal(t, tx.Nonce(), txBack.Nonce())
	require.Equal(t, tx.Value(), txBack.Value())
	require.Equal(t, tx.ChainId(), txBack.ChainId())
	require.Equal(t, tx.Data(), txBack.Data())
	require.Equal(t, tx.GasTipCap(), txBack.GasTipCap())
	require.Equal(t, tx.GasFeeCap(), txBack.GasFeeCap())
	require.Equal(t, tx.Gas(), txBack.Gas())
	require.Equal(t, tx.AccessList(), txBack.AccessList())
}

func zlibDecompressBase64EncodedBytes(t *testing.T, b64 string) []byte {
	compressed, err := base64.StdEncoding.DecodeString(b64)
	require.NoError(t, err)

	zReader, err := zlib.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)

	var bb bytes.Buffer
	readBuf := make([]byte, 1024)

	for n := len(readBuf); n == len(readBuf); {
		n, err = zReader.Read(readBuf)
		if err != io.EOF {
			require.NoError(t, err)
		}
		bb.Write(readBuf[:n])
	}

	return bb.Bytes()
}
