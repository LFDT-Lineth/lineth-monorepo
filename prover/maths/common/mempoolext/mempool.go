package mempoolext

import (
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type MemPool interface {
	Prewarm(nbPrewarm int) MemPool
	Alloc() *[]fext.Element
	Free(vec *[]fext.Element) error
	Size() int
}

// ExtractCheckOptionalStrict returns
//   - p[0], true if the expectedSize matches the one of the provided pool
//   - nil, false if no `p` is provided
//   - panic if the assigned size of the pool does not match
//   - panic if the caller provides `nil` as argument for `p`
//
// This is used to unwrap a [FromSyncPool] that is commonly passed to functions as an
// optional variadic parameter and at the same time validating that the pool
// object has the right size.
func ExtractCheckOptionalStrict(expectedSize int, p ...MemPool) (pool MemPool, ok bool) {
	// Checks if there is a pool
	hasPool := len(p) > 0 && p[0] != nil
	if hasPool {
		pool = p[0]
	}

	// Sanity-check that the size of the pool is actually what we expected
	if hasPool && pool.Size() != expectedSize {
		utils.Panic("pooled vector size are %v, but required %v", pool.Size(), expectedSize)
	}

	return pool, hasPool
}

// ExtractCheckOptionalSoft returns
//   - p[0], true if the expectedSize matches the one of the provided pool
//   - nil, false if no `p` is provided
//   - nil, false if the length of the vector does not match the one of the pool
//   - panic if the caller provides `nil` as argument for `p`
//
// This is used to unwrap a [FromSyncPool] that is commonly passed to functions as an
// optional variadic parameter.
func ExtractCheckOptionalSoft(expectedSize int, p ...MemPool) (pool MemPool, ok bool) {
	// Checks if there is a pool
	hasPool := len(p) > 0
	if hasPool {
		pool = p[0]
	}

	// Sanity-check that the size of the pool is actually what we expected
	if hasPool && pool.Size() != expectedSize {
		return nil, false
	}

	return pool, hasPool
}
