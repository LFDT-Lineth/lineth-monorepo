package codegen

import (
	"fmt"
	"math/bits"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// PcsSystem is the compile-time FRI/PCS description the Zig verifier consumes:
// the FRI envelope params, the SYMBOLIC committed-column descriptors (Columns)
// the verifier reconstructs its canonical layout from at verify time, the claim
// maps that re-slice the PCS-authenticated entry_claims into the vanishing
// witness/quotient claims, and the flat all_coins index of the shared opening
// point zeta.
//
// It is extracted from an ALREADY-compiled (global.Compile + pcs.Compile)
// protocol, so it can never drift from the prover's committed column ordering:
// batch order, per-batch layout, and the LagrangeEval openings all come from
// the prover-ray PCS compiler's own exported helpers. The proof-specific
// claimed evaluations are extracted separately by ExtractPcsOpening.
//
// Columns carries only the size-independent invariants (batch, base/ext, raw
// shift schedule); the Zig engine (`src/query/pcs.zig`) reconstructs the
// size-dependent bundle placement, entry order, and restricted FRI params from
// Columns + the proof's own runtime `module_sizes`, so one baked System
// verifies proofs of different dynamic sizes.
type PcsSystem struct {
	SourceName string

	// FRI ENVELOPE params (the prover-ray static maxCommittableSizeLog2
	// schedule, NOT restricted to any one proof): LogPlaintextSize == FRIMaxCommittableSizeLog2,
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
	// these + the proof's runtime module_sizes.
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
}

// PcsColumnDesc is one committed column in prover DECLARATION order (the engine's
// `pcs.ColumnDesc`). Its Size is either a static comptime size_log2 (a
// static-module column) or a DynamicIndex into module_sizes (a dynamic-module
// column, whose size varies per proof). Dynamic columns also carry the minimum
// runtime size_log2 the raw shift schedule is valid for. The verifier
// reconstructs each column's bundle / position / entry index from these +
// module_sizes.
type PcsColumnDesc struct {
	BatchIdx int
	IsExt    bool
	// IsDynamic selects the Size source: when true, DynamicIndex names the
	// module_sizes slot and DynamicMinSizeLog2 is the smallest runtime size_log2
	// this raw shift schedule is valid for; when false, SizeLog2 is the fixed
	// static size.
	IsDynamic          bool
	SizeLog2           int
	DynamicIndex       int
	DynamicMinSizeLog2 int
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

// pcsColumnDeclIndex maps each committed column of sys to its prover
// DECLARATION index (batch-major, then round.Columns order) — the index of the
// matching PcsSystem.Columns entry.
func pcsColumnDeclIndex(sys *wiop.System) map[wiop.ObjectID]int {
	idx := map[wiop.ObjectID]int{}
	for _, b := range pcscompiler.CommittedBatches(sys) {
		for _, col := range b.Round.Columns {
			idx[col.Context.ID] = len(idx)
		}
	}
	return idx
}

// pcsShiftFor computes the shift-schedule key for one opening of a column
// view cv:
//   - STATIC column: its size never changes, so the schedule stores the
//     NORMALIZED shift ((offset % size) + size) % size. This matches
//     prover-ray's own per-proof dedup (two raw offsets that alias mod the
//     fixed size ARE the same opening — e.g. -1 and 3 both -> 3 at size 4), so
//     the point set and entry_claims line up exactly.
//   - DYNAMIC column: the size varies per proof, so the schedule stores the
//     RAW offset and lets the verifier normalize it mod the RUNTIME size.
//     omega_N^(offset mod N) == omega_N^offset, so the reconstructed point
//     matches the prover's at every size.
func pcsShiftFor(cv *wiop.ColumnView) (isDynamic bool, shift int) {
	if cv.Column.Module.IsDynamic() {
		return true, cv.ShiftingOffset
	}
	size := cv.Column.Module.Size()
	return false, ((cv.ShiftingOffset % size) + size) % size
}

// pcsShiftSlots returns, for each committed column opened by a LagrangeEval,
// its distinct opened shifts in first-seen declaration order (the order
// sys.LagrangeEvals is walked in; see pcsShiftFor for the static/dynamic
// distinction). ShiftingOffset is fixed when a constraint declares its column
// view, so this is a property of sys alone — identical for every proof of sys.
func pcsShiftSlots(sys *wiop.System) map[wiop.ObjectID][]int {
	slots := map[wiop.ObjectID][]int{}
	seen := map[wiop.ObjectID]map[int]bool{}
	for _, le := range sys.LagrangeEvals {
		for _, cv := range le.Polynomials {
			id := cv.Column.Context.ID
			_, shift := pcsShiftFor(cv)
			if seen[id] == nil {
				seen[id] = map[int]bool{}
			}
			if seen[id][shift] {
				continue
			}
			seen[id][shift] = true
			slots[id] = append(slots[id], shift)
		}
	}
	return slots
}

// pcsDynamicMinSizeLog2 returns the smallest runtime size_log2 a dynamic
// column's RAW shift schedule is valid for. If two distinct offsets differ by a
// multiple of 2^s, they alias at every size <= 2^s, so the minimum safe size is
// one bit larger than the largest shared power-of-two divisor across all offset
// pairs. Size 1 (s=0) is included because the verifier currently accepts
// module_sizes[idx] == 1: on that one-row domain the prover deduplicates every
// raw shift to the same opening, so a baked schedule with multiple distinct raw
// shifts would still diverge from an honest proof there.
//
// This must be checked over the FULL size range, not just the size any one
// proof happens to use: shifts that are distinct at one size can collide at a
// smaller one the SAME module is free to take in another proof of this sys.
// Aliasing is not merely a completeness gap — reconstructQueryValueAt's
// DEEP-quotient sum walks every shift in a column's schedule and adds one term
// per shift; if two raw shifts alias mod the runtime size, shiftedPoint(size_log2,
// shift, zeta) is IDENTICAL for both (since omega_N^a == omega_N^b whenever a ==
// b mod N), so the sum double-counts that term. prover-ray's own pipeline never
// observes this: it dedups by normalized shift before opening
// (recoverBatchClaims), producing exactly one claim, so the two claims baked
// here can never both be supplied correctly by an honest prover at an aliasing
// size — there is no valid completion, only rejection (see ExtractPcsOpening,
// which enforces this bound against each specific proof's actual size).
func pcsDynamicMinSizeLog2(colPath string, shifts []int, maxSizeLog2 int) (int, error) {
	minSizeLog2 := 0
	var (
		worstA int
		worstB int
	)
	for i := 0; i < len(shifts); i++ {
		for j := i + 1; j < len(shifts); j++ {
			a, b := shifts[i], shifts[j]
			if a == b {
				continue // exact duplicates are deduplicated earlier, not aliases
			}
			diff := a - b
			if diff < 0 {
				diff = -diff
			}
			required := bits.TrailingZeros(uint(diff)) + 1
			if required > minSizeLog2 {
				minSizeLog2 = required
				worstA, worstB = a, b
			}
		}
	}
	if minSizeLog2 > maxSizeLog2 {
		return 0, fmt.Errorf(
			"codegen: BuildPcsSystem: dynamic column %q opens at offsets %d and %d that require "+
				"runtime size >= %d, but the verifier envelope only supports sizes up to %d; no "+
				"supported dynamic size can represent this raw shift schedule",
			colPath, worstA, worstB, 1<<minSizeLog2, 1<<maxSizeLog2)
	}
	return minSizeLog2, nil
}

// BuildPcsSystem extracts the FRI/PCS System from a compiled protocol.
//
// Requires pcs.Compile to have run after global.Compile: the LagrangeEval
// openings (witness views + quotient shares) that the claim maps re-slice are
// registered by global.Compile, and pcs.Compile commits the batches and
// registers the opening actions. It reads only the committed batches and the
// LagrangeEvals — nothing runtime- or scenario-specific — so a single System
// covers every proof of sys, whatever their dynamic-module sizes. `routing` is
// the shared coin layout from BuildCoinRouting; it locates the zeta coin in
// the flat all_coins array.
func BuildPcsSystem(sys *wiop.System, routing CoinRouting) (PcsSystem, error) {
	batches := pcscompiler.CommittedBatches(sys)
	if len(batches) == 0 {
		return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no committed batches; did pcs.Compile run?")
	}
	if len(sys.LagrangeEvals) == 0 {
		return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: no LagrangeEval openings; did global.Compile run before pcs.Compile?")
	}

	// Each batch's root provenance. An interactive batch's root is the oracle
	// commitment absorbed for its round (proof.rounds index == Round.ID, since
	// rounds are emitted in ID order); the precomputed batch's root is the
	// compile-time PrecomputedCommitment. This is what lets the Zig verifier bind
	// the authenticated root to the transcript instead of trusting the proof.
	batchRoots := make([]PcsBatchRoot, len(batches))
	for i, b := range batches {
		if b.IsPrecomp {
			batchRoots[i] = PcsBatchRoot{Precomputed: true, Root: sys.PrecomputedCommitment}
		} else {
			batchRoots[i] = PcsBatchRoot{Precomputed: false, RoundIndex: b.Round.ID}
		}
	}

	// Envelope params: the prover's process-wide static FRI schedule. The Zig
	// verifier restricts these to each proof's largest opened size, so ONE baked
	// System covers every dynamic size. Emitted from the prover's own exported
	// envelope so it can never drift from what the prover commits with.
	envelope := pcscompiler.FRIStaticParams()
	maxSizeLog2 := int(pcscompiler.FRIMaxCommittableSizeLog2())

	shiftSlots := pcsShiftSlots(sys)
	dynIdx := DynamicModuleIndex(sys)
	colDeclByID := pcsColumnDeclIndex(sys)

	// Columns: every committed column in prover DECLARATION order (batch-major,
	// then round.Columns order). Each column carries its batch, is_ext, size
	// source (static size_log2 from the module's fixed size, or a DynamicIndex
	// into module_sizes for a dynamic module) and its size-independent shift
	// schedule. colDeclByID (built above) maps a column ObjectID to its
	// declaration index so the claim maps can reference a column instead of a
	// size-frozen entry index.
	var columns []PcsColumnDesc
	for i, b := range batches {
		for _, col := range b.Round.Columns {
			desc := PcsColumnDesc{
				BatchIdx: i,
				IsExt:    col.IsExtension,
				Shifts:   append([]int(nil), shiftSlots[col.Context.ID]...),
			}
			if col.Module.IsDynamic() {
				idx, ok := dynIdx[col.Module]
				if !ok {
					return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: dynamic module %q of committed column %q has no module_sizes index",
						col.Module.Context.Path(), col.Context.Path())
				}
				desc.IsDynamic = true
				desc.DynamicIndex = idx
				// See pcsDynamicMinSizeLog2: a dynamic column's raw shift schedule is
				// valid only above some minimum runtime size.
				minSizeLog2, err := pcsDynamicMinSizeLog2(col.Context.Path(), desc.Shifts, maxSizeLog2)
				if err != nil {
					return PcsSystem{}, err
				}
				desc.DynamicMinSizeLog2 = minSizeLog2
			} else {
				// Static column: size_log2 is the padded, fixed module size. SetSize
				// already rounds it up to a power of two, so log2 is exact.
				if !col.Module.IsSized() {
					return PcsSystem{}, fmt.Errorf("codegen: BuildPcsSystem: static module %q of committed column %q has no size set",
						col.Module.Context.Path(), col.Context.Path())
				}
				desc.SizeLog2 = bits.Len(uint(col.Module.Size())) - 1
			}
			columns = append(columns, desc)
		}
	}

	witnessMap, quotientMap, err := buildPcsClaimMaps(sys, colDeclByID, shiftSlots)
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
	}, nil
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
// column's shift schedule (shiftSlots, the same schedule PcsColumnDesc.Shifts
// carries), so a routed claim lands on the exact authenticated value.
func buildPcsClaimMaps(
	sys *wiop.System,
	colDeclByID map[wiop.ObjectID]int,
	shiftSlots map[wiop.ObjectID][]int,
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
		id := cv.Column.Context.ID
		_, shift := pcsShiftFor(cv)
		decl, ok := colDeclByID[id]
		if !ok {
			return PcsClaimRef{}, fmt.Errorf("codegen: BuildPcsSystem: opened column %q has no declaration index", cv.Column.Context.Path())
		}
		slot := slices.Index(shiftSlots[id], shift)
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
