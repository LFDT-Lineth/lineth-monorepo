package lookuptologderivsum

import (
	"fmt"
	"runtime"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// mAssignmentTask is the prover-side task that fills the multiplicity columns
// M for one [lookupGroup]. It is registered in the group's witness round and
// runs after the witness columns of every B fragment and every A fragment in
// the group have been committed.
//
// The lookup table is the union of the group's B fragments; the task emits one
// M column per fragment (t.ms is index-aligned with t.includings). It hashes
// each B row and each active A row into a single extension-field value using an
// internal random scalar (independent of the symbolic α used by the
// LogDerivativeSum reduction), then for every active A row increments the M
// entry of the *matching* (fragment, row) of the union. If the same value
// appears in several B rows — within one fragment or across fragments — the
// count is charged to the latest occurrence (highest fragment index, then
// highest row); this mirrors linea/logderivativesum's "preserve the latest
// occurrence" convention and keeps the honest-prover identity exact (each
// looked-up value cancels against exactly one B term). Filtered-out B rows
// (selector = 0) keep M = 0 by construction: their prepended head differs from
// the constant-1 head of every A row, so no A row ever matches them.
//
// The match is found with a fragment-tagged radix-partitioned hash join,
// ported from linea/logderivativesum's M-assignment: both sides are hashed
// into a power-of-two number of buckets (sized to the CPU count) on the low
// bits of the row hash, and each bucket is joined independently in parallel.
// Because every (fragment, row) of the union holds a single value, it lands in
// exactly one bucket, so the per-bucket writes into the M vectors are disjoint
// and need no synchronisation.
//
// Hash collisions in the internal hash function would only mis-direct
// multiplicity counts within the prover; they cannot break soundness because
// the verifier's check is the symbolic LogDerivativeSum identity, which is
// secured by the externally-sampled γ and α coins.
type mAssignmentTask struct {
	// ms holds one multiplicity column per B fragment, index-aligned with
	// includings.
	ms []*wiop.Column
	// includings holds the B-side fragments forming the union lookup table.
	includings []includingTable
	included   []includedSpec
	// prependOneOnAOk records whether compileGroup prepended a constant 1 to
	// every A side (and a per-fragment head to the B side) as part of the
	// IsFilteredOnIncluding trick. The hashing routine below incorporates the
	// same prepend so A/B hashes match when and only when their effective row
	// values match.
	prependOneOnAOk bool
}

// tEntry is one B-side (lookup-table) row: its collapsed hash value together
// with the (fragment, row) it lives at in the union.
type tEntry struct {
	val  field.Ext
	frag uint32
	row  uint32
}

// sEntry is one active A-side (checked) row: its collapsed hash value and the
// (fragment, row) it came from, kept for diagnostics on a failed match.
type sEntry struct {
	val  field.Ext
	frag uint32
	row  uint32
}

// Run implements [wiop.ProverAction].
func (t *mAssignmentTask) Run(rt *wiop.Runtime) {
	// Hashing scalar — fresh per run, independent of the symbolic α used in
	// the constraint system. Collisions are tolerable: they would yield a
	// proof the verifier rejects, never a false acceptance.
	alpha := field.RandomElementExt()

	// --- Build the B side. For each fragment, evaluate its rows, allocate its
	// M vector, and produce one tEntry per row tagged with (fragment, row).
	mValues := make([][]field.Element, len(t.includings))
	var tEntries []tEntry
	for frag, it := range t.includings {
		n := it.module.RuntimeSize(rt)
		mValues[frag] = make([]field.Element, n)

		bColExt := wiop.EvaluateVectorsAsExt(rt, it.cols, n)
		var bSelectorExt []field.Ext
		if it.selector != nil {
			bSelectorExt = it.selector.EvaluateVectorAsExt(rt, n)
		}

		for i := 0; i < n; i++ {
			head := t.bHead(it, bSelectorExt, i)
			h := rowHash(alpha, head, t.prependOneOnAOk, bColExt, i)
			tEntries = append(tEntries, tEntry{val: h, frag: uint32(frag), row: uint32(i)})
		}
	}

	// --- Build the A side. Only active rows contribute; the A-side head is the
	// constant 1 whenever the prepend trick is in effect.
	var sEntries []sEntry
	var aHead field.Ext
	if t.prependOneOnAOk {
		aHead = field.OneExt()
	}
	for aFrag, inc := range t.included {
		an := inc.cols[0].Module().RuntimeSize(rt)
		aColExt := wiop.EvaluateVectorsAsExt(rt, inc.cols, an)
		var aSelectorExt []field.Ext
		if inc.selector != nil {
			aSelectorExt = inc.selector.EvaluateVectorAsExt(rt, an)
		}

		for j := 0; j < an; j++ {
			if inc.selector != nil {
				if aSelectorExt[j].IsZero() {
					continue
				}
				// The included-side filter is treated as a 0/1 selector by the
				// LogDerivativeSum reduction: M is incremented by one per
				// active row, so any other value would silently break the
				// honest-prover identity. Abort early with a clear error
				// instead of letting the verifier reject a malformed proof.
				if !aSelectorExt[j].IsOne() {
					panic(fmt.Sprintf(
						"wiop/compilers/lookuptologderivsum: included filter %q has a non-binary value at row %d: %v",
						inc.selector.Column.Context.Path(), j, aSelectorExt[j].String(),
					))
				}
			}
			h := rowHash(alpha, aHead, t.prependOneOnAOk, aColExt, j)
			sEntries = append(sEntries, sEntry{val: h, frag: uint32(aFrag), row: uint32(j)})
		}
	}

	// --- Join both sides and fill the M vectors.
	t.hashJoin(tEntries, sEntries, mValues)

	for frag := range t.ms {
		rt.AssignColumn(t.ms[frag], &wiop.ConcreteVector{Plain: field.VecFromBase(mValues[frag])})
	}
}

// bHead returns the row's prepended head on the B side: the zero element when
// no prepend is in effect, the fragment's selector value when it carries one,
// and the constant 1 for an unfiltered fragment inside a filtered group.
func (t *mAssignmentTask) bHead(it includingTable, bSelectorExt []field.Ext, i int) field.Ext {
	if !t.prependOneOnAOk {
		return field.Ext{}
	}
	if it.selector != nil {
		return bSelectorExt[i]
	}
	return field.OneExt()
}

// hashJoin performs the fragment-tagged radix-partitioned hash join. It
// partitions T and S into buckets on the low bits of [extHash], then for each
// bucket builds a value→(fragment,row) map from T (latest occurrence wins) and
// increments mValues[frag][row] once per matching S row. Panics if an S row has
// no match in T (an unsound lookup the honest prover must never produce).
func (t *mAssignmentTask) hashJoin(tEntries []tEntry, sEntries []sEntry, mValues [][]field.Element) {
	numBuckets := 1
	for numBuckets < runtime.NumCPU()*4 {
		numBuckets *= 2
	}
	// Power-of-two bucket count so we can mask instead of modulo on the hot path.
	mask := uint32(numBuckets - 1)

	// Partition both sides by bucket. A single pass suffices; buckets are
	// append-only slices built serially, then joined in parallel.
	tBuckets := make([][]tEntry, numBuckets)
	for _, e := range tEntries {
		b := extHash(&e.val) & mask
		tBuckets[b] = append(tBuckets[b], e)
	}
	sBuckets := make([][]sEntry, numBuckets)
	for _, e := range sEntries {
		b := extHash(&e.val) & mask
		sBuckets[b] = append(sBuckets[b], e)
	}

	parallel.Execute(numBuckets, func(start, stop int) {
		for b := start; b < stop; b++ {
			mapM := make(map[field.Ext][2]uint32, len(tBuckets[b]))
			for _, e := range tBuckets[b] {
				existing, ok := mapM[e.val]
				if !ok {
					mapM[e.val] = [2]uint32{e.frag, e.row}
					continue
				}
				// Preserve the latest occurrence (highest fragment, then
				// highest row) so the multiplicity is charged to exactly one
				// slot of the union.
				if e.frag > existing[0] || (e.frag == existing[0] && e.row >= existing[1]) {
					mapM[e.val] = [2]uint32{e.frag, e.row}
				}
			}

			for _, e := range sBuckets[b] {
				pos, ok := mapM[e.val]
				if !ok {
					panic(fmt.Sprintf(
						"wiop/compilers/lookuptologderivsum: A row %d (fragment %s) has no match "+
							"in the lookup table",
						e.row, t.included[e.frag].cols[0].Column.Context.Path(),
					))
				}
				mFrag, mRow := pos[0], pos[1]
				mValues[mFrag][mRow].Add(&mValues[mFrag][mRow], &fieldOne)
			}
		}
	})
}

// extHash folds the six base-field coordinates of an extension element into a
// 32-bit hash with a ×31 multiply/XOR mix. It is used only to bucket rows for
// the hash join; the low bits select the bucket and exact equality resolves
// matches within a bucket, so a weak hash costs at most performance.
func extHash(v *field.Ext) uint32 {
	h := v.B0.A0.Uint64()
	h = (h * 31) ^ v.B0.A1.Uint64()
	h = (h * 31) ^ v.B1.A0.Uint64()
	h = (h * 31) ^ v.B1.A1.Uint64()
	h = (h * 31) ^ v.B2.A0.Uint64()
	h = (h * 31) ^ v.B2.A1.Uint64()
	return uint32(h)
}

// fieldOne is the multiplicative identity in the base field, kept as a
// package-level variable to avoid recomputing it per row.
var fieldOne = field.One()

// rowHash computes the extension-field hash
//
//	head + α·cols[0][i] + α²·cols[1][i] + … + α^k·cols[k-1][i]
//
// when usePrepend is true, and
//
//	cols[0][i] + α·cols[1][i] + α²·cols[2][i] + …
//
// when it is false. Both forms match [rlcExpression]'s convention so the
// prover-side hash agrees with the symbolic RLC under the same scalar.
func rowHash(alpha field.Ext, head field.Ext, usePrepend bool, cols [][]field.Ext, i int) field.Ext {
	if !usePrepend {
		// acc = cols[0][i] + α·cols[1][i] + α²·cols[2][i] + …
		acc := cols[0][i]
		pow := alpha
		for k := 1; k < len(cols); k++ {
			var term field.Ext
			term.Mul(&pow, &cols[k][i])
			acc.Add(&acc, &term)
			if k+1 < len(cols) {
				pow.Mul(&pow, &alpha)
			}
		}
		return acc
	}
	// acc = head + α·cols[0][i] + α²·cols[1][i] + …
	acc := head
	pow := alpha
	for k := 0; k < len(cols); k++ {
		var term field.Ext
		term.Mul(&pow, &cols[k][i])
		acc.Add(&acc, &term)
		if k+1 < len(cols) {
			pow.Mul(&pow, &alpha)
		}
	}
	return acc
}
