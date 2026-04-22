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
