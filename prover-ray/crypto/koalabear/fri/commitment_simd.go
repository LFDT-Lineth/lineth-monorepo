package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
)

// simdLanes is the SIMD width of the AVX-512 batch Poseidon2 permutation: 16
// independent Merkle-Damgard chains run per Compressx16 call.
const simdLanes = 16

// leafLayout describes how one Merkleize leaf's element stream is laid out in a
// Compressx16 matrix row so the front-to-back 8-element chunking reproduces
// MDHasher's block sequence (see poseidon2.Compressx16). All leaves of a size
// share this layout because they share (baseWidth, extWidth) and pairing.
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
	l.header[0].SetUint64(leafDomainTag)
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

// fillGroup fills the column-major matrix for the leaf group [g, g+16), laid out
// matrix[pos*16+lane] as poseidon2.Compressx16Columns expects. In this layout a
// column's 16 leaves are contiguous, so base columns are filled with a single
// bulk copy of 16 consecutive leaf values (no per-element scatter), and the
// kernel loads instead of gathering.
func (l leafLayout) fillGroup(matrix []field.Element, t SizedTable, g int) {
	// Header: identical for every lane.
	for i := range l.header {
		base := l.dpos(i) * simdLanes
		h := l.header[i]
		for lane := range simdLanes {
			matrix[base+lane] = h
		}
	}

	// row(lane) = base + stride*lane. Non-bottom (paired) leaves digest two
	// adjacent rows, so their 16 leaves are stride-2; the bottom level is
	// stride-1 (contiguous), which enables the bulk-copy fast path.
	if l.paired {
		l.fillColumns(matrix, t, len(l.header), 2*g, 2)
		off := len(l.header) + l.baseWidth + 6*l.extWidth
		l.fillColumns(matrix, t, off, 2*g+1, 2)
	} else {
		l.fillColumns(matrix, t, len(l.header), g, 1)
	}
}

// fillColumns writes one row's base then ext columns into the column-major
// matrix, starting at stream offset off, reading source row base+stride*lane
// for lane 0..15.
func (l leafLayout) fillColumns(matrix []field.Element, t SizedTable, off, base, stride int) {
	s := off
	for k := range t.Base {
		col := t.Base[k]
		dp := l.dpos(s) * simdLanes
		if stride == 1 {
			// 16 consecutive leaves of this column land contiguously.
			copy(matrix[dp:dp+simdLanes], col[base:base+simdLanes])
		} else {
			for lane := range simdLanes {
				matrix[dp+lane] = col[base+stride*lane]
			}
		}
		s++
	}
	for k := range t.Ext {
		col := t.Ext[k]
		// The 6 limbs are contiguous unless the pad gap falls inside this
		// column (at most one column across the stream), so resolve each
		// destination once, outside the lane loop.
		var dp [6]int
		for i := range dp {
			dp[i] = l.dpos(s+i) * simdLanes
		}
		for lane := range simdLanes {
			e := &col[base+stride*lane]
			matrix[dp[0]+lane] = e.B0.A0
			matrix[dp[1]+lane] = e.B0.A1
			matrix[dp[2]+lane] = e.B1.A0
			matrix[dp[3]+lane] = e.B1.A1
			matrix[dp[4]+lane] = e.B2.A0
			matrix[dp[5]+lane] = e.B2.A1
		}
		s += 6
	}
}

// hashLeafScalar reproduces the MDHasher path for a single leaf; used for the
// <16-leaf tail and as the reference fallback.
func (l leafLayout) hashLeafScalar(hasher *poseidon2.MDHasher, t SizedTable, j int) field.Octuplet {
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
func hashSizedLeaves(t SizedTable, paired bool, out []field.Octuplet) {
	l := newLeafLayout(t, paired)
	nbLeaves := len(out)

	parallel.Execute(nbLeaves, func(start, end int) {
		matrix := make([]field.Element, simdLanes*l.colSize)
		var hasher *poseidon2.MDHasher

		j := start
		for ; j+simdLanes <= end; j += simdLanes {
			l.fillGroup(matrix, t, j)
			poseidon2.Compressx16Columns(matrix, l.colSize, out[j:j+simdLanes])
		}
		for ; j < end; j++ {
			if hasher == nil {
				hasher = poseidon2.NewMDHasher()
			}
			out[j] = l.hashLeafScalar(hasher, t, j)
		}
	})
}
