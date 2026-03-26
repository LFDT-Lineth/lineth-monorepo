package vortex

import (
	"github.com/consensys/linea-monorepo/prover/crypto/reedsolomon"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Params collects the public parameters of the commitment scheme. The object
// should not be constructed directly (use [NewParamsSis] or [NewParamsNoSis])
// instead nor be modified after having been constructed.
type Params struct {
	// Key stores the public parameters of the ring-SIS instance in use to
	// hash the columns.
	Key *ringsis.Key

	// Reed Solomon code params
	RsParams *reedsolomon.RsParams

	// NbColumns number of columns of the matrix storing the polynomials. The
	// total size of the polynomials which are committed is NbColumns x NbRows.
	// The Number of columns is a power of 2, it corresponds to the original
	// size of the codewords of the Reed Solomon code.
	NbColumns int

	// MaxNbRows number of rows of the matrix storing the polynomials. If a
	// polynomial p is appended whose size if not 0 mod MaxNbRows, it is padded
	// as p' so that len(p')=0 mod MaxNbRows.
	MaxNbRows int
}

// NewParams creates and returns a [Params]:
//
//   - blowUpFactor: inverse-rate of the RS code ( > 1). Must be a power of 2.
//   - nbColumns: the number of columns in the witness matrix
//   - maxNbRows: the maximum number of rows in the witness matrix
//   - sisParams: the parameters of the SIS instance to use to hash the columns
//   - leafHashFunc: the hash function to use to hash the SIS hashes into the
//     leaves of the Merkle-tree (when using the SIS hashing) or hash the field elements
//     directly while not using the SIS hashing.
//   - merkleHashFunc: the hash function to use to hash the nodes of the Merkle-tree.
func NewParams(
	rate int,
	nbColumns int,
	maxNbRows int,
	logTwoDegree int,
	logTwoBound int,
) *Params {

	if !utils.IsPowerOfTwo(nbColumns) {
		utils.Panic("The number of columns has to be a power of two, got %v", nbColumns)
	}

	if !utils.IsPowerOfTwo(rate) {
		utils.Panic("The number of columns has to be a power of two, got %v", nbColumns)
	}

	if maxNbRows < 1 {
		utils.Panic("The number of rows per matrix cannot be zero of negative: %v", maxNbRows)
	}
	res := &Params{
		RsParams:  reedsolomon.NewRsParams(nbColumns, rate),
		NbColumns: nbColumns,
		MaxNbRows: maxNbRows,
		Key:       ringsis.GenerateKey(logTwoDegree, logTwoBound, maxNbRows),
	}

	return res
}
