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
		encoded[i].Groups = table[i].Groups
	}

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
//
// An ungrouped table (no size declares [SizedTable.Groups]) hashes its bottom
// table one row per leaf.
//
// A grouped table digests every group of every size on its own, with its own
// leaf header, as conjugate row pairs, the bottom table included: a group's
// digest is H(header, row[2j] ‖ row[2j+1]) whether its size is the largest or
// not. The group digests of a size are injected into their level in group
// order, and the bottom table's level becomes the leaves, each one the first
// group's digest with the other groups' digests chained after it,
// C(…C(h_1, h_2)…, h_m). The tree is then one level shorter, and the opening
// proof keeps its shape.
func (table MultiSizeTable) Merkleize() *Tree {

	bottom := len(table) - 1
	if table[bottom].NumRows() == 0 {
		panic("the bottom level must be non-empty")
	}
	pairLeaves := table.grouped()

	// Leaf hashing dominates Commit; it is parallel over leaf index and
	// vectorizes 16-wide with AVX-512, so hashSizedLeaves handles each size.
	// The bottom leaves are hashed directly into the tree's node storage,
	// which saves allocating and copying a leaf array as large as the encoded
	// bottom table's height.
	size := table[bottom].Size()
	numLeaves := size
	if pairLeaves {
		if size < 2 {
			panic("fri: pair leaves need a largest size of at least two encoded rows")
		}
		numLeaves = size / 2
	}
	tree := allocTree(numLeaves)
	leaves := tree.Nodes[numLeaves-1:]
	if pairLeaves {
		hashPairLeaves(table[bottom].groupViews(), leaves)
	} else {
		hashSizedLeaves(table[bottom], false, leaves)
	}

	// Every table but the bottom is digested as conjugate pairs, one tree
	// depth shallower than its own size: a table of encoded height s yields,
	// per group, s/2 auxiliary leaves attached at the level holding s/2 nodes.
	injections := make([][][]field.Octuplet, utils.Log2Ceil(numLeaves))
	for i := range bottom {
		if table[i].NumRows() == 0 {
			continue
		}
		s := table[i].Size()
		if s == 1 {
			continue
		}
		level := utils.Log2Ceil(s / 2)
		for _, group := range table[i].groupViews() {
			digests := make([]field.Octuplet, s/2)
			hashSizedLeaves(group, true, digests)
			injections[level] = append(injections[level], digests)
		}
	}

	tree.buildLevels(injections)
	return tree
}

// hashPairLeaves writes the pair-leaf digests of the bottom groups into leaves:
// leaf j is the first group's digest of rows (2j, 2j+1), with the next groups'
// digests chained after it.
func hashPairLeaves(groups []SizedTable, leaves []field.Octuplet) {
	hashSizedLeaves(groups[0], true, leaves)
	if len(groups) == 1 {
		return
	}
	digests := make([]field.Octuplet, len(leaves))
	for _, group := range groups[1:] {
		hashSizedLeaves(group, true, digests)
		parallel.Execute(len(leaves), func(start, end int) {
			for j := start; j < end; j++ {
				leaves[j] = poseidon2.Compress(leaves[j], digests[j])
			}
		})
	}
}

// chainDigests is the pair-leaf node of one position: the first digest with
// the others chained after it, C(…C(d_1, d_2)…, d_m).
func chainDigests(digests []field.Octuplet) field.Octuplet {
	res := digests[0]
	for _, d := range digests[1:] {
		res = poseidon2.Compress(res, d)
	}
	return res
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
			Groups:    table[sizeLog2].Groups,
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
