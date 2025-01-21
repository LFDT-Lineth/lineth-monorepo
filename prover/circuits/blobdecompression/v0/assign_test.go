package v0_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	v0 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v0"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1/test_utils"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/circuits/blobdecompression"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0/compress/lzss"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestBlobV0(t *testing.T) {
	dict := lzss.AugmentDict(test_utils.GetDict(t))
	dictStore, err := dictionary.SingletonStore(dict, 0)
	assert.NoError(t, err)

	resp, blobBytes := mustGetTestCompressedData(t, dictStore)
	circ := v0.Allocate(dict)

	logrus.Infof("Building the constraint system")

	logrus.Infof("Creating an assignment")

	var (
		x [32]byte
		y fr381.Element
	)

	b, err := utils.HexDecodeString(resp.ExpectedX)
	assert.NoError(t, err)
	copy(x[:], b)

	b, err = utils.HexDecodeString(resp.ExpectedY)
	assert.NoError(t, err)
	y.SetBytes(b)

	givenSnarkHash, err := utils.HexDecodeString(resp.SnarkHash)
	assert.NoError(t, err)

	a, _, snarkHash, err := blobdecompression.Assign(blobBytes, dictStore, true, x, y)
	assert.NoError(t, err)
	_, ok := a.(*v0.Circuit)
	assert.True(t, ok)

	assert.Equal(t, snarkHash[:], givenSnarkHash)

	blobPackedBytes := a.(*v0.Circuit).BlobPackedBytes
	assert.Equal(t, len(blobPackedBytes), len(circ.BlobPackedBytes))

	logrus.Infof("Calling the solver")

	err = test.IsSolved(&circ, a, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
}

// mustGetTestCompressedData is a test utility function that we use to get
// actual compressed data from the
func mustGetTestCompressedData(t *testing.T, dictStore dictionary.Store) (resp blobsubmission.Response, blobBytes []byte) {
	respJson, err := os.ReadFile("sample-blob.json")
	assert.NoError(t, err)

	assert.NoError(t, json.Unmarshal(respJson, &resp))

	blobBytes, err = base64.StdEncoding.DecodeString(resp.CompressedData)
	assert.NoError(t, err)

	_, _, _, err = blob.DecompressBlob(blobBytes, dictStore)
	assert.NoError(t, err)

	return
}
