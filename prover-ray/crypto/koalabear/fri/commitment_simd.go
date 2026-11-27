package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
)

// simdLanes is the SIMD width of the AVX-512 batch Poseidon2 permutation: 16
// independent Merkle-Damgard chains run per
// vortex.CompressPoseidon2x16Columns call.
const simdLanes = 16

// leafLayout describes how one Merkleize leaf's element stream is laid out in
// the batch-permutation matrix: the stream is blocked into groups of 8 with the
// final partial block LEFT-padded with zeros, so the kernel's front-to-back
// chunking reproduces MDHasher's block sequence (the sponge starts from a zero
// state and absorbs each block with feed-forward, so the result is bit-identical
// to hashLeafScalar). All leaves of a size share this layout because they share
// (baseWidth, extWidth) and pairing.
type leafLayout struct {
	baseWidth int
	extWidth  int
	paired    bool // leaf digests two adjacent rows (non-bottom levels), else one
	header    [3]field.Element

	frontLen int // elements before the (left-padded) final block: streamLen - streamLen%8
	pad      int // leading zeros inside the final block: (8 - streamLen%8) % 8
	colSize  int // padded row length, a multiple of 8: streamLen + pad
}

func newLeafLayout(t SizedTable, paired bool) leafLayout {
	l := leafLayout{
		baseWidth: len(t.Base),
		extWidth:  len(t.Ext),
		paired:    paired,
	}
	l.header[0].SetUint64(LeafDomainTag)
	l.header[1].SetUint64(uint64(l.baseWidth))
	l.header[2].SetUint64(uint64(l.extWidth))

	rows := 1
	if paired {
		rows = 2
	}
	rowElems := l.baseWidth + 6*l.extWidth
	streamLen := len(l.header) + rows*rowElems
	l.pad = (poseidon2.BlockSize - streamLen%poseidon2.BlockSize) % poseidon2.BlockSize
	l.frontLen = streamLen - streamLen%poseidon2.BlockSize
	l.colSize = streamLen + l.pad
	return l
}

// dpos maps a logical stream index to its matrix stream position, inserting the
// pad gap so the final block ends up left-padded (matching MDHasher). Gap
// positions [frontLen, frontLen+pad) are never a dpos output, so they stay zero
// (the matrix is allocated zeroed and reused).
func (l leafLayout) dpos(p int) int {
	if p >= l.frontLen {
		return p + l.pad
	}
	return p
}

// fillGroup fills the column-major matrix for the leaf group [j, j+16), laid
// out matrix[pos*16+lane] as vortex.CompressPoseidon2x16Columns expects: a
// column's 16 leaves are contiguous, so the kernel loads each rate coordinate
// instead of gathering.
func (l leafLayout) fillGroup(matrix []field.Element, t SizedTable, j int) {
	// Header: identical for every lane.
	for i := range l.header {
		base := l.dpos(i) * simdLanes
		h := l.header[i]
		for lane := range simdLanes {
			matrix[base+lane] = h
		}
	}
	if l.paired {
		l.fillPairedRows(matrix, t, j)
		return
	}
	l.fillRows(matrix, t, j)
}

// fillRows writes rows [row, row+16) of every base then ext column into the
// matrix, one row per lane (bottom level, unpaired leaves).
func (l leafLayout) fillRows(matrix []field.Element, t SizedTable, row int) {
	s := len(l.header)
	for k := range t.Base {
		dp := l.dpos(s) * simdLanes
		// 16 consecutive leaves of this column land contiguously.
		copy(matrix[dp:dp+simdLanes], t.Base[k][row:row+simdLanes])
		s++
	}
	for k := range t.Ext {
		col := t.Ext[k]
		dp := l.extPositions(s)
		for lane := range simdLanes {
			writeExtLane(matrix, &dp, lane, &col[row+lane])
		}
		s += 6
	}
}

// fillPairedRows writes the group of 16 pair leaves starting at leaf j: leaf
// j+lane digests rows 2(j+lane) and 2(j+lane)+1, the first row's columns at
// one stream offset and the second row's after them.
//
// It reads each column once, rows [2j, 2j+32) in one run, and splits even
// rows to the first row's position and odd rows to the second's. Filling the
// two rows in separate passes over all columns reads every line twice, and
// the second read misses: the columns are page-aligned slices, so the same
// line of every column maps to the same L1 set, and the first pass has
// evicted it by the time the second pass returns.
func (l leafLayout) fillPairedRows(matrix []field.Element, t SizedTable, j int) {
	off0 := len(l.header)
	off1 := off0 + l.baseWidth + 6*l.extWidth
	r0 := 2 * j
	for k := range t.Base {
		rows := t.Base[k][r0 : r0+2*simdLanes]
		d0 := matrix[l.dpos(off0+k)*simdLanes:][:simdLanes]
		d1 := matrix[l.dpos(off1+k)*simdLanes:][:simdLanes]
		for lane := range simdLanes {
			d0[lane] = rows[2*lane]
			d1[lane] = rows[2*lane+1]
		}
	}
	for k := range t.Ext {
		rows := t.Ext[k][r0 : r0+2*simdLanes]
		dp0 := l.extPositions(off0 + l.baseWidth + 6*k)
		dp1 := l.extPositions(off1 + l.baseWidth + 6*k)
		for lane := range simdLanes {
			writeExtLane(matrix, &dp0, lane, &rows[2*lane])
			writeExtLane(matrix, &dp1, lane, &rows[2*lane+1])
		}
	}
}

// extPositions returns the matrix offsets of the 6 limbs of an ext element at
// stream offset s. The limbs are contiguous unless the pad gap falls inside
// this element (at most one per stream), so they are resolved once, outside
// the lane loops.
func (l leafLayout) extPositions(s int) [6]int {
	var dp [6]int
	for i := range dp {
		dp[i] = l.dpos(s+i) * simdLanes
	}
	return dp
}

// writeExtLane writes the 6 limbs of e to lane of the limb positions dp.
func writeExtLane(matrix []field.Element, dp *[6]int, lane int, e *field.Ext) {
	matrix[dp[0]+lane] = e.B0.A0
	matrix[dp[1]+lane] = e.B0.A1
	matrix[dp[2]+lane] = e.B1.A0
	matrix[dp[3]+lane] = e.B1.A1
	matrix[dp[4]+lane] = e.B2.A0
	matrix[dp[5]+lane] = e.B2.A1
}

// hashLeafScalar reproduces the MDHasher path for a single leaf; used for the
// <16-leaf tail and as the reference fallback.
func (l leafLayout) hashLeafScalar(hasher leafHasher, t SizedTable, j int) field.Octuplet {
	hasher.Reset()
	absorbLeafHeader(hasher, l.baseWidth, l.extWidth)
	if l.paired {
		writeRowElements(hasher, t, 2*j)
		writeRowElements(hasher, t, 2*j+1)
	} else {
		writeRowElements(hasher, t, j)
	}
	return hasher.SumDigest()
}

// hashSizedLeaves hashes one SizedTable's leaves into out, 16 at a time with
// the AVX-512 batch permutation and parallelized across leaf ranges. When
// paired, out has size/2 entries and leaf j digests rows 2j and 2j+1
// (non-bottom levels); otherwise out has size entries and leaf j digests row j
// (bottom level). Digests are bit-identical to the scalar MDHasher path.
func hashSizedLeaves(hash MerkleHash, t SizedTable, paired bool, out []field.Octuplet) {
	l := newLeafLayout(t, paired)
	nbLeaves := len(out)

	// Without a batched path every leaf takes the scalar one, in parallel.
	if !hash.batched() {
		parallel.Execute(nbLeaves, func(start, end int) {
			hasher := hash.newLeafHasher()
			for j := start; j < end; j++ {
				out[j] = l.hashLeafScalar(hasher, t, j)
			}
		})
		return
	}

	// Parallelize over SIMD groups, not individual leaves: parallel.Execute
	// splits its range across GOMAXPROCS workers, so parallelizing over leaves
	// gives each worker a chunk of nbLeaves/GOMAXPROCS. On a many-core machine
	// that chunk is often < simdLanes, which sent whole sizes down the scalar
	// path and never touched the AVX-512 batch permutation. Making the group
	// (16 leaves) the unit of work keeps every full group on the SIMD path and
	// leaves a single scalar tail of nbLeaves%16 for the whole array.
	nbGroups := nbLeaves / simdLanes
	if nbGroups > 0 {
		parallel.Execute(nbGroups, func(start, end int) {
			matrix := make([]field.Element, simdLanes*l.colSize)
			for g := start; g < end; g++ {
				j := g * simdLanes
				l.fillGroup(matrix, t, j)
				vortex.CompressPoseidon2x16Columns(matrix, l.colSize, out[j:j+simdLanes])
			}
		})
	}

	// Scalar tail: the final nbLeaves%16 leaves that do not fill a group.
	if tail := nbGroups * simdLanes; tail < nbLeaves {
		hasher := hash.newLeafHasher()
		for j := tail; j < nbLeaves; j++ {
			out[j] = l.hashLeafScalar(hasher, t, j)
		}
	}
}
