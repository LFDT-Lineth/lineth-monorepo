//go:build !fuzzlight

package v0

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"
	"os"
	"slices"
	"testing"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	encodeTesting "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode/test_utils"
	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0/compress/lzss"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/stretchr/testify/assert"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/mimc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/require"
)

const testDictPath = "../../compressor_dict.bin"

func TestCompressorNoBatches(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := NewBlobMaker(64*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// Compress blocks
	cptBlock := 0
	for i, block := range testBlocks {
	reprocessBlock:
		appended, err := bm.Write(block, false)
		if appended {
			cptBlock++
			if i+1 != len(testBlocks) { // even if the blob is not completely full, try sealing it
				// TODO need more testdata in testdata/prover-v2/prover-execution/requests to fill a blob
				continue
			}
		}

		assert.NoError(err, "append a valid block should not generate an error")

		// decompress the
		batches, err := decompressBlob(bm.Bytes())
		assert.NoError(err)

		assert.Equal(1, len(batches), "number of batches should match")
		assert.Equal(cptBlock, len(batches[0]), "number of blocks should match")

		assertBatchesConsistent(t, testBlocks[:cptBlock], batches[0])

		cptBlock = 0

		bm.Reset()
		if !appended {
			goto reprocessBlock
		}
	}
}

func TestEncodeBlockForCompression(t *testing.T) {
	var encoded bytes.Buffer

	for _, blockRaw := range testBlocks {
		encoded.Reset()
		var block types.Block
		assert.NoError(t, rlp.Decode(bytes.NewReader(blockRaw), &block))
		assert.NoError(t, EncodeBlockForCompression(&block, &encoded))
		assertBatchesConsistent(t, [][]byte{blockRaw}, [][]byte{encoded.Bytes()})
	}
}

type readerWithRewind struct {
	b []byte
	i int
}

func (r *readerWithRewind) Read(p []byte) (n int, err error) {
	n = len(p)
	if r.i+n > len(r.b) {
		n = len(r.b) - r.i
		if n == 0 {
			return 0, io.EOF
		}
	}
	copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

func (r *readerWithRewind) readByte() (byte, error) {
	var res [1]byte
	_, err := r.Read(res[:])
	return res[0], err
}

func (r *readerWithRewind) rewind(n int) {
	if n > r.i {
		panic("can't rewind")
	}
	r.i -= n
}

func newReaderWithRewind(b []byte) *readerWithRewind {
	return &readerWithRewind{b: b}
}

type writeCounter uint32

func (w *writeCounter) Write(p []byte) (n int, err error) {
	*w += writeCounter(len(p))
	return len(p), nil
}

func assertBatchesConsistent(t *testing.T, raw, decoded [][]byte) {
	assert.Equal(t, len(raw), len(decoded), "number of blocks should match")
	for i := range raw {
		var block types.Block
		assert.NoError(t, rlp.Decode(bytes.NewReader(raw[i]), &block))

		r := newReaderWithRewind(decoded[i])
		var time uint64
		assert.NoError(t, binary.Read(r, binary.LittleEndian, &time))
		assert.Equal(t, block.Time(), time, "block time should match")

		for _, tx := range block.Transactions() {
			n0 := r.i
			txType, err := r.readByte()
			assert.NoError(t, err)
			if txType != types.DynamicFeeTxType && txType != types.AccessListTxType {
				r.rewind(1)
			}
			var decodedTx []interface{}
			assert.NoError(t, rlp.Decode(r, &decodedTx))
			from := decodedTx[3]
			if txType == types.DynamicFeeTxType {
				from = decodedTx[4]
			}

			txFrom := ethereum.GetFrom(tx)
			assert.Equal(t, txFrom[:], from, "tx from should match")

			var decodedTxLen writeCounter
			assert.NoError(t, EncodeTxForCompression(tx, &decodedTxLen))
			r.rewind((r.i - n0) - int(decodedTxLen))
		}
	}
}

func (bm *BlobMaker) testCanWriteMutates(assert *require.Assertions, block []byte) (canWrite bool) {
	c := bm.Clone()

	// "CanWrite"
	canWrite, err := bm.Write(block, true)
	assert.NoError(err, "canWrite should not generate an error")

	// this should never modify the state!
	assert.True(bm.Equals(c), "canWrite should not mutate the bm")

	return canWrite
}

func TestCanWrite(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := NewBlobMaker(64*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// Compress blocks
	var blobs [][]byte
	var nbBlocksPerBatch []uint16 // tracking number of blocks, no batch in this test.
	cptBlock := 0
	for i, block := range testBlocks {
		// get a random from 1 to 5
		bSize := rand.IntN(3) + 1 // #nosec G404 -- false positive

		if cptBlock > bSize && i%3 == 0 {
			nbBlocksPerBatch = append(nbBlocksPerBatch, uint16(cptBlock))
			cptBlock = 0
			bm.StartNewBatch()
		}
	reprocessBlock:
		// ensure that CanWrite (writeGo(block, true)) never mutates the state
		canWrite := bm.testCanWriteMutates(assert, block)
		appended, err := bm.Write(block, false)
		assert.Equal(canWrite, appended, "write should match canWrite")
		if appended {
			cptBlock++
			if i+1 != len(testBlocks) { // even if the blob is not completely full, try sealing it
				continue
			}
		}
		assert.NoError(err, "append a valid block should not generate an error")

		// get compressed bytes
		compressed := make([]byte, bm.Len())
		copy(compressed, bm.Bytes())

		// append to blobs
		blobs = append(blobs, compressed)
		if cptBlock > 0 {
			// can be 0 if we start a new batch but didn't append any block
			nbBlocksPerBatch = append(nbBlocksPerBatch, uint16(cptBlock))
			cptBlock = 0
		}

		bm.Reset()
		if !appended {
			goto reprocessBlock
		}
	}

	blockOffset := 0
	batchOffset := 0
	var buf bytes.Buffer
	for _, b := range blobs {
		batches, err := decompressBlob(b)
		assert.NoError(err)

		for _, batch := range batches {
			// check number of blocks
			nbBlocks := uint16(len(batch))

			// check number of blocks
			assert.Equal(nbBlocksPerBatch[batchOffset], nbBlocks, "number of blocks should match")
			batchOffset++

			// check blocks
			for j, decodedBlock := range batch {
				// encode block from test data to compare
				buf.Reset()
				// decode the RLP block.
				var block types.Block
				assert.NoError(rlp.Decode(bytes.NewReader(testBlocks[j+blockOffset]), &block))
				EncodeBlockForCompression(&block, &buf)
				originalBlock := buf.Bytes()

				assert.Equal(len(decodedBlock), len(originalBlock), "block length should match")
				assert.True(bytes.Equal(originalBlock, decodedBlock), "block should match")
			}
			blockOffset += int(nbBlocks)
		}
	}
}

func TestCompressorWithBatches(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := NewBlobMaker(120*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// Compress blocks
	var blobs [][]byte
	var nbBlocksPerBatch []uint16 // tracking number of blocks, no batch in this test.
	cptBlock := 0
	for i, block := range testBlocks {
		t.Logf("processing block %d over %d", i, len(testBlocks))
		// get a random from 1 to 5
		bSize := rand.IntN(5) + 1 // #nosec G404 -- false positive

		if cptBlock > bSize && i%3 == 0 {
			nbBlocksPerBatch = append(nbBlocksPerBatch, uint16(cptBlock))
			cptBlock = 0
			bm.StartNewBatch()
		}
	reprocessBlock:
		appended, err := bm.Write(block, false)
		if appended {
			cptBlock++
			if i+1 != len(testBlocks) { // even if the blob is not completely full, try sealing it
				continue
			}
		}
		assert.NoError(err, "append a valid block should not generate an error")

		// get compressed bytes
		compressed := make([]byte, bm.Len())
		copy(compressed, bm.Bytes())

		// append to blobs
		blobs = append(blobs, compressed)
		if cptBlock > 0 {
			// can be 0 if we start a new batch but didn't append any block
			nbBlocksPerBatch = append(nbBlocksPerBatch, uint16(cptBlock))
			cptBlock = 0
		}

		bm.Reset()
		if !appended {
			goto reprocessBlock
		}
	}

	// decompress the blobs.
	t.Log("Decompress blobs")

	blockOffset := 0
	batchOffset := 0
	totalRawSize := 0
	totalCompressedSize := 0
	var buf bytes.Buffer
	for i, b := range blobs {
		t.Logf("processing blob %d over %d", i, len(blobs))
		batches, err := decompressBlob(b)
		assert.NoError(err)

		totalCompressedSize += len(b)
		// we get a conservative estimate of the raw size; we don't take into account the
		// overhead of the header, but it still gives a good idea of the compression ratio.
		for _, batch := range batches {
			for _, block := range batch {
				totalRawSize += len(block)
			}
		}

		for _, batch := range batches {

			// check number of blocks
			assert.Equal(nbBlocksPerBatch[batchOffset], uint16(len(batch)), "number of blocks should match")
			batchOffset++

			// check blocks
			for j, decodedBlock := range batch {

				// encode block from test data to compare
				buf.Reset()
				// decode the RLP block.
				var block types.Block
				assert.NoError(rlp.Decode(bytes.NewReader(testBlocks[j+blockOffset]), &block))
				EncodeBlockForCompression(&block, &buf)
				originalBlock := buf.Bytes()

				assert.Equal(len(decodedBlock), len(originalBlock), "block length should match")
				assert.True(bytes.Equal(originalBlock, decodedBlock), "block should match")
			}
			blockOffset += len(batch)
		}
	}

	// average compression ratio
	t.Logf("Average compression ratio: %.2f", float64(totalRawSize)/float64(totalCompressedSize))
}

func TestCompressorWithExpandingTx(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := NewBlobMaker(120*1024, testDictPath)
	assert.NoError(err, "init should succeed")
	dict, err := os.ReadFile(testDictPath)
	assert.NoError(err)

	// get a block with a malicious transaction
	maliciousBlock := makeMaliciousBlock(dict, 100*1024)

	var buf bytes.Buffer

	// get the uncompressed size of the malicious block
	EncodeBlockForCompression(maliciousBlock, &buf)
	rawLen := buf.Len()
	assert.Less(rawLen, 120*1024, "malicious block should be smaller than 120kb")

	// compress it directly with lzss to get the expected compressed size with the best compression ratio
	{
		bm, err := lzss.NewCompressor(dict, lzss.BestCompression)
		assert.NoError(err)

		compressed, err := bm.Compress(buf.Bytes())
		assert.NoError(err)

		assert.Greater(len(compressed), 120*1024, "malicious block compressed should be larger than 120kb")
	}

	// RLP encode it to simulate a valid input to the bm
	buf.Reset()
	rlp.Encode(&buf, maliciousBlock)
	bMaliciousBlock := make([]byte, buf.Len())
	copy(bMaliciousBlock, buf.Bytes())

	// call to Write should not error;
	ok, err := bm.Write(bMaliciousBlock, false)
	assert.NoError(err)
	assert.True(ok, "malicious block should be appended")

	// len of the compressed data should fit in 120kb
	assert.Less(bm.Len(), 120*1024, "malicious block with no compression should be smaller than 120kb")

	// now we want to check that the compression level is unchanged if we revert;
	bm.Reset()
	buf.Reset()
	fakeBlock := makeFakeBlock(10 * 1024)
	EncodeBlockForCompression(fakeBlock, &buf)
	rawLen = buf.Len()
	assert.Less(rawLen, 120*1024, "fake block should be smaller than 120kb")
	assert.Greater(rawLen, 10*1024, "fake block should be larger than 50kb")

	buf.Reset()
	rlp.Encode(&buf, fakeBlock)
	ok, err = bm.Write(buf.Bytes(), false)
	assert.NoError(err)
	assert.True(ok, "fake block should be appended")

	// len of the compressed data should fit in 1 (even less...)
	l := bm.Len()
	assert.Less(l, 1*1024, "fake block with no compression should be smaller than 1kb")

	// write the malicious block again, but with forceReset = true
	// this should say "yes we can append this block (by bypassing compression)"
	// but since we force reset, the len of the bm shouldn't change.
	ok, err = bm.Write(bMaliciousBlock, true)
	assert.NoError(err)
	assert.True(ok, "malicious block should be writeable")

	// bm len should be unchanged
	assert.Equal(l, bm.Len(), "bm len should be unchanged")
	ok, err = bm.Write(buf.Bytes(), false)
	assert.NoError(err)
	assert.True(ok, "fake block should be appended again")

	// len of compressed data should not have grown more than rawLen/2
	assert.Less(bm.Len(), l+rawLen/2, "compression rate restore failed")

}

func TestCompressorWithDecompressorLimit(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := NewBlobMaker(120*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// get a block that should compress very well and should fit in the blob.
	targetSize := (MaxUncompressedBytes) / 8
	assert.Less(targetSize, bm.limit)
	block1 := makeFakeBlock(targetSize)

	var buf bytes.Buffer
	// get the uncompressed size of the block
	EncodeBlockForCompression(block1, &buf)
	rawLen := buf.Len()
	assert.Less(rawLen, MaxUncompressedBytes, "block1 should be smaller than maxUncompressedSize")

	assert.Greater(rawLen*8, MaxUncompressedBytes)

	// RLP encode it to simulate a valid input to the bm
	buf.Reset()
	rlp.Encode(&buf, block1)
	blockBytes := make([]byte, buf.Len())
	copy(blockBytes, buf.Bytes())

	// write once, it should not error and we get the expected compressed size
	ok, err := bm.Write(blockBytes, false)
	assert.NoError(err)
	assert.True(ok, "block should be appended")

	roughSizeForOneBlock := bm.Len()

	// shout compress so well that it should fit in 2kB
	assert.Less(roughSizeForOneBlock, 2*1024, "block compressed should be less than 2kB")

	// the next 6 writes should work --
	// we should be able to append 6 more blocks, and the size should be less than 12kB
	for i := 0; i < 6; i++ {
		ok, err = bm.Write(blockBytes, false)
		assert.NoError(err)
		assert.True(ok, "block should be appended")
	}

	assert.Less(bm.Len(), 12*1024, "6 blocks should be less than 12kB") // i.e. we have plenty of space left.

	// 7th write should fail; we are over the maxUncompressedSize
	ok, err = bm.Write(blockBytes, false)
	assert.NoError(err)
	assert.False(ok, "block should NOT be appended")

}

func TestCompressedSizeEstimation(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := NewBlobMaker(120*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	for _, block := range testBlocks {

		// Write the block to the blob maker and get the effective compressed size
		bm.Reset()
		ok, err := bm.Write(block, false)
		assert.NoError(err)
		assert.True(ok, "block should be appended")

		// ~due to padding, the effective size will be rounded
		// (but it is rounded too in the estimation, so it is fine)
		effectiveCompressedSize := packAlignSizeToRefactor(bm.compressor.Len(), 0)

		bm.Reset()

		// Perform the same operation using the WorstCompressedBlockSize function
		expandingBlock, worstCompressedSize, err := bm.WorstCompressedBlockSize(block)
		assert.NoError(err)
		assert.Less(worstCompressedSize, len(block), "compressed size should be less than block size")

		if effectiveCompressedSize != worstCompressedSize {
			// can only happen if we have an expending malicious block, not tested by this method.
			// the strategy of the blob maker is to bypass compression only when it detects that the
			// latest append doesn't fit in the blob, whereas this test only considers 1 block at a time
			// for size estimation.
			assert.True(expandingBlock, "mismatch in size estimation")
		}
	}
}

func BenchmarkWrite(b *testing.B) {

	// Init bm
	// Init bm
	bm, err := NewBlobMaker(120*1024, testDictPath)
	if err != nil {
		b.Fatal("init should succeed", err.Error())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 49; j++ {
			bm.Write(testBlocks[j], false)
		}
	}
}

// testBlocks is a slice of RLP encoded blocks
var testBlocks [][]byte

func init() {
	const testDataDir = "../../../../../testdata/prover-v2/prover-execution/requests"
	const rlpBlockBinDestination = "../../../../../jvm-libs/blob-compressor/src/test/resources/net/consensys/linea/nativecompressor/rlp_blocks.bin"

	jsons, err := utils.ReadAllJsonFiles(testDataDir)
	if err != nil {
		panic(err)
	}

	for _, jsonString := range jsons {
		var proverInput execution.Request
		if err = json.Unmarshal(jsonString, &proverInput); err != nil {
			panic(err)
		}

		for _, block := range proverInput.Blocks() {
			var bb bytes.Buffer
			if err = block.EncodeRLP(&bb); err != nil {
				panic(err)
			}
			testBlocks = append(testBlocks, bb.Bytes())
		}
	}

	// writes the rlp_block.bin
	f := files.MustOverwrite(rlpBlockBinDestination)
	binary.Write(f, binary.LittleEndian, uint32(len(testBlocks)))
	for i := range testBlocks {
		binary.Write(f, binary.LittleEndian, uint32(len(testBlocks[i])))
		f.Write(testBlocks[i])
	}
	f.Close()
}

func decompressBlob(b []byte) ([][][]byte, error) {

	// we should be able to hash the blob with MiMC with no errors;
	// this is a good indicator that the blob is valid.
	if len(b)%fr.Bytes != 0 {
		return nil, errors.New("invalid blob length; not a multiple of 32")
	}

	// ensure we can hash the blob with MiMC
	if _, err := mimc.NewMiMC().Write(b); err != nil {
		return nil, fmt.Errorf("can't hash with MiMC: %w", err)
	}

	dict, err := os.ReadFile(testDictPath)
	if err != nil {
		return nil, fmt.Errorf("can't read dict: %w", err)
	}
	dictStore, err := dictionary.SingletonStore(dict, 0)
	if err != nil {
		return nil, err
	}
	header, _, blocks, err := DecompressBlob(b, dictStore)
	if err != nil {
		return nil, fmt.Errorf("can't decompress blob: %w", err)
	}

	batches := make([][][]byte, len(header.table))
	offset := 0
	for i, batch := range header.table {
		batches[i] = make([][]byte, len(batch))
		for j := range batch {
			batches[i][j] = blocks[offset]
			offset++
		}
	}

	return batches, nil
}

func signTxFake(tx **types.Transaction) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	if *tx, err = types.SignTx(*tx, types.NewEIP155Signer(nil), privateKey); err != nil {
		panic(err)
	}
}

// makeMaliciousBlock encodes a block with a malicious transaction;
// this transaction is crafted to expand when compressed.
func makeMaliciousBlock(dict []byte, targetSize int) *types.Block {
	// craft a malicious data blob
	maliciousData := craftExpandingInput(dict, targetSize)

	address := common.HexToAddress("0x000042")

	maliciousTx := types.NewTx(&types.LegacyTx{
		Nonce: 42,
		Gas:   0,
		To:    &address,
		Value: big.NewInt(0),
		Data:  maliciousData,
	})

	signTxFake(&maliciousTx)

	return types.NewBlock(&types.Header{}, &types.Body{Transactions: []*types.Transaction{maliciousTx}, Uncles: nil, Withdrawals: nil}, nil, trie.NewStackTrie(nil))
}

// makeFakeBlock encodes a block with a fake transaction;
// that compresses very well (full of 0s)
func makeFakeBlock(targetSize int) *types.Block {
	address := common.HexToAddress("0x000042")
	tx := types.NewTx(&types.LegacyTx{
		Nonce: 42,
		Gas:   0,
		To:    &address,
		Value: big.NewInt(0),
		Data:  make([]byte, targetSize),
	})
	signTxFake(&tx)
	return types.NewBlock(&types.Header{}, &types.Body{Transactions: []*types.Transaction{tx}, Uncles: nil, Withdrawals: nil}, nil, trie.NewStackTrie(nil))
}

// adapted from lzss tests.
func craftExpandingInput(dict []byte, size int) []byte {
	const nbBytesExpandingBlock = 4

	// the following two methods convert between a byte slice and a number; just for convenient use as map keys and counters
	bytesToNum := func(b []byte) uint64 {
		var res uint64
		for i := range b {
			res += uint64(b[i]) << uint64(i*8)
		}
		return res
	}

	fillNum := func(dst []byte, n uint64) {
		for i := range dst {
			dst[i] = byte(n)
			n >>= 8
		}
	}

	covered := make(map[uint64]struct{}) // combinations present in the dictionary, to avoid
	for i := range dict {
		if dict[i] == 255 {
			covered[bytesToNum(dict[i+1:i+nbBytesExpandingBlock])] = struct{}{}
		}
	}
	isCovered := func(n uint64) bool {
		_, ok := covered[n]
		return ok
	}

	res := make([]byte, size)
	var blockCtr uint64
	for i := 0; i < len(res); i += nbBytesExpandingBlock {
		for isCovered(blockCtr) {
			blockCtr++
			if blockCtr == 0 {
				panic("overflow")
			}
		}
		res[i] = 255
		fillNum(res[i+1:i+nbBytesExpandingBlock], blockCtr)
		blockCtr++
		if blockCtr == 0 {
			panic("overflow")
		}
	}
	return res
}

func TestPack(t *testing.T) {
	assert := require.New(t)
	var buf bytes.Buffer
	var rng = rand.New(rand.NewChaCha8([32]byte{}))

	for i := 0; i < 100; i++ {
		// create 2 random slices
		n1 := rng.IntN(100) + 1 // #nosec G404 -- false positive
		n2 := rng.IntN(100) + 1 // #nosec G404 -- false positive

		s1 := make([]byte, n1)
		s2 := make([]byte, n2)

		utils.ReadPseudoRand(rng, s1)
		utils.ReadPseudoRand(rng, s2)

		// pack them
		buf.Reset()
		written, err := PackAlign(&buf, s1, s2)
		assert.NoError(err, "pack should not generate an error")
		assert.Equal(PackAlignSize(s1, s2), int(written), "written bytes should match expected PackAlignSize")
		original, err := UnpackAlign(buf.Bytes())
		assert.NoError(err, "unpack should not generate an error")

		assert.Equal(s1, original[:n1], "slices should match")
		assert.Equal(s2, original[n1:], "slices should match")
	}
}

func TestEncode(t *testing.T) {
	var block types.Block
	assert.NoError(t, rlp.DecodeBytes(testBlocks[0], &block))
	tx := block.Transactions()[0]
	var bb bytes.Buffer
	assert.NoError(t, EncodeTxForCompression(tx, &bb))

	var from common.Address
	txBackData, err := DecodeTxFromUncompressed(bytes.NewReader(slices.Clone(bb.Bytes())), &from)
	assert.NoError(t, err)
	txBack := types.NewTx(txBackData)

	encodeTesting.CheckSameTx(t, tx, txBack, from)
}
