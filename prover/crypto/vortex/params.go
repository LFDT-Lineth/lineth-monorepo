package vortex

import (
	"hash"

	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Params collects the public parameters of the commitment scheme. The object
// should not be constructed directly (use [NewParamsSis] or [NewParamsNoSis])
// instead nor be modified after having been constructed.
type Params struct {
	// Key stores the public parameters of the ring-SIS instance in use to
	// hash the columns.
	Key *ringsis.Key
	// BlowUpFactor corresponds to the inverse-rate of the Reed-Solomon code
	// in use to encode the rows of the committed matrices. This is a power of
	// two and
	BlowUpFactor int
	// Domain[0]: domain to perform the FFT^-1, of size NbColumns is meant to
	// be run over the non-encoded rows when RS encoding.
	// Domain[1]: domain to perform FFT, of size BlowUp * NbColumns is meant
	// to be obtain the codeword when RS encoding.
	Domains [2]*fft.Domain
	// NbColumns number of columns of the matrix storing the polynomials. The
	// total size of the polynomials which are committed is NbColumns x NbRows.
	// The Number of columns is a power of 2, it corresponds to the original
	// size of the codewords of the Reed Solomon code.
	NbColumns int
	// MaxNbRows number of rows of the matrix storing the polynomials. If a
	// polynomial p is appended whose size if not 0 mod MaxNbRows, it is padded
	// as p' so that len(p')=0 mod MaxNbRows.
	MaxNbRows int
	// LeafHashFunc returns a `hash.Hash` which is used
	// to compute the leaves of the Merkle tree.
	LeafHashFunc func() hash.Hash
	// MerkleHashFunc returns a `hash.Hash` which is used
	// to hash the nodes of the Merkle tree.
	MerkleHashFunc func() hash.Hash
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
	blowUpFactor int,
	nbColumns int,
	maxNbRows int,
	sisParams ringsis.Params,
	leafHashFunc func() hash.Hash,
	merkleHashFunc func() hash.Hash,
) *Params {

	if !utils.IsPowerOfTwo(nbColumns) {
		utils.Panic("The number of columns has to be a power of two, got %v", nbColumns)
	}

	if !utils.IsPowerOfTwo(blowUpFactor) {
		utils.Panic("The number of columns has to be a power of two, got %v", nbColumns)
	}

	if leafHashFunc == nil {
		utils.Panic("`nil` leaf hash function provided")
	}

	if merkleHashFunc == nil {
		utils.Panic("`nil` merkle hash function provided")
	}

	if maxNbRows < 1 {
		utils.Panic("The number of rows per matrix cannot be zero of negative: %v", maxNbRows)
	}

	res := &Params{
		Domains: [2]*fft.Domain{
			fft.NewDomain(nbColumns),
			fft.NewDomain(blowUpFactor * nbColumns),
		},
		NbColumns:      nbColumns,
		MaxNbRows:      maxNbRows,
		BlowUpFactor:   blowUpFactor,
		Key:            ringsis.GenerateKey(sisParams, maxNbRows),
		LeafHashFunc:   leafHashFunc,
		MerkleHashFunc: merkleHashFunc,
	}

	return res
}

// NumEncodedCols returns the number of columns in the encoded matrix,
// equivalently this is the size of the codeword-rows.
func (p *Params) NumEncodedCols() int {
	return utils.NextPowerOfTwo(p.NbColumns) * p.BlowUpFactor
}
