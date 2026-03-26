package fft

import (
	"runtime"
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// lock to save on precomputations
var twiddleLock = sync.Mutex{}

// The maximal order but as an int
var maxOrderInt int = int(field.RootOfUnityOrder)

// The twiddle and twiddleInv arrays are computed lazily
var twiddles [][]field.Element = make([][]field.Element, 0, maxOrderInt+1)
var twiddleInvs [][]field.Element = make([][]field.Element, 0, maxOrderInt+1)

// Returns the twiddles for a domain of size `n`
func GetTwiddleForDomainOfSize(domainSize int) (twid, twidInvs [][]field.Element) {

	if !utils.IsPowerOfTwo(domainSize) {
		utils.Panic("domainSize should be a power of two : %v", domainSize)
	}

	order := utils.Log2Ceil(domainSize)

	// The check-lock-check works because we use it for a single append-only structure.
	if len(twidInvs) <= order {
		twiddleLock.Lock()
		if len(twidInvs) <= order {
			precomputeTwiddlesAndInvs(order)
		}
		twiddleLock.Unlock()
	}

	// `Domain` expects the list of twiddles to be in reversed order to what is precomputed
	// Thus, we arrange it in reverse order.
	twid, twidInvs = make([][]field.Element, order), make([][]field.Element, order)
	for i := range twid {
		posInPrecomp := order - i
		twid[i] = twiddles[posInPrecomp]
		twidInvs[i] = twiddleInvs[posInPrecomp]
	}

	return twid, twidInvs
}

// Ensures that all the twiddles are computed at a given order
func precomputeTwiddlesAndInvs(n int) {

	// Sanity-check : twiddle and inverses have the same length
	if len(twiddles) != len(twiddleInvs) {
		utils.Panic("twiddle and inverse do not have the same length (resp. lengths : %v %v)", len(twiddles), len(twiddleInvs))
	}

	// Sanity-check : twiddle and inverses have the same cap
	if cap(twiddles) != cap(twiddleInvs) {
		utils.Panic("twiddle and inverse do not have the same caps (resp. capacities : %v %v)", cap(twiddles), cap(twiddleInvs))
	}

	currLen := len(twiddles)
	capacity := cap(twiddles)

	// Check that n is not over capacity
	if n >= capacity {
		utils.Panic("n (%v) is over capacity : %v", n, cap(twiddleInvs))
	}

	// Sanity-check : the twiddle and inverse should have the same length
	for i := currLen; i <= n; i++ {

		domainISize := 1 << i
		twidSize := 1 + domainISize/2
		twiddleI := make([]field.Element, twidSize)
		twiddleI[0].SetOne()

		w := generators[i] // The capacity check prevents this access to go out of bound
		for j := 1; j < twidSize; j++ {
			twiddleI[j].Mul(&twiddleI[j-1], &w)
		}

		// And the twiddle Invs, would certainly be better solved by shuffling
		// the entries of twiddleI
		twiddleInvI := field.ParBatchInvert(twiddleI, runtime.GOMAXPROCS(0))

		twiddles = append(twiddles, twiddleI)
		twiddleInvs = append(twiddleInvs, twiddleInvI)
	}

}
