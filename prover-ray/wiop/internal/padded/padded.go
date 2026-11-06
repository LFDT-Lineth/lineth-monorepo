// Package padded reads blocks of rows of a padded, cyclically indexed column
// without materialising it.
//
// A padded column has n rows: rows [start, start+len(data)) hold data, every
// other row holds the padding value. start may be negative (data overflowing
// a left-padded column: only its last rows are visible) and start+len(data)
// may exceed n (data overflowing on the right: only its first n−start rows
// are visible). Rows are indexed modulo n, so a read may wrap around the end
// of the column.
package padded

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

// Read writes rows idx, idx+1, …, idx+len(out)−1 (mod n) of the padded column
// into out. idx must be in [0, n).
func Read[T any](out, data []T, pad T, start, n, idx int) {
	for o, p := 0, idx; o < len(out); o, p = o+(n-p), 0 {
		seg := out[o:min(len(out), o+n-p)]
		lo, hi := overlap(start-p, len(data), len(seg))
		fill(seg[:lo], pad)
		if lo < hi {
			copy(seg[lo:hi], data[p+lo-start:p+hi-start])
		}
		fill(seg[hi:], pad)
	}
}

// ReadLift is [Read] of base-field data and padding into the extension
// field, lifting every value.
func ReadLift(out []field.Ext, data []field.Element, pad field.Element, start, n, idx int) {
	padExt := field.Lift(pad)
	for o, p := 0, idx; o < len(out); o, p = o+(n-p), 0 {
		seg := out[o:min(len(out), o+n-p)]
		lo, hi := overlap(start-p, len(data), len(seg))
		fill(seg[:lo], padExt)
		if lo < hi {
			for i, x := range data[p+lo-start : p+hi-start] {
				seg[lo+i] = field.Lift(x)
			}
		}
		fill(seg[hi:], padExt)
	}
}

// overlap returns the rows [lo, hi) of a segment of length rows starting at
// column row p that hold data, given rel = start − p: segment row i holds
// data[i−rel] when lo <= i < hi and the padding value otherwise. lo == hi when
// the segment is all padding.
func overlap(rel, dataLen, length int) (lo, hi int) {
	lo = min(max(rel, 0), length)
	hi = max(lo, min(rel+dataLen, length))
	return lo, hi
}

func fill[T any](v []T, x T) {
	for i := range v {
		v[i] = x
	}
}
