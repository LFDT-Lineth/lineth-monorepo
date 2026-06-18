package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// CommitterState collects the data that are built during the commitment phase
// of FRI. This includes the RS codewords and their Merkle tree.
type CommitterState struct {
	// EncodedTable is the list of the codewords sorted in tables.
	EncodedTable MultiSizeTable
	// Tree is the Merkle tree for the EncodeTable.
	Tree *Tree
}

// Commit commits to a sorted list of tables. The table must satisfy the format
// expected by [MultiSizeTable.checkWellFormedness] with a K of 1.
func Commit(encoder []*RSEncoder, witness MultiSizeTable) CommitterState {

	k, err := witness.checkWellFormedness()
	if err != nil {
		panic(err)
	}

	if k != 1 {
		panic("k must be one")
	}

	encoded := witness.Encode(encoder)
	tree := encoded.Merkleize()

	return CommitterState{
		EncodedTable: encoded,
		Tree:         tree,
	}
}

// Encode encodes all the subtable of the MultiSizeTable using the provided
// list of encoder.
//
// The function expects that the encoder is well-formed: see
// [assertValidMultiEncoder].
func (table MultiSizeTable) Encode(encoders []*RSEncoder) MultiSizeTable {

	assertValidMultiEncoder(encoders)
	encoded := make([]SizedTable, len(table))

	for i := range table {

		encoded[i].Base = make([][]field.Element, len(table[i].Base))
		for k, base := range table[i].Base {
			encoded[i].Base[k] = encoders[i].Encode(base)
		}

		encoded[i].Ext = make([][]field.Ext, len(table[i].Ext))
		for k, ext := range table[i].Ext {
			encoded[i].Ext[k] = encoders[i].EncodeExt(ext)
		}
	}

	return encoded
}

// Merkleize merkleizes the table using Poseidon2. The encoded table is hashed
// line-by-line to form the leaves and auxiliary leaves of a [Tree]. The tree
// is then built from these leaves.
func (table MultiSizeTable) Merkleize() *Tree {

	// leaves stores the Merkle leaves of the tree
	leaves := make([][]field.Octuplet, len(table))
	hasher := poseidon2.NewMDHasher()

	for i := range table {
		if table[i].NumRows() == 0 {
			continue
		}

		size := table[i].Size()
		leaves[i] = make([]field.Octuplet, size)

		for j := range size {
			hasher.Reset()

			for k := range table[i].Base {
				hasher.WriteElements(table[i].Base[k][j])
			}

			for k := range table[i].Ext {
				ext := table[i].Ext[k][j]
				hasher.WriteElements(
					ext.B0.A0, ext.B0.A1,
					ext.B1.A0, ext.B1.A1,
					ext.B2.A0, ext.B2.A1)
			}

			leaves[i][j] = hasher.SumDigest()
		}
	}

	// NewTree expects the levels in increasing-size order, from the top of the
	// tree (smallest) down to the bottom layer (largest). The table is already
	// sorted by increasing size, so the largest committed table is the last one.
	//
	// The blowup makes the largest committed table have len(leaves[last]) rows; a
	// complete binary tree over them has Log2Ceil(len)+1 levels. The levels above
	// the smallest committed table carry no auxiliary leaves, so prepend empty
	// levels at the top until we reach the tree height.
	targetLevels := utils.Log2Ceil(len(leaves[len(leaves)-1])) + 1
	if pad := targetLevels - len(leaves); pad > 0 {
		leaves = append(make([][]field.Octuplet, pad), leaves...)
	}

	return NewTree(leaves)
}

// assertValidMultiEncoder checks that the provided list of encoder:
//   - share the same inverse rate
//   - coder[i].PlainTextSize == 2**i
//
// It panics on failure.
func assertValidMultiEncoder(encoders []*RSEncoder) {

	inverseRate := encoders[0].InverseRate()

	for i := range encoders {

		if inverseRate != encoders[i].InverseRate() {
			panic("the encoder do not all have the same rate")
		}

		if encoders[i].PlainTextSize != 1<<i {
			panic("the encoder does not have the right plaintext size")
		}
	}
}
