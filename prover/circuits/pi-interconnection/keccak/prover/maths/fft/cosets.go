package fft

import (
	"math/big"
	"math/bits"
	"runtime"
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

/*
Synchronizes the precomputations over multiple threads
*/
var cosetLocks = sync.Mutex{}
var cosetLocksLenChecking = sync.Mutex{}

/*
Indexes a set of precomputed precomCosetTables tables

  - First slices k indexes by domainSize N = 2 ^ k
  - Second level indexes the cosetTables in a custom manner (see `findCosetTablePosition`).
*/
var precomCosetTables [][][]field.Element = make([][][]field.Element, maxOrderInt)
var precomCosetInvTables [][][]field.Element = make([][][]field.Element, maxOrderInt)
var precomCosetTablesBitReversed [][][]field.Element = make([][][]field.Element, maxOrderInt)
var precomCosetInvTablesBitReversed [][][]field.Element = make([][][]field.Element, maxOrderInt)

/*
Find the position of the coset table

	Let H be the domain of "N" root of unity
	Let Hr be the supergroup of "Nr" roots of unity.
	Let gr be a generator of Hr, and g = g^r the generator for H
	Let a be the multiplicative generator of F^*

	`CosetID` returns the cosetID of the coset a*gr^{numCoset}*H
*/
func cosetID(r, numCoset int) (cosetID int) {
	maxDomain := 1 << maxOrderInt
	cosetID64 := uint64(maxDomain / r * numCoset)
	cosetID64 = bits.Reverse64(cosetID64)
	cosetID64 >>= 64 - field.RootOfUnityOrder
	return utils.ToInt(cosetID64)
}

/*
Find the position of the coset table

	Let H be the domain of "N" root of unity
	Let Hr be the supergroup of "Nr" roots of unity.
	Let gr be a generator of Hr, and g = g^r the generator for H
	Let a be the multiplicative generator of F^*

	`CosetID` returns a coset table for the coset a*gr^{numCoset}*H

	* N the size of the cosets
	* r the ratio
	* numCoset, the ID of the coset in the given ratio
*/
func GetCoset(N, r, numCoset int) (cos, cosInv, cosBR, cosInvBR []field.Element) {

	if !utils.IsPowerOfTwo(N) {
		utils.Panic("N is not a power of two %v", N)
	}

	if !utils.IsPowerOfTwo(r) {
		utils.Panic("r is not a power of two %v", r)
	}

	if numCoset >= r {
		utils.Panic("numCoset %v must be smaller than r %v", numCoset, r)
	}

	if N*r > 1<<maxOrderInt {
		utils.Panic("The current field does not have that coset (N %v, r %v, maxOrder %v)", N, r, maxOrderInt)
	}

	cosetID := cosetID(r, numCoset)
	order := utils.Log2Floor(N)

	// If necessary, grows the slice of precomputed coset tables
	cosetLocksLenChecking.Lock()
	if len(precomCosetTables[order]) <= cosetID {

		// Append empty slices to the current precomputed coset tables
		nbToAppend := utils.NextPowerOfTwo(cosetID + 1)
		nbToAppend -= len(precomCosetTables[order])

		zeroes := make([][]field.Element, nbToAppend)

		// Extend the table so that it contains the cosetID
		// Each append creates a deep-copy of the `zeroes` slice
		precomCosetTables[order] = append(precomCosetTables[order], zeroes...)
		precomCosetInvTables[order] = append(precomCosetInvTables[order], zeroes...)
		precomCosetTablesBitReversed[order] = append(precomCosetTablesBitReversed[order], zeroes...)
		precomCosetInvTablesBitReversed[order] = append(precomCosetInvTablesBitReversed[order], zeroes...)
	}
	cosetLocksLenChecking.Unlock()

	if len(precomCosetTables[order]) <= cosetID {
		utils.Panic("Required cosetID %v but the precomp list is %v\n", cosetID, len(precomCosetInvTables[order]))
	}

	// Precompute the coset table if we need it
	cosetLocksLenChecking.Lock()
	if len(precomCosetTables[order][cosetID]) == 0 {
		cosetLocks.Lock()
		if len(precomCosetTables[order][cosetID]) == 0 {
			cos, cosInv, cosBR, cosInvBR := computeCosetTable(N, r, numCoset)
			precomCosetTables[order][cosetID] = cos
			precomCosetInvTables[order][cosetID] = cosInv
			precomCosetTablesBitReversed[order][cosetID] = cosBR
			precomCosetInvTablesBitReversed[order][cosetID] = cosInvBR
		}
		cosetLocks.Unlock()
	}
	cosetLocksLenChecking.Unlock()

	return precomCosetTables[order][cosetID],
		precomCosetInvTables[order][cosetID],
		precomCosetTablesBitReversed[order][cosetID],
		precomCosetInvTablesBitReversed[order][cosetID]

}

func computeCosetTable(N, r, numCoset int) (cos, cosInv, cosBR, cosInvBR []field.Element) {

	var a field.Element
	a.SetUint64(field.MultiplicativeGen)
	x := GetOmega(N * r)                  // x = gr
	x.Exp(x, big.NewInt(int64(numCoset))) // x = gr^numcoset
	x.Mul(&x, &a)                         // x = a gr^numcoset

	cos = vector.PowerVec(x, N)
	cosInv = field.ParBatchInvert(cos, runtime.GOMAXPROCS(0))

	// Also computes the bit-reversed counter-parts
	cosBR = vector.DeepCopy(cos)
	cosInvBR = vector.DeepCopy(cosInv)
	BitReverse(cosBR)
	BitReverse(cosInvBR)

	return cos, cosInv, cosBR, cosInvBR
}
