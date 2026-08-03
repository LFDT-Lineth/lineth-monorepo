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

	// FRI ENVELOPE params (the prover-ray static maxCommittableSizeLog2 schedule,
	// NOT restricted to any one proof): LogPlaintextSize == FRIMaxCommittableSizeLog2,
	// LogCodewordSize == that + FRILogInverseRate. The Zig verifier reconstructs
	// the layout and restricts these to each proof's largest opened size, so ONE
	// baked System verifies proofs of different dynamic sizes.
	LogCodewordSize  int
	LogPlaintextSize int
	LogFinalPolySize int
	NumQueries       int
	NumBatches       int

	// Columns is the symbolic committed-column descriptor list in prover
	// DECLARATION order (batch-major, then round.Columns order). The Zig verifier
	// reconstructs the canonical layout (bundles / entry order / positions) from
	// these + the proof's runtime module_sizes. Replaces the frozen Layout.
	Columns []PcsColumnDesc

	// MaxEntries == len(Columns) and MaxSizeLog2 == the envelope log_plaintext_size:
	// the comptime bounds the Zig verifier sizes its stack reconstruction buffers by.
	MaxEntries  int
	MaxSizeLog2 int

	// Claim maps: WitnessMap[k] / QuotientMap[k] is the (col_decl_idx, shift) the
	// vanishing System's k-th witness / quotient claim is re-sliced from. The
	// col_decl_idx names a column by its declaration index; the Zig verifier
	// resolves it to the runtime canonical entry. Their lengths equal the
	// vanishing System's TotalWitnessClaims / TotalQuotientClaims.
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

// PcsColumnDesc is one committed column in prover DECLARATION order (the engine's
// `pcs.ColumnDesc`). Its Size is either a static comptime size_log2 (a
// static-module column) or a DynamicIndex into module_sizes (a dynamic-module
// column, whose size varies per proof). The verifier reconstructs each column's
// bundle / position / entry index from these + module_sizes.
type PcsColumnDesc struct {
	BatchIdx int
	IsExt    bool
	// IsDynamic selects the Size source: when true, DynamicIndex names the
	// module_sizes slot; when false, SizeLog2 is the fixed static size.
	IsDynamic    bool
	SizeLog2     int
	DynamicIndex int
	// Shifts is the size-independent opening schedule of this column (normalized
	// shifts, in the slot order the claim maps' Shift references).
	Shifts []int
}

// PcsClaimRef is the verifier-ray verify.ClaimRef: the column DECLARATION index
// plus the shift slot within that column a single opened (column, shift) maps to.
// The verifier resolves ColDeclIdx to the runtime canonical entry.
type PcsClaimRef struct {
	ColDeclIdx int
	Shift      int
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
		// The layout the verifier reconstructs (bundle placement, entry order,
		// positions) is a RUNTIME function of module_sizes — dynamic-module columns
		// are fully supported. Codegen still runs GetLayout at the proving runtime
		// to capture the shift schedules and the claimed values (which ARE
		// size-independent openings); the verifier re-derives the size-dependent
		// bundle placement from ColumnDesc + module_sizes.
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
	// normSeen[key][normalizedShift] = the raw offset that produced it, used to
	// detect dynamic-column shift aliasing (two raw offsets congruent mod the
	// size) that prover-ray dedups but the size-independent schedule cannot.
	normSeen := map[pcsEntryKey]map[int]int{}
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
		// The shift the schedule stores depends on the column's size source:
		//   - STATIC column: its size never changes, so we store the NORMALIZED
		//     shift ((offset % size) + size) % size. This matches prover-ray's own
		//     per-proof dedup (two raw offsets that alias mod the fixed size ARE the
		//     same opening — e.g. -1 and 3 both -> 3 at size 4), so the point set
		//     and entry_claims line up exactly.
		//   - DYNAMIC column: the size varies per proof, so we store the RAW offset
		//     and let the verifier normalize it mod the RUNTIME size. omega_N^(offset
		//     mod N) == omega_N^offset, so the reconstructed point matches the
		//     prover's at every size.
		//
		// SOUNDNESS/COMPLETENESS GUARD (dynamic columns): prover-ray dedups openings
		// by NORMALIZED shift (mod the runtime size), but here we dedup a dynamic
		// column's schedule by RAW offset. If two of a dynamic column's raw offsets
		// alias — i.e. are congruent mod the size — the prover collapses them to one
		// opening while the raw schedule would keep two slots. The verifier then
		// expects one more claim than the prover produced and double-counts the DEEP
		// quotient (honest proof rejected; and the extra slot is an unauthenticated
		// claim value). We CANNOT reproduce the prover's per-proof (runtime-size)
		// dedup from a size-independent schedule, so we reject it at codegen rather
		// than emit a System that silently diverges. (Detected at the codegen size;
		// aliasing there is a necessary condition — the observed proof already
		// exhibits the collapse the raw schedule cannot represent.)
		size := 1 << lb.loc.SizeID
		isDynamic := cv.Column.Module != nil && cv.Column.Module.IsDynamic()
		normShift := ((cv.ShiftingOffset % size) + size) % size
		shift := normShift
		if isDynamic {
			shift = cv.ShiftingOffset
		}
		key := pcsEntryKey{batchIdx: lb.batchIdx, sizeLog2: lb.loc.SizeID, isExt: lb.loc.IsExt, position: lb.loc.Position}
		if seen[key] == nil {
			seen[key] = map[int]bool{}
			claimValues[key] = map[int]field.Ext{}
			normSeen[key] = map[int]int{}
		}
		if isDynamic {
			// Reject aliasing: a previously-recorded raw offset for this column that
			// normalizes to the same value would be the prover's single opening.
			if prevRaw, aliased := normSeen[key][normShift]; aliased && prevRaw != cv.ShiftingOffset {
				return fmt.Errorf(
					"codegen: BuildPcsSystem: dynamic column %q opens at offsets %d and %d that alias "+
						"(both %d mod %d); prover-ray dedups them to one opening but the size-independent "+
						"schedule cannot, so one baked System would diverge. Aliasing dynamic-column shifts "+
						"are not supported",
					cv.Column.Context.Path(), prevRaw, cv.ShiftingOffset, normShift, size)
			}
			normSeen[key][normShift] = cv.ShiftingOffset
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

	// Canonical entry order at the codegen size: size DESC, batch ASC,
	// base-then-ext, position ASC — the SAME order the verifier's runtime
	// reconstruction produces at this size, so EntryClaims lines up entry-for-entry.
	entries, entryIndex := computePcsLayout(batches, shapes, shifts)

	// Materialize entry_claims in canonical entry order: each entry's shiftOrder
	// fixes its per-shift slots, so entryClaims[e][slot] ==
	// claimValues[key][shiftOrder[key][slot]].
	entryClaims := make([][]field.Ext, 0)
	for _, e := range entries {
		key := pcsEntryKey{batchIdx: e.batchIdx, sizeLog2: e.sizeLog2, isExt: e.isExt, position: e.rowIdx}
		row := make([]field.Ext, len(e.shifts))
		for slot, shift := range e.shifts {
			v, ok := claimValues[key][shift]
			if !ok {
				return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no claim value for entry %+v shift %d", key, shift)
			}
			row[slot] = v
		}
		entryClaims = append(entryClaims, row)
	}

	_ = entryIndex

	// Columns: every committed column in prover DECLARATION order (batch-major,
	// then round.Columns order). Each column carries its batch, is_ext, size
	// source (static size_log2 from the module's fixed size, or a DynamicIndex
	// into module_sizes for a dynamic module) and its size-independent shift
	// schedule. colDeclByID maps a column ObjectID to its declaration index so the
	// claim maps can reference a column instead of a size-frozen entry index.
	dynIdx := DynamicModuleIndex(sys)
	columns := make([]PcsColumnDesc, 0)
	colDeclByID := map[wiop.ObjectID]int{}
	for i, b := range batches {
		for _, col := range b.Round.Columns {
			lb := colLoc[col.Context.ID]
			key := pcsEntryKey{batchIdx: lb.batchIdx, sizeLog2: lb.loc.SizeID, isExt: lb.loc.IsExt, position: lb.loc.Position}
			desc := PcsColumnDesc{
				BatchIdx: i,
				IsExt:    col.IsExtension,
				Shifts:   append([]int(nil), shiftOrder[key]...),
			}
			if col.Module != nil && col.Module.IsDynamic() {
				idx, ok := dynIdx[col.Module]
				if !ok {
					return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: dynamic module %q of committed column %q has no module_sizes index",
						col.Module.Context.Path(), col.Context.Path())
				}
				desc.IsDynamic = true
				desc.DynamicIndex = idx
			} else {
				// Static column: size_log2 is the padded, fixed module size —
				// the SAME value GetLayout used (loc.SizeID) at this proving run,
				// but for a static module it is proof-independent.
				desc.SizeLog2 = lb.loc.SizeID
			}
			colDeclByID[col.Context.ID] = len(columns)
			columns = append(columns, desc)
		}
	}

	// Envelope params: the prover's process-wide static FRI schedule. The Zig
	// verifier restricts these to each proof's largest opened size, so ONE baked
	// System covers every dynamic size. Emitted from the prover's own exported
	// envelope so it can never drift from what the prover commits with.
	envelope := pcscompiler.FRIStaticParams()
	maxSizeLog2 := int(pcscompiler.FRIMaxCommittableSizeLog2())

	witnessMap, quotientMap, err := buildPcsClaimMaps(sys, colLoc, colDeclByID)
	if err != nil {
		return PcsSystem{}, err
	}

	zetaIdx, err := pcsZetaCoinIndex(sys, routing)
	if err != nil {
		return PcsSystem{}, err
	}

	return PcsSystem{
		SourceName:       sys.Context.Path(),
		LogCodewordSize:  int(envelope.LogCodewordSize),
		LogPlaintextSize: int(envelope.LogPlainTextSize),
		LogFinalPolySize: 0,
		NumQueries:       pcscompiler.FRINumQueries(),
		NumBatches:       len(batches),
		Columns:          columns,
		MaxEntries:       len(columns),
		MaxSizeLog2:      maxSizeLog2,
		WitnessMap:       witnessMap,
		QuotientMap:      quotientMap,
		ZetaCoinIndex:    zetaIdx,
		BatchRoots:       batchRoots,
		EntryClaims:      entryClaims,
	}, nil
}

// pcsLayoutEntry is one opened column in the flat canonical entry order (size
// DESC / batch ASC / base-then-ext / position ASC), used only to materialize
// EntryClaims in exactly the order the verifier's runtime reconstruction
// produces AT the codegen size. It is NOT emitted; the verifier reconstructs its
// own entry order from ColumnDesc + module_sizes.
type pcsLayoutEntry struct {
	batchIdx int
	sizeLog2 int
	isExt    bool
	rowIdx   int
	shifts   []int
}

// computePcsLayout enumerates the canonical entry order at the codegen proving
// size, mirroring prover-ray's canonicalLayout (and the verifier's reconstruct).
// The returned entries drive EntryClaims materialization; the returned
// entryIndex (pcsEntryKey -> flat idx) lets internal cross-checks align a claim
// cell to its entry. Both MUST agree with the verifier's runtime reconstruction
// at this same size.
func computePcsLayout(batches []pcscompiler.BatchRef, shapes []fri.Shape, shifts []fri.BatchShifts) ([]pcsLayoutEntry, map[pcsEntryKey]int) {
	maxSizeLog2 := -1
	for _, s := range shapes {
		if len(s) > maxSizeLog2+1 {
			maxSizeLog2 = len(s) - 1
		}
	}

	var entries []pcsLayoutEntry
	entryIndex := map[pcsEntryKey]int{}
	entryIdx := 0
	for sizeLog2 := maxSizeLog2; sizeLog2 >= 0; sizeLog2-- {
		for batchIdx := range batches {
			if sizeLog2 >= len(shapes[batchIdx]) {
				continue
			}
			shape := shapes[batchIdx][sizeLog2]
			rowShifts := shifts[batchIdx][sizeLog2]
			for rowIdx := 0; rowIdx < shape.BaseWidth; rowIdx++ {
				entries = append(entries, pcsLayoutEntry{
					batchIdx: batchIdx, sizeLog2: sizeLog2, isExt: false, rowIdx: rowIdx,
					shifts: append([]int(nil), rowShifts.Base[rowIdx]...),
				})
				entryIndex[pcsEntryKey{batchIdx: batchIdx, sizeLog2: sizeLog2, isExt: false, position: rowIdx}] = entryIdx
				entryIdx++
			}
			for rowIdx := 0; rowIdx < shape.ExtWidth; rowIdx++ {
				entries = append(entries, pcsLayoutEntry{
					batchIdx: batchIdx, sizeLog2: sizeLog2, isExt: true, rowIdx: rowIdx,
					shifts: append([]int(nil), rowShifts.Ext[rowIdx]...),
				})
				entryIndex[pcsEntryKey{batchIdx: batchIdx, sizeLog2: sizeLog2, isExt: true, position: rowIdx}] = entryIdx
				entryIdx++
			}
		}
	}
	return entries, entryIndex
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
// A ClaimRef names a column by its DECLARATION index (colDeclByID); the verifier
// resolves that to the runtime canonical entry. The Shift slot indexes into the
// column's shift schedule (the same shiftOrder that fixes EntryClaims' per-shift
// slots), so a routed claim lands on the exact authenticated value.
func buildPcsClaimMaps(
	sys *wiop.System,
	colLoc map[wiop.ObjectID]pcsLocWithBatch,
	colDeclByID map[wiop.ObjectID]int,
) (witnessMap, quotientMap []PcsClaimRef, err error) {
	// cell ObjectID -> the column view it opens, via the LagrangeEvals.
	cellView := map[wiop.ObjectID]*wiop.ColumnView{}
	for _, le := range sys.LagrangeEvals {
		for k, cv := range le.Polynomials {
			cellView[le.EvaluationClaims[k].Context.ID] = cv
		}
	}

	// The per-column shift order, keyed by column ObjectID: recovered by replaying
	// the LagrangeEvals in declaration order with the same normalization + dedup as
	// recordClaim, so slot indices match EntryClaims exactly.
	// shiftFor computes the schedule key for one opening: normalized for a static
	// column, raw offset for a dynamic one — matching recordClaim exactly.
	shiftFor := func(cv *wiop.ColumnView) int {
		lb := colLoc[cv.Column.Context.ID]
		size := 1 << lb.loc.SizeID
		if cv.Column.Module != nil && cv.Column.Module.IsDynamic() {
			return cv.ShiftingOffset
		}
		return ((cv.ShiftingOffset % size) + size) % size
	}

	shiftSlots := map[wiop.ObjectID][]int{}
	seen := map[wiop.ObjectID]map[int]bool{}
	for _, le := range sys.LagrangeEvals {
		for _, cv := range le.Polynomials {
			id := cv.Column.Context.ID
			shift := shiftFor(cv)
			if seen[id] == nil {
				seen[id] = map[int]bool{}
			}
			if seen[id][shift] {
				continue
			}
			seen[id][shift] = true
			shiftSlots[id] = append(shiftSlots[id], shift)
		}
	}

	refFor := func(cell *wiop.Cell) (PcsClaimRef, error) {
		cv, ok := cellView[cell.Context.ID]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf(
				"codegen: BuildPcsSystem: vanishing claim cell %q has no LagrangeEval opening — "+
					"the column it opens is not PCS-authenticated", cell.Context.Path())
		}
		id := cv.Column.Context.ID
		shift := shiftFor(cv)
		decl, ok := colDeclByID[id]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: opened column %q has no declaration index", cv.Column.Context.Path())
		}
		slot := -1
		for i, s := range shiftSlots[id] {
			if s == shift {
				slot = i
				break
			}
		}
		if slot < 0 {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: shift %d not found for column %q", shift, cv.Column.Context.Path())
		}
		return PcsClaimRef{ColDeclIdx: decl, Shift: slot}, nil
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
