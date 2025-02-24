package main

import "C"

import (
	"errors"
	"strings"
	"sync"
	"unsafe"

	decompressor "github.com/consensys/linea-monorepo/prover/lib/compressor/blob"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
)

//go:generate go build -tags nocorset -ldflags "-s -w" -buildmode=c-shared -o libdecompressor.so libdecompressor.go
func main() {}

var (
	dictStore dictionary.Store
	lastError error
	lock      sync.Mutex // probably unnecessary if coordinator guarantees single-threaded access
)

// Init initializes the decompressor.
//
//export Init
func Init() {
	lock.Lock()
	dictStore = dictionary.NewStore()
	lock.Unlock()
}

// LoadDictionaries loads a number of dictionaries into the decompressor
// according to colon-separated paths.
// Returns the number of dictionaries loaded, or -1 if unsuccessful.
// If -1 is returned, the Error() method will return a string describing the error.
//
//export LoadDictionaries
func LoadDictionaries(dictPaths *C.char) C.int {
	lock.Lock()
	defer lock.Unlock()

	pathsConcat := C.GoString(dictPaths)
	paths := strings.Split(pathsConcat, ":")

	if err := dictStore.Load(paths...); err != nil {
		lastError = err
		return -1
	}
	return C.int(len(paths))
}

// Decompress processes a Linea blob and outputs an RLP encoded list of RLP encoded blocks.
// Due to information loss during pre-compression encoding, two pieces of information are represented "hackily":
// The block hash is in the ParentHash field.
// The transaction from address is in the signature.R field.
//
// Returns the number of bytes in out, or -1 in case of failure
// If -1 is returned, the Error() method will return a string describing the error.
//
//export Decompress
func Decompress(blob *C.char, blobLength C.int, out *C.char, outMaxLength C.int) C.int {

	lock.Lock()
	defer lock.Unlock()

	bGo := C.GoBytes(unsafe.Pointer(blob), blobLength)

	blocks, err := decompressor.DecompressBlob(bGo, dictStore)
	if err != nil {
		lastError = err
		return -1
	}

	if len(blocks) > int(outMaxLength) {
		lastError = errors.New("decoded blob does not fit in output buffer")
		return -1
	}

	outSlice := unsafe.Slice((*byte)(unsafe.Pointer(out)), len(blocks))
	copy(outSlice, blocks)

	return C.int(len(blocks))
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
