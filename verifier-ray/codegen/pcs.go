package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// PcsSystem is the compile-time FRI/PCS description the Zig verifier consumes:
// the FRI params, the EXPLICIT canonical layout (the already-compiled
// prover-ray `canonicalLayout` output, one SizeBundle per opened size with each
// opened column carrying its flat EntryIdx), the claim maps that re-slice the
// PCS-authenticated entry_claims into the vanishing witness/quotient claims, and
// the flat all_coins index of the shared opening point zeta.
//
// It is extracted from an ALREADY-compiled (global.Compile + pcs.Compile) and
// proven (sys, rt) protocol, so it can never drift from the prover's committed
// column ordering: batch order, per-batch layout, and the LagrangeEval openings
// all come from the prover-ray PCS compiler's own exported helpers.
//
// Adaptation vs the fri-pcs branch: that branch emitted per-batch Shapes/Shifts
// and let the Zig `pcs.layout.buildLayout` reconstruct the canonical layout at
// runtime. The current-branch engine (`src/query/pcs.zig`) has NO buildLayout;
// its `pcs.System` carries the layout as an explicit literal `[]const
// SizeBundle` whose entries reference a flat `entry_idx`. So the layout is fully
// materialized here (Layout []PcsSizeBundle, with EntryIdx assigned in the same
// canonical order the engine's entry_claims indexing uses).
type PcsSystem struct {
	SourceName string

	// FRI params (already restricted to the largest opened size, mirroring the
	// fixture: LogPlaintextSize == the max opened size, LogCodewordSize ==
	// that + FRILogInverseRate; the Zig verifier's restrictTo is then a no-op).
	LogCodewordSize  int
	LogPlaintextSize int
	LogFinalPolySize int
	NumQueries       int
	NumBatches       int

	// Layout is the explicit canonical layout the current-branch engine consumes
	// directly: one PcsSizeBundle per opened size in descending-size order, each
	// carrying its opened columns with a running EntryIdx. Replaces the fri-pcs
	// branch's Shapes/Shifts pair (which the old engine turned into a layout via
	// buildLayout).
	Layout []PcsSizeBundle

	// Claim maps: WitnessMap[k] / QuotientMap[k] is the (entry, shift) the
	// vanishing System's k-th witness / quotient claim is re-sliced from. Their
	// lengths equal the vanishing System's TotalWitnessClaims / TotalQuotientClaims.
	WitnessMap  []PcsClaimRef
	QuotientMap []PcsClaimRef

	// ZetaCoinIndex is the flat all_coins index of the shared LagrangeEval eval
	// coin (zeta), which is also every vanishing module's eval coin.
	ZetaCoinIndex int

	// BatchRoots gives each batch's root provenance, in canonical batch order.
	// The Zig verifier rebuilds the authenticated roots from this — reading
	// interactive-batch roots from the transcript-bound round oracle commitments
	// and precomputed roots from the emitted constant — so the root a batch is
	// Merkle-authenticated against is provably the one zeta is bound to (mirrors
	// prover-ray's single-source collectRoots). Length == NumBatches.
	BatchRoots []PcsBatchRoot

	// EntryClaims are the runtime claimed evaluations captured from the proving
	// runtime, jagged `[entry][shift]` in the SAME canonical order EntryIdx uses
	// (so EntryClaims[g] belongs to the entry whose EntryIdx == g, and its length
	// equals that entry's shift count). This is the prover-supplied
	// `verifier.PcsOpening.entry_claims` — extracted here, in the one place that
	// owns the canonical ordering, so it can never drift from Layout/WitnessMap.
	EntryClaims [][]field.Ext
}

// PcsSizeBundle is one size-level of the explicit canonical layout: all opened
// columns at a single size, in canonical (batch ASC, base-before-ext, position
// ASC) order. Mirrors the engine's `pcs.SizeBundle`.
type PcsSizeBundle struct {
	SizeLog2 uint8
	Entries  []PcsDeepEntry
}

// PcsDeepEntry is one opened committed column within a size bundle (the engine's
// `pcs.DeepEntry`). EntryIdx is the flat index of this opened column into
// VerifyInput.entry_claims, assigned as a running counter across the whole
// layout in canonical order; it is exactly what a PcsClaimRef.Entry names.
type PcsDeepEntry struct {
	BatchIdx int
	IsExt    bool
	RowIdx   int
	EntryIdx int
	Shifts   []int
}

// PcsClaimRef is the verifier-ray verify.ClaimRef: the flat entry index plus the
// shift slot within that entry a single opened (column, shift) maps to.
type PcsClaimRef struct {
	Entry int
	Shift int
}

// PcsBatchRoot is one batch's root provenance (verifier-ray verify.BatchRoot).
// Exactly one of the two forms applies: an interactive batch names the
// proof.rounds index whose oracle commitment is its root; a precomputed batch
// carries the compile-time root constant.
type PcsBatchRoot struct {
	// Precomputed is true for the static precomputed batch. When true, Root holds
	// the compile-time commitment and RoundIndex is unused; when false, RoundIndex
	// names the interactive round and Root is unused.
	Precomputed bool
	// RoundIndex is the proof.rounds index (== wiop Round.ID) whose sole oracle
	// commitment is this batch's root. Valid only when Precomputed is false.
	RoundIndex int
	// Root is the precomputed-batch commitment. Valid only when Precomputed is true.
	Root field.Octuplet
}

// pcsEntryKey identifies one opened column in the flat canonical entry order.
type pcsEntryKey struct {
	batchIdx int
	sizeLog2 int
	isExt    bool
	position int
}

// pcsLocWithBatch is a column location plus the batch index it belongs to.
type pcsLocWithBatch struct {
	batchIdx int
	loc      pcscompiler.ColumnLocation
}

// HasCommittedDynamicColumn reports whether any PCS-committed batch contains a
// column owned by a dynamically-sized module. Such a column's FRI bundle
// (SizeLog2) is derived from its runtime size, so a comptime pcs.System baked
// from one proving run cannot verify proofs of other sizes — BuildPcsSystem
// rejects it. Callers that emit fixtures per protocol can use this to skip such
// protocols until the engine gains a runtime-size-reconstructed layout.
func HasCommittedDynamicColumn(sys *wiop.System) bool {
	for _, b := range pcscompiler.CommittedBatches(sys) {
		for _, col := range b.Round.Columns {
			if col.Module != nil && col.Module.IsDynamic() {
				return true
			}
		}
	}
	return false
}

// BuildPcsSystem extracts the FRI/PCS System from a compiled, proven protocol.
//
// Requires pcs.Compile to have run after global.Compile: the LagrangeEval
// openings (witness views + quotient shares) that the claim maps re-slice are
// registered by global.Compile, and pcs.Compile commits the batches and produces
// the opening proof. It reads only the committed batches, the LagrangeEvals, the
// coin routing, and the per-batch layout — nothing scenario-specific — so it
// works for any real protocol, not just the fixtures.
//
// `rt` must be the runtime that proved `sys` (it supplies the per-round layout
// sizes and the claimed evaluation values). `routing` is the shared coin layout
// from BuildCoinRouting; it locates the zeta coin in the flat all_coins array.
func BuildPcsSystem(sys *wiop.System, rt *wiop.Runtime, routing CoinRouting) (PcsSystem, error) {
	batches := pcscompiler.CommittedBatches(sys)
	if len(batches) == 0 {
		return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no committed batches; did pcs.Compile run?")
	}
	if len(sys.LagrangeEvals) == 0 {
		return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no LagrangeEval openings; did global.Compile run before pcs.Compile?")
	}

	// Per-batch layout + shapes, a global column->location map (with batch), and
	// each batch's root provenance. An interactive batch's root is the oracle
	// commitment absorbed for its round (proof.rounds index == Round.ID, since
	// rounds are emitted in ID order); the precomputed batch's root is the
	// compile-time PrecomputedCommitment. This is what lets the Zig verifier bind
	// the authenticated root to the transcript instead of trusting the proof.
	colLoc := map[wiop.ObjectID]pcsLocWithBatch{}
	shapes := make([]fri.Shape, len(batches))
	batchRoots := make([]PcsBatchRoot, len(batches))
	for i, b := range batches {
		// SINGLE-SIZE LIMITATION (guard, fail-loud): a committed column's FRI
		// bundle placement (SizeLog2) is derived by GetLayout from that column's
		// runtime size (utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))). For a
		// DYNAMIC module that size varies per proof, so the layout we freeze into
		// the comptime pcs.System here would only match proofs whose dynamic
		// module_sizes equal this single proving run — a different valid proof
		// would place the column in a different bundle and fail verification.
		//
		// The verifier engine reconstructs neither the layout nor entry_idx from
		// runtime module_sizes (size_log2 is comptime), so we cannot emit a
		// size-independent System. Reject rather than silently emit a System that
		// only accepts one size. (Static modules, and dynamic modules whose columns
		// are NOT committed, are unaffected — the transcript-side dynamic-size
		// absorption in root.zig still handles their annihilator sizing.)
		//
		// Lifting this requires a runtime-size-reconstructed layout in the engine
		// (a larger change); see docs. Until then a protocol with committed dynamic
		// columns needs one generated System per distinct size.
		for _, col := range b.Round.Columns {
			if col.Module != nil && col.Module.IsDynamic() {
				return PcsSystem{}, fmt.Errorf(
					"codegen: BuildPcsSystem: committed column %q belongs to dynamic module %q; "+
						"its PCS bundle size is frozen to this proof's runtime size and would reject "+
						"other-size proofs. Committed dynamic-module columns are not yet supported "+
						"(needs a runtime-size-reconstructed layout in the verifier engine)",
					col.Context.Path(), col.Module.Context.Path(),
				)
			}
		}
		locs, shape := pcscompiler.GetLayout(b.Round, rt)
		shapes[i] = shape
		for id, l := range locs {
			colLoc[id] = pcsLocWithBatch{batchIdx: i, loc: l}
		}
		if b.IsPrecomp {
			batchRoots[i] = PcsBatchRoot{Precomputed: true, Root: sys.PrecomputedCommitment}
		} else {
			batchRoots[i] = PcsBatchRoot{Precomputed: false, RoundIndex: b.Round.ID}
		}
	}

	// Shifts: for each opened (column, shift), record the normalized shift into
	// the batch's per-size schedule, keyed to the column's row. shiftOrder keeps
	// the per-key ordering that fixes each opened value's shift slot.
	shifts := make([]fri.BatchShifts, len(batches))
	for i := range batches {
		shifts[i] = initPcsShifts(shapes[i])
	}
	shiftOrder := map[pcsEntryKey][]int{}
	seen := map[pcsEntryKey]map[int]bool{}
	// claimValues[key][shift] is the runtime claimed evaluation for that opened
	// (column, shift) — captured in the same dedup pass so EntryClaims can be
	// materialized in canonical order below.
	claimValues := map[pcsEntryKey]map[int]field.Ext{}

	recordClaim := func(cv *wiop.ColumnView, value field.Ext) error {
		lb, ok := colLoc[cv.Column.Context.ID]
		if !ok {
			return fmt.Errorf("codegen: BuildPcsSystem: opened column %q not in any committed batch",
				cv.Column.Context.Path())
		}
		size := 1 << lb.loc.SizeID
		shift := ((cv.ShiftingOffset % size) + size) % size
		key := pcsEntryKey{batchIdx: lb.batchIdx, sizeLog2: lb.loc.SizeID, isExt: lb.loc.IsExt, position: lb.loc.Position}
		if seen[key] == nil {
			seen[key] = map[int]bool{}
			claimValues[key] = map[int]field.Ext{}
		}
		if seen[key][shift] {
			return nil // deduplicate repeated (column, shift) openings
		}
		seen[key][shift] = true
		claimValues[key][shift] = value
		shiftOrder[key] = append(shiftOrder[key], shift)
		ss := &shifts[key.batchIdx][key.sizeLog2]
		if key.isExt {
			ss.Ext[key.position] = append(ss.Ext[key.position], shift)
		} else {
			ss.Base[key.position] = append(ss.Base[key.position], shift)
		}
		return nil
	}

	for _, le := range sys.LagrangeEvals {
		for k, cv := range le.Polynomials {
			value := rt.GetCellValue(le.EvaluationClaims[k]).AsExt()
			if err := recordClaim(cv, value); err != nil {
				return PcsSystem{}, err
			}
		}
	}

	// Explicit canonical layout: size DESC, batch ASC, base-then-ext, position
	// ASC — matching the current engine's entry_claims indexing. computePcsLayout
	// assigns each entry a running EntryIdx in this order; entryIndex records the
	// same assignment keyed by pcsEntryKey so buildPcsClaimMaps can map a
	// vanishing claim cell to the exact same flat entry index.
	layout, entryIndex := computePcsLayout(batches, shapes, shifts)

	// Materialize entry_claims in EntryIdx order: the layout bundles are already
	// in canonical order, and each entry's shiftOrder fixes its per-shift slots,
	// so entryClaims[EntryIdx][slot] == claimValues[key][shiftOrder[key][slot]].
	entryClaims := make([][]field.Ext, 0)
	for _, bundle := range layout {
		for _, e := range bundle.Entries {
			key := pcsEntryKey{batchIdx: e.BatchIdx, sizeLog2: int(bundle.SizeLog2), isExt: e.IsExt, position: e.RowIdx}
			row := make([]field.Ext, len(e.Shifts))
			for slot, shift := range e.Shifts {
				v, ok := claimValues[key][shift]
				if !ok {
					return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no claim value for entry %d shift %d", e.EntryIdx, shift)
				}
				row[slot] = v
			}
			entryClaims = append(entryClaims, row)
		}
	}

	// maxSize == the largest opened size (the top bundle's size), the value the
	// FRI params are restricted to; derived from the layout so it can never
	// disagree with the emitted bundles.
	maxSize := 0
	for _, sh := range shapes {
		if len(sh)-1 > maxSize {
			maxSize = len(sh) - 1
		}
	}

	witnessMap, quotientMap, err := buildPcsClaimMaps(sys, colLoc, entryIndex, shiftOrder)
	if err != nil {
		return PcsSystem{}, err
	}

	zetaIdx, err := pcsZetaCoinIndex(sys, routing)
	if err != nil {
		return PcsSystem{}, err
	}

	return PcsSystem{
		SourceName:       sys.Context.Path(),
		LogCodewordSize:  maxSize + pcscompiler.FRILogInverseRate,
		LogPlaintextSize: maxSize,
		LogFinalPolySize: 0,
		NumQueries:       pcscompiler.FRINumQueries(),
		NumBatches:       len(batches),
		Layout:           layout,
		WitnessMap:       witnessMap,
		QuotientMap:      quotientMap,
		ZetaCoinIndex:    zetaIdx,
		BatchRoots:       batchRoots,
		EntryClaims:      entryClaims,
	}, nil
}

// computePcsLayout materializes the explicit canonical layout the current-branch
// engine consumes, mirroring `computeLayout` in testdata/generate/fri/main.go:
// for each size in descending order, batches in declaration order, base rows then
// ext rows, row declaration order, with EntryIdx accumulating across the whole
// layout (the flat entry_claims index).
//
// It returns the layout AND the entryIndex map (pcsEntryKey -> EntryIdx) so
// buildPcsClaimMaps can align each vanishing claim to the same flat entry index —
// the two MUST enumerate entries in exactly the same order for the ClaimRefs to
// be correct.
func computePcsLayout(batches []pcscompiler.BatchRef, shapes []fri.Shape, shifts []fri.BatchShifts) ([]PcsSizeBundle, map[pcsEntryKey]int) {
	maxSizeLog2 := -1
	for _, s := range shapes {
		if len(s) > maxSizeLog2+1 {
			maxSizeLog2 = len(s) - 1
		}
	}

	var layout []PcsSizeBundle
	entryIndex := map[pcsEntryKey]int{}
	entryIdx := 0
	for sizeLog2 := maxSizeLog2; sizeLog2 >= 0; sizeLog2-- {
		bundle := PcsSizeBundle{SizeLog2: uint8(sizeLog2)}
		for batchIdx := range batches {
			if sizeLog2 >= len(shapes[batchIdx]) {
				continue
			}
			shape := shapes[batchIdx][sizeLog2]
			rowShifts := shifts[batchIdx][sizeLog2]
			for rowIdx := 0; rowIdx < shape.BaseWidth; rowIdx++ {
				bundle.Entries = append(bundle.Entries, PcsDeepEntry{
					BatchIdx: batchIdx, IsExt: false, RowIdx: rowIdx,
					EntryIdx: entryIdx, Shifts: append([]int(nil), rowShifts.Base[rowIdx]...),
				})
				entryIndex[pcsEntryKey{batchIdx: batchIdx, sizeLog2: sizeLog2, isExt: false, position: rowIdx}] = entryIdx
				entryIdx++
			}
			for rowIdx := 0; rowIdx < shape.ExtWidth; rowIdx++ {
				bundle.Entries = append(bundle.Entries, PcsDeepEntry{
					BatchIdx: batchIdx, IsExt: true, RowIdx: rowIdx,
					EntryIdx: entryIdx, Shifts: append([]int(nil), rowShifts.Ext[rowIdx]...),
				})
				entryIndex[pcsEntryKey{batchIdx: batchIdx, sizeLog2: sizeLog2, isExt: true, position: rowIdx}] = entryIdx
				entryIdx++
			}
		}
		if len(bundle.Entries) > 0 {
			layout = append(layout, bundle)
		}
	}
	return layout, entryIndex
}

func initPcsShifts(shape fri.Shape) fri.BatchShifts {
	bs := make(fri.BatchShifts, len(shape))
	for i, ss := range shape {
		bs[i] = fri.SizedShifts{
			Base: make([][]int, ss.BaseWidth),
			Ext:  make([][]int, ss.ExtWidth),
		}
	}
	return bs
}

// buildPcsClaimMaps produces the witness/quotient claim maps in the SAME order
// BuildVanishingSystem enumerates its flat witness/quotient claims (per
// global.Verifier: WitnessClaims, then per bucket QuotientClaims). Each claim
// CELL is a LagrangeEval.EvaluationClaims entry whose paired Polynomials[k] gives
// the opened (column, shift); that maps to a flat entry index + shift slot. This
// is the concrete realization of the invariant that the vanishing claims ARE a
// re-slicing of entry_claims.
//
// The `entryIndex` it consults is the SAME map computePcsLayout populated (same
// pcsEntryKey ordering), so a ClaimRef.Entry always names the layout entry whose
// EntryIdx matches.
func buildPcsClaimMaps(
	sys *wiop.System,
	colLoc map[wiop.ObjectID]pcsLocWithBatch,
	entryIndex map[pcsEntryKey]int,
	shiftOrder map[pcsEntryKey][]int,
) (witnessMap, quotientMap []PcsClaimRef, err error) {
	// cell ObjectID -> the column view it opens, via the LagrangeEvals.
	cellView := map[wiop.ObjectID]*wiop.ColumnView{}
	for _, le := range sys.LagrangeEvals {
		for k, cv := range le.Polynomials {
			cellView[le.EvaluationClaims[k].Context.ID] = cv
		}
	}

	refFor := func(cell *wiop.Cell) (PcsClaimRef, error) {
		cv, ok := cellView[cell.Context.ID]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf(
				"codegen: BuildPcsSystem: vanishing claim cell %q has no LagrangeEval opening — "+
					"the column it opens is not PCS-authenticated", cell.Context.Path())
		}
		lb := colLoc[cv.Column.Context.ID]
		size := 1 << lb.loc.SizeID
		shift := ((cv.ShiftingOffset % size) + size) % size
		key := pcsEntryKey{batchIdx: lb.batchIdx, sizeLog2: lb.loc.SizeID, isExt: lb.loc.IsExt, position: lb.loc.Position}
		entry, ok := entryIndex[key]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: no flat entry for %+v", key)
		}
		slot := -1
		for i, s := range shiftOrder[key] {
			if s == shift {
				slot = i
				break
			}
		}
		if slot < 0 {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: shift %d not found for entry %+v", shift, key)
		}
		return PcsClaimRef{Entry: entry, Shift: slot}, nil
	}

	for _, verifier := range pcsGlobalVerifiers(sys) {
		for _, cell := range verifier.WitnessClaims {
			ref, e := refFor(cell)
			if e != nil {
				return nil, nil, e
			}
			witnessMap = append(witnessMap, ref)
		}
		for _, bucket := range verifier.Buckets {
			for _, cell := range bucket.QuotientClaims {
				ref, e := refFor(cell)
				if e != nil {
					return nil, nil, e
				}
				quotientMap = append(quotientMap, ref)
			}
		}
	}
	return witnessMap, quotientMap, nil
}

// pcsGlobalVerifiers collects the compiled global.Verifier actions in
// round/registration order — the SAME order BuildVanishingSystem walks, so the
// claim maps align index-for-index with the vanishing System's flat claims.
func pcsGlobalVerifiers(sys *wiop.System) []*global.Verifier {
	var verifiers []*global.Verifier
	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			if v, ok := action.(*global.Verifier); ok {
				verifiers = append(verifiers, v)
			}
		}
	}
	return verifiers
}

// pcsZetaCoinIndex returns the flat all_coins index of the shared LagrangeEval
// eval coin (zeta). All LagrangeEvals share this coin (global.Compile's
// evalCoin), so reading it from the first is sufficient.
func pcsZetaCoinIndex(sys *wiop.System, routing CoinRouting) (int, error) {
	coin, ok := sys.LagrangeEvals[0].EvaluationPoint.(*wiop.CoinField)
	if !ok {
		return 0, fmt.Errorf("codegen: BuildPcsSystem: LagrangeEval EvaluationPoint is not a CoinField")
	}
	round := coin.Context.ID.Slot()
	pos := coin.Context.ID.Position()
	if round >= len(routing.RoundCoinOffsets) {
		return 0, fmt.Errorf("codegen: BuildPcsSystem: zeta coin round %d out of range", round)
	}
	idx := routing.RoundCoinOffsets[round] + pos
	if idx >= routing.TotalRoundCoins {
		return 0, fmt.Errorf("codegen: BuildPcsSystem: zeta coin flat index %d >= total %d", idx, routing.TotalRoundCoins)
	}
	return idx, nil
}
