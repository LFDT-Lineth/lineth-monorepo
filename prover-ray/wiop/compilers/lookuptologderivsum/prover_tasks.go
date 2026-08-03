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

	// --- Build the B side. For each fragment, hash its rows, allocate its
	// M vector, and produce one tEntry per row tagged with (fragment, row).
	mValues := make([][]field.Element, len(t.includings))
	tTotal := 0
	for _, it := range t.includings {
		tTotal += it.module.RuntimeSize(rt)
	}
	tEntries := make([]tEntry, 0, tTotal)
	for frag, it := range t.includings {
		n := it.module.RuntimeSize(rt)
		mValues[frag] = make([]field.Element, n)

		// hashes[i] = cols[0][i] + α·cols[1][i] + …, then, in prepend mode,
		// hashes[i] = head[i] + α·hashes[i] where head is the fragment's
		// selector column (or the constant 1 for an unfiltered fragment).
		// This matches [rlcExpression]'s convention so the prover-side hash
		// agrees with the symbolic RLC under the same scalar.
		hashes := wiop.EvaluateRLCAsExt(rt, alpha, it.cols, n)
		if t.prependOneOnAOk {
			field.VecScaleExtExt(hashes, alpha, hashes)
			if it.selector != nil {
				addColumnInPlace(rt, hashes, it.selector)
			} else {
				addOneInPlace(hashes)
			}
		}

		for i := 0; i < n; i++ {
			tEntries = append(tEntries, tEntry{val: hashes[i], frag: uint32(frag), row: uint32(i)})
		}
	}

	// --- Build the A side. Only active rows contribute; the A-side head is the
	// constant 1 whenever the prepend trick is in effect. The capacity is an
	// upper bound: filtered-out rows are skipped.
	sTotal := 0
	for _, inc := range t.included {
		sTotal += inc.cols[0].Module().RuntimeSize(rt)
	}
	sEntries := make([]sEntry, 0, sTotal)
	for aFrag, inc := range t.included {
		an := inc.cols[0].Module().RuntimeSize(rt)
		hashes := wiop.EvaluateRLCAsExt(rt, alpha, inc.cols, an)
		if t.prependOneOnAOk {
			field.VecScaleExtExt(hashes, alpha, hashes)
			addOneInPlace(hashes)
		}
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
			sEntries = append(sEntries, sEntry{val: hashes[j], frag: uint32(aFrag), row: uint32(j)})
		}
	}

	// --- Join both sides and fill the M vectors.
	t.hashJoin(tEntries, sEntries, mValues)

	for frag := range t.ms {
		rt.AssignColumn(t.ms[frag], &wiop.ConcreteVector{Plain: field.VecFromBase(mValues[frag])})
	}
}

// addColumnInPlace adds a column's values into hashes element-wise, consuming
// the column un-lifted: a base-field column costs one base addition per row.
func addColumnInPlace(rt *wiop.Runtime, hashes []field.Ext, cv *wiop.ColumnView) {
	plain := cv.EvaluateVector(rt).Plain
	if plain.IsBase() {
		field.VecAddExtBase(hashes, hashes, plain.AsBase())
	} else {
		field.VecAddExtExt(hashes, hashes, plain.AsExt())
	}
}

// addOneInPlace adds the constant 1 to every entry of hashes, touching only
// the first base-field coordinate.
func addOneInPlace(hashes []field.Ext) {
	for i := range hashes {
		hashes[i].B0.A0.Add(&hashes[i].B0.A0, &fieldOne)
	}
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
