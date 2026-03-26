package serde

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

var (

	// numConcurrentDiskWrite governs the number of concurrent disk writes
	numConcurrentDiskWrite = 1
	// diskWriteSemaphore limits concurrent writes to prevent IO congestion.
	// Even at 1, it ensures that massive assets (like Module 11) don't compete
	// for disk bandwidth, which is critical for mechanical or shared cloud drives.
	diskWriteSemaphore = semaphore.NewWeighted(int64(numConcurrentDiskWrite))

	// concurrentDiskRead governs the number of concurrent disk reads
	numConcurrentDiskRead = 1
	// diskReadSemaphore ensures we don't saturate the IO bus during heavy asset loading.
	diskReadSemaphore = semaphore.NewWeighted(int64(numConcurrentDiskRead))
)

// This function contains compile-time "guard rails" to ensure that critical binary-format structs do not
// change their layout. If the size or alignment of these structs changes, the compiler will throw an error.
//
// This is better than a Runtime Panic
// Zero Runtime Cost: These checks happen during go build or go test. The function _() is never actually called,
// so it adds zero bytes to your binary and zero nanoseconds to your execution time.
// Immediate Feedback: Developers get a red squiggle in their IDE (or a CI failure) the moment they accidentally
// touch a protected struct.
// Documentation: It serves as an "executable comment" that explicitly warns other developers about the binary
// requirements of the serde package.
func _() {

	// Array Size Trick: Since Go allows us to define an array length using a constant expression.
	// If that expression evaluates to a negative number, the compiler throws an error: array bound must be non-negative.
	// Size Checks: By doing [32 - unsafe.Sizeof(T)] and [unsafe.Sizeof(T) - 32], we create a "trap." If the size is 33,
	// the first line is [-1], which fails. If the size is 31, the second line is [-1], which fails. The code only compiles
	// if the size is exactly 32.
	// Offset Checks: unsafe.Offsetof allows us to verify that specific fields haven't moved. If someone swaps Len and Cap
	// in FileSlice, the Offsetof(fs.Len) check will trigger a compiler error because it will be 16 instead of 8.

	// --- Guard Rail for FileHeader (Must be exactly 32 bytes) ---
	// Layout: Magic(4) + Version(4) + PayloadType(8) + PayloadOff(8) + DataSize(8) = 32
	var _ [1]struct{}
	_ = [32 - unsafe.Sizeof(FileHeader{})]struct{}{}
	_ = [unsafe.Sizeof(FileHeader{}) - 32]struct{}{}

	// --- Guard Rail for InterfaceHeader (Must be exactly 16 bytes) ---
	// Layout: TypeID(2) + PtrIndirection(1) + Reserved(5) + Offset(8) = 16
	_ = [16 - unsafe.Sizeof(InterfaceHeader{})]struct{}{}
	_ = [unsafe.Sizeof(InterfaceHeader{}) - 16]struct{}{}

	// Check alignment of the 'Offset' field in InterfaceHeader (Must start at byte 8)
	// This ensures our explicit padding [5]uint8 is doing its job.
	var ih InterfaceHeader
	_ = [8 - unsafe.Offsetof(ih.Offset)]struct{}{}
	_ = [unsafe.Offsetof(ih.Offset) - 8]struct{}{}

	// --- Guard Rail for FileSlice (Must be exactly 24 bytes) ---
	// Layout: Offset(8) + Len(8) + Cap(8) = 24
	_ = [24 - unsafe.Sizeof(FileSlice{})]struct{}{}
	_ = [unsafe.Sizeof(FileSlice{}) - 24]struct{}{}

	// Check order: Offset must be at byte 0, Len at 8, Cap at 16
	var fs FileSlice
	_ = [0 - unsafe.Offsetof(fs.Offset)]struct{}{}
	_ = [8 - unsafe.Offsetof(fs.Len)]struct{}{}
	_ = [16 - unsafe.Offsetof(fs.Cap)]struct{}{}
}

// StoreToDisk serializes the asset and writes it atomically to disk.
func StoreToDisk(filePath string, asset any, withCompression bool) error {
	var (
		data     []byte
		serErr   error
		writeErr error
		tSer     time.Duration
		tComp    time.Duration
	)

	// --- 1. Serialization (Linearization) ---
	tSer = profiling.TimeIt(func() {
		data, serErr = Serialize(asset)
	})
	if serErr != nil {
		return fmt.Errorf("serialization failed: %w", serErr)
	}
	rawSize := len(data)

	// --- 2. Optional Compression (Zstd) ---
	if withCompression {
		tComp = profiling.TimeIt(func() {
			var buf bytes.Buffer
			encoder, err := zstd.NewWriter(&buf)
			if err != nil {
				panic(err)
			}
			_, _ = encoder.Write(data)
			_ = encoder.Close()
			data = buf.Bytes()
		})
		logrus.Debugf("Compression: %s -> %s", formatSize(rawSize), formatSize(len(data)))
	}

	// --- 3. Atomic Write Strategy ---
	// We write to a .tmp file and rename it. This ensures that if the prover crashes
	// during a write, we don't leave a corrupted asset on disk.
	dir := filepath.Dir(filePath)
	_ = os.MkdirAll(dir, 0755)

	tmpFile, err := os.CreateTemp(dir, filepath.Base(filePath)+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	// Cleanup closure: delete the temp file if we don't finish the Rename.
	defer func() {
		_ = tmpFile.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	_ = diskWriteSemaphore.Acquire(context.Background(), 1)
	tWrite := profiling.TimeIt(func() {
		_, writeErr = tmpFile.Write(data)
		if writeErr == nil {
			writeErr = tmpFile.Sync() // Ensure bits are actually on the platter.
		}
	})
	diskWriteSemaphore.Release(1)

	if writeErr != nil {
		return fmt.Errorf("write/sync failed: %w", writeErr)
	}

	_ = tmpFile.Close()

	// Atomic Rename: On POSIX, this replaces the file instantly.
	// #nosec G703 -- path traversal are not a concern as this is not meant to be run in a browser.
	if err := os.Rename(tmpName, filePath); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}
	tmpName = "" // Success: disable the defer removal.

	// Optional: Sync the directory itself to ensure the directory entry is durable.
	if dfd, err := os.Open(dir); err == nil {
		_ = dfd.Sync()
		_ = dfd.Close()
	}

	logrus.Infof("Saved %s | Total: %s (Ser: %s, Comp: %s, DiskIO: %s)",
		filePath, formatSize(len(data)), tSer, tComp, tWrite)

	return nil
}

// LoadFromDisk loads a serialized asset from disk.
// Returns an io.Closer that MUST be closed by the caller to release the Mmap region.
// NOTE: The requirement to return an io.Closer is driven entirely by the Uncompressed (Mmap) Path.
// In that path, the memory is borrowed from the OS kernel, and we must have a way to tell the OS to unmap it (Munmap)
// when we are done. For compressed path, we use a default io.NopCloser.
func LoadFromDisk(filePath string, assetPtr any, withCompression bool) (io.Closer, error) {
	var (
		mfile    *MappedFile
		data     []byte
		readErr  error
		deserErr error
		tDecomp  time.Duration
	)

	// We use a NopCloser as the default for the "Compressed" path where data is in Heap.
	// When using Mmap (the uncompressed path), the memory isn't actually "allocated"—it's borrowed from the OS
	// The closer that we set for the uncompressed path is essentially a handle to that "loan."
	// Calling closer.Close() is the signal to the OS that you are done with the memory.
	// See Ln.177 we set closer to mfile for uncompressed path
	var closer io.Closer = io.NopCloser(nil)

	// --- 1. Data Acquisition Phase ---
	if withCompression {
		// Path A: Compressed (Must load into Heap memory)
		logrus.Infof("Loading compressed asset %s...", filePath)

		_ = diskReadSemaphore.Acquire(context.Background(), 1)
		compressedData, err := os.ReadFile(filePath)
		diskReadSemaphore.Release(1)

		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		tDecomp = profiling.TimeIt(func() {
			// Zstd is significantly faster than LZ4 for high-compression ratios in Prover assets.
			decoder, err := zstd.NewReader(bytes.NewReader(compressedData))
			if err != nil {
				panic(err)
			}
			defer decoder.Close()
			data, readErr = io.ReadAll(decoder)
		})

		if readErr != nil {
			return nil, fmt.Errorf("decompression failed: %w", readErr)
		}

	} else {
		// Path B: Uncompressed (Zero-Copy Mmap)
		logrus.Infof("Memory-mapping asset %s...", filePath)

		_ = diskReadSemaphore.Acquire(context.Background(), 1)
		tMmap := profiling.TimeIt(func() {
			mfile, readErr = openMappedFile(filePath)
		})
		diskReadSemaphore.Release(1)

		if readErr != nil {
			return nil, fmt.Errorf("mmap failed: %w", readErr)
		}

		data = mfile.Data()
		closer = mfile // The caller must call Close() to trigger Munmap.
		logrus.Debugf("Mmap established in %s", tMmap)
	}

	// --- 2. Deserialization (Overlay/Swizzle) Phase ---
	// This is where the magic happens: we map Go headers directly onto 'data'.
	tDeser := profiling.TimeIt(func() {
		deserErr = Deserialize(data, assetPtr)
	})

	if deserErr != nil {
		_ = closer.Close() // Safety: Don't leak the mmap if swizzling fails.
		return nil, fmt.Errorf("deserialization failed: %w", deserErr)
	}

	logrus.Infof("Loaded %s [Size: %s] | Deser: %s | Decomp: %s",
		filePath, formatSize(len(data)), tDeser, tDecomp)

	return closer, nil
}

func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// MmapBackedBuffer holds a buffer allocated via anonymous mmap (MAP_ANON|MAP_PRIVATE).
// Unlike Go heap allocations, mmap memory can be explicitly returned to the OS via
// Munmap, bypassing Go's garbage collector entirely.
type MmapBackedBuffer struct {
	data []byte
}

// Release unmaps the buffer, immediately returning memory to the OS.
// After calling Release, any Go objects deserialized onto this buffer become invalid.
// The caller MUST nil all Go references to deserialized objects before calling Release.
func (b *MmapBackedBuffer) Release() {
	if b != nil && b.data != nil {
		if err := syscall.Munmap(b.data); err != nil {
			logrus.Warnf("munmap release failed: %v", err)
		}
		b.data = nil
	}
}

// LoadFromDiskMmapBacked loads a compressed serialized asset from disk, decompressing
// it into an anonymous mmap-backed buffer instead of Go heap memory.
//
// Unlike LoadFromDisk (compressed path), where decompressed data lives on the Go heap
// and can only be freed by GC (which may never return pages to the OS), this function
// places decompressed data in mmap memory that can be instantly reclaimed via
// MmapBackedBuffer.Release() → syscall.Munmap.
//
// Go's GC does not track mmap-allocated memory, so:
//   - The large decompressed data does NOT increase Go heap size or GC pressure
//   - Release() returns physical+virtual memory instantly (no GC cycle needed)
//   - The Go runtime never attempts to scan or free this memory
//
// The caller MUST call buf.Release() when the asset is no longer needed.
// Before calling Release(), nil all Go references to the deserialized asset.
func LoadFromDiskMmapBacked(filePath string, assetPtr any) (*MmapBackedBuffer, error) {
	logrus.Infof("Loading compressed asset to mmap buffer: %s...", filePath)

	// 1. Read compressed file from disk (respects IO semaphore)
	_ = diskReadSemaphore.Acquire(context.Background(), 1)
	compressedData, err := os.ReadFile(filePath)
	diskReadSemaphore.Release(1)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 2. Peek at the serde FileHeader (first 32 bytes of decompressed data) to
	//    learn the total decompressed size (FileHeader.DataSize). This requires
	//    only a minimal partial decompression (one zstd block) and lets us
	//    pre-allocate the mmap region before full decompression — eliminating
	//    the ~25-50 GiB temporary heap buffer that io.ReadAll required per module.
	peekDecoder, err := zstd.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("decompression setup failed: %w", err)
	}
	headerBuf := make([]byte, SizeOf[FileHeader]())
	if _, err := io.ReadFull(peekDecoder, headerBuf); err != nil {
		peekDecoder.Close()
		return nil, fmt.Errorf("failed to read serde header from %s: %w", filePath, err)
	}
	peekDecoder.Close()

	serdeHeader := (*FileHeader)(unsafe.Pointer(&headerBuf[0]))
	if serdeHeader.Magic != Magic {
		return nil, fmt.Errorf("invalid magic bytes in %s", filePath)
	}
	decompSize := int(serdeHeader.DataSize)

	// 3. Allocate anonymous mmap region of exact decompressed size.
	// MAP_ANON|MAP_PRIVATE: pages are lazily allocated by the kernel on first
	// write and can be returned instantly via Munmap.
	mmapData, err := syscall.Mmap(-1, 0, decompSize,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return nil, fmt.Errorf("mmap allocation failed (%s): %w", formatSize(decompSize), err)
	}

	// 4. Stream-decompress directly into the mmap buffer. The zstd decoder
	//    reads compressed blocks and writes decoded output straight to the mmap
	//    region, using only its internal block buffer (~128 KB) on the heap.
	//    With 4 concurrent loads, this saves ~180 GiB of peak heap compared to
	//    the previous io.ReadAll + copy approach.
	var tDecomp time.Duration
	var decompErr error
	tDecomp = profiling.TimeIt(func() {
		decoder, err := zstd.NewReader(bytes.NewReader(compressedData))
		if err != nil {
			decompErr = err
			return
		}
		defer decoder.Close()
		_, decompErr = io.ReadFull(decoder, mmapData)
	})
	compressedData = nil // allow GC to collect compressed data
	if decompErr != nil {
		_ = syscall.Munmap(mmapData)
		return nil, fmt.Errorf("decompression failed: %w", decompErr)
	}

	// 5. Deserialize (overlay Go headers onto mmap buffer)
	var deserErr error
	tDeser := profiling.TimeIt(func() {
		deserErr = Deserialize(mmapData, assetPtr)
	})
	if deserErr != nil {
		_ = syscall.Munmap(mmapData)
		return nil, fmt.Errorf("deserialization failed: %w", deserErr)
	}

	logrus.Infof("Loaded %s to mmap [Size: %s] | Deser: %s | Decomp: %s",
		filePath, formatSize(len(mmapData)), tDeser, tDecomp)

	return &MmapBackedBuffer{data: mmapData}, nil
}
