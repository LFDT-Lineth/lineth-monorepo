package keccak

import (
	"math/bits"
	"unsafe"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// bytesAsBlockPtrUnsafe unsafely cast a slice into an array. The caller is
// responsible for checking the length of the slice is at least as large as
// a block.
func bytesAsBlockPtrUnsafe(s []byte) *Block {
	return (*Block)(unsafe.Pointer(&s[0]))
}

// castDigest casts a 4-uplets of uint64 into a Keccak digest
func castDigest(a0, a1, a2, a3 uint64) Digest {
	resU64 := [4]uint64{a0, a1, a2, a3}
	return *(*Digest)(unsafe.Pointer(&resU64[0])) // #nosec G115 -- TODO look into this. Seems impossible to overflow here
}

// cycShf is an alias for [bits.RotateLeft64]. The function performs a bit
// cyclic shift over a uin64.
var cycShf = bits.RotateLeft64

// mod5 returns x mod5 5. Used as a shorthand for wrapping around the coordinates
// of the keccak state concisely.
func mod5(x int) int {
	return utils.PositiveMod(x, Dim)
}
