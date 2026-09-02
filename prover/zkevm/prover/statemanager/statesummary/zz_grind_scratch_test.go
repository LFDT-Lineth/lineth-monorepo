package statesummary

import (
	"encoding/binary"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	poseidon2kb "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// hashFullBytes32 mirrors StorageHash / poseidon2Hash on the native side.
func hashFullBytes32(v types.FullBytes32) [8]field.Element {
	h := poseidon2kb.NewMDHasher()
	v.WriteTo(h)
	d := h.Sum(nil)
	var res [8]field.Element
	for i := range 8 {
		res[i].SetBytes(d[i*4 : (i+1)*4])
	}
	return res
}

func TestZZBenchHashRate(t *testing.T) {
	var v types.FullBytes32
	start := time.Now()
	const n = 200_000
	for i := range n {
		binary.BigEndian.PutUint64(v[24:], uint64(i))
		_ = hashFullBytes32(v)
	}
	el := time.Since(start)
	t.Logf("%d hashes in %v -> %.0f ns/hash; 2^28 single-threaded = %v",
		n, el, float64(el.Nanoseconds())/n, time.Duration(float64(el.Nanoseconds())/n*(1<<28)))
}

func TestZZGrindPartialZeroMatch(t *testing.T) {
	target := hashOfZeroStorage()
	t.Logf("zeroStorageHash limbs: %v", target)

	var (
		found   atomic.Bool
		result  types.FullBytes32
		mu      sync.Mutex
		workers = runtime.NumCPU()
		wg      sync.WaitGroup
	)

	start := time.Now()
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var v types.FullBytes32
			for i := uint64(w); !found.Load(); i += uint64(workers) {
				binary.BigEndian.PutUint64(v[24:], i)
				h := hashFullBytes32(v)
				nMatch := 0
				for k := range 8 {
					if h[k] == target[k] {
						nMatch++
					}
				}
				if nMatch >= 1 && nMatch < 8 {
					mu.Lock()
					if !found.Swap(true) {
						result = v
					}
					mu.Unlock()
					return
				}
			}
		}(w)
	}
	wg.Wait()

	t.Logf("found after %v: value=%x hash=%v", time.Since(start), result, hashFullBytes32(result))
}

func hashAccount(a statemanager.Account) [8]field.Element {
	h := poseidon2kb.NewMDHasher()
	a.WriteTo(h)
	d := h.Sum(nil)
	var res [8]field.Element
	for i := range 8 {
		res[i].SetBytes(d[i*4 : (i+1)*4])
	}
	return res
}

func TestZZGrindAccountPartialMatch(t *testing.T) {
	const nonce = int64(1)
	base := hashAccount(statemanager.NewEOA(nonce, big.NewInt(1_000)))

	var (
		found   atomic.Bool
		result  int64
		mu      sync.Mutex
		workers = runtime.NumCPU()
		wg      sync.WaitGroup
	)

	start := time.Now()
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := int64(w); !found.Load(); i += int64(workers) {
				h := hashAccount(statemanager.NewEOA(nonce, big.NewInt(2_000_000+i)))
				nMatch := 0
				for k := range 8 {
					if h[k] == base[k] {
						nMatch++
					}
				}
				if nMatch >= 1 && nMatch < 8 {
					mu.Lock()
					if !found.Swap(true) {
						result = 2_000_000 + i
					}
					mu.Unlock()
					return
				}
			}
		}(w)
	}
	wg.Wait()

	t.Logf("found after %v: balance=%d", time.Since(start), result)
	t.Logf("base=%v", base)
	t.Logf("new =%v", hashAccount(statemanager.NewEOA(nonce, big.NewInt(result))))
}

// grind for a storage value whose hash has at least one zero limb (target for
// FinalHValIsZero, which compares against field.Zero()).
func TestZZGrindZeroLimb(t *testing.T) {
	var (
		found   atomic.Bool
		result  types.FullBytes32
		mu      sync.Mutex
		workers = runtime.NumCPU()
		wg      sync.WaitGroup
	)
	start := time.Now()
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var v types.FullBytes32
			for i := uint64(w); !found.Load(); i += uint64(workers) {
				binary.BigEndian.PutUint64(v[24:], i)
				h := hashFullBytes32(v)
				n := 0
				for k := range 8 {
					if h[k].IsZero() {
						n++
					}
				}
				if n >= 1 && n < 8 {
					mu.Lock()
					if !found.Swap(true) {
						result = v
					}
					mu.Unlock()
					return
				}
			}
		}(w)
	}
	wg.Wait()
	t.Logf("found after %v: value=%x hash=%v", time.Since(start), result, hashFullBytes32(result))
}
