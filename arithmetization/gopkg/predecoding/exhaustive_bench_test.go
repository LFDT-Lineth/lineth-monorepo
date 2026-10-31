package predecoding

// Question 1 — feasibility: can every 32-bit word be checked at all, and how
// many of them carry meaning? Also holds forEachWord, the parallel scan that
// every exhaustive test in this package uses.

import (
	"hash/fnv"
	"runtime"
	"sync"
	"testing"
	"time"
)

// forEachWord splits all 2^32 words into one contiguous half-open range
// [lo, hi) per CPU, runs scan on each range concurrently, and returns the
// per-worker results for the caller to merge. The last range ends exactly at
// 2^32, so no word is skipped.
func forEachWord[T any](scan func(lo, hi uint64) T) []T {
	workers := uint64(runtime.NumCPU())
	const total = uint64(1) << 32
	out := make([]T, workers)
	var wg sync.WaitGroup
	for w := uint64(0); w < workers; w++ {
		wg.Add(1)
		go func(w uint64) {
			defer wg.Done()
			out[w] = scan(total*w/workers, total*(w+1)/workers)
		}(w)
	}
	wg.Wait()
	return out
}

// Decodes every 32-bit word once and fingerprints the output.
func TestExhaustiveDecodeTiming(t *testing.T) {
	start := time.Now()
	sums := forEachWord(func(lo, hi uint64) uint64 {
		h := fnv.New64a()
		var bw bitWriter
		for i := lo; i < hi; i++ {
			bw = bitWriter{buf: bw.buf[:0]}
			decodeInstruction(&bw, uint32(i))
			h.Write(bw.buf)
		}
		return h.Sum64()
	})
	t.Logf("workers=%d words=%d elapsed=%s fp0=%x", len(sums), uint64(1)<<32, time.Since(start), sums[0])
}

// Counts how many words decode to COMPUTE_INVALID, to NO_OP, and to a real operation.
func TestCountValidWords(t *testing.T) {
	type counts struct{ invalid, noop uint64 }
	parts := forEachWord(func(lo, hi uint64) counts {
		var c counts
		for i := lo; i < hi; i++ {
			switch classifyInstruction(uint32(i)) {
			case computeInvalid:
				c.invalid++
			case computeNoOp:
				c.noop++
			}
		}
		return c
	})
	var inv, nop uint64
	for _, c := range parts {
		inv += c.invalid
		nop += c.noop
	}
	const total = uint64(1) << 32
	t.Logf("total=%d invalid=%d noop=%d valid_nonnoop=%d", total, inv, nop, total-inv-nop)
}
