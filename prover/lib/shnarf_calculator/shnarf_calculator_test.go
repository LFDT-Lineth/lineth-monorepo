//go:build !fuzzlight

package main_test

import (
	"encoding/base64"
	"encoding/binary"
	"io"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/require"
)

const (
	blockRlpBin       = "../../../jvm-libs/blob-compressor/src/test/resources/net/consensys/linea/nativecompressor/rlp_blocks.bin"
	compressorDictBin = "../compressor/compressor_dict.bin"
)

func TestShnarfCalculatorEIP4844(t *testing.T) {

	assert := require.New(t)

	blocks := mustLoadRlpBin(blockRlpBin)

	// Initialize the compressor
	compressor, err := blob.NewBlobMaker(64*1024, compressorDictBin)
	assert.NoError(err, "could not instantiate the compressor")

	// Compress the blocks as much as possible to create one blob
	for _, block := range blocks {
	reprocessBlock:
		canAppendMore, err := compressor.Write(block, false)
		assert.NoError(err, "while attempting to write a block in the compressor")
		if !canAppendMore {
			// we can make a blob.
			compressedData := compressor.Bytes()

			// Create the corresponding shnarf calculator request
			shnarfCalculatorReq := blobsubmission.Request{
				Eip4844Enabled:      true,
				CompressedData:      base64.StdEncoding.EncodeToString(compressedData),
				DataParentHash:      types.FullBytes32([32]byte{}).Hex(),
				ParentStateRootHash: types.FullBytes32([32]byte{}).Hex(),
				FinalStateRootHash:  types.FullBytes32([32]byte{}).Hex(),
				PrevShnarf:          types.FullBytes32([32]byte{}).Hex(),
				ConflationOrder: blobsubmission.ConflationOrder{
					StartingBlockNumber: 0,
					UpperBoundaries:     []int{10, 20},
				},
			}

			// Attempts to get a response from it
			_, err = blobsubmission.CraftResponse(&shnarfCalculatorReq)
			assert.NoError(err, "while attempting to craft a response from the shnarf calculator")

			compressor.Reset()
			goto reprocessBlock
		}
	}
}

func TestShnarfCalculatorCalldata(t *testing.T) {

	assert := require.New(t)

	blocks := mustLoadRlpBin(blockRlpBin)

	// Initialize the compressor
	compressor, err := blob.NewBlobMaker(64*1024, compressorDictBin)
	assert.NoError(err, "could not instantiate the compressor")

	// Compress the blocks as much as possible to create one blob
	for _, block := range blocks {
	reprocessBlock:
		canAppendMore, err := compressor.Write(block, false)
		assert.NoError(err, "while attempting to write a block in the compressor")
		if !canAppendMore {
			// we can make a blob.
			compressedData := compressor.Bytes()

			// Create the corresponding shnarf calculator request
			shnarfCalculatorReq := blobsubmission.Request{
				Eip4844Enabled:      false,
				CompressedData:      base64.StdEncoding.EncodeToString(compressedData),
				DataParentHash:      types.FullBytes32([32]byte{}).Hex(),
				ParentStateRootHash: types.FullBytes32([32]byte{}).Hex(),
				FinalStateRootHash:  types.FullBytes32([32]byte{}).Hex(),
				PrevShnarf:          types.FullBytes32([32]byte{}).Hex(),
				ConflationOrder: blobsubmission.ConflationOrder{
					StartingBlockNumber: 0,
					UpperBoundaries:     []int{10, 20},
				},
			}

			// Attempts to get a response from it
			_, err = blobsubmission.CraftResponse(&shnarfCalculatorReq)
			assert.NoError(err, "while attempting to craft a response from the shnarf calculator")

			compressor.Reset()
			goto reprocessBlock
		}
	}
}

func mustLoadRlpBin(fPath string) [][]byte {

	f, err := os.Open(fPath)
	if err != nil {
		utils.Panic("while loading the rlp.bin: %v", err.Error())
	}
	defer f.Close()

	// read number of blocks
	var nbBlocks uint32
	if err := binary.Read(f, binary.LittleEndian, &nbBlocks); err != nil {
		utils.Panic("while reading the number of blocks: %v", err.Error())
	}

	if nbBlocks > 25000 {
		nbBlocks = 25000
	}

	// read blocks
	blocks := make([][]byte, nbBlocks)
	for i := range blocks {
		var blockLength uint32
		if err := binary.Read(f, binary.LittleEndian, &blockLength); err != nil {
			utils.Panic("while trying to read the block length: %v", err.Error())
		}
		blocks[i] = make([]byte, blockLength)
		if _, err := io.ReadFull(f, blocks[i]); err != nil {
			utils.Panic("while trying to read the block data")
		}
	}

	return blocks
}
