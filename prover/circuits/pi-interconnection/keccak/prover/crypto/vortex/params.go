package vortex

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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

	// CosetTableBitReverse is the coset table of the small domain in bit
	// reversed order. It is used to speed-up the encoding of the rows.
	CosetTableBitReverse field.Vector
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
) *Params {

	if !utils.IsPowerOfTwo(nbColumns) {
		utils.Panic("The number of columns has to be a power of two, got %v", nbColumns)
	}

	if !utils.IsPowerOfTwo(blowUpFactor) {
		utils.Panic("The number of columns has to be a power of two, got %v", nbColumns)
	}

	if maxNbRows < 1 {
		utils.Panic("The number of rows per matrix cannot be zero of negative: %v", maxNbRows)
	}

	shift, err := fr.Generator(uint64(nbColumns * blowUpFactor))
	if err != nil {
		panic(err)
	}

	res := &Params{
		Domains: [2]*fft.Domain{
			fft.NewDomain(uint64(nbColumns), fft.WithShift(shift)),
			fft.NewDomain(uint64(blowUpFactor*nbColumns), fft.WithCache()),
		},
		NbColumns:    nbColumns,
		MaxNbRows:    maxNbRows,
		BlowUpFactor: blowUpFactor,
		Key:          ringsis.GenerateKey(sisParams, maxNbRows),
	}

	smallDomain := res.Domains[0]
	cosetTable, err := smallDomain.CosetTable()
	if err != nil {
		panic(err)
	}
	cosetTableBitReverse := make(field.Vector, len(cosetTable))
	copy(cosetTableBitReverse, cosetTable)
	fft.BitReverse(cosetTableBitReverse)

	res.CosetTableBitReverse = cosetTableBitReverse

	return res
}

// NumEncodedCols returns the number of columns in the encoded matrix,
// equivalently this is the size of the codeword-rows.
func (p *Params) NumEncodedCols() int {
	return utils.NextPowerOfTwo(p.NbColumns) * p.BlowUpFactor
}
