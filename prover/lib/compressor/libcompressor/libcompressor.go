package main

import "C"

import (
	"errors"
	"sync"
	"unsafe"

	blob_v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	blob_v2 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2"
)

//go:generate go build -tags nocorset -ldflags "-s -w" -buildmode=c-shared -o libcompressor.so libcompressor.go
func main() {}

type instance struct {
	compressor *blob_v1.BlobMaker
	lastError  error
	mu         sync.Mutex
}

var (
	instances        = map[C.int]*instance{}
	nextHandle C.int = 1
	registryMu sync.Mutex
)

func getInstance(handle C.int) *instance {
	registryMu.Lock()
	defer registryMu.Unlock()
	inst, ok := instances[handle]
	if !ok {
		panic("libcompressor: unknown handle")
	}
	return inst
}

// Init initializes a new compressor instance.
// The dataLimit argument is the maximum size of the compressed data.
// Returns a positive handle on success, or -1 on failure.
// On failure, errOut is set to a newly allocated C string describing the error; the caller must free it.
//
//export Init
func Init(dataLimit int, dictPath *C.char, errOut **C.char) C.int {
	fPath := C.GoString(dictPath)

	blobMaker, err := blob_v2.NewBlobMaker(dataLimit, fPath)
	if err != nil {
		*errOut = C.CString(err.Error())
		return -1
	}

	inst := &instance{compressor: blobMaker}

	registryMu.Lock()
	defer registryMu.Unlock()
	handle := nextHandle
	nextHandle++
	instances[handle] = inst

	return handle
}

// Free releases a compressor instance created by Init.
// The handle must not be used after this call.
//
//export Free
func Free(handle C.int) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(instances, handle)
}

// Reset resets the compressor. Must be called between each Blob.
//
//export Reset
func Reset(handle C.int) {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()

	inst.lastError = nil
	inst.compressor.Reset()
}

// Write appends the input to the compressed data.
// The Go code doesn't keep a pointer to the input slice and the caller is free to modify it.
// Returns true if the chunk was appended, false if the chunk was discarded.
//
// The input []byte is interpreted as a RLP encoded Block.
//
//export Write
func Write(handle C.int, input *C.char, inputLength C.int) (chunkAppended bool) {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()
	rlpBlock := unsafe.Slice((*byte)(unsafe.Pointer(input)), inputLength)
	chunkAppended, err := inst.compressor.Write(rlpBlock, false)
	if err != nil {
		inst.lastError = err
		return false
	}

	return chunkAppended
}

// CanWrite behaves as Write, except that it doesn't append the input to the compressed data
// (but return true if it could)
//
//export CanWrite
func CanWrite(handle C.int, input *C.char, inputLength C.int) (chunkAppended bool) {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()
	rlpBlock := unsafe.Slice((*byte)(unsafe.Pointer(input)), inputLength)
	chunkAppended, err := inst.compressor.Write(rlpBlock, true)
	if err != nil {
		inst.lastError = err
		return false
	}

	return chunkAppended
}

// Error returns the last encountered error for the given instance.
// If no error was encountered, returns nil.
//
//export Error
func Error(handle C.int) *C.char {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.lastError != nil {
		// this leaks memory, but since this represents a fatal error, it's probably ok.
		return C.CString(inst.lastError.Error())
	}
	return nil
}

// StartNewBatch starts a new batch; must be called between each batch in the blob.
//
//export StartNewBatch
func StartNewBatch(handle C.int) {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()

	inst.compressor.StartNewBatch()
}

// Len returns the length of the compressed data.
//
//export Len
func Len(handle C.int) (length int) {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.compressor.Len()
}

// Bytes returns the compressed data.
// The caller is responsible for allocating the memory for the output slice.
// Length of the output slice must equal the value returned by Len().
//
//export Bytes
func Bytes(handle C.int, dataOut *C.char) {
	inst := getInstance(handle)
	inst.mu.Lock()
	defer inst.mu.Unlock()
	compressed := inst.compressor.Bytes()

	outSlice := unsafe.Slice((*byte)(unsafe.Pointer(dataOut)), len(compressed))
	copy(outSlice, compressed)
}

// WorstCompressedBlockSize returns the size of the given block, as compressed by an "empty" blob maker.
// That is, with more context, blob maker could compress the block further, but this function
// returns the maximum size that can be achieved.
//
// The input is a RLP encoded block.
// Returns the length of the compressed data, or -1 if an error occurred.
// User must call Error() to get the error message.
//
// This function is thread-safe. Concurrent calls with different handles are allowed.
//
//export WorstCompressedBlockSize
func WorstCompressedBlockSize(handle C.int, input *C.char, inputLength C.int) C.int {
	inst := getInstance(handle)
	rlpBlock := C.GoBytes(unsafe.Pointer(input), inputLength)

	_, n, err := inst.compressor.WorstCompressedBlockSize(rlpBlock)
	if err != nil {
		inst.mu.Lock()
		inst.lastError = errors.Join(inst.lastError, err)
		inst.mu.Unlock()
		return -1
	}
	return C.int(n)
}

// WorstCompressedTxSize returns the size of the given transaction, as compressed by an "empty" blob maker.
// That is, with more context, blob maker could compress the transaction further, but this function
// returns the maximum size that can be achieved.
//
// The input is a RLP encoded transaction.
// Returns the length of the compressed data, or -1 if an error occurred.
// User must call Error() to get the error message.
//
// This function is thread-safe. Concurrent calls with different handles are allowed.
//
//export WorstCompressedTxSize
func WorstCompressedTxSize(handle C.int, input *C.char, inputLength C.int) C.int {
	inst := getInstance(handle)
	rlpTx := C.GoBytes(unsafe.Pointer(input), inputLength)

	n, err := inst.compressor.WorstCompressedTxSize(rlpTx)
	if err != nil {
		inst.mu.Lock()
		inst.lastError = errors.Join(inst.lastError, err)
		inst.mu.Unlock()
		return -1
	}
	return C.int(n)
}

// RawCompressedSize compresses the (raw) input and returns the length of the compressed data.
// The returned length account for the "padding" used by the blob maker to
// fit the data in field elements.
// Input size must be less than 256kB.
// If an error occurred, returns -1.
// User must call Error() to get the error message.
//
// This function is thread-safe. Concurrent calls with different handles are allowed.
//
//export RawCompressedSize
func RawCompressedSize(handle C.int, input *C.char, inputLength C.int) C.int {
	inst := getInstance(handle)
	inputSlice := C.GoBytes(unsafe.Pointer(input), inputLength)

	n, err := inst.compressor.RawCompressedSize(inputSlice)
	if err != nil {
		inst.mu.Lock()
		inst.lastError = errors.Join(inst.lastError, err)
		inst.mu.Unlock()
		return -1
	}
	return C.int(n)
}
