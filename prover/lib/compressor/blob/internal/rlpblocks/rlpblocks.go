package rlpblocks

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
)

var Get = sync.OnceValue(func() [][]byte {
	rootPath, err := test_utils.GetRepoRootPath()
	if err != nil {
		panic(err)
	}

	rlpBlocksBinPath := filepath.Join(rootPath, "coordinator/core/src/test/resources/net/consensys/zkevm/coordination/blob/rlp_blocks.bin")

	// Try to read execution response data and generate blocks
	var blocks [][]byte
	if err := func() error {
		testDataDir := filepath.Join(rootPath, "testdata/prover-v2/prover-execution/requests")
		entries, err := os.ReadDir(testDataDir)
		if err != nil {
			return fmt.Errorf("could not read test data directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			jsonString, err := os.ReadFile(filepath.Join(testDataDir, entry.Name()))
			if err != nil {
				return fmt.Errorf("could not read file %s: %w", entry.Name(), err)
			}
			var proverInput execution.Request
			if err = json.Unmarshal(jsonString, &proverInput); err != nil {
				return fmt.Errorf("could not decode json prover input from %s: %w", entry.Name(), err)
			}

			for _, block := range proverInput.Blocks() {
				var bb bytes.Buffer
				if err = block.EncodeRLP(&bb); err != nil {
					return fmt.Errorf("could not encode rlp block: %w", err)
				}
				blocks = append(blocks, bb.Bytes())
			}
		}
		return nil
	}(); err != nil {
		// Log the error and try to read the existing rlp_blocks.bin file
		log.Printf("Error reading execution response data: %v\n", err)
		log.Println("Attempting to read existing rlp_blocks.bin file.")

		data, err := os.ReadFile(rlpBlocksBinPath)
		if err != nil {
			panic(fmt.Errorf("failed to read rlp_blocks.bin: %w", err))
		}

		if len(data) < 4 {
			panic(fmt.Errorf("rlp_blocks.bin file too short to contain block count"))
		}

		buf := bytes.NewReader(data)
		var blockCount uint32
		if err := binary.Read(buf, binary.LittleEndian, &blockCount); err != nil {
			panic(fmt.Errorf("could not read block count from existing file: %w", err))
		}

		blocks := make([][]byte, 0, blockCount)
		for i := range blockCount {
			var blockSize uint32
			if err := binary.Read(buf, binary.LittleEndian, &blockSize); err != nil {
				panic(fmt.Errorf("could not read block %d size from existing file: %w", i, err))
			}

			block := make([]byte, blockSize)
			if _, err := buf.Read(block); err != nil {
				panic(fmt.Errorf("could not read block %d data from existing file: %w", i, err))
			}
			blocks = append(blocks, block)
		}

		log.Printf("Successfully read %d blocks from existing rlp_blocks.bin file", len(blocks))
		return blocks
	}

	// Write rlp_blocks.bin for JVM tests
	f := files.MustOverwrite(rlpBlocksBinPath)
	binary.Write(f, binary.LittleEndian, uint32(len(blocks)))
	for i := range blocks {
		binary.Write(f, binary.LittleEndian, uint32(len(blocks[i])))
		f.Write(blocks[i])
	}
	f.Close()

	return blocks
})
