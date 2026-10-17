package pcs

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Column elision.
//
// A committed interactive round carries, next to its Merkle root, a column
// manifest: one base-field cell per column of the round (in round.Columns
// order) stating how that column is represented in the commitment:
//
//   - ManifestPresent: committed and opened as usual.
//   - ManifestZero: omitted from the commitment. The column's padded vector is
//     identically zero, so every claimed evaluation of it is zero.
//   - ManifestAlias(k): omitted from the commitment. The column's padded vector
//     equals that of column k of the same round (k < i, k Present, same
//     base/extension kind, same padded size), so every claimed evaluation of
//     it equals the claim of column k at the same shift.
//
// The manifest cells are declared by [Compile] and assigned by the commit
// prover action, so they travel in the proof as ordinary cells and are absorbed
// by [wiop.Runtime.AdvanceRound] in the same round as the root they describe
// (cells follow the commitment in the transcript; no challenge is squeezed in
// between, so the binding is the same as absorbing them first). The FRI layer
// only ever sees the Present columns: elided columns are absent from the
// committed rows, consume no alpha_DEEP power and have no FRI claims. The
// verifier pins the elided columns' claim cells instead (see
// [RecoverBatchClaims]): a Zero column's claims must be zero, an aliased
// column's claims must equal the target's authenticated claims.
//
// Detection runs over the FULL padded vector (padding included), never over the
// opened rows, so a column is only dropped when the committed polynomial really
// is zero / a duplicate.
//
// Soundness rests on the binding stated above:
//
//   - the manifest cells are absorbed by [wiop.Runtime.AdvanceRound] in the
//     same round as the root they describe, before any challenge is drawn, so
//     they fix the oracle the prover commits to (Zero means the zero
//     polynomial, Alias(k) means a copy of column k);
//   - the verifier pins every elided claim to that oracle in
//     [RecoverBatchClaims].

// Manifest codes. Alias targets are encoded as manifestAliasBase + k.
const (
	ManifestPresent   uint32 = 0
	ManifestZero      uint32 = 1
	manifestAliasBase uint32 = 2
)

// ManifestAlias returns the manifest code for "alias of column k of the same
// round".
func ManifestAlias(k int) uint32 { return manifestAliasBase + uint32(k) }

// ManifestAliasTarget decodes an alias code into its target column index. ok is
// false for ManifestPresent and ManifestZero.
func ManifestAliasTarget(code uint32) (k int, ok bool) {
	if code < manifestAliasBase {
		return 0, false
	}
	return int(code - manifestAliasBase), true
}

// ColumnManifest is one round's manifest, indexed like round.Columns.
type ColumnManifest []uint32

// allPresentManifest is the manifest of a round committed without elision (the
// precomputed round, or any round when elision is disabled).
func allPresentManifest(n int) ColumnManifest { return make(ColumnManifest, n) }

// hasPresent reports whether at least one column is committed.
func (m ColumnManifest) hasPresent() bool {
	for _, code := range m {
		if code == ManifestPresent {
			return true
		}
	}
	return false
}

// paddedColumn is one column's committed vector: the assignment padded to its
// power-of-two size with the module's padding value, exactly as it is
// committed. Either base or ext is set.
type paddedColumn struct {
	sizeIndex int
	isExt     bool
	base      []field.Element
	ext       []field.Ext
}

func (p *paddedColumn) equal(q *paddedColumn) bool {
	if p.isExt != q.isExt || p.sizeIndex != q.sizeIndex {
		return false
	}
	if p.isExt {
		for i := range p.ext {
			if !p.ext[i].Equal(&q.ext[i]) {
				return false
			}
		}
		return true
	}
	for i := range p.base {
		if !p.base[i].Equal(&q.base[i]) {
			return false
		}
	}
	return true
}

// fnvOffset and fnvPrime are the FNV-1 64-bit parameters of the column
// fingerprints.
const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

// scanChunk scans elements [lo, hi) of p: whether they are all zero, and
// their FNV fingerprint.
func (p *paddedColumn) scanChunk(lo, hi int) (zero bool, h uint64) {
	h, zero = fnvOffset, true
	mix := func(e *field.Element) {
		zero = zero && e.IsZero()
		h ^= uint64(e[0])
		h *= fnvPrime
	}
	if p.isExt {
		for i := lo; i < hi; i++ {
			e := &p.ext[i]
			mix(&e.B0.A0)
			mix(&e.B0.A1)
			mix(&e.B1.A0)
			mix(&e.B1.A1)
			mix(&e.B2.A0)
			mix(&e.B2.A1)
		}
		return zero, h
	}
	for i := lo; i < hi; i++ {
		mix(&p.base[i])
	}
	return zero, h
}

func (p *paddedColumn) len() int {
	if p.isExt {
		return len(p.ext)
	}
	return len(p.base)
}

// columnSizeIndex returns log2 of the column's padded size in rt.
func columnSizeIndex(col *wiop.Column, rt *wiop.Runtime) int {
	size := utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))
	sizeIndex := utils.Log2Ceil(size)
	if size != 1<<sizeIndex {
		panic("wiop: only powers of 2 are supported")
	}
	return sizeIndex
}

// materializeColumns writes down every column of round as committed (padded to
// its power-of-two size), in round.Columns order.
func materializeColumns(round *wiop.Round, rt *wiop.Runtime) []paddedColumn {
	out := make([]paddedColumn, len(round.Columns))
	for i, col := range round.Columns {
		sizeIndex := columnSizeIndex(col, rt)
		size := 1 << sizeIndex
		assignment := rt.GetColumnAssignment(col)
		out[i] = paddedColumn{sizeIndex: sizeIndex, isExt: col.IsExtension}
		if col.IsExtension {
			out[i].ext = writeDownVectorExt(assignment, size, col.Module.Padding)
		} else {
			out[i].base = writeDownVectorBase(assignment, size, col.Module.Padding)
		}
	}
	return out
}

// normalizedShifts returns the set of opening shifts of col normalized into
// [0, 2^sizeIndex), from the raw offsets collected at compile time.
func (c *compiled) normalizedShifts(col *wiop.Column, sizeIndex int) map[int]struct{} {
	size := 1 << sizeIndex
	out := make(map[int]struct{})
	for _, raw := range c.colShifts[col.Context.ID] {
		out[((raw%size)+size)%size] = struct{}{}
	}
	return out
}

// shiftSubset reports whether every normalized opening shift of alias is also
// an opening shift of target (both at the same padded size). This is what keeps
// the target's FRI shift schedule identical to the unelided one when the
// alias's claims are folded into it.
func (c *compiled) shiftSubset(alias, target *wiop.Column, sizeIndex int) bool {
	targetShifts := c.normalizedShifts(target, sizeIndex)
	for s := range c.normalizedShifts(alias, sizeIndex) {
		if _, ok := targetShifts[s]; !ok {
			return false
		}
	}
	return true
}

// buildManifest decides, for every column of round, whether it is committed,
// dropped as zero, or dropped as an alias of an earlier Present column. The
// decision is taken over the full padded vectors.
func (c *compiled) buildManifest(round *wiop.Round, vectors []paddedColumn) ColumnManifest {
	n := len(vectors)
	manifest := allPresentManifest(n)
	if c.elisionDisabled || n == 0 {
		return manifest
	}

	type bucket struct {
		sizeIndex int
		isExt     bool
		hash      uint64
	}

	// The zero test and the fingerprint read every committed cell; they run in
	// parallel chunks.
	zero, hashes := scanColumns(vectors)

	// A column can only alias an equal column, and equal columns share a
	// bucket, so each bucket decides its columns independently, in column
	// order: the buckets are processed concurrently.
	var buckets [][]int
	bucketOf := make(map[bucket]int)
	for i := range vectors {
		v := &vectors[i]
		if zero[i] {
			manifest[i] = ManifestZero
			continue
		}
		b := bucket{v.sizeIndex, v.isExt, hashes[i]}
		idx, ok := bucketOf[b]
		if !ok {
			idx = len(buckets)
			bucketOf[b] = idx
			buckets = append(buckets, nil)
		}
		buckets[idx] = append(buckets[idx], i)
	}
	parallel.ExecuteDynamic(len(buckets), func(bi int) {
		c.decideBucket(round, vectors, buckets[bi], manifest)
	})

	// A batch is never emptied: CommittedBatches is a static function of the
	// System and the FRI layer needs at least one committed row per batch. If
	// everything is zero (no alias can exist without a Present target), keep one
	// column. It must be a column that is actually opened: the FRI layer requires
	// every committed row to carry at least one shift, so keeping an unopened
	// column would fail canonicalLayout with "empty shift list". Fall back to
	// column 0 only when the round opens nothing at all, which cannot reach FRI
	// anyway.
	if !manifest.hasPresent() {
		manifest[c.firstOpenedColumn(round)] = ManifestPresent
	}
	return manifest
}

// scanChunkSize is the number of elements of a column scanned per work item.
const scanChunkSize = 1 << 14

// scanColumns reports, for every vector, whether it is identically zero and,
// when it is not, a 64-bit fingerprint: the FNV hash of the FNV hashes of its
// chunks. The fingerprint only buckets alias candidates, every match being
// confirmed by [paddedColumn.equal]. The chunks of every column are scanned
// concurrently, so a single tall column is not scanned on one core.
func scanColumns(vectors []paddedColumn) (zero []bool, hashes []uint64) {
	type chunk struct{ col, lo, hi int }
	var chunks []chunk
	first := make([]int, len(vectors)+1) // chunks of column i: [first[i], first[i+1])
	for i := range vectors {
		first[i] = len(chunks)
		n := vectors[i].len()
		for lo := 0; lo < n; lo += scanChunkSize {
			chunks = append(chunks, chunk{i, lo, min(lo+scanChunkSize, n)})
		}
	}
	first[len(vectors)] = len(chunks)
	chunkZero := make([]bool, len(chunks))
	chunkHash := make([]uint64, len(chunks))
	parallel.Execute(len(chunks), func(start, end int) {
		for c := start; c < end; c++ {
			chunkZero[c], chunkHash[c] = vectors[chunks[c].col].scanChunk(chunks[c].lo, chunks[c].hi)
		}
	})

	zero = make([]bool, len(vectors))
	hashes = make([]uint64, len(vectors))
	for i := range vectors {
		zero[i] = true
		h := uint64(fnvOffset)
		for c := first[i]; c < first[i+1]; c++ {
			zero[i] = zero[i] && chunkZero[c]
			h ^= chunkHash[c]
			h *= fnvPrime
		}
		if !zero[i] {
			hashes[i] = h
		}
	}
	return zero, hashes
}

// decideBucket sets the manifest of the non-zero columns of one fingerprint
// bucket, in column order: a column aliases the first earlier Present column
// it equals whose shift schedule covers its own, and is Present otherwise.
//
// Equality is checked once per column, against the bucket's first column and
// in parallel: the columns equal to it form one class, within which only the
// shift schedules decide. The others, fingerprint collisions, are decided by
// comparing them pairwise. A column never aliases a column it does not
// equal, so the classes are decided independently with the same result.
func (c *compiled) decideBucket(round *wiop.Round, vectors []paddedColumn, members []int, manifest ColumnManifest) {
	equalFirst := make([]bool, len(members))
	equalFirst[0] = true
	parallel.ExecuteDynamic(len(members)-1, func(j int) {
		equalFirst[j+1] = vectors[members[j+1]].equal(&vectors[members[0]])
	})
	var class, rest []int
	for j, i := range members {
		if equalFirst[j] {
			class = append(class, i)
		} else {
			rest = append(rest, i)
		}
	}
	decide := func(cols []int, equal func(k, i int) bool) {
		var present []int
	next:
		for _, i := range cols {
			for _, k := range present {
				if equal(k, i) && c.shiftSubset(round.Columns[i], round.Columns[k], vectors[i].sizeIndex) {
					manifest[i] = ManifestAlias(k)
					continue next
				}
			}
			present = append(present, i)
		}
	}
	decide(class, func(_, _ int) bool { return true })
	decide(rest, func(k, i int) bool { return vectors[k].equal(&vectors[i]) })
}

// firstOpenedColumn returns the index of the first column of round that is
// opened by at least one [wiop.LagrangeEval], or 0 if none is. It is the pick
// for the never-empty-batch fallback in [compiled.buildManifest].
func (c *compiled) firstOpenedColumn(round *wiop.Round) int {
	for i, col := range round.Columns {
		if len(c.colShifts[col.Context.ID]) > 0 {
			return i
		}
	}
	return 0
}

// validateManifest checks the structural rules every manifest must satisfy,
// independently of the committed data: codes in range, alias targets earlier,
// Present, of the same kind and padded size, with a covering shift schedule,
// and at least one Present column. The verifier runs it on the transported
// manifest; the prover runs it on its own as a self-check.
func (c *compiled) validateManifest(round *wiop.Round, rt *wiop.Runtime, manifest ColumnManifest) error {
	cols := round.Columns
	if len(manifest) != len(cols) {
		return fmt.Errorf("pcs: round %d manifest has %d entries for %d columns", round.ID, len(manifest), len(cols))
	}
	for i, code := range manifest {
		switch code {
		case ManifestPresent, ManifestZero:
			continue
		}
		k, ok := ManifestAliasTarget(code)
		if !ok || k >= i {
			return fmt.Errorf("pcs: round %d column %d: invalid manifest code %d", round.ID, i, code)
		}
		if manifest[k] != ManifestPresent {
			return fmt.Errorf("pcs: round %d column %d aliases column %d which is not present", round.ID, i, k)
		}
		if cols[k].IsExtension != cols[i].IsExtension {
			return fmt.Errorf("pcs: round %d column %d aliases column %d of a different field kind", round.ID, i, k)
		}
		sizeIndex := columnSizeIndex(cols[i], rt)
		if columnSizeIndex(cols[k], rt) != sizeIndex {
			return fmt.Errorf("pcs: round %d column %d aliases column %d of a different size", round.ID, i, k)
		}
		if !c.shiftSubset(cols[i], cols[k], sizeIndex) {
			return fmt.Errorf("pcs: round %d column %d aliases column %d whose shift schedule does not cover it", round.ID, i, k)
		}
	}
	if len(cols) > 0 && !manifest.hasPresent() {
		return fmt.Errorf("pcs: round %d manifest elides every column", round.ID)
	}
	return nil
}

// ManifestCells returns the column manifest cells of a committed interactive
// round, one per column in round.Columns order, or nil if the round is not
// PCS-committed. This is the seam verifier codegen uses to locate the manifest
// in the transcript.
func ManifestCells(round *wiop.Round) []*wiop.Cell {
	for _, a := range round.ProverActions {
		if commit, ok := a.(*commitRoundAction); ok && commit.round == round {
			return commit.c.manifestCells[round.ID]
		}
	}
	return nil
}

// assignManifest writes manifest into the round's manifest cells.
func (c *compiled) assignManifest(round *wiop.Round, rt *wiop.Runtime, manifest ColumnManifest) {
	cells := c.manifestCells[round.ID]
	if len(cells) != len(manifest) {
		panic(fmt.Sprintf("pcs: round %d has %d manifest cells for %d columns", round.ID, len(cells), len(manifest)))
	}
	for i, code := range manifest {
		rt.AssignCell(cells[i], field.ElemFromBase(field.NewElement(uint64(code))))
	}
}

// readManifest reads a batch's manifest back from the runtime: the transported
// cells for an interactive batch, all-Present for the precomputed batch. It
// validates the structure and returns an error for a malformed manifest.
func (c *compiled) readManifest(rt *wiop.Runtime, b BatchRef) (ColumnManifest, error) {
	if b.IsPrecomp {
		return allPresentManifest(len(b.Round.Columns)), nil
	}
	cells := c.manifestCells[b.Round.ID]
	manifest := make(ColumnManifest, len(cells))
	for i, cell := range cells {
		v := rt.GetCellValue(cell)
		if !v.IsBase() {
			return nil, fmt.Errorf("pcs: round %d manifest cell %d is not a base-field element", b.Round.ID, i)
		}
		base := v.AsBase()
		code := base.Uint64()
		if code > uint64(^uint32(0)) {
			return nil, fmt.Errorf("pcs: round %d manifest cell %d out of range", b.Round.ID, i)
		}
		manifest[i] = uint32(code)
	}
	if err := c.validateManifest(b.Round, rt, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

// readManifests reads every batch's manifest, aligned with batches.
func (c *compiled) readManifests(rt *wiop.Runtime, batches []BatchRef) ([]ColumnManifest, error) {
	out := make([]ColumnManifest, len(batches))
	for i, b := range batches {
		m, err := c.readManifest(rt, b)
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}
