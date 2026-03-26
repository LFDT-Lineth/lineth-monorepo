package vortex_bls12377

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// When SIS: hash_columns = poseidon2_bls12377(SIS(col))
// When no SIS: hash_columns = poseidon2_bls12377(encode_bls12377(col))

// EncodedMatrix represents the witness of a Vortex matrix commitment, it is
// represented as an array of rows.
type EncodedMatrix = []smartvectors.SmartVector

// Commitment represents the root of a Merkle tree
type Commitment = fr.Element

// Params Embeds vortex.Params to define local methods
type Params struct {
	vortex.Params
}

func NewParams(rate, nbColumns, maxNbRows, logTwoDegree, logTwoBound int) Params {

	_params := vortex.NewParams(
		rate,
		nbColumns,
		maxNbRows,
		logTwoDegree,
		logTwoBound)

	return Params{*_params}
}

// CommitMerkleWithSIS
//
// let h: koala* -> frElement
// 				a,b,.. -> poseidon2_bls12377(SIS(a,b,...)

// commits to ps by hashing the columns like this:
// [v11 v12 .... v1n ]
// ..
// [vm1 vm2 .... vmn ]
//
//	|             |
//	v             v
//
// [h() .. ....   h()] := v
// Compute MT of v
func (p *Params) CommitMerkleWithSIS(polysMatrix []smartvectors.SmartVector) (EncodedMatrix, Commitment, *smt_bls12377.Tree, []field.Element) {

	if len(polysMatrix) > p.MaxNbRows {
		utils.
			Panic("too many rows: %v, capacity is %v\n", len(polysMatrix), p.MaxNbRows)
	}

	encodedMatrix := p.EncodeRows(polysMatrix)

	var commitment Commitment

	leaf, colHashes := p.sisTransversalHash(encodedMatrix)

	tree := smt_bls12377.BuildComplete(
		leaf,
	)

	commitment = tree.Root

	return encodedMatrix, commitment, tree, colHashes
}

func (p *Params) sisTransversalHash(v []smartvectors.SmartVector) ([]fr.Element, []field.Element) {

	// sisHashes = [ [a, b ...], ... ] where [a, b, ...] is the sis hash of a column, a, b etc are on koalabear
	// let h = poseidon2_bls377
	// compute [ h([a, b ...]), .. ]
	sisHashes := make([]field.Element, p.RsParams.NbEncodedColumns()*p.Key.OutputSize())
	sisHashes = p.Key.TransversalHash(v, sisHashes)
	chunkSize := p.Key.OutputSize()
	numCols := p.RsParams.NbEncodedColumns()
	leaves := make([]fr.Element, numCols)

	parallel.Execute(numCols, func(start, stop int) {

		hasher := poseidon2_bls12377.NewMDHasher()

		for chunkID := start; chunkID < stop; chunkID++ {
			startChunk := chunkID * chunkSize
			hasher.Reset()
			hasher.WriteKoalabearElements(sisHashes[startChunk : startChunk+chunkSize]...)
			leaves[chunkID] = hasher.SumElement()
		}
	})
	return leaves, sisHashes
}

// CommitMerkleWithoutSIS
//
// let h: koala* -> frElement
// 				a,b,.. -> poseidon2_bls12377(a,b,...)

// commits to ps by hashing the columns like this:
// [v11 v12 .... v1n ]
// ..
// [vm1 vm2 .... vmn ]
//
//	|             |
//	v             v
//
// [h() .. ....   h()] := v
// Compute MT of v
func (p *Params) CommitMerkleWithoutSIS(polysMatrix []smartvectors.SmartVector) (EncodedMatrix, Commitment, *smt_bls12377.Tree, []fr.Element) {

	if len(polysMatrix) > p.MaxNbRows {
		utils.Panic("too many rows: %v, capacity is %v\n", len(polysMatrix), p.MaxNbRows)
	}

	encodedMatrix := p.EncodeRows(polysMatrix)

	var commitment Commitment
	colHashes := p.noSisTransversalHash(encodedMatrix)

	tree := smt_bls12377.BuildComplete(
		colHashes,
	)

	commitment = tree.Root
	return encodedMatrix, commitment, tree, colHashes
}

// Uses the no-sis hash function to hash the columns. It uses the leafHasher
// function to hash the columns.
func (p *Params) noSisTransversalHash(v []smartvectors.SmartVector) []fr.Element {

	// Assert that all smart-vectors have the same numCols
	nbCols := v[0].Len()
	for i := range v {
		if v[i].Len() != nbCols {
			utils.Panic("Unexpected : all inputs smart-vectors should have the same length the first one has length %v, but #%v has length %v",
				nbCols, i, v[i].Len())
		}
	}

	nbRows := len(v)
	res := make([]fr.Element, nbCols)
	parallel.Execute(nbCols, func(start, end int) {
		curCol := make([]field.Element, nbRows)
		h := poseidon2_bls12377.NewMDHasher()
		for i := start; i < end; i++ {
			for j := 0; j < nbRows; j++ {
				curCol[j] = v[j].Get(i)
			}
			h.WriteKoalabearElements(curCol...)
			res[i] = h.SumElement()
			h.Reset()
		}
	})

	return res
}

// EncodeRows returns the encodes `ps` using Reed-Solomon. ps is interpreted as
// a list of rows of the Vortex witness and encodedMatrix is obtained by
// encoding each of the [smartvectors.SmartVector] it contains separately.
func (params *Params) EncodeRows(ps []smartvectors.SmartVector) (encodedMatrix EncodedMatrix) {

	// Sanity-check, all the vectors must have the right length
	for i := range ps {
		if ps[i].Len() != params.NbColumns {
			utils.Panic("Bad length : expected %v columns but col %v has size %v", params.NbColumns, i, ps[i].Len())
		}
	}

	// The committed matrix is obtained by encoding the input vectors
	// and laying them in rows.
	encodedMatrix = make(EncodedMatrix, len(ps))
	parallel.Execute(len(ps), func(start, stop int) {
		for i := start; i < stop; i++ {
			encodedMatrix[i] = params.RsParams.RsEncodeBase(ps[i])
		}
	})

	return encodedMatrix
}
