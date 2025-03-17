package v1

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/consensys/compress/lzss"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

const (
	maxOutputSize        = 1 << 20 // 1MB
	PackingSizeU256      = fr381.Bits - 1
	ByteLenEncodingBytes = 3
	NbElemsEncodingBytes = 2

	// These also impact the circuit constraints (compile / setup time)
	MaxUncompressedBytes = 793125    // ~774.54KB defines the max size we can handle for a blob (uncompressed) input
	MaxUsableBytes       = 32 * 4096 // defines the number of bytes available in a blob
)

// BlobMaker is a bm for RLP encoded blocks (see EIP-4844).
// It takes a batch of blocks as input (see StartNewBatch and Write).
// And it compresses them into a "blob" (see Bytes).
type BlobMaker struct {
	Limit      int              // maximum size of the compressed data
	compressor *lzss.Compressor // compressor used to compress the blob body
	dict       []byte           // dictionary used for compression
	dictStore  dictionary.Store // dictionary store comprising only dict, used for decompression sanity checks

	header Header

	// contains currentBlob data from latest **valid** call to Write
	// that is the header (uncompressed) and the body (compressed)
	// byte aligned to match fr.Element boundary.
	currentBlob       [maxOutputSize]byte
	currentBlobLength int

	// some buffers to avoid repeated allocations
	buf        bytes.Buffer
	packBuffer bytes.Buffer
}

// NewBlobMaker returns a new bm.
func NewBlobMaker(dataLimit int, dictPath string) (*BlobMaker, error) {
	blobMaker := BlobMaker{
		Limit: dataLimit,
	}
	blobMaker.buf.Grow(1 << 17)

	// initialize compressor
	dict, err := os.ReadFile(dictPath)
	if err != nil {
		return nil, err
	}
	dict = lzss.AugmentDict(dict)
	blobMaker.dict = dict
	if blobMaker.dictStore, err = dictionary.SingletonStore(dict, 1); err != nil {
		return nil, err
	}

	dictChecksum, err := encode.MiMCChecksumPackedData(dict, 8)
	if err != nil {
		return nil, err
	}
	copy(blobMaker.header.DictChecksum[:], dictChecksum)

	blobMaker.compressor, err = lzss.NewCompressor(dict)
	if err != nil {
		return nil, err
	}

	// initialize state
	blobMaker.StartNewBatch()

	return &blobMaker, nil
}

// StartNewBatch starts a new batch of blocks.
func (bm *BlobMaker) StartNewBatch() {
	bm.header.sealBatch()
}

// Reset resets the bm to its initial state.
func (bm *BlobMaker) Reset() {
	bm.header.resetTable()
	bm.currentBlobLength = 0
	bm.buf.Reset()
	bm.packBuffer.Reset()
	bm.compressor.Reset()

	bm.header.sealBatch()
}

// Len returns the length of the compressed data, which includes the header.
func (bm *BlobMaker) Len() int {
	return bm.currentBlobLength
}

func (bm *BlobMaker) Written() int {
	return bm.compressor.Written()
}

// Bytes returns the compressed data. Note that it returns a slice of the internal buffer,
// it is the caller's responsibility to copy the data if needed.
func (bm *BlobMaker) Bytes() []byte {
	if bm.currentBlobLength > 0 {
		// sanity check that we can always decompress.
		resp, err := DecompressBlob(bm.currentBlob[:bm.currentBlobLength], bm.dictStore)
		if err != nil {
			var sbb strings.Builder
			fmt.Fprintf(&sbb, "invalid blob: %v\n", err)
			fmt.Fprintf(&sbb, "header: %v\n", bm.header)
			fmt.Fprintf(&sbb, "bm.currentBlobLength: %v\n", bm.currentBlobLength)
			fmt.Fprintf(&sbb, "bm.currentBlob: %x\n", bm.currentBlob[:bm.currentBlobLength])

			panic(sbb.String())
		}
		// compare the header
		if !resp.Header.Equals(&bm.header) {
			panic("invalid blob: header mismatch")
		}
		if !bytes.Equal(resp.RawPayload, bm.compressor.WrittenBytes()) {
			panic(fmt.Sprintf("invalid blob: body mismatch expected %x, got %x", resp.RawPayload, bm.compressor.WrittenBytes()))
		}
	}
	return bm.currentBlob[:bm.currentBlobLength]
}

// Write attempts to append the RLP block to the current batch.
// if forceReset is set; this will NOT append the bytes but still returns true if the chunk could have been appended
func (bm *BlobMaker) Write(rlpBlock []byte, forceReset bool) (ok bool, err error) {
	prevLen := bm.compressor.Written()

	// decode the RLP block.
	var block types.Block
	if err = rlp.Decode(bytes.NewReader(rlpBlock), &block); err != nil {
		return false, fmt.Errorf("when decoding input RLP block: %w", err)
	}

	// re-encode it for compression
	bm.buf.Reset()
	if err = EncodeBlockForCompression(&block, &bm.buf); err != nil {
		return false, fmt.Errorf("when re-encoding block for compression: %w", err)
	}
	blockLen := bm.buf.Len()

	if blockLen > bm.Limit {
		// we should panic but logging / alerting is handled by the caller.
		logrus.Warn("block size is larger than the blob Limit. This should be checked by the coordinator, keeping the log for sanity", "block size", blockLen, "Limit", bm.Limit)
	}

	// write the block to the bm
	if _, err = bm.compressor.Write(bm.buf.Bytes()); err != nil {
		// The 2 possibles errors are:
		// 1. underlying writer error (shouldn't happen we use a simple in memory buffer)
		// 2. we exceed the maximum input size of 2Mb (shouldn't happen either)
		// In both cases, we can't do anything, so we reset the state.
		if innerErr := bm.compressor.Revert(); innerErr != nil {
			return false, fmt.Errorf("when reverting compressor because writing failed: %w\noriginal error: %w", innerErr, err)
		}
		return false, fmt.Errorf("when writing block to compressor: %w", err)
	}

	// increment length of the current batch
	bm.header.addBlock(blockLen)
	// write the header to get its length.
	bm.buf.Reset()
	if _, err = bm.header.WriteTo(&bm.buf); err != nil {
		// only possible error is an underlying writer error (shouldn't happen we use a simple in-memory buffer)
		bm.header.removeLastBlock()
		return false, fmt.Errorf("when writing header to buffer: %w", err)
	}

	// check that the header + the uncompressed data is "decompressable" in the circuit
	if uint64(bm.compressor.Written()+bm.buf.Len()) > MaxUncompressedBytes {
		// it means we are not exploiting the full blob capacity; our compression ratio is "too good"
		// and our decompression circuit is not able to handle the uncompressed data.
		// we should reset the state.
		if err := bm.compressor.Revert(); err != nil {
			return false, fmt.Errorf("when reverting compressor because uncompressed blob is > maxUncompressedSize: %w", err)
		}
		bm.header.removeLastBlock()
		return false, nil
	}

	fitsInBlob := func() bool {
		return encode.PackAlignSize(bm.buf.Len()+bm.compressor.Len(), fr381.Bits-1) <= bm.Limit
	}

	payload := bm.compressor.WrittenBytes()
	recompressionAttempted := false
	revert := func() error { // from this point on, we may have recompressed the entire payload in one go
		// that makes the compressor's own Revert method unusable.
		bm.header.removeLastBlock()
		if !recompressionAttempted { // fast path for most "CanWrite" calls
			return bm.compressor.Revert()
		}
		// we can't use the compressor's own Revert method because we tried to compress in one go.
		bm.compressor.Reset()
		_, err := bm.compressor.Write(payload[:prevLen])
		return wrapError(err, "reverting the compressor")
	}

	// check that the header + the compressed data fits in the blob
	if !fitsInBlob() {
		recompressionAttempted = true

		// first thing to check is whether we can fit the block if we recompress everything in one go, known to achieve a higher ratio.
		bm.compressor.Reset()
		if _, err = bm.compressor.Write(payload); err != nil {
			err = fmt.Errorf("when recompressing the blob: %w", err)

			if innerErr := revert(); innerErr != nil {
				err = fmt.Errorf("%w\n\tto recover from write failure: %w", innerErr, err)
			}

			return false, err
		}
		if fitsInBlob() {
			goto bypass
		}

		// that didn't work. a "desperate" attempt is not to compress at all.
		if bm.compressor.ConsiderBypassing() {
			// we can bypass compression and get a better ratio.
			// let's check if now we fit in the blob.
			if fitsInBlob() {
				goto bypass
			}
		}

		// discard.
		if err = revert(); err != nil {
			return false, fmt.Errorf("when reverting compressor because blob is full: %w", err)
		}
		return false, nil
	}
bypass:
	if forceReset {
		// we don't want to append the data, but we could have.
		if err = revert(); err != nil {
			return false, fmt.Errorf("%w\nreverting because forceReset == true even though the blob isn't full", err)
		}
		return true, nil
	}

	// copy the compressed data to the blob
	bm.packBuffer.Reset()
	n2, err := encode.PackAlign(&bm.packBuffer, bm.buf.Bytes(), fr381.Bits-1, encode.WithAdditionalInput(bm.compressor.Bytes()))
	if err != nil {
		err = fmt.Errorf("when packing blob: %w", err)
		innerErr := revert()
		if innerErr != nil {
			err = fmt.Errorf("%w\n\twhen attempting to recover from: %w", innerErr, err)
		}
		return false, fmt.Errorf("when packing blob: %w", err)
	}
	bm.currentBlobLength = int(n2)
	copy(bm.currentBlob[:bm.currentBlobLength], bm.packBuffer.Bytes())

	return true, nil
}

// Clone returns a (almost) deep copy of the bm -- this is used for test purposes.
func (bm *BlobMaker) Clone() *BlobMaker {
	deepCopy := *bm
	deepCopy.header.BatchSizes = make([]int, len(bm.header.BatchSizes))

	copy(deepCopy.header.BatchSizes, bm.header.BatchSizes)

	return &deepCopy
}

// Equals returns true if the two compressors are ~equal -- this is used for test purposes.
func (bm *BlobMaker) Equals(other *BlobMaker) bool {
	if bm.Limit != other.Limit {
		return false
	}
	if bm.currentBlobLength != other.currentBlobLength {
		return false
	}
	if !bytes.Equal(bm.currentBlob[:bm.currentBlobLength], other.currentBlob[:other.currentBlobLength]) {
		return false
	}
	if len(bm.header.BatchSizes) != len(other.header.BatchSizes) {
		return false
	}
	if !slices.Equal(bm.header.BatchSizes, other.header.BatchSizes) {
		return false
	}
	return true
}

type BlobDecompressionResponse struct {
	Header     *Header
	Blocks     [][]byte
	RawPayload []byte
	Dict       []byte
}

// DecompressBlob decompresses a blob and returns the header and the blocks as they were compressed.
func DecompressBlob(b []byte, dictStore dictionary.Store) (resp BlobDecompressionResponse, err error) {
	// UnpackAlign the blob
	b, err = encode.UnpackAlign(b, fr381.Bits-1, false)
	if err != nil {
		return
	}

	// read the header
	resp.Header = new(Header)
	read, err := resp.Header.ReadFrom(bytes.NewReader(b))
	if err != nil {
		err = fmt.Errorf("failed to read blob header: %w", err)
		return
	}
	// retrieve dictionary
	if resp.Dict, err = dictStore.Get(resp.Header.DictChecksum[:], 1); err != nil {
		return
	}

	b = b[read:]

	// decompress the data
	resp.RawPayload, err = lzss.Decompress(b, resp.Dict)
	if err != nil {
		err = fmt.Errorf("failed to decompress blob body: %w", err)
		return
	}

	offset := 0
	for _, batchLen := range resp.Header.BatchSizes {

		batchOffset := offset
		for offset < batchOffset+batchLen {
			if blockLen, err := ScanBlockByteLen(resp.RawPayload[offset:]); err != nil {
				return resp, err
			} else {
				resp.Blocks = append(resp.Blocks, resp.RawPayload[offset:offset+blockLen])
				offset += blockLen
			}
		}

		if offset != batchOffset+batchLen {
			err = errors.New("incorrect batch length")
			return
		}
	}

	return
}

// WorstCompressedBlockSize returns the size of the given block, as compressed by an "empty" blob maker.
// That is, with more context, blob maker could compress the block further, but this function
// returns the maximum size that can be achieved.
//
// The input is a RLP encoded block.
// Returns the length of the compressed data, or -1 if an error occurred.
//
// This function is thread-safe. Concurrent calls are allowed,
// but the other functions may not be thread-safe.
func (bm *BlobMaker) WorstCompressedBlockSize(rlpBlock []byte) (bool, int, error) {
	// decode the RLP block.
	var block types.Block
	if err := rlp.Decode(bytes.NewReader(rlpBlock), &block); err != nil {
		return false, -1, fmt.Errorf("failed to decode RLP block: %w", err)
	}

	// encode the block in Linea format.
	var buf bytes.Buffer
	if err := EncodeBlockForCompression(&block, &buf); err != nil {
		return false, -1, fmt.Errorf("failed to encode block: %w", err)
	}

	inputSlice := buf.Bytes()
	n, err := bm.compressor.CompressedSize256k(inputSlice)
	if err != nil {
		return false, -1, err
	}
	expandingBlock := n > len(inputSlice)
	if expandingBlock {
		// this simulates the fallback to "no compression"
		// this case may happen if the input is not compressible
		// in which case the compressed size is the input size + the header size
		n = len(inputSlice) + lzss.HeaderSize
	}

	// account for the padding
	n = encode.PackAlignSize(n, fr381.Bits-1, encode.NoTerminalSymbol())

	return expandingBlock, n, nil
}

// WorstCompressedTxSize returns the size of the given transaction, as compressed by an "empty" blob maker.
// That is, with more context, blob maker could compress the transaction further, but this function
// returns the maximum size that can be achieved.
//
// The input is a RLP encoded transaction.
// Returns the length of the compressed data, or -1 if an error occurred.
//
// This function is thread-safe. Concurrent calls are allowed,
// but the other functions may not be thread-safe.
func (bm *BlobMaker) WorstCompressedTxSize(rlpTx []byte) (int, error) {

	// decode the RLP transaction.
	var tx types.Transaction
	if err := rlp.Decode(bytes.NewReader(rlpTx), &tx); err != nil {
		return -1, err
	}

	// encode the transaction in Linea format.
	var buf bytes.Buffer
	if err := EncodeTxForCompression(&tx, &buf); err != nil {
		return -1, fmt.Errorf("failed to encode transaction: %w", err)
	}

	inputSlice := buf.Bytes()
	return bm.compressor.CompressedSize256k(inputSlice)
}

// RawCompressedSize compresses the (raw) input and returns the length of the compressed data.
// The returned length account for the "padding" used by the blob maker to
// fit the data in field elements.
// Input size must be less than 256kB.
// If an error occurred, returns -1.
//
// This function is thread-safe. Concurrent calls are allowed,
// but the other functions are not thread-safe.
func (bm *BlobMaker) RawCompressedSize(data []byte) (int, error) {
	n, err := bm.compressor.CompressedSize256k(data)
	if err != nil {
		return -1, err
	}
	if n > len(data) {
		// this simulates the fallback to "no compression"
		// this case may happen if the input is not compressible
		// in which case the compressed size is the input size + the header size
		n = len(data) + lzss.HeaderSize
	}

	// account for the padding
	n = encode.PackAlignSize(n, fr381.Bits-1, encode.NoTerminalSymbol())

	return n, nil
}

func wrapError(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
