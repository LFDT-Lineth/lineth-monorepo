package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// leafDomainTag domain-separates Merkle leaves so a table with the same row
// values but a different (BaseWidth, ExtWidth) shape hashes to a different
// digest. Without this, e.g. an all-zero base row and an all-zero ext row
// collide, letting two structurally distinct commitments share a Merkle root
// and get deduplicated inside inputOpeningRoots.
const leafDomainTag uint64 = 0x4c66_7269_5f6c_6631 // "Lfri_lf1"

// absorbLeafHeader writes the domain tag and (baseWidth, extWidth) into h
// before any row values. Prover ([MultiSizeTable.Merkleize]) and verifier
// ([hashRowOpening]) MUST call this identically or roots will not
// reconstruct.
func absorbLeafHeader(h *poseidon2.MDHasher, baseWidth, extWidth int) {
	var tag, b, e field.Element
	tag.SetUint64(leafDomainTag)
	b.SetUint64(uint64(baseWidth))
	e.SetUint64(uint64(extWidth))
	h.WriteElements(tag, b, e)
}

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
		// One contiguous slab per size, pre-sliced per column: per-column
		// codeword allocations are large objects that contend on the page heap
		// and the kernel fault path under 96-way parallelism (measured >90%
		// system time on a cold one-shot wide commit).
		N := int(encoders[i].Domain.Cardinality)
		encoded[i].Base = slabColumns[field.Element](len(table[i].Base), N)
		encoded[i].Ext = slabColumns[field.Ext](len(table[i].Ext), N)
	}
	parallel.Execute(len(work), func(start, end int) {
		for w := start; w < end; w++ {
			it := work[w]
			if it.ext {
				encoded[it.i].Ext[it.k] = encoders[it.i].EncodeExt(table[it.i].Ext[it.k], fft.WithNbTasks(1))
			} else {
				encoded[it.i].Base[it.k] = encoders[it.i].Encode(table[it.i].Base[it.k], fft.WithNbTasks(1))
			}
		}
	})

	// Each row's RS encode is an independent per-row FFT writing a disjoint
	// output slice, so flatten (size, base/ext, row) into work items and encode
	// them in parallel. gnark's FFT barely parallelizes at these row sizes, so
	// the parallelism must be across rows; fft.WithNbTasks(1) keeps each FFT
	// single-threaded so the outer parallelism isn't nested.
	type encodeItem struct {
		i, k int
		ext  bool
	}
	var work []encodeItem
	for i := range table {
		for k := range table[i].Base {
			work = append(work, encodeItem{i: i, k: k})
		}
		for k := range table[i].Ext {
			work = append(work, encodeItem{i: i, k: k, ext: true})
		}
	}
	encodeOpts := []fft.Option{fft.WithNbTasks(1)}
	parallel.Execute(len(work), func(start, end int) {
		for w := start; w < end; w++ {
			it := work[w]
			if it.ext {
				encoders[it.i].EncodeExtInto(table[it.i].Ext[it.k], encoded[it.i].Ext[it.k], encodeOpts...)
			} else {
				encoders[it.i].EncodeInto(table[it.i].Base[it.k], encoded[it.i].Base[it.k], encodeOpts...)
			}
		}
	})

	return encoded
}

// slabColumns allocates count columns of n elements as sub-slices of one
// contiguous slab, each capped so a column cannot grow into its neighbor.
func slabColumns[T any](count, n int) [][]T {
	columns := make([][]T, count)
	if count == 0 {
		return columns
	}
	slab := make([]T, count*n)
	for k := range columns {
		columns[k] = slab[k*n : (k+1)*n : (k+1)*n]
	}
	return columns
}

// Merkleize merkleizes the table using Poseidon2. Every table but the bottom
// (largest) one is digested as conjugate pairs, one tree depth shallower than
// its own size, so it folds the same way the bottom table's leaf pairs do.
func (table MultiSizeTable) Merkleize() *Tree {

	bottom := len(table) - 1
	if table[bottom].NumRows() == 0 {
		panic("the bottom level must be non-empty")
	}

	// Leaf hashing dominates Commit; it is parallel over leaf index and
	// vectorizes 16-wide with AVX-512, so hashSizedLeaves handles each size.
	// The bottom leaves are hashed directly into the tree's node storage,
	// which saves allocating and copying a leaf array as large as the encoded
	// bottom table's height.
	size := table[bottom].Size()
	tree := allocTree(size)
	hashSizedLeaves(table[bottom], false, tree.Nodes[size-1:])

	// Every table but the bottom is digested as conjugate pairs, one tree
	// depth shallower than its own size: a table of encoded height s yields
	// s/2 auxiliary leaves attached at the level holding s/2 nodes.
	upperLeaves := make([][]field.Octuplet, utils.Log2Ceil(size))
	for i := range bottom {
		if table[i].NumRows() == 0 {
			continue
		}
		s := table[i].Size()
		if s == 1 {
			continue
		}
		digests := make([]field.Octuplet, s/2)
		hashSizedLeaves(table[i], true, digests)
		upperLeaves[utils.Log2Ceil(s/2)] = digests
	}

	tree.buildLevels(upperLeaves)
	return tree
}

// writeRowElements absorbs one row into hasher without resetting or summing,
// so a caller can digest several rows into one combined value.
func writeRowElements(hasher *poseidon2.MDHasher, t SizedTable, row int) {
	for k := range t.Base {
		hasher.WriteElements(t.Base[k][row])
	}
	for k := range t.Ext {
		limbs := extLimbs(t.Ext[k][row])
		hasher.WriteElements(limbs[:]...)
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
