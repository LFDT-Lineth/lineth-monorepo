package field

import "github.com/consensys/gnark-crypto/field/koalabear"

// BatchInvertScratch sets res[i] = 1/a[i] for every i, and res[i] = 0 where
// a[i] is 0, with a single field inversion. res may alias a; scratch must
// hold at least len(a) elements, is overwritten, and may alias neither res
// nor a. It allocates nothing.
//
// It is [BatchInvertExtScratch] over the base field: interleaved Montgomery
// chains whose row products are one vectorised koalabear.Vector.Mul.
func BatchInvertScratch(res, a, scratch []Element) {
	batchInvertBaseLanes(res, a, scratch, baseInvLanesFor(len(a)))
}

// baseInvLanesFor returns the number of interleaved chains
// [BatchInvertScratch] runs for n elements, 0 meaning one scalar chain. A
// base product is cheap enough that the per-call cost of the vectorised
// product only pays off on long inputs: one scalar chain matches
// gnark-crypto's BatchInvert below baseInvWideFrom elements, and 128 lanes
// lead from there (BenchmarkBatchInvertBaseLanes; 64 lanes never lead).
func baseInvLanesFor(n int) int {
	if n >= baseInvWideFrom {
		return maxInvLanes
	}
	return 0
}

// baseInvWideFrom is the length from which [BatchInvertScratch] runs
// maxInvLanes interleaved chains instead of one scalar chain.
const baseInvWideFrom = 768

// batchInvBaseStackLen is the length up to which [BatchInvertInto] keeps its
// scratch on the stack rather than in [BasePool].
const batchInvBaseStackLen = 1024

// BatchInvertInto sets res[i] = 1/a[i] for every i; zeroes map to zero. res
// may alias a. Its scratch comes from the stack or [BasePool]; see
// [BatchInvertScratch] to pass it.
func BatchInvertInto(res, a []Element) {
	if len(a) != len(res) {
		panic("field: BatchInvertInto: res and a must have the same length")
	}
	if len(a) <= batchInvBaseStackLen {
		var scratch [batchInvBaseStackLen]Element
		BatchInvertScratch(res, a, scratch[:len(a)])
		return
	}
	scratch := BasePool.Get(len(a))
	BatchInvertScratch(res, a, scratch)
	BasePool.Put(scratch)
}

// batchInvertBaseLanes is [BatchInvertScratch] with rows of the given width,
// a multiple of narrowInvLanes no larger than maxInvLanes, or 0 to run every
// element through the scalar chain.
func batchInvertBaseLanes(res, a, scratch []Element, lanes int) {
	n := len(a)
	if len(res) != n {
		panic("field: BatchInvertScratch: res and a must have the same length")
	}
	if len(scratch) < n {
		panic("field: BatchInvertScratch: scratch is shorter than a")
	}
	if n == 0 {
		return
	}
	prefix := scratch[:n]

	// Row boundaries: [0, wide) in rows of lanes, [wide, narrow) in rows of
	// narrowInvLanes, [narrow, n) one scalar chain.
	wide, narrow := 0, 0
	if lanes > 0 {
		wide = n / lanes * lanes
		narrow = wide + (n-wide)/narrowInvLanes*narrowInvLanes
	}

	// acc[l] is the running product of chain l; acc[lanes] is the scalar
	// chain's.
	var accBuf [maxInvLanes + 1]Element
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
				prefix[i] = Element{}
			} else {
				res[i] = a[i]
			}
		}
		row := koalabear.Vector(acc[:w])
		row.Mul(row, res[j:j+w])
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
			prefix[i] = Element{}
		} else {
			res[i] = a[i]
			prefix[i] = *tail
			tail.Mul(tail, &res[i])
		}
	}

	invertBaseChainProducts(acc)

	// Backward pass: the inverse of element i is its chain's running inverse
	// times prefix[i]; res[i] is read before it is overwritten.
	for i := n - 1; i >= narrow; i-- {
		x := res[i]
		res[i].Mul(tail, &prefix[i])
		tail.Mul(tail, &x)
	}
	var rowBuf [maxInvLanes]Element
	store := func(j, w int) {
		copy(rowBuf[:w], res[j:j+w])
		out := koalabear.Vector(res[j : j+w])
		out.Mul(acc[:w], prefix[j:j+w])
		row := koalabear.Vector(acc[:w])
		row.Mul(row, rowBuf[:w])
	}
	for j := narrow - narrowInvLanes; j >= wide; j -= narrowInvLanes {
		store(j, narrowInvLanes)
	}
	if lanes > 0 {
		for j := wide - lanes; j >= 0; j -= lanes {
			store(j, lanes)
		}
	}
}

// invertBaseChainProducts inverts the nonzero chain products p in place: a
// few with a scalar Montgomery trick and one field inversion, more with a
// narrow interleaved pass whose own chain products are few.
func invertBaseChainProducts(p []Element) {
	if len(p) > narrowInvLanes+1 {
		var scratch [maxInvLanes + 1]Element
		batchInvertBaseLanes(p, p, scratch[:len(p)], narrowInvLanes)
		return
	}
	var prefix [narrowInvLanes + 1]Element
	var run Element
	run.SetOne()
	for l := range p {
		prefix[l] = run
		run.Mul(&run, &p[l])
	}
	run.Inverse(&run)
	for l := len(p) - 1; l >= 0; l-- {
		var inv Element
		inv.Mul(&run, &prefix[l])
		run.Mul(&run, &p[l])
		p[l] = inv
	}
}
