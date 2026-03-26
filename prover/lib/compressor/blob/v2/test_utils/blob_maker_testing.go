package test_utils

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"path/filepath"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/internal/rlpblocks"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"

	v2 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func GenTestBlob(t require.TestingT, maxNbBlocks int) []byte {
	testBlocks, bm := TestBlocksAndBlobMaker(t)

	// Compress blocks
	cptBlock := 0
	for i, block := range testBlocks {
		// get a random from 1 to 5

		bSize := RandIntn(5) + 1 // #nosec G404 -- false positive

		if cptBlock > bSize && i%3 == 0 {
			cptBlock = 0
			bm.StartNewBatch()
		}

		appended, err := bm.Write(block, false)
		if !appended || i == maxNbBlocks {
			assert.NoError(t, err, "append a valid block should not generate an error")
			break
		} else {
			cptBlock++
		}
	}
	return bm.Bytes()
}

func RandIntn(n int) int { // TODO @Tabaie remove
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int(binary.BigEndian.Uint64(b[:]) % uint64(n))
}

func SingleBlockBlob(t require.TestingT) []byte {
	testBlocks, bm := TestBlocksAndBlobMaker(t)

	ok, err := bm.Write(testBlocks[0], false)
	assert.NoError(t, err)
	assert.True(t, ok)

	return bm.Bytes()
}

// TinyTwoBatchBlob produces a blob with two batches, each consisting of one block
func TinyTwoBatchBlob(t require.TestingT) []byte {

	testBlocks, bm := TestBlocksAndBlobMaker(t)

	for _, block := range testBlocks[:2] {
		ok, err := bm.Write(block, false)
		assert.NoError(t, err)
		assert.True(t, ok)
		bm.StartNewBatch()
	}

	return bm.Bytes()
}

// ConsecutiveBlobs (t, i, j, k) produces three blobs from the ranges [0:i], [i:i+j], [i+j:i+j+k] of test blocks
// each block is in its own batch
func ConsecutiveBlobs(t require.TestingT, n ...int) [][]byte {
	testBlocks, bm := TestBlocksAndBlobMaker(t)

	res := make([][]byte, 0, len(n))
	for _, n := range n {
		for j := 0; j < n; j++ {
			ok, err := bm.Write(testBlocks[j], false)
			assert.NoError(t, err)
			assert.True(t, ok)
			bm.StartNewBatch()
		}
		testBlocks = testBlocks[n:]
		res = append(res, bm.Bytes())
		bm.Reset()
	}

	return res
}

func TestBlocksAndBlobMaker(t require.TestingT) ([][]byte, *v1.BlobMaker) {
	repoRoot, err := test_utils.GetRepoRootPath()
	assert.NoError(t, err)
	testBlocks := rlpblocks.Get()
	// Init bm
	bm, err := v2.NewBlobMaker(40000, filepath.Join(repoRoot, "prover/lib/compressor/compressor_dict.bin"))
	assert.NoError(t, err)
	return testBlocks, bm
}

func GetDict(t require.TestingT) []byte {
	dict, err := getDictForTest()
	require.NoError(t, err)
	return dict
}

func getDictForTest() ([]byte, error) {
	repoRoot, err := test_utils.GetRepoRootPath()
	if err != nil {
		return nil, err
	}
	dictPath := filepath.Join(repoRoot, "prover/lib/compressor/compressor_dict.bin")
	return os.ReadFile(dictPath)
}
