package v2

import (
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

const (
	// These also impact the circuit constraints (compile / setup time)
	MaxUncompressedBytes = 780000    // This defines the max we can do including some leeway with 2**27 constraints
	MaxUsableBytes       = 32 * 4096 // defines the number of bytes available in a blob
	PackingSizeU256      = v1.PackingSizeU256
)

// NewBlobMaker returns a new bm.
func NewBlobMaker(dataLimit int, dictPath string) (*v1.BlobMaker, error) {
	return v1.NewVersionedBlobMaker(2, dataLimit, dictPath)
}

// DecompressBlob decompresses a v2 blob and returns the header and the blocks as they were compressed.
func DecompressBlob(b []byte, dictStore dictionary.Store) (resp v1.BlobDecompressionResponse, err error) {
	return v1.DecompressBlobVersioned(2, b, dictStore)
}
