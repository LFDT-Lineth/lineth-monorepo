//go:build !fuzzlight

package v1_test

import (
	"bytes"
	cRand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"

	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1/test_utils"

	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/stretchr/testify/assert"

	"github.com/consensys/compress/lzss"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/require"
)

const testDictPath = "../../compressor_dict.bin"

func TestCompressorOneBlock(t *testing.T) { // most basic test just to see if block encoding/decoding works
	testCompressorSingleSmallBatch(t, testBlocks[1:2])
}

func TestCompressorTwoBlocks(t *testing.T) { // most basic test just to see if block encoding/decoding works
	testCompressorSingleSmallBatch(t, testBlocks[:2])
}

func testCompressorSingleSmallBatch(t *testing.T, blocks [][]byte) {
	bm, err := v1.NewBlobMaker(64*1024, testDictPath)
	assert.NoError(t, err, "init should succeed")

	for _, block := range blocks {
		ok, err := bm.Write(block, false)
		assert.NoError(t, err)
		assert.True(t, ok, "block should be appended")
	}

	dict, err := os.ReadFile(testDictPath)
	assert.NoError(t, err)
	dictStore, err := dictionary.SingletonStore(dict, 1)
	_, _, blocksBack, err := v1.DecompressBlob(bm.Bytes(), dictStore)
	assert.NoError(t, err)
	assert.Equal(t, len(blocks), len(blocksBack), "number of blocks should match")
	// TODO compare the blocks
}

func TestCompressorNoBatches(t *testing.T) {
	assert := require.New(t)

	// Init bm
	bm, err := v1.NewBlobMaker(64*1024, testDictPath)
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

		// decompress the batches
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
		assert.NoError(t, v1.EncodeBlockForCompression(&block, &encoded))
		assertBatchesConsistent(t, [][]byte{blockRaw}, [][]byte{encoded.Bytes()})
	}
}

func assertBatchesConsistent(t *testing.T, raw, decoded [][]byte) {
	assert.Equal(t, len(raw), len(decoded), "number of blocks should match")
	for i := range raw {
		var block types.Block
		assert.NoError(t, rlp.Decode(bytes.NewReader(raw[i]), &block))

		blockBack, err := v1.DecodeBlockFromUncompressed(bytes.NewReader(decoded[i]))
		assert.NoError(t, err)
		assert.Equal(t, block.Time(), blockBack.Timestamp, "block time should match")
	}
}

func testCanWriteMutates(assert *require.Assertions, bm *v1.BlobMaker, block []byte) (canWrite bool) {
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
	bm, err := v1.NewBlobMaker(64*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// Compress blocks
	var blobs [][]byte
	var nbBlocksPerBatch []uint16 // tracking number of blocks, no batch in this test.
	cptBlock := 0
	for i, block := range testBlocks {
		// get a random from 1 to 5
		bSize := rand.Intn(3) + 1 // #nosec G404 -- false positive

		if cptBlock > bSize && i%3 == 0 {
			nbBlocksPerBatch = append(nbBlocksPerBatch, uint16(cptBlock))
			cptBlock = 0
			bm.StartNewBatch()
		}
	reprocessBlock:
		// ensure that CanWrite (writeGo(block, true)) never mutates the state
		canWrite := testCanWriteMutates(assert, bm, block)
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
				v1.EncodeBlockForCompression(&block, &buf)
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
	bm, err := v1.NewBlobMaker(120*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// Compress blocks
	var blobs [][]byte
	var nbBlocksPerBatch []uint16 // tracking number of blocks, no batch in this test.
	cptBlock := 0
	for i, block := range testBlocks {
		t.Logf("processing block %d over %d", i, len(testBlocks))
		// get a random from 1 to 5
		bSize := rand.Intn(5) + 1 // #nosec G404 -- false positive

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
				v1.EncodeBlockForCompression(&block, &buf)
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
	const blobSizeLimit = 120 * 1024
	assert := require.New(t)

	// Init bm
	bm, err := v1.NewBlobMaker(blobSizeLimit, testDictPath)
	assert.NoError(err, "init should succeed")
	dict, err := os.ReadFile(testDictPath)
	assert.NoError(err)

	// get a block with a malicious transaction
	maliciousBlock := makeMaliciousBlock(dict, 100*1024)

	var buf bytes.Buffer

	// get the uncompressed size of the malicious block
	if err := v1.EncodeBlockForCompression(maliciousBlock, &buf); err != nil {
		t.Fatal(err)
	}
	rawLen := buf.Len()
	assert.Less(rawLen, blobSizeLimit, "malicious block should be smaller than 120kb")

	// compress it directly with lzss to get the expected compressed size with the best compression ratio
	{
		bm, err := lzss.NewCompressor(dict)
		assert.NoError(err)

		compressed, err := bm.Compress(buf.Bytes())
		assert.NoError(err)

		assert.Greater(len(compressed), blobSizeLimit, "malicious block compressed should be larger than 120kb")
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
	assert.Less(bm.Len(), blobSizeLimit, "malicious block with no compression should be smaller than 120kb")

	// now we want to check that the compression level is unchanged if we revert;
	bm.Reset()
	buf.Reset()
	fakeBlock := makeFakeBlock(10 * 1024)
	v1.EncodeBlockForCompression(fakeBlock, &buf)
	rawLen = buf.Len()
	assert.Less(rawLen, blobSizeLimit, "fake block should be smaller than 120kb")
	assert.Greater(rawLen, 10*1024, "fake block should be larger than 10kb")

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
	bm, err := v1.NewBlobMaker(120*1024, testDictPath)
	assert.NoError(err, "init should succeed")

	// get a block that should compress very well and should fit in the blob.
	targetSize := (v1.MaxUncompressedBytes) / 8
	assert.Less(targetSize, bm.Limit)
	block1 := makeFakeBlock(int(targetSize))

	var buf bytes.Buffer
	// get the uncompressed size of the block
	v1.EncodeBlockForCompression(block1, &buf)
	rawLen := buf.Len()
	assert.Less(rawLen, int(v1.MaxUncompressedBytes), "block1 should be smaller than MaxBlobPayloadBytes")

	assert.Greater(rawLen*8, int(v1.MaxUncompressedBytes))

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

func BenchmarkWrite(b *testing.B) {

	// Init bm
	// Init bm
	bm, err := v1.NewBlobMaker(120*1024, testDictPath)
	if err != nil {
		b.Fatal("init should succeed", err.Error())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(testBlocks); j++ {
			bm.Write(testBlocks[j], false)
		}
	}
}

// testBlocks is a slice of RLP encoded blocks
var testBlocks [][]byte

func init() {
	rootPath, err := test_utils.GetRepoRootPath()
	if err != nil {
		panic(err)
	}

	if testBlocks, err = test_utils.LoadTestBlocks(filepath.Join(rootPath, "testdata/prover-v2/prover-execution/requests")); err != nil {
		panic(err)
	}

	// writes the rlp_block.bin
	f := files.MustOverwrite(filepath.Join(rootPath, "jvm-libs/blob-compressor/src/test/resources/net/consensys/linea/nativecompressor/rlp_blocks.bin"))
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

	dict, err := os.ReadFile(testDictPath)
	if err != nil {
		return nil, fmt.Errorf("can't read dict: %w", err)
	}
	dictStore, err := dictionary.SingletonStore(dict, 1)
	if err != nil {
		return nil, err
	}
	header, _, blocks, err := v1.DecompressBlob(b, dictStore)
	if err != nil {
		return nil, fmt.Errorf("can't decompress blob: %w", err)
	}

	batches := make([][][]byte, len(header.BatchSizes))
	for i, batchNbBytes := range header.BatchSizes {
		batches[i] = make([][]byte, 0)
		batchLenYet := 0
		for batchLenYet < batchNbBytes {
			batches[i] = append(batches[i], blocks[0])
			batchLenYet += len(blocks[0])
			blocks = blocks[1:]
		}
		if batchLenYet != batchNbBytes {
			return nil, errors.New("invalid batch size")
		}
	}
	if len(blocks) != 0 {
		return nil, errors.New("not all blocks were consumed")
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

	runTest := func(s1, s2 []byte) {
		// pack them
		buf.Reset()
		written, err := encode.PackAlign(&buf, s1, fr381.Bits-1, encode.WithAdditionalInput(s2))
		assert.NoError(err, "pack should not generate an error")
		assert.Equal(encode.PackAlignSize(len(s1)+len(s2), fr381.Bits-1), int(written), "written bytes should match expected PackAlignSize")
		original, err := encode.UnpackAlign(buf.Bytes(), fr381.Bits-1, false)
		assert.NoError(err, "unpack should not generate an error")

		assert.Equal(s1, original[:len(s1)], "slices should match")
		assert.Equal(s2, original[len(s1):], "slices should match")
	}

	decodeHex := func(s string) []byte {
		res, err := hex.DecodeString(s)
		assert.NoError(err)
		return res
	}

	runTest([]byte{1}, []byte{})

	runTest(decodeHex("76fd8aaf309f74f0"), decodeHex("10fb80f4bbed040df7ebcde55331ce7d6c2f5f0ba190b4"))

	for i := 0; i < 100; i++ {
		// create 2 random slices
		n1 := rand.Intn(100) + 1 // #nosec G404 -- false positive
		n2 := rand.Intn(100) + 1 // #nosec G404 -- false positive

		s1 := make([]byte, n1)
		s2 := make([]byte, n2)

		cRand.Read(s1)
		cRand.Read(s2)

		runTest(s1, s2)
	}
}
