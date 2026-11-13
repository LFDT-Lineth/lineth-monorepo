package field

import "github.com/consensys/gnark-crypto/field/koalabear/extensions"

// narrowInvLanes is the row width of the vectorised extension product, and
// the narrowest row of [BatchInvertExtScratch].
const narrowInvLanes = 16

// maxInvLanes is the widest row of [BatchInvertExtScratch].
const maxInvLanes = 128

// invLanesFor returns the number of interleaved chains [BatchInvertExtScratch]
// runs for n elements. A wide row amortises the per-call cost of the
// vectorised product and keeps more independent products in flight, while
// every chain adds to the inversion of the chain products; the thresholds are
// measured (BenchmarkBatchInvertExtLanes): 16 lanes lead up to 128 elements,
// 64 lanes from 256, 128 lanes from 2048.
func invLanesFor(n int) int {
	switch {
	case n >= 2048:
		return 128
	case n >= 256:
		return 64
	default:
		return narrowInvLanes
	}
}

// BatchInvertExtScratch sets res[i] = 1/a[i] for every i, and res[i] = 0
// where a[i] is 0, with a single extension inversion. res may alias a;
// scratch must hold at least len(a) elements, is overwritten, and may alias
// neither res nor a. It allocates nothing.
//
// It runs Montgomery's trick as interleaved chains: the elements are laid out
// in rows, a chain per column, so that the running products of a row are one
// vectorised extension product. Rows are invLanesFor(len(a)) wide, then
// [narrowInvLanes] wide for the remainder, and the last few elements form one
// more chain multiplied scalar by scalar. A zero element enters its chain as
// one and its prefix as zero, so its inverse comes out as zero.
func BatchInvertExtScratch(res, a, scratch []Ext) {
	batchInvertExtLanes(res, a, scratch, invLanesFor(len(a)))
}

// batchInvertExtLanes is [BatchInvertExtScratch] with rows of the given width,
// a multiple of narrowInvLanes no larger than maxInvLanes.
func batchInvertExtLanes(res, a, scratch []Ext, lanes int) {
	n := len(a)
	if len(res) != n {
		panic("field: BatchInvertExtScratch: res and a must have the same length")
	}
	if len(scratch) < n {
		panic("field: BatchInvertExtScratch: scratch is shorter than a")
	}
	if n == 0 {
		return
	}
	prefix := scratch[:n]

	// Row boundaries: [0, wide) in rows of lanes, [wide, narrow) in rows of
	// narrowInvLanes, [narrow, n) one scalar chain.
	wide := n / lanes * lanes
	narrow := wide + (n-wide)/narrowInvLanes*narrowInvLanes

	// acc[l] is the running product of chain l; acc[lanes] is the scalar
	// chain's.
	var accBuf [maxInvLanes + 1]Ext
	acc := accBuf[:lanes+1]
	for l := range acc {
		acc[l].SetOne()
	}
	tail := &acc[lanes]

	// Forward pass: res takes a with its zeros replaced by one, prefix[i] the
	// product of the earlier elements of i's chain (zero for a zero element).
	load := func(j, w int) {
		copy(prefix[j:j+w], acc[:w])
		for i := j; i < j+w; i++ {
			if a[i].IsZero() {
				res[i].SetOne()
				prefix[i] = Ext{}
			} else {
				res[i] = a[i]
			}
		}
		extensions.VectorE6(acc[:w]).Mul(acc[:w], res[j:j+w])
	}
	for j := 0; j < wide; j += lanes {
		load(j, lanes)
	}
	for j := wide; j < narrow; j += narrowInvLanes {
		load(j, narrowInvLanes)
	}
	for i := narrow; i < n; i++ {
		if a[i].IsZero() {
			res[i].SetOne()
			prefix[i] = Ext{}
		} else {
			res[i] = a[i]
			prefix[i] = *tail
			tail.Mul(tail, &res[i])
		}
	}

	invertChainProducts(acc)

	// Backward pass: the inverse of element i is its chain's running inverse
	// times prefix[i]; res[i] is read before it is overwritten.
	for i := n - 1; i >= narrow; i-- {
		x := res[i]
		res[i].Mul(tail, &prefix[i])
		tail.Mul(tail, &x)
	}
	var row [maxInvLanes]Ext
	store := func(j, w int) {
		copy(row[:w], res[j:j+w])
		extensions.VectorE6(res[j:j+w]).Mul(acc[:w], prefix[j:j+w])
		extensions.VectorE6(acc[:w]).Mul(acc[:w], row[:w])
	}
	for j := narrow - narrowInvLanes; j >= wide; j -= narrowInvLanes {
		store(j, narrowInvLanes)
	}
	for j := wide - lanes; j >= 0; j -= lanes {
		store(j, lanes)
	}
}

// invertChainProducts inverts the nonzero chain products p in place: a few
// with a scalar Montgomery trick and one extension inversion, more with a
// narrow interleaved pass whose own chain products are few.
func invertChainProducts(p []Ext) {
	if len(p) > narrowInvLanes+1 {
		var scratch [maxInvLanes + 1]Ext
		batchInvertExtLanes(p, p, scratch[:len(p)], narrowInvLanes)
		return
	}
	var prefix [narrowInvLanes + 1]Ext
	var run Ext
	run.SetOne()
	for l := range p {
		prefix[l] = run
		run.Mul(&run, &p[l])
	}
	run.Inverse(&run)
	for l := len(p) - 1; l >= 0; l-- {
		var inv Ext
		inv.Mul(&run, &prefix[l])
		run.Mul(&run, &p[l])
		p[l] = inv
	}
}
