package v0

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	"io"
	"os"
	"strings"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0/compress/lzss"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark/std/compress"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/hash"
	"github.com/icza/bitio"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	maxOutputSize      = 1 << 20 // 1MB
	packingSizeU256    = 248     // = 31 bytes
	packingSizeLastU64 = 64 - 256 + packingSizeU256

	// These also impact the circuit constraints (compile / setup time)
	MaxUncompressedBytes = 780_000   // defines the max size we can handle for a blob (uncompressed) input
	MaxUsableBytes       = 32 * 4096 // TODO @gbotrel confirm this value // defines the number of bytes available in a blob
)

// BlobMaker is a bm for RLP encoded blocks (see EIP-4844).
// It takes a batch of blocks as input (see StartNewBatch and Write).
// And it compresses them into a "blob" (see Bytes).
type BlobMaker struct {
	limit      int              // maximum size of the compressed data
	compressor *lzss.Compressor // compressor used to compress the blob body
	dict       []byte           // dictionary used for compression
	dictStore  dictionary.Store

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
		limit: dataLimit,
	}
	blobMaker.buf.Grow(1 << 17)

	// initialize compressor
	dict, err := os.ReadFile(dictPath)
	if err != nil {
		return nil, err
	}
	dict = lzss.AugmentDict(dict)
	blobMaker.dict = dict
	blobMaker.dictStore, err = dictionary.SingletonStore(dict, 0)
	if err != nil {
		return nil, err
	}

	dictChecksum := compress.ChecksumPaddedBytes(dict, len(dict), hash.MIMC_BLS12_377.New(), fr.Bits)
	copy(blobMaker.header.DictChecksum[:], dictChecksum)

	blobMaker.compressor, err = lzss.NewCompressor(dict, lzss.BestCompression)
	if err != nil {
		return nil, err
	}

	// initialize state
	blobMaker.StartNewBatch()

	return &blobMaker, nil
}

// StartNewBatch starts a new batch of blocks.
func (bm *BlobMaker) StartNewBatch() {
	// if nbBlocks in last batch == 0, do nothing.
	nbBatches := bm.header.NbBatches()
	if nbBatches > 0 && bm.header.NbBlocksInBatch(nbBatches-1) == 0 {
		return
	}

	// add a new batch to the stream
	bm.header.addBatch()
}

// Reset resets the bm to its initial state.
func (bm *BlobMaker) Reset() {
	bm.header.resetTable()
	bm.currentBlobLength = 0
	bm.buf.Reset()
	bm.packBuffer.Reset()
	bm.compressor.Reset()

	bm.StartNewBatch()
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
		header, rawBlocks, _, err := DecompressBlob(bm.currentBlob[:bm.currentBlobLength], bm.dictStore)
		if err != nil {
			var sbb strings.Builder
			fmt.Fprintf(&sbb, "invalid blob: %v\n", err)
			fmt.Fprintf(&sbb, "header: %v\n", bm.header)
			fmt.Fprintf(&sbb, "bm.currentBlobLength: %v\n", bm.currentBlobLength)
			fmt.Fprintf(&sbb, "bm.currentBlob: %x\n", bm.currentBlob[:bm.currentBlobLength])

			panic(sbb.String())
		}
		// compare the header
		if err = header.CheckEquality(&bm.header); err != nil {
			panic(fmt.Errorf("invalid blob: header mismatch %v", err))
		}
		rawBlocksUnpacked, err := UnpackAlign(rawBlocks)
		if err != nil {
			panic(fmt.Errorf("could not unpack rawBlocks: %v", err))
		}
		if !bytes.Equal(rawBlocksUnpacked, bm.compressor.WrittenBytes()) {
			panic(fmt.Sprintf("invalid blob: body mismatch expected %x, got %x", rawBlocks, bm.compressor.WrittenBytes()))
		}
	}
	return bm.currentBlob[:bm.currentBlobLength]
}

// Write attempts to append the RLP block to the current batch.
// if forceReset is set; this will NOT append the bytes but still returns true if the chunk could have been appended
func (bm *BlobMaker) Write(rlpBlock []byte, forceReset bool, encodingOptions ...encode.Option) (ok bool, err error) {

	// decode the RLP block.
	var block types.Block
	if err := rlp.Decode(bytes.NewReader(rlpBlock), &block); err != nil {
		return false, fmt.Errorf("when decoding input RLP block: %w", err)
	}

	// re-encode it for compression
	bm.buf.Reset()
	if err := EncodeBlockForCompression(&block, &bm.buf, encodingOptions...); err != nil {
		return false, fmt.Errorf("when re-encoding block for compression: %w", err)
	}
	blockLen := bm.buf.Len()

	if blockLen > bm.limit {
		// we should panic but logging / alerting is handled by the caller.
		logrus.Warn("block size is larger than the blob limit. This should be checked by the coordinator, keeping the log for sanity", "block size", blockLen, "limit", bm.limit)
	}

	// write the block to the bm
	if _, err := bm.compressor.Write(bm.buf.Bytes()); err != nil {
		// The 2 possibles errors are:
		// 1. underlying writer error (shouldn't happen we use a simple in memory buffer)
		// 2. we exceed the maximum input size of 2Mb (shouldn't happen either)
		// In both cases, we can't do anything, so we reset the state.
		return false, fmt.Errorf("when writing block to compressor: %w", err)
	}

	// increment number of blocks in the current batch
	bm.header.addBlock(blockLen)

	// write the header to get its length.
	bm.buf.Reset()
	if _, err := bm.header.WriteTo(&bm.buf); err != nil {
		// only possible error is underlying writer error (shouldn't happen we use a simple in memory buffer)
		bm.header.removeLastBlock()
		return false, fmt.Errorf("when writing header to buffer: %w", err)
	}

	// check that the header + the uncompressed data is "decompressable" in the circuit
	if bm.compressor.Written()+bm.buf.Len() > MaxUncompressedBytes {
		// it means we are not exploiting the full blob capacity; our compression ratio is "too good"
		// and our decompression circuit is not able to handle the uncompressed data.
		// we should reset the state.
		if err := bm.compressor.Revert(); err != nil {
			return false, fmt.Errorf("when reverting compressor because uncompressed blob is > maxUncompressedSize: %w", err)
		}
		bm.header.removeLastBlock()
		return false, nil
	}

	// check that the header + the compressed data fits in the blob
	fitsInBlob := PackAlignSize(bm.buf.Bytes(), bm.compressor.Bytes()) <= bm.limit
	if !fitsInBlob {
		// first thing to check is if we bypass compression, would that fit?
		if bm.compressor.ConsiderBypassing() {
			// we can bypass compression and get a better ratio.
			// let's check if now we fit in the blob.
			if PackAlignSize(bm.buf.Bytes(), bm.compressor.Bytes()) <= bm.limit {
				goto bypass
			}
		}

		// discard.
		if err := bm.compressor.Revert(); err != nil {
			return false, fmt.Errorf("when reverting compressor because blob is full: %w", err)
		}
		bm.header.removeLastBlock()
		return false, nil
	}
bypass:
	if forceReset {
		// we don't want to append the data, but we could have.
		if err := bm.compressor.Revert(); err != nil {
			return false, fmt.Errorf("when reverting compressor (blob is not full but forceReset == true): %w", err)
		}
		bm.header.removeLastBlock()
		return true, nil
	}

	// copy the compressed data to the blob
	bm.packBuffer.Reset()
	n2, err := PackAlign(&bm.packBuffer, bm.buf.Bytes(), bm.compressor.Bytes())
	if err != nil {
		bm.compressor.Revert()
		bm.header.removeLastBlock()
		return false, fmt.Errorf("when packing blob: %w", err)
	}
	bm.currentBlobLength = int(n2)
	copy(bm.currentBlob[:bm.currentBlobLength], bm.packBuffer.Bytes())

	return true, nil
}

// Clone returns a (almost) deep copy of the bm -- this is used for test purposes.
func (bm *BlobMaker) Clone() *BlobMaker {
	deepCopy := *bm
	deepCopy.header.table = make([][]int, len(bm.header.table))
	for i := range bm.header.table {
		deepCopy.header.table[i] = make([]int, len(bm.header.table[i]))
		copy(deepCopy.header.table[i], bm.header.table[i])
	}

	return &deepCopy
}

// Equals returns true if the two compressors are ~equal -- this is used for test purposes.
func (bm *BlobMaker) Equals(other *BlobMaker) bool {
	if bm.limit != other.limit {
		return false
	}
	if bm.currentBlobLength != other.currentBlobLength {
		return false
	}
	if !bytes.Equal(bm.currentBlob[:bm.currentBlobLength], other.currentBlob[:other.currentBlobLength]) {
		return false
	}
	if len(bm.header.table) != len(other.header.table) {
		return false
	}
	for i := range bm.header.table {
		if len(bm.header.table[i]) != len(other.header.table[i]) {
			return false
		}
		for j := range bm.header.table[i] {
			if bm.header.table[i][j] != other.header.table[i][j] {
				return false
			}
		}
	}
	return true
}

// DecompressBlob decompresses a blob and returns the header and the blocks as they were compressed.
// rawBlocks is the raw payload of the blob, delivered in packed format @TODO bad idea. fix
func DecompressBlob(b []byte, dictStore dictionary.Store) (blobHeader *Header, rawBlocks []byte, blocks [][]byte, err error) {
	// UnpackAlign the blob
	b, err = UnpackAlign(b)
	if err != nil {
		return nil, nil, nil, err
	}

	// read the header
	blobHeader = new(Header)
	read, err := blobHeader.ReadFrom(bytes.NewReader(b))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read blob header: %w", err)
	}

	// retrieve dict
	dict, err := dictStore.Get(blobHeader.DictChecksum[:], 0)
	if err != nil {
		return nil, nil, nil, err
	}

	b = b[read:]

	// decompress the data
	rawBlocks, err = lzss.Decompress(b, dict)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decompress blob body: %w", err)
	}

	offset := 0
	for _, batch := range blobHeader.table {
		for _, blockLength := range batch {
			blocks = append(blocks, rawBlocks[offset:offset+blockLength])
			offset += blockLength
		}
	}

	buf := &bytes.Buffer{}
	if _, err := PackAlign(buf, rawBlocks, nil); err != nil {
		return nil, nil, nil, fmt.Errorf("error packing raw blocks: %w", err)
	}

	return blobHeader, buf.Bytes(), blocks, nil
}

// PackAlignSize returns the size of the data when packed with PackAlign.
func PackAlignSize(a, b []byte) (n int) {
	nbBytes := len(a) + len(b) + 1 // + 1 for the padding length
	nbBits := nbBytes * 8

	// we may need to add some bits to a and b to ensure we can process some blocks of 248 bits
	extraBits := (packingSizeU256 - nbBits%packingSizeU256) % packingSizeU256
	nbBits += extraBits

	return ((nbBits / packingSizeU256) * 32)
}

// packAlignSizeToRefactor returns the size of the data when packed with PackAlign (but takes len of the slices as input).
// TODO @gbotrel refactor and reconcile with @Tabaie PR
func packAlignSizeToRefactor(lena, lenb int) (n int) {
	nbBytes := lena + lenb + 1 // + 1 for the padding length
	nbBits := nbBytes * 8

	// we may need to add some bits to a and b to ensure we can process some blocks of 248 bits
	extraBits := (packingSizeU256 - nbBits%packingSizeU256) % packingSizeU256
	nbBits += extraBits

	return ((nbBits / packingSizeU256) * 32)
}

// PackAlign writes a and b to w, aligned to fr.Element (bls12-377) boundary.
// It returns the length of the data written to w.
func PackAlign(w io.Writer, a, b []byte) (n int64, err error) {

	nbBytes := len(a) + len(b) + 1 // + 1 for the padding length
	nbBits := nbBytes * 8

	// we may need to add some bits to a and b to ensure we can process some blocks of 248 bits
	extraBits := (packingSizeU256 - nbBits%packingSizeU256) % packingSizeU256
	nbBits += extraBits

	if nbBits%packingSizeU256 != 0 {
		return 0, fmt.Errorf("nbBits mod %d != 0, (nbBits = %d)", packingSizeU256, nbBits)
	}

	// padding will always be less than 31 bytes
	bytePadding := (extraBits + 7) / 8
	var buf [32]byte

	// the last non-zero byte of the stream is the padding length
	// we add +1 to handle the case where the padding length is 0
	buf[0] = uint8(bytePadding + 1)

	r := bitio.NewReader(io.MultiReader(bytes.NewReader(a), bytes.NewReader(b), bytes.NewReader(buf[:bytePadding+1])))

	var tryWriteErr error
	tryWrite := func(v uint64) {
		if tryWriteErr == nil {
			tryWriteErr = binary.Write(w, binary.BigEndian, v)
		}
	}

	for i := 0; i < nbBits/packingSizeU256; i++ {
		tryWrite(r.TryReadBits(packingSizeLastU64))
		tryWrite(r.TryReadBits(64))
		tryWrite(r.TryReadBits(64))
		tryWrite(r.TryReadBits(64))
	}

	if tryWriteErr != nil {
		return 0, fmt.Errorf("when writing to w: %w", tryWriteErr)
	}

	if r.TryError != nil {
		return 0, fmt.Errorf("when reading from multi-reader: %w", r.TryError)
	}

	n1 := ((nbBits / packingSizeU256) * 32)
	if n1 != PackAlignSize(a, b) {
		return 0, errors.New("inconsistent PackAlignSize")
	}
	return int64(n1), nil
}

// UnpackAlign unpacks r (packed with PackAlign) and returns the unpacked data.
func UnpackAlign(r []byte) ([]byte, error) {
	if len(r)%32 != 0 {
		return nil, errors.New("invalid data length; expected multiple of 32")
	}
	n := len(r) / 32
	var out bytes.Buffer
	w := bitio.NewWriter(&out)
	for i := 0; i < n; i++ {
		// read 32bytes
		element := r[32*i : 32*(i+1)]
		// write 248 bits
		w.TryWriteBits(binary.BigEndian.Uint64(element[0:8]), packingSizeLastU64)
		w.TryWriteBits(binary.BigEndian.Uint64(element[8:16]), 64)
		w.TryWriteBits(binary.BigEndian.Uint64(element[16:24]), 64)
		w.TryWriteBits(binary.BigEndian.Uint64(element[24:32]), 64)
	}
	if w.TryError != nil {
		return nil, fmt.Errorf("when writing to bitio.Writer: %w", w.TryError)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("when closing bitio.Writer: %w", err)
	}

	// the last non-zero byte is the padding length + 1
	// this is a cheap sanity check; we could also just resize the output buffer
	// to the correct size.
	cpt := 0
	for out.Bytes()[out.Len()-1] == 0 {
		out.Truncate(out.Len() - 1)
		cpt++
	}
	// last byte should be equal to cpt
	lastNonZero := out.Bytes()[out.Len()-1]
	if (cpt % 31) != int(lastNonZero)-1 {
		return nil, errors.New("invalid padding length")
	}
	out.Truncate(out.Len() - 1)

	return out.Bytes(), nil
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
	n = packAlignSizeToRefactor(n, 0)

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
	n = packAlignSizeToRefactor(n, 0)

	return n, nil
}
