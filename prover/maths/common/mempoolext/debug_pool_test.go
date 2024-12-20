package mempoolext

import (
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestDebugPool(t *testing.T) {

	t.Run("leak-detection", func(t *testing.T) {

		pool := NewDebugPool(CreateFromSyncPool(32))

		for i := 0; i < 16; i++ {
			func() {
				_ = pool.Alloc()
			}()
		}

		err := pool.Errors().Error()
		assert.True(t, strings.HasPrefix(err, "leaked a vector out of the pool"))
	})

	t.Run("double-free", func(t *testing.T) {

		pool := NewDebugPool(CreateFromSyncPool(32))

		v := pool.Alloc()

		for i := 0; i < 16; i++ {
			pool.Free(v)
		}

		err := pool.Errors().Error()
		assert.Truef(t, strings.HasPrefix(err, "vector was freed multiple times concurrently"), err)
	})

	t.Run("foreign-free", func(t *testing.T) {

		pool := NewDebugPool(CreateFromSyncPool(32))

		v := make([]fext.Element, 32)
		pool.Free(&v)

		err := pool.Errors().Error()
		assert.Truef(t, strings.HasPrefix(err, "freed a vector that was not from the pool"), err)
	})

}
