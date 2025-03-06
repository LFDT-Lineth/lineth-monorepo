//go:build !fuzzlight

package v1_test

import (
	"encoding/base64"
	"encoding/hex"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/circuits/blobdecompression"
	v1 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	blobcompressorv1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	blobtestutils "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareTestBlob(t require.TestingT) (c, a frontend.Circuit) {
	return prepare(t, blobtestutils.GenTestBlob(t, 1000000))
}

func prepare(t require.TestingT, blobBytes []byte) (c *v1.Circuit, a frontend.Circuit) {

	dictStore, err := dictionary.SingletonStore(blobtestutils.GetDict(t), 1)
	assert.NoError(t, err)
	r, err := blobcompressorv1.DecompressBlob(blobBytes, dictStore)
	assert.NoError(t, err)

	resp, err := blobsubmission.CraftResponse(&blobsubmission.Request{
		Eip4844Enabled: true,
		CompressedData: base64.StdEncoding.EncodeToString(blobBytes),
	})
	assert.NoError(t, err)

	b, err := hex.DecodeString(resp.ExpectedX[2:])
	assert.NoError(t, err)
	var x [32]byte
	copy(x[:], b)

	b, err = hex.DecodeString(resp.ExpectedY[2:])
	assert.NoError(t, err)
	var y fr381.Element
	y.SetBytes(b)

	blobBytes = append(blobBytes, make([]byte, blobcompressorv1.MaxUsableBytes-len(blobBytes))...)
	a, _, snarkHash, err := blobdecompression.Assign(blobBytes, dictStore, true, x, y)
	assert.NoError(t, err)

	_, ok := a.(*v1.Circuit)
	assert.True(t, ok)

	assert.Equal(t, resp.SnarkHash[2:], hex.EncodeToString(snarkHash))

	return &v1.Circuit{
		Dict:                  make([]frontend.Variable, len(r.Dict)),
		BlobBytes:             make([]frontend.Variable, blobcompressorv1.MaxUsableBytes),
		MaxBlobPayloadNbBytes: len(r.RawPayload) * 3 / 2, // small max blobcompressorv1 size so it compiles in manageable time
	}, a
}

func TestSmallBlob(t *testing.T) {
	c, a := prepareTestBlob(t)
	assert.NoError(t, test.IsSolved(c, a, ecc.BLS12_377.ScalarField()))
}

func TestTinyTwoBatchBlob(t *testing.T) {
	c, a := prepare(t, blobtestutils.TinyTwoBatchBlob(t))
	assert.NoError(t, test.IsSolved(c, a, ecc.BLS12_377.ScalarField()))
}

func TestSingleBlockBlob(t *testing.T) {
	c, a := prepare(t, blobtestutils.SingleBlockBlob(t))
	assert.NoError(t, test.IsSolved(c, a, ecc.BLS12_377.ScalarField()))
}

func TestSingleBlockBlobNoEngine(t *testing.T) {

	c, a := prepare(t, blobtestutils.SingleBlockBlob(t))
	c.UseGkrMiMC = true
	cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, c)
	assert.NoError(t, err)

	w, err := frontend.NewWitness(a, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)

	assert.NoError(t, cs.IsSolved(w))
}
