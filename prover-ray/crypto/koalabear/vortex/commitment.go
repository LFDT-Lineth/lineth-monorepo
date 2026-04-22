package vortex

import (
	vgnark "github.com/consensys/gnark-crypto/field/koalabear/vortex"
	poseidon2 "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
	smt "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/smt"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
	"github.com/consensys/linea-monorepo/prover-ray/utils/parallel"
)

// When SIS: hash_columns = poseidon2(SIS(col))
// When no SIS: hash_columns = poseidon2(encode_bls12377(col))

// EncodedMatrix represents the witness of a Vortex matrix commitment, it is
// represented as an array of rows.
type EncodedMatrix = [][]field.Element

// Commitment represents the root of a Merkle tree
type Commitment = field.Octuplet

// CommitMerkleWithSIS commits to ps by hashing the columns like this:
// [v11 v12 .... v1n ]
// ..
// [vm1 vm2 .... vmn ]
//
//	|             |
//	v             v
//
// [h() .. ....   h()] := v
// Compute MT of v
func (p *Params) CommitMerkleWithSIS(polysMatrix [][]field.Element) (EncodedMatrix, Commitment, *smt.Tree,
	[]field.Element) {

	if len(polysMatrix) > p.MaxNbRows {
		utils.
			Panic("too many rows: %v, capacity is %v\n", len(polysMatrix), p.MaxNbRows)
	}

	var (
		encodedMatrix   = p.EncodeRows(polysMatrix)
		leaf, colHashes = p.sisTransversalHash(encodedMatrix)
		tree            = smt.NewTree(leaf)
		commitment      = tree.Root
	)

	return encodedMatrix, commitment, tree, colHashes
}

func (p *Params) sisTransversalHash(v [][]field.Element) ([]field.Octuplet, []field.Element) {

	// sisHashes = [ [a, b ...], ... ] where [a, b, ...] is the sis hash of a column, a, b etc are on koalabear
	// let h = poseidon2
	// compute [ h([a, b ...]), .. ]
	sisHashes := make([]field.Element, p.RsParams.NbEncodedColumns()*p.Key.OutputSize())
	sisHashes = p.Key.TransversalHash(v, sisHashes)
	chunkSize := p.Key.OutputSize()
	numCols := p.RsParams.NbEncodedColumns()
	leaves := make([]field.Octuplet, numCols)

	r := numCols % 16
	n := numCols / 16

	if chunkSize%8 != 0 {
		// @gbotrel make the fast path generic with different SIS params
		parallel.Execute(numCols, func(start, stop int) {
			hasher := poseidon2.NewMDHasher()
			for chunkID := start; chunkID < stop; chunkID++ {
				startChunk := chunkID * chunkSize
				hasher.Reset()
				hasher.WriteElements(sisHashes[startChunk : startChunk+chunkSize]...)
				leaves[chunkID] = hasher.SumElement()
			}
		})
		return leaves, sisHashes
	}

	// process the n full chunks of 16 columns using optimized SIMD implementation
	// if available
	parallel.Execute(n, func(start, stop int) {
		for chunkID := start; chunkID < stop; chunkID++ {
			startChunk := chunkID * 16 * chunkSize
			vgnark.CompressPoseidon2x16(sisHashes[startChunk:startChunk+16*chunkSize], chunkSize,
				leaves[chunkID*16:(chunkID+1)*16])
		}
	})

	// process the remaining r columns
	hasher := poseidon2.NewMDHasher()
	for i := n * 16; i < n*16+r; i++ {
		startChunk := i * chunkSize
		hasher.Reset()
		hasher.WriteElements(sisHashes[startChunk : startChunk+chunkSize]...)
		leaves[i] = hasher.SumElement()
	}

	return leaves, sisHashes
}

// CommitMerkleWithoutSIS commits to ps by hashing the columns without SIS.
func (p *Params) CommitMerkleWithoutSIS(polysMatrix [][]field.Element) (EncodedMatrix, Commitment, *smt.Tree,
	[]field.Element) {

	if len(polysMatrix) > p.MaxNbRows {
		utils.Panic("too many rows: %v, capacity is %v\n", len(polysMatrix), p.MaxNbRows)
	}

	encodedMatrix := p.EncodeRows(polysMatrix)

	var commitment Commitment

	colHashesOcts := p.noSisTransversalHash(encodedMatrix)
	colHashes := make([]field.Element, 0, len(colHashesOcts)*len(field.Octuplet{}))
	for i := range colHashesOcts {
		colHashes = append(colHashes, colHashesOcts[i][:]...)
	}

	tree := smt.NewTree(
		colHashesOcts,
	)

	commitment = tree.Root
	return encodedMatrix, commitment, tree, colHashes
}

// Uses the no-sis hash function to hash the columns. It uses the leafHasher
// function to hash the columns.
func (p *Params) noSisTransversalHash(v [][]field.Element) []field.Octuplet {

	// Assert that all smart-vectors have the same numCols
	nbCols := len(v[0])
	for i := range v {
		if len(v[i]) != nbCols {
			utils.Panic("Unexpected : all inputs smart-vectors should have the same length the first one has length %v, but #%v has length %v",
				nbCols, i, len(v[i]))
		}
	}

	nbRows := len(v)
	res := make([]field.Octuplet, nbCols)
	parallel.Execute(nbCols, func(start, end int) {
		curCol := make([]field.Element, nbRows)
		h := poseidon2.NewMDHasher()
		for i := start; i < end; i++ {
			for j := 0; j < nbRows; j++ {
				curCol[j] = v[j][i]
			}
			h.WriteElements(curCol...)
			res[i] = h.SumElement()
			h.Reset()
		}
	})

	return res
}

// EncodeRows returns the encodes `ps` using Reed-Solomon. ps is interpreted as
// a list of rows of the Vortex witness and encodedMatrix is obtained by
// encoding each of the [smartvectors.SmartVector] it contains separately.
func (p *Params) EncodeRows(ps [][]field.Element) (encodedMatrix EncodedMatrix) {

	// Sanity-check, all the vectors must have the right length
	for i := range ps {
		if len(ps[i]) != p.NbColumns {
			utils.Panic("Bad length : expected %v columns but col %v has size %v", p.NbColumns, i, len(ps[i]))
		}
	}

	// The committed matrix is obtained by encoding the input vectors
	// and laying them in rows.
	encodedMatrix = make(EncodedMatrix, len(ps))
	parallel.Execute(len(ps), func(start, stop int) {
		for i := start; i < stop; i++ {
			encodedMatrix[i] = p.RsParams.RsEncodeBase(ps[i])
		}
	})

	return encodedMatrix
}
