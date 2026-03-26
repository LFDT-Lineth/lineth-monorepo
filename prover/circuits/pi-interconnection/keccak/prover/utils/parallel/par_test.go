package parallel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

func TestParChunky(t *testing.T) {

	testArr := make([]int, 4096)

	worker := func(start, stop int) {
		for i := start; i < stop; i++ {
			testArr[i]++
		}
	}

	parallel.ExecuteChunky(4096, worker)

	for i := range testArr {
		if testArr[i] != 1 {
			t.Logf("went %v times over the pos %v", testArr[i], i)
			t.Fail()
		}
	}

}

// TestAtomicCounter checks that the atomic counter returns the expected values
// and that it returns false when the maximum value is reached.
func TestAtomicCounter(t *testing.T) {
	n := 5
	counter := parallel.NewAtomicCounter(n)

	for i := 0; i < n; i++ {
		value, ok := counter.Next()
		assert.True(t, ok, "expected ok to be true at iteration %d", i)

		assert.Equal(t, i, value, "expected value to be %d at iteration %d", i, i)
	}

	// After n iterations, Next should return ok as false
	_, ok := counter.Next()
	assert.False(t, ok, "expected ok to be false after reaching the maximum value")
}
