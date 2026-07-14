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
func Commit(encoders []*RSEncoder, witness MultiSizeTable) CommitterState {

	k, err := witness.checkWellFormedness()
	if err != nil {
		panic(err)
	}

	if k != 1 {
		panic("k must be one")
	}

	encoded := witness.Encode(encoders)
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

// Merkleize merkleizes the table using Poseidon2. Every table but the bottom
// (largest) one is digested as conjugate pairs, one tree depth shallower than
// its own size, so it folds the same way the bottom table's leaf pairs do.
func (table MultiSizeTable) Merkleize() *Tree {

	bottom := len(table) - 1

	// One slot wider than table: shallowest slot (index 0's shifted pair)
	// would otherwise be a negative index.
	leaves := make([][]field.Octuplet, len(table)+1)
	hasher := poseidon2.NewMDHasher()

	if table[bottom].NumRows() > 0 {
		size := table[bottom].Size()
		leaves[len(leaves)-1] = make([]field.Octuplet, size)
		for j := range size {
			hasher.Reset()
			writeRowElements(hasher, table[bottom], j)
			leaves[len(leaves)-1][j] = hasher.SumDigest()
		}
	}

	for i := range bottom {
		if table[i].NumRows() == 0 {
			continue
		}
		size := table[i].Size()
		leaves[i] = make([]field.Octuplet, size/2)
		for j := range size / 2 {
			hasher.Reset()
			writeRowElements(hasher, table[i], 2*j)
			writeRowElements(hasher, table[i], 2*j+1)
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

// writeRowElements absorbs one row into hasher without resetting or summing,
// so a caller can digest several rows into one combined value.
func writeRowElements(hasher *poseidon2.MDHasher, t SizedTable, row int) {
	for k := range t.Base {
		hasher.WriteElements(t.Base[k][row])
	}
	for k := range t.Ext {
		ext := t.Ext[k][row]
		hasher.WriteElements(
			ext.B0.A0, ext.B0.A1,
			ext.B1.A0, ext.B1.A1,
			ext.B2.A0, ext.B2.A1)
	}
}

// Shape returns the per-size row counts of the batch, discarding the
// polynomial values. It is the verifier-side view of a committed batch: a
// caller that holds the committed table builds VerifyInputs.Shapes from it,
// without needing the witness data.
func (table MultiSizeTable) Shape() Shape {
	shape := make(Shape, len(table))
	for sizeLog2 := range table {
		shape[sizeLog2] = SizedShape{
			BaseWidth: len(table[sizeLog2].Base),
			ExtWidth:  len(table[sizeLog2].Ext),
		}
	}
	return shape
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
