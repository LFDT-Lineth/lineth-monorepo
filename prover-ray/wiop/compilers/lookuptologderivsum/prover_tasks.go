package lookuptologderivsum

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
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
// The match is found with a radix-partitioned hash join, ported from
// linea/logderivativesum's M-assignment: the B side is loaded into one map per
// bucket (a power of two sized to the data) on the low bits of the row hash,
// built in parallel, and every A row probes the map of its bucket.
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
// with its index in the union of the group's fragments (fragment-major, so a
// later fragment, then a later row, has a larger index).
type tEntry struct {
	val field.Ext
	idx uint32
}

// Run implements [wiop.ProverAction].
func (t *mAssignmentTask) Run(rt *wiop.Runtime) {
	t.run(rt, runtime.GOMAXPROCS(0))
}

// run fills the M columns using at most workers goroutines for its inner
// parallel loops; workers == 1 runs the task entirely inline.
//
// The B side, the lookup table, is hashed into entries and loaded into one
// map per hash bucket. The A side, usually much larger, is streamed: each
// chunk of rows reads its columns straight from their assignments, hashes
// them, and probes the maps, so nothing proportional to the A side is
// materialised.
func (t *mAssignmentTask) run(rt *wiop.Runtime, workers int) {
	// Hashing scalar — fresh per run, independent of the symbolic α used in
	// the constraint system. Collisions are tolerable: they would yield a
	// proof the verifier rejects, never a false acceptance.
	alpha := field.RandomElementExt()

	// --- Build the B side. Every B row becomes an entry, tagged with its
	// union index; each fragment's entries sit at a fixed offset.
	tOffsets := make([]int, len(t.includings)+1)
	for frag, it := range t.includings {
		tOffsets[frag+1] = tOffsets[frag] + it.module.RuntimeSize(rt)
	}
	tEntries := make([]tEntry, tOffsets[len(t.includings)])
	for frag, it := range t.includings {
		n := it.module.RuntimeSize(rt)
		head := headNone
		if t.prependOneOnAOk {
			head = headOne
			if it.selector != nil {
				head = headSelector
			}
		}
		h := newRowHasher(rt, alpha, it.cols, it.selector, head, n)
		dst := tEntries[tOffsets[frag] : tOffsets[frag]+n]
		chunks := splitRows(n, workers)
		parallel.Execute(len(chunks), func(start, stop int) {
			buf := h.newBuffers()
			for c := start; c < stop; c++ {
				for lo := chunks[c][0]; lo < chunks[c][1]; lo += hashBlock {
					hi := min(lo+hashBlock, chunks[c][1])
					vals := h.hash(lo, hi, buf)
					for i, v := range vals {
						dst[lo+i] = tEntry{val: v, idx: uint32(tOffsets[frag] + lo + i)}
					}
				}
			}
		}, workers)
	}
	table := buildTable(tEntries, workers)

	// --- Stream the A side. Only active rows are looked up; the A-side head
	// is the constant 1 whenever the prepend trick is in effect. The chunks
	// of every A fragment form one list of work items, so each worker sets up
	// and flushes its counters once, however many fragments the group has.
	type aSide struct {
		h   *rowHasher
		sel *columnReader
	}
	type aChunk struct{ frag, lo, hi int }
	sides := make([]aSide, len(t.included))
	var items []aChunk
	for aFrag, inc := range t.included {
		an := inc.cols[0].Module().RuntimeSize(rt)
		head := headNone
		if t.prependOneOnAOk {
			head = headOne
		}
		sides[aFrag].h = newRowHasher(rt, alpha, inc.cols, nil, head, an)
		if inc.selector != nil {
			sides[aFrag].sel = newColumnReader(rt, inc.selector, an)
		}
		for _, c := range splitRows(an, workers) {
			items = append(items, aChunk{aFrag, c[0], c[1]})
		}
	}
	counts := newCounts(len(tEntries), workers)
	parallel.Execute(len(items), func(start, stop int) {
		var (
			bufs   = make([]*hashBuffers, len(sides))
			selBuf = make([]field.Ext, hashBlock)
			local  = counts.local()
		)
		for _, it := range items[start:stop] {
			side, inc := &sides[it.frag], t.included[it.frag]
			if bufs[it.frag] == nil {
				bufs[it.frag] = side.h.newBuffers()
			}
			for lo := it.lo; lo < it.hi; lo += hashBlock {
				hi := min(lo+hashBlock, it.hi)
				vals := side.h.hash(lo, hi, bufs[it.frag])
				var selVals []field.Ext
				if side.sel != nil {
					selVals = side.sel.readExt(lo, hi, selBuf)
				}
				for i := range vals {
					if selVals != nil && !t.activeRow(inc, &selVals[i], lo+i) {
						continue
					}
					idx, ok := table.lookup(&vals[i])
					if !ok {
						panic(fmt.Sprintf(
							"wiop/compilers/lookuptologderivsum: A row %d (fragment %s) has no match "+
								"in the lookup table",
							lo+i, inc.cols[0].Column.Context.Path(),
						))
					}
					local.add(idx)
				}
			}
		}
		local.flush()
	}, workers)

	// --- Fill the M vectors from the counts.
	for frag := range t.ms {
		m := make([]field.Element, tOffsets[frag+1]-tOffsets[frag])
		counts.read(tOffsets[frag], m)
		rt.AssignColumn(t.ms[frag], &wiop.ConcreteVector{Plain: field.VecFromBase(m)})
	}
}

// activeRow reports whether an A row with selector value s is looked up.
//
// The included-side filter is treated as a 0/1 selector by the
// LogDerivativeSum reduction: M is incremented by one per active row, so any
// other value would silently break the honest-prover identity. Abort early
// with a clear error instead of letting the verifier reject a malformed proof.
func (t *mAssignmentTask) activeRow(inc includedSpec, s *field.Ext, row int) bool {
	if s.IsZero() {
		return false
	}
	if !s.IsOne() {
		panic(fmt.Sprintf(
			"wiop/compilers/lookuptologderivsum: included filter %q has a non-binary value at row %d: %v",
			inc.selector.Column.Context.Path(), row, s.String(),
		))
	}
	return true
}

// hashBlock is the number of rows hashed at a time: the column chunks of a
// block stay in cache while they are combined.
const hashBlock = 1024

// headKind is what a fragment's hash prepends to its columns.
type headKind uint8

const (
	headNone     headKind = iota // hash = Σ_k α^k·cols[k]
	headOne                      // hash = 1 + α·Σ_k α^k·cols[k]
	headSelector                 // hash = selector + α·Σ_k α^k·cols[k]
)

// rowHasher hashes the rows of a fragment into one extension element each:
// hashes[i] = cols[0][i] + α·cols[1][i] + …, then, with a head,
// head[i] + α·hashes[i], matching [wiop.RLCExpression]'s convention. The
// powers of α are precomputed, so each column costs an extension-by-base
// multiply-accumulate per row instead of an extension multiplication.
type rowHasher struct {
	cols     []*columnReader
	coefs    []field.Ext // coefs[k] multiplies cols[k]
	head     headKind
	selector *columnReader // headSelector only
}

func newRowHasher(rt *wiop.Runtime, alpha field.Ext, cols []*wiop.ColumnView, selector *wiop.ColumnView,
	head headKind, n int) *rowHasher {
	h := &rowHasher{head: head, coefs: make([]field.Ext, len(cols))}
	var pow field.Ext
	pow.SetOne()
	if head != headNone {
		pow = alpha
	}
	for k, cv := range cols {
		h.cols = append(h.cols, newColumnReader(rt, cv, n))
		h.coefs[k] = pow
		pow.Mul(&pow, &alpha)
	}
	if head == headSelector {
		h.selector = newColumnReader(rt, selector, n)
	}
	return h
}

// hashBuffers is one worker's scratch for [rowHasher.hash].
type hashBuffers struct {
	out  []field.Ext
	base []field.Element
	ext  []field.Ext
}

func (h *rowHasher) newBuffers() *hashBuffers {
	return &hashBuffers{
		out:  make([]field.Ext, hashBlock),
		base: make([]field.Element, hashBlock),
		ext:  make([]field.Ext, hashBlock),
	}
}

// hash returns the hashes of rows [lo, hi), in buf.
func (h *rowHasher) hash(lo, hi int, buf *hashBuffers) []field.Ext {
	out := buf.out[:hi-lo]
	switch h.head {
	case headOne:
		for i := range out {
			out[i].SetOne()
		}
	case headSelector:
		copy(out, h.selector.readExt(lo, hi, buf.ext))
	default:
		clear(out)
	}
	acc := extensions.VectorE6(out)
	for k, col := range h.cols {
		if col.isBase() {
			acc.ScalarMulAccByElement(col.readBase(lo, hi, buf.base), &h.coefs[k])
		} else {
			acc.ScalarMulAcc(col.readExt(lo, hi, buf.ext), &h.coefs[k])
		}
	}
	return out
}

// columnReader reads rows of a column view straight from the column's
// assignment, as [wiop.ColumnView.EvaluateVector] would return them: row i is
// the module's padded row (i + shift) mod n.
type columnReader struct {
	plain     field.Vec
	pad       field.Element
	n         int
	shift     int
	dataStart int // padded row of plain[0]
}

func newColumnReader(rt *wiop.Runtime, cv *wiop.ColumnView, n int) *columnReader {
	assignment := rt.GetColumnAssignment(cv.Column)
	r := &columnReader{
		plain: assignment.Plain,
		pad:   assignment.Padding,
		n:     n,
		shift: ((cv.ShiftingOffset % n) + n) % n,
	}
	if cv.Column.Module.Padding == wiop.PaddingDirectionLeft {
		r.dataStart = n - r.plain.Len()
	}
	return r
}

func (r *columnReader) isBase() bool { return r.plain.IsBase() }

// segments calls f for the contiguous padded-row runs [p, p+len) making up
// rows [lo, hi), with the offset of the run in the output.
func (r *columnReader) segments(lo, hi int, f func(out, p, length int)) {
	out, p := 0, (lo+r.shift)%r.n
	for out < hi-lo {
		length := min(hi-lo-out, r.n-p)
		f(out, p, length)
		out += length
		p = 0
	}
}

// readBase returns rows [lo, hi) of a base-field column, in buf.
func (r *columnReader) readBase(lo, hi int, buf []field.Element) []field.Element {
	plain := r.plain.AsBase()
	dst := buf[:hi-lo]
	r.segments(lo, hi, func(out, p, length int) {
		// Padded rows [p, p+length): data where they overlap plain, the
		// padding value elsewhere.
		dLo, dHi := max(p, r.dataStart), min(p+length, r.dataStart+len(plain))
		if dLo >= dHi {
			field.VecFillBase(dst[out:out+length], r.pad)
			return
		}
		field.VecFillBase(dst[out:out+dLo-p], r.pad)
		copy(dst[out+dLo-p:out+dHi-p], plain[dLo-r.dataStart:dHi-r.dataStart])
		field.VecFillBase(dst[out+dHi-p:out+length], r.pad)
	})
	return dst
}

// readExt returns rows [lo, hi) of the column in the extension field, in buf.
func (r *columnReader) readExt(lo, hi int, buf []field.Ext) []field.Ext {
	dst := buf[:hi-lo]
	pad := field.Lift(r.pad)
	r.segments(lo, hi, func(out, p, length int) {
		dLo, dHi := max(p, r.dataStart), min(p+length, r.dataStart+r.plain.Len())
		if dLo >= dHi {
			field.VecFillExt(dst[out:out+length], pad)
			return
		}
		field.VecFillExt(dst[out:out+dLo-p], pad)
		field.VecFillExt(dst[out+dHi-p:out+length], pad)
		seg := dst[out+dLo-p : out+dHi-p]
		if r.plain.IsBase() {
			for i, v := range r.plain.AsBase()[dLo-r.dataStart : dHi-r.dataStart] {
				seg[i] = field.Lift(v)
			}
			return
		}
		copy(seg, r.plain.AsExt()[dLo-r.dataStart:dHi-r.dataStart])
	})
	return dst
}

// lookupTable maps the hash of every B row to its union index. It is split
// into one map per bucket of [extHash], built in parallel; when a value
// appears in several B rows, the largest union index (the latest occurrence:
// highest fragment, then highest row) wins, so the multiplicity is charged to
// exactly one slot of the union. It is read-only once built.
type lookupTable struct {
	mask uint32
	maps []map[field.Ext]uint32
}

func buildTable(tEntries []tEntry, workers int) *lookupTable {
	// Size the partition to the data: about joinEntriesPerBucket entries per
	// bucket, capped at four buckets per CPU. Small tables then build a single
	// map instead of spreading a few rows over a thousand maps and goroutines,
	// which matters when many groups run concurrently (see [mAssignmentBatch]).
	numBuckets := 1
	for numBuckets < runtime.NumCPU()*4 && numBuckets*joinEntriesPerBucket < len(tEntries) {
		numBuckets *= 2
	}
	// Power-of-two bucket count so we can mask instead of modulo on the hot path.
	tab := &lookupTable{mask: uint32(numBuckets - 1), maps: make([]map[field.Ext]uint32, numBuckets)}
	bucketOf := func(e *tEntry) uint32 { return extHash(&e.val) & tab.mask }
	buckets := partitionByBucket(tEntries, numBuckets, bucketOf, workers)
	parallel.Execute(numBuckets, func(start, stop int) {
		for b := start; b < stop; b++ {
			m := make(map[field.Ext]uint32, len(buckets[b]))
			for _, e := range buckets[b] {
				if existing, ok := m[e.val]; !ok || e.idx > existing {
					m[e.val] = e.idx
				}
			}
			tab.maps[b] = m
		}
	}, workers)
	return tab
}

func (tab *lookupTable) lookup(v *field.Ext) (uint32, bool) {
	idx, ok := tab.maps[extHash(v)&tab.mask][*v]
	return idx, ok
}

// counts accumulates the multiplicity of every union index. Workers count
// into private arrays merged at the end when the table is small enough for a
// copy per worker, and into shared atomic counters otherwise: a small table
// is typically hit by many rows per entry, which would contend on the shared
// counters.
type counts struct {
	shared  []uint32
	private bool
	mu      sync.Mutex
}

// privateCountBudget bounds the total size of the private count arrays.
const privateCountBudget = 1 << 24

func newCounts(size, workers int) *counts {
	return &counts{shared: make([]uint32, size), private: size*max(1, workers) <= privateCountBudget}
}

// localCounts is one worker's view of [counts].
type localCounts struct {
	c   *counts
	own []uint32 // nil when counting into the shared atomic counters
}

func (c *counts) local() *localCounts {
	l := &localCounts{c: c}
	if c.private {
		l.own = make([]uint32, len(c.shared))
	}
	return l
}

func (l *localCounts) add(idx uint32) {
	if l.own != nil {
		l.own[idx]++
		return
	}
	atomic.AddUint32(&l.c.shared[idx], 1)
}

func (l *localCounts) flush() {
	if l.own == nil {
		return
	}
	l.c.mu.Lock()
	defer l.c.mu.Unlock()
	for i, v := range l.own {
		l.c.shared[i] += v
	}
}

// read writes the counts of union indices [offset, offset+len(m)) into m.
func (c *counts) read(offset int, m []field.Element) {
	for i := range m {
		m[i].SetUint64(uint64(c.shared[offset+i]))
	}
}

// splitRows splits [0, n) into at most workers contiguous [start, end) chunks
// of at least rowsPerChunk rows.
func splitRows(n, workers int) [][2]int {
	size := max(rowsPerChunk, (n+workers-1)/max(1, workers))
	chunks := make([][2]int, 0, (n+size-1)/max(1, size))
	for start := 0; start < n; start += size {
		chunks = append(chunks, [2]int{start, min(start+size, n)})
	}
	return chunks
}

// rowsPerChunk is the smallest chunk a worker of the M-assignment handles:
// below it, splitting costs more than it saves.
const rowsPerChunk = 8192

// partitionByBucket groups entries by bucket(e) into one flat array and
// returns each bucket as a sub-slice of it. It is a counting sort: chunks of
// the input count their entries per bucket in parallel, the counts give every
// (chunk, bucket) pair its output offset, and the chunks then scatter in
// parallel. Within a bucket, entries keep their input order.
func partitionByBucket[E any](entries []E, numBuckets int, bucket func(*E) uint32, workers int) [][]E {
	chunks := splitRows(len(entries), workers)
	counts := make([][]int, len(chunks))
	parallel.Execute(len(chunks), func(start, stop int) {
		for c := start; c < stop; c++ {
			counts[c] = make([]int, numBuckets)
			for i := chunks[c][0]; i < chunks[c][1]; i++ {
				counts[c][bucket(&entries[i])]++
			}
		}
	}, workers)

	// Bucket b starts after every smaller bucket; within it, chunk c writes
	// after every earlier chunk.
	bucketStart := make([]int, numBuckets+1)
	for b := range numBuckets {
		total := 0
		for c := range chunks {
			total += counts[c][b]
		}
		bucketStart[b+1] = bucketStart[b] + total
	}
	for b := range numBuckets {
		next := bucketStart[b]
		for c := range chunks {
			n := counts[c][b]
			counts[c][b] = next
			next += n
		}
	}

	flat := make([]E, len(entries))
	parallel.Execute(len(chunks), func(start, stop int) {
		for c := start; c < stop; c++ {
			pos := counts[c]
			for i := chunks[c][0]; i < chunks[c][1]; i++ {
				b := bucket(&entries[i])
				flat[pos[b]] = entries[i]
				pos[b]++
			}
		}
	}, workers)

	buckets := make([][]E, numBuckets)
	for b := range numBuckets {
		buckets[b] = flat[bucketStart[b]:bucketStart[b+1]:bucketStart[b+1]]
	}
	return buckets
}

// joinEntriesPerBucket is the target number of T and S entries per hash-join
// bucket.
const joinEntriesPerBucket = 4096

// mAssignmentBatch runs the M-assignment tasks of every lookup group sharing a
// witness round. The tasks are independent -- each reads committed witness
// columns and writes only its own M columns -- so they run concurrently,
// largest first to balance the workers. It replaces one prover action per
// group, which ran the groups one after another.
//
// Each task's inner parallel loops get a share of the CPUs proportional to its
// row count (at least one), so the shares add up to about GOMAXPROCS. Letting
// every task fan out over all CPUs instead multiplies goroutines by the
// number of tasks, and the scheduler overhead outweighs the work of the small
// groups.
type mAssignmentBatch struct {
	tasks []*mAssignmentTask
}

// Run implements [wiop.ProverAction].
func (b *mAssignmentBatch) Run(rt *wiop.Runtime) {
	sizes := make([]int, len(b.tasks))
	order := make([]int, len(b.tasks))
	for i, t := range b.tasks {
		for _, it := range t.includings {
			sizes[i] += it.module.RuntimeSize(rt)
		}
		for _, inc := range t.included {
			sizes[i] += inc.cols[0].Module().RuntimeSize(rt)
		}
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return sizes[order[i]] > sizes[order[j]] })

	cpus := runtime.GOMAXPROCS(0)
	total := 0
	for _, sz := range sizes {
		total += sz
	}
	parallel.ExecuteDynamic(len(order), func(k int) {
		i := order[k]
		workers := 1
		if total > 0 {
			workers = max(1, cpus*sizes[i]/total)
		}
		b.tasks[i].run(rt, workers)
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
