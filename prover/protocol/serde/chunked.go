package serde

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/pierrec/lz4/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/consensys/linea-monorepo/prover/utils/profiling"
)

// Chunked asset format
//
// Instead of a single zstd-compressed file, we split the serialized data into
// independently-compressed lz4 chunks stored in a dedicated subdirectory.
// This enables:
//   - Parallel decompression across cores (lz4 is ~4 GB/s/core)
//   - Parallel disk reads (saturates EFS/NVMe bandwidth)
//   - Each chunk decompresses directly into its target mmap offset (zero-copy)
//   - Clean directory structure: hundreds of chunk files stay inside their asset folder
//
// On-disk layout (basePath = ".../execution-limitless/dw-compiled-gl-KECCAK"):
//
//	execution-limitless/
//	├── dw-compiled-gl-KECCAK/       ← chunked asset directory
//	│   ├── manifest                 ← binary: decompressed size + chunk table
//	│   ├── chunk-0000.lz4           ← independently compressed chunk 0
//	│   ├── chunk-0001.lz4
//	│   └── ...
//	├── disc.bin                     ← small assets stay as flat files
//	└── dw-blueprint-gl-0.bin

const (
	// DefaultChunkSize is the decompressed size per chunk (256 MB).
	// Chosen to balance parallelism (enough chunks for 32 goroutines) against
	// per-chunk overhead (file open/close, lz4 frame headers).
	DefaultChunkSize = 256 << 20

	// manifestMagic identifies a chunked asset manifest.
	manifestMagic uint32 = 0x43484E4B // "CHNK"
)

// chunkManifest is the binary header of the .chunked manifest file.
// All fields are little-endian on disk.
type chunkManifest struct {
	Magic          uint32 // manifestMagic
	NumChunks      uint32 // number of chunk files
	DecompressedSz uint64 // total decompressed size in bytes
	// Followed by NumChunks entries of chunkEntry.
}

// chunkEntry describes one chunk in the manifest.
type chunkEntry struct {
	Offset       uint64 // byte offset into the decompressed buffer
	DecompSz     uint32 // decompressed size of this chunk
	CompressedSz uint32 // compressed size on disk (informational)
}

// StoreChunked serializes the asset, splits into chunks, lz4-compresses each
// chunk independently, and writes them to disk alongside a manifest.
func StoreChunked(basePath string, asset any) error {
	return StoreChunkedWithSize(basePath, asset, DefaultChunkSize)
}

// StoreChunkedWithSize is like StoreChunked but allows overriding the chunk size.
func StoreChunkedWithSize(basePath string, asset any, chunkSize int) error {

	// 1. Serialize
	var data []byte
	var serErr error
	tSer := profiling.TimeIt(func() {
		data, serErr = Serialize(asset)
	})
	if serErr != nil {
		return fmt.Errorf("serialization failed: %w", serErr)
	}
	totalSize := len(data)
	logrus.Infof("Serialized %s (%s)", basePath, formatSize(totalSize))

	// 2. Determine chunk boundaries
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	numChunks := (totalSize + chunkSize - 1) / chunkSize
	if numChunks == 0 {
		numChunks = 1
	}

	// Create the asset subdirectory
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create chunked dir %s: %w", basePath, err)
	}

	entries := make([]chunkEntry, numChunks)

	// 3. Compress and write chunks in parallel
	maxWorkers := runtime.NumCPU()
	if maxWorkers > numChunks {
		maxWorkers = numChunks
	}
	eg := &errgroup.Group{}
	eg.SetLimit(maxWorkers)

	var compErr error
	var tComp time.Duration
	tComp = profiling.TimeIt(func() {
		for i := range numChunks {
			i := i
			eg.Go(func() error {
				start := i * chunkSize
				end := start + chunkSize
				if end > totalSize {
					end = totalSize
				}
				chunk := data[start:end]

				// lz4 compress
				compressed := make([]byte, lz4.CompressBlockBound(len(chunk)))
				n, err := lz4.CompressBlock(chunk, compressed, nil)
				if err != nil {
					return fmt.Errorf("lz4 compress chunk %d: %w", i, err)
				}
				if n == 0 {
					// Incompressible data: store raw with a marker.
					// lz4.CompressBlock returns 0 when the output would be
					// larger than the input. We store the raw chunk instead.
					compressed = chunk
					n = len(chunk)
				}
				compressed = compressed[:n]

				entries[i] = chunkEntry{
					Offset:       uint64(start),
					DecompSz:     uint32(end - start),
					CompressedSz: uint32(n),
				}

				chunkPath := filepath.Join(basePath, fmt.Sprintf("chunk-%04d.lz4", i))
				return atomicWrite(chunkPath, compressed)
			})
		}
		compErr = eg.Wait()
	})

	if compErr != nil {
		return compErr
	}

	// 4. Write manifest
	manifest := chunkManifest{
		Magic:          manifestMagic,
		NumChunks:      uint32(numChunks),
		DecompressedSz: uint64(totalSize),
	}
	manifestPath := filepath.Join(basePath, "manifest")
	if err := writeManifest(manifestPath, &manifest, entries); err != nil {
		return err
	}

	logrus.Infof("Stored chunked %s | %d chunks | Ser: %s, Comp+Write: %s",
		basePath, numChunks, tSer, tComp)

	return nil
}

// LoadChunkedMmapBacked reads a chunked asset from disk using parallel lz4
// decompression into an anonymous mmap buffer.
//
// It reads the manifest to learn the total decompressed size and chunk layout,
// then launches parallel goroutines that each:
//  1. Read their chunk file from disk
//  2. lz4-decompress directly into the target mmap offset
//
// Returns an MmapBackedBuffer that must be released by the caller.
func LoadChunkedMmapBacked(basePath string, assetPtr any) (*MmapBackedBuffer, error) {

	manifestPath := filepath.Join(basePath, "manifest")
	logrus.Infof("Loading chunked asset %s...", manifestPath)

	// 1. Read manifest
	manifest, entries, err := readManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	decompSize := int(manifest.DecompressedSz)
	numChunks := int(manifest.NumChunks)

	// 2. Allocate mmap buffer
	mmapData, err := syscall.Mmap(-1, 0, decompSize,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return nil, fmt.Errorf("mmap allocation failed (%s): %w", formatSize(decompSize), err)
	}

	// 3. Parallel read + decompress into mmap
	maxWorkers := runtime.NumCPU()
	if maxWorkers > 32 {
		maxWorkers = 32
	}
	if maxWorkers > numChunks {
		maxWorkers = numChunks
	}

	eg := &errgroup.Group{}
	eg.SetLimit(maxWorkers)

	var loadErr error
	var tLoad time.Duration
	tLoad = profiling.TimeIt(func() {
		for i := range numChunks {
			i := i
			eg.Go(func() error {
				entry := entries[i]
				chunkPath := filepath.Join(basePath, fmt.Sprintf("chunk-%04d.lz4", i))

				compressed, err := os.ReadFile(chunkPath)
				if err != nil {
					return fmt.Errorf("read chunk %d: %w", i, err)
				}

				dst := mmapData[entry.Offset : entry.Offset+uint64(entry.DecompSz)]

				if uint32(len(compressed)) == entry.DecompSz {
					// Was stored raw (incompressible)
					copy(dst, compressed)
				} else {
					n, err := lz4.UncompressBlock(compressed, dst)
					if err != nil {
						return fmt.Errorf("lz4 decompress chunk %d: %w", i, err)
					}
					if n != int(entry.DecompSz) {
						return fmt.Errorf("chunk %d: expected %d decompressed bytes, got %d", i, entry.DecompSz, n)
					}
				}
				return nil
			})
		}
		loadErr = eg.Wait()
	})

	if loadErr != nil {
		_ = syscall.Munmap(mmapData)
		return nil, loadErr
	}

	// 4. Deserialize
	var deserErr error
	tDeser := profiling.TimeIt(func() {
		deserErr = Deserialize(mmapData, assetPtr)
	})
	if deserErr != nil {
		_ = syscall.Munmap(mmapData)
		return nil, fmt.Errorf("deserialization failed: %w", deserErr)
	}

	logrus.Infof("Loaded chunked %s [Size: %s, Chunks: %d] | Load+Decomp: %s | Deser: %s",
		basePath, formatSize(decompSize), numChunks, tLoad, tDeser)

	return &MmapBackedBuffer{data: mmapData}, nil
}

// HasChunkedAsset returns true if a chunked manifest exists for the given base path.
func HasChunkedAsset(basePath string) bool {
	_, err := os.Stat(filepath.Join(basePath, "manifest"))
	return err == nil
}

// --- internal helpers ---

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	_ = tmp.Close()

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	tmpName = "" // disable cleanup
	return nil
}

func writeManifest(path string, m *chunkManifest, entries []chunkEntry) error {
	size := int(unsafe.Sizeof(*m)) + len(entries)*int(unsafe.Sizeof(chunkEntry{}))
	buf := make([]byte, size)

	binary.LittleEndian.PutUint32(buf[0:4], m.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], m.NumChunks)
	binary.LittleEndian.PutUint64(buf[8:16], m.DecompressedSz)

	off := 16
	for _, e := range entries {
		binary.LittleEndian.PutUint64(buf[off:off+8], e.Offset)
		binary.LittleEndian.PutUint32(buf[off+8:off+12], e.DecompSz)
		binary.LittleEndian.PutUint32(buf[off+12:off+16], e.CompressedSz)
		off += 16
	}

	return atomicWrite(path, buf)
}

func readManifest(path string) (*chunkManifest, []chunkEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	if len(data) < 16 {
		return nil, nil, fmt.Errorf("manifest too small: %d bytes", len(data))
	}

	m := &chunkManifest{
		Magic:          binary.LittleEndian.Uint32(data[0:4]),
		NumChunks:      binary.LittleEndian.Uint32(data[4:8]),
		DecompressedSz: binary.LittleEndian.Uint64(data[8:16]),
	}

	if m.Magic != manifestMagic {
		return nil, nil, fmt.Errorf("invalid manifest magic: 0x%08X", m.Magic)
	}

	expected := 16 + int(m.NumChunks)*16
	if len(data) < expected {
		return nil, nil, fmt.Errorf("manifest truncated: have %d bytes, need %d", len(data), expected)
	}

	entries := make([]chunkEntry, m.NumChunks)
	off := 16
	for i := range entries {
		entries[i] = chunkEntry{
			Offset:       binary.LittleEndian.Uint64(data[off : off+8]),
			DecompSz:     binary.LittleEndian.Uint32(data[off+8 : off+12]),
			CompressedSz: binary.LittleEndian.Uint32(data[off+12 : off+16]),
		}
		off += 16
	}

	return m, entries, nil
}
