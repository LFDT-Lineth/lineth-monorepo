package utils

import (
	"math/rand/v2"
	"unsafe"
)

// NewRandSource returns a [rand.ChaCha8] initialized with an integer seed. The
// function is meant to sugar-coat the numerous testcases of the repo.
func NewRandSource(seed int64) *rand.ChaCha8 {

	if seed < 0 {
		seed = -seed
	}

	var (
		seedU64     = uint64(seed)
		seed8Bytes  = *(*[8]byte)(unsafe.Pointer(&seedU64))
		seed32Bytes = [32]byte{}
	)

	copy(seed32Bytes[:], seed8Bytes[:])
	return rand.NewChaCha8(seed32Bytes)
}

// ReadPseudoRand populate slices with bytes generated from rand. It returns the
// number of bytes read and an error to match with [io.Read]. This function is
// intended as a drop-in replacement for [math/rand.Read]. `n` is always the
// len(slice) and err is always `nil`.
func ReadPseudoRand(rng *rand.Rand, slice []byte) (n int, err error) {
	for i := range slice {
		slice[i] = byte(rng.Uint32() & 0xff)
	}
	return len(slice), nil
}
