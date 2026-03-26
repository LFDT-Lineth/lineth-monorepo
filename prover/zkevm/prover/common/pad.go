package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// LeftPadToFrBytes pads a []bytes element by adding zeroes to
// the left until the slice has field.Bytes bytes (4 bytes for koalabear).
// For larger values, use LeftPadToBytes with explicit size.
func LeftPadToFrBytes(b []byte) []byte {
	if len(b) > field.Bytes {
		utils.Panic("Passed a string of %v element but the max is %v", len(b), field.Bytes)
	}
	c := append(make([]byte, field.Bytes-len(b)), b...)
	return c
}

// LeftPadToBytes pads a []bytes element by adding zeroes to
// the left until the slice has the specified size.
func LeftPadToBytes(b []byte, size int) []byte {
	if len(b) > size {
		utils.Panic("Passed a string of %v element but the max is %v", len(b), size)
	}
	c := append(make([]byte, size-len(b)), b...)
	return c
}
