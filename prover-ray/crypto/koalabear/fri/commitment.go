package fri

import (
	"math/bits"
	"runtime"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/bufpool"
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
		// The codewords come from the field pools (see [CommitterState.Release]):
		// a proof reuses the previous one's, instead of zeroing and faulting
		// in a few GB of fresh memory per commit. The encoders overwrite every
		// element.
		N := int(encoders[i].Domain.Cardinality)
		encoded[i].Base = pooledColumns(&field.BasePool, len(table[i].Base), N)
		encoded[i].Ext = pooledColumns(&field.ExtPool, len(table[i].Ext), N)
	}

	// Each row's RS encode is an independent per-row FFT writing a disjoint
	// output slice, so flatten (size, base/ext, row) into work items and encode
	// them in parallel. gnark's FFT barely parallelizes at these row sizes, so
	// the parallelism is mostly across rows.
	//
	// Row costs span several orders of magnitude (sizes 2^0 .. 2^22, base or
	// extension), so the items are pulled dynamically, largest size first and
	// extension before base within a size: a contiguous split put every one of
	// the largest rows on the last few workers. Each row's FFTs get a share of
	// the CPUs proportional to its share of the work, at least one, so that a
	// handful of the largest rows do not each run on a single core while the
	// rest of the table is already done.
	type encodeItem struct {
		i, k int
		ext  bool
		cost float64
	}
	var (
		work  []encodeItem
		total float64
	)
	for i := len(table) - 1; i >= 0; i-- {
		N := float64(encoders[i].Domain.Cardinality)
		cost := N * max(1, float64(bits.Len(uint(N))))
		for k := range table[i].Ext {
			work = append(work, encodeItem{i: i, k: k, ext: true, cost: 6 * cost})
			total += 6 * cost
		}
		for k := range table[i].Base {
			work = append(work, encodeItem{i: i, k: k, cost: cost})
			total += cost
		}
	}
	cpus := float64(runtime.GOMAXPROCS(0))
	parallel.ExecuteDynamic(len(work), func(w int) {
		it := work[w]
		encodeOpts := []fft.Option{fft.WithNbTasks(max(1, int(cpus*it.cost/total)))}
		if it.ext {
			encoders[it.i].EncodeExtInto(table[it.i].Ext[it.k], encoded[it.i].Ext[it.k], encodeOpts...)
		} else {
			encoders[it.i].EncodeInto(table[it.i].Base[it.k], encoded[it.i].Base[it.k], encodeOpts...)
		}
	})

	return encoded
}

// pooledColumns takes count columns of n elements from pool, in parallel:
// columns allocated afresh are zeroed by the goroutine allocating them.
func pooledColumns[T any](pool *bufpool.Pool[T], count, n int) [][]T {
	columns := make([][]T, count)
	parallel.Execute(count, func(start, end int) {
		for k := start; k < end; k++ {
			columns[k] = pool.Get(n)
		}
	})
	return columns
}

// Release returns the codewords and the Merkle tree nodes of st to the field
// pools. st must not be used afterwards, and none of its slices may still be
// referenced: an opening proof only holds copies.
func (st *CommitterState) Release() {
	for _, sized := range st.EncodedTable {
		for _, col := range sized.Base {
			field.BasePool.Put(col)
		}
		for _, col := range sized.Ext {
			field.ExtPool.Put(col)
		}
	}
	if st.Tree != nil {
		field.OctupletPool.Put(st.Tree.Nodes)
	}
	st.EncodedTable, st.Tree = nil, nil
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
	// The tables of the other sizes are independent: they are digested
	// concurrently, largest first, each over the CPUs as well, so that the
	// many small ones do not each wait for a parallel pass of their own.
	upperLeaves := make([][]field.Octuplet, utils.Log2Ceil(size))
	var sizes []int
	for i := bottom - 1; i >= 0; i-- {
		if table[i].NumRows() != 0 && table[i].Size() > 1 {
			sizes = append(sizes, i)
		}
	}
	parallel.ExecuteDynamic(len(sizes), func(k int) {
		i := sizes[k]
		s := table[i].Size()
		digests := make([]field.Octuplet, s/2)
		hashSizedLeaves(table[i], true, digests)
		upperLeaves[utils.Log2Ceil(s/2)] = digests
	})

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
