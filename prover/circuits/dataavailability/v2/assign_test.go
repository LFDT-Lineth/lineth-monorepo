//go:build !fuzzlight

package v2_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability"
	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability/config"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	v2 "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2"
	blobcompressorv2 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2"
	blobtestutils "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareTestBlob(t require.TestingT) (c, a frontend.Circuit) {
	return prepare(t, blobtestutils.GenTestBlob(t, 1000000))
}

func prepare(t require.TestingT, blobBytes []byte) (c, a *v2.Circuit) {

	dictStore, err := dictionary.SingletonStore(blobtestutils.GetDict(t), 2)
	assert.NoError(t, err)
	r, err := blobcompressorv2.DecompressBlob(blobBytes, dictStore)
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

	circuitSizes := config.CircuitSizes{
		MaxUncompressedNbBytes: len(r.RawPayload) * 3 / 2, // small max blobcompressorv1 size so it compiles in manageable time
		MaxNbBatches:           r.Header.NbBatches() + 2,
		DictNbBytes:            65536,
	}

	blobBytes = append(blobBytes, make([]byte, blobcompressorv2.MaxUsableBytes-len(blobBytes))...)
	_a, _, snarkHash, err := dataavailability.Assign(circuitSizes, blobBytes, dictStore, true, x, y)
	assert.NoError(t, err)

	a, ok := _a.(*v2.Circuit)
	assert.True(t, ok)

	assert.Equal(t, resp.SnarkHash[2:], hex.EncodeToString(snarkHash))

	return v2.Allocate(circuitSizes), a
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
	cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, c)
	assert.NoError(t, err)

	w, err := frontend.NewWitness(a, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)

	assert.NoError(t, cs.IsSolved(w))
}

func TestAssignAllocateSizeMatch(t *testing.T) {
	c, a := prepare(t, blobtestutils.SingleBlockBlob(t))
	require.Equal(t, len(c.Dict), len(a.Dict))
	require.Equal(t, c.DictNbBytes, len(c.Dict))
	require.Equal(t, len(c.BlobBytes), len(a.BlobBytes))
	require.Equal(t, len(c.FuncPI.BatchSums), len(a.FuncPI.BatchSums))
	require.Equal(t, c.MaxNbBatches, len(c.FuncPI.BatchSums))
}
