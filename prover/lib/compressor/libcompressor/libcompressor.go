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

var (
	compressor *blob_v1.BlobMaker
	lastError  error
	lock       sync.Mutex // probably unnecessary if coordinator guarantees single-threaded access
)

// Init initializes the compressor.
// The dataLimit argument is the maximum size of the compressed data.
// Returns true if the compressor was initialized, false otherwise.
// If false is returned, the Error() method will return a string describing the error.
//
//export Init
func Init(dataLimit int, dictPath *C.char) bool {
	fPath := C.GoString(dictPath)
	return initGo(dataLimit, fPath)
}

func initGo(dataLimit int, dictPath string) bool {
	lock.Lock()
	defer lock.Unlock()
	compressor, lastError = blob_v2.NewBlobMaker(dataLimit, dictPath)

	return lastError == nil
}

// Reset resets the compressor. Must be called between each Blob.
//
//export Reset
func Reset() {
	lock.Lock()
	defer lock.Unlock()

	lastError = nil

	compressor.Reset()
}

// Write appends the input to the compressed data.
// The Go code doesn't keep a pointer to the input slice and the caller is free to modify it.
// Returns true if the chunk was appended, false if the chunk was discarded.
//
// The input []byte is interpreted as a RLP encoded Block.
//
//export Write
func Write(input *C.char, inputLength C.int) (chunkAppended bool) {
	lock.Lock()
	defer lock.Unlock()
	rlpBlock := unsafe.Slice((*byte)(unsafe.Pointer(input)), inputLength)
	chunkAppended, err := compressor.Write(rlpBlock, false)
	if err != nil {
		lastError = err
		return false
	}

	return chunkAppended
}

// CanWrite behaves as Write, except that it doesn't append the input to the compressed data
// (but return true if it could)
//
//export CanWrite
func CanWrite(input *C.char, inputLength C.int) (chunkAppended bool) {
	lock.Lock()
	defer lock.Unlock()
	rlpBlock := unsafe.Slice((*byte)(unsafe.Pointer(input)), inputLength)
	chunkAppended, err := compressor.Write(rlpBlock, true)
	if err != nil {
		lastError = err
		return false
	}

	return chunkAppended
}

// Error returns the last encountered error.
// If no error was encountered, returns nil.
//
//export Error
func Error() *C.char {
	lock.Lock()
	defer lock.Unlock()
	if lastError != nil {
		// this leaks memory, but since this represents a fatal error, it's probably ok.
		return C.CString(lastError.Error())
	}
	return nil
}

// StartNewBatch starts a new batch; must be called between each batch in the blob.
//
//export StartNewBatch
func StartNewBatch() {
	lock.Lock()
	defer lock.Unlock()

	compressor.StartNewBatch()
}

// Len returns the length of the compressed data.
//
//export Len
func Len() (length int) {
	lock.Lock()
	defer lock.Unlock()
	return compressor.Len()
}

// Bytes returns the compressed data.
// The caller is responsible for allocating the memory for the output slice.
// Length of the output slice must equal the value returned by Len().
//
//export Bytes
func Bytes(dataOut *C.char) {
	lock.Lock()
	defer lock.Unlock()
	compressed := compressor.Bytes()

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
// This function is thread-safe. Concurrent calls are allowed,
// but the other functions may not be thread-safe.
//
//export WorstCompressedBlockSize
func WorstCompressedBlockSize(input *C.char, inputLength C.int) C.int {
	rlpBlock := C.GoBytes(unsafe.Pointer(input), inputLength)

	_, n, err := compressor.WorstCompressedBlockSize(rlpBlock)
	if err != nil {
		lock.Lock()
		lastError = errors.Join(lastError, err)
		lock.Unlock()
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
// This function is thread-safe. Concurrent calls are allowed,
// but the other functions may not be thread-safe.
//
//export WorstCompressedTxSize
func WorstCompressedTxSize(input *C.char, inputLength C.int) C.int {
	rlpTx := C.GoBytes(unsafe.Pointer(input), inputLength)

	n, err := compressor.WorstCompressedTxSize(rlpTx)
	if err != nil {
		lock.Lock()
		lastError = errors.Join(lastError, err)
		lock.Unlock()
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
// This function is thread-safe. Concurrent calls are allowed,
// but the other functions are not thread-safe.
//
//export RawCompressedSize
func RawCompressedSize(input *C.char, inputLength C.int) C.int {
	inputSlice := C.GoBytes(unsafe.Pointer(input), inputLength)

	n, err := compressor.RawCompressedSize(inputSlice)
	if err != nil {
		lock.Lock()
		lastError = errors.Join(lastError, err)
		lock.Unlock()
		return -1
	}
	return C.int(n)
}
