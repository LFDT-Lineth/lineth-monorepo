package blob_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	v0 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	blobv2testing "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2/test_utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersion(t *testing.T) {
	_blob := blobv2testing.GenTestBlob(t, 1)
	assert.Equal(t, uint32(0x10000), uint32(0xfffe)+uint32(blob.GetVersion(_blob)), "version should match the current one")
}

const dictPath = "../compressor_dict.bin"

func TestAddToBlob(t *testing.T) {
	dictStore := dictionary.NewStore()
	require.NoError(t, dictStore.Load(dictPath))
	blobData := withNoError(t, os.ReadFile, "testdata/v0/sample-blob-01b9918c3f0ceb6a.bin")
	header, _, blocksSerialized, err := v0.DecompressBlob(blobData, dictStore)
	require.NoError(t, err)

	blobData = withNoError(t, os.ReadFile, "testdata/v0/sample-blob-0151eda71505187b5.bin")
	_, _, blocksSerializedNext, err := v0.DecompressBlob(blobData, dictStore)
	require.NoError(t, err)

	bm, err := v0.NewBlobMaker(v0.MaxUsableBytes, "../compressor_dict.bin")
	require.NoError(t, err)
	var ok bool
	writeBlock := func(blocks *[][]byte) {
		dbd, err := v0.DecodeBlockFromUncompressed(bytes.NewReader((*blocks)[0]))
		assert.NoError(t, err)

		stdBlockRlp, err := rlp.EncodeToBytes(dbd.ToStd())

		ok, err = bm.Write(stdBlockRlp, false, encode.WithTxAddressGetter(encode.GetAddressFromR))
		assert.NoError(t, err)

		*blocks = (*blocks)[1:]
	}

	for i := 0; i < header.NbBatches(); i++ {
		for j := 0; j < header.NbBlocksInBatch(i); j++ {
			writeBlock(&blocksSerialized)
			assert.True(t, ok)
		}
		bm.StartNewBatch()
	}
	assert.Empty(t, blocksSerialized)

	util0 := 100 * bm.Len() / v0.MaxUsableBytes

	require.NoError(t, err)
	for ok { // all in one batch
		writeBlock(&blocksSerializedNext)
	}

	util1 := 100 * bm.Len() / v0.MaxUsableBytes

	fmt.Printf("%d%%\n%d%%\n", util0, util1)
}

func withNoError[X, Y any](t *testing.T, f func(X) (Y, error), x X) Y {
	y, err := f(x)
	require.NoError(t, err)
	return y
}

func TestDecompressBlob(t *testing.T) {
	store := dictionary.NewStore("../compressor_dict.bin")
	files := newRecursiveFolderIterator(t, "testdata")
	for files.hasNext() {
		f := files.next()
		if filepath.Ext(f.path) == ".bin" {
			t.Run(f.path, func(t *testing.T) {
				decompressed, err := blob.DecompressBlob(f.content, store)
				assert.NoError(t, err)
				t.Log("decompressed length", len(decompressed))

				// load decompressed blob as blocks
				var blocksSerialized [][]byte
				assert.NoError(t, rlp.DecodeBytes(decompressed, &blocksSerialized))
				t.Log("number of decoded blocks", len(blocksSerialized))
				for _, blockSerialized := range blocksSerialized {
					var b types.Block
					assert.NoError(t, rlp.DecodeBytes(blockSerialized, &b))
				}
			})
		}
	}
}

type dirEntryWithFullPath struct {
	path    string
	content os.DirEntry
}

// goes through all files in a directory and its subdirectories
type recursiveFolderIterator struct {
	toVisit []dirEntryWithFullPath
	t       *testing.T
	pathLen int
}

type file struct {
	content []byte
	path    string
}

func (i *recursiveFolderIterator) openDir(path string) {
	content, err := os.ReadDir(path)
	require.NoError(i.t, err)
	for _, c := range content {
		i.toVisit = append(i.toVisit, dirEntryWithFullPath{path: filepath.Join(path, c.Name()), content: c})
	}
}

func (i *recursiveFolderIterator) hasNext() bool {
	return i.peek() != nil
}

func (i *recursiveFolderIterator) next() *file {
	f := i.peek()
	if f != nil {
		i.toVisit = i.toVisit[:len(i.toVisit)-1]
	}
	return f
}

// counter-intuitively, peek does most of the work by ensuring the top of the stack is always a file
func (i *recursiveFolderIterator) peek() *file {
	for len(i.toVisit) != 0 {
		lastIndex := len(i.toVisit) - 1
		c := i.toVisit[lastIndex]
		if c.content.IsDir() {
			i.toVisit = i.toVisit[:lastIndex]
			i.openDir(c.path)
		} else {
			b, err := os.ReadFile(c.path)
			require.NoError(i.t, err)
			return &file{content: b, path: c.path[i.pathLen:]}
		}
	}
	return nil
}

func newRecursiveFolderIterator(t *testing.T, path string) *recursiveFolderIterator {
	res := recursiveFolderIterator{t: t, pathLen: len(path) + 1}
	res.openDir(path)
	return &res
}

func TestReadBlobWithType4Tx(t *testing.T) {
	blobBytes, err := base64.StdEncoding.DecodeString("P//C26X2eMZsu4vnsz58sGQtmDkOK10btSlslC04tC8WwaAAEAAaUAAQAAA2d2hVEF3qKPiCwsAYZ69zMfyAyRDMie6HYaATTyz33Os0Qe3I2CoiqR3jNGr7x9zUOu/tMU6vpkXVCv8FAr4gGtsEBEf4OB6DID+EwCkC+NuCBTke/8KAr/AHBOEBEDeC2s6dkAABDGDwYmRmafFt8LMp/gmCycfhCoAD/KQCET/ioOew/hUBaAv4hAWhH+HwHsShKXQvpd6/Z/tuyq8ODxvMv9RbVIBPhMgucDDoUU9GsEAD+BQAX/BAJYZh1HDbtnhXrl60wNAadQ3OMR5fJkBCQCU/g5gbe1AfwmB+gEANwEA/wAAAAAAAAAAAAAAAAAAA")
	dictStore := dictionary.NewStore("../dict/25-04-21.bin")
	_, err = blob.DecompressBlob(blobBytes, dictStore)
	require.NoError(t, err)
}
