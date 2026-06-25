// Package fri's PCS layer wraps the existing multi-degree FRI primitives
// (Commit / RSEncoder / Tree / ProverState / Proof) into a batch
// polynomial-commitment scheme with an Open/Verify surface.
//
// This file is a DESIGN PROPOSAL — types and signatures only. Bodies
// panic with "TODO(pcs)". Review the layout, the OpeningProof shape,
// and the open questions at the bottom of this doc before any
// implementation lands.
//
// =============================================================================
// Design overview
// =============================================================================
//
// Fiat-Shamir is the caller's responsibility, matching the convention
// already established by [fri.ProverState] and [Verify]: every PCS
// method that "needs a challenge" takes that challenge as an explicit
// parameter. The PCS never reaches into a transcript.
//
// The PCS speaks the same data shapes as the underlying FRI primitives:
//   - One Batch == one MultiSizeTable. A batch's polynomials are
//     committed via [Commit] into a single CommitterState (Merkle tree
//     over the multi-size aux-leaf structure).
//   - The verifier sees only Shape (per-size row counts) for each
//     batch, since it doesn't hold the witness data.
//   - Shifts describe which rotation shifts each row must be opened
//     at; the canonical layout enumerates (size desc, shift asc, batch
//     decl order, base-then-ext, row decl order).
//
// At Open time the prover:
//
//  1. Computes the claimed value of every (batch, size, row, shift) at
//     zeta * omega_N^shift.
//  2. Caller absorbs the claimed values into its transcript, derives
//     alpha_DEEP, hands it back.
//  3. PCS builds one DEEP-quotient codeword per distinct native size,
//     commits each as a FRI level. Returns the per-size roots.
//  4. Caller absorbs the DEEP roots, derives the first FRI fold
//     challenge alpha_0, hands it back.
//  5. PCS folds, returns the new layer's root; caller derives
//     alpha_{j+1} from it; repeat.
//  6. After the last fold, PCS reveals the final polynomial; caller
//     absorbs it and derives the query positions.
//  7. PCS opens every batch at every query position and produces the
//     final OpeningProof.
//
// Verify mirrors steps 2-7: same challenges in, validates the bridge
// between original commitments and the FRI proof on the DEEP-quotient
// codewords.
//
// Two API styles are provided. Pick whichever fits the caller better;
// both produce identical OpeningProofs.
//
//   - One-shot:   pcs.Open(in OpenInputs) (OpeningProof, error)
//     pcs.Verify(in VerifyInputs, proof OpeningProof) error
//
//     For callers that have all challenges + query positions ready
//     up-front (precomputed transcript, tests, integration recipes).
//
//   - State-machine: NewOpenerState(...) ->
//     ComputeClaimedValues(zeta) ->
//     CommitDeepQuotient(alphaDEEP) ->
//     Fold(alpha_j)        // numRounds times
//     Open(queryPositions)
//
//     Mirrors [fri.ProverState]'s coin-fed pattern: caller binds each
//     returned root to its transcript and derives the next challenge.
//     Both styles share the same internal implementation; the one-shot
//     variant is a thin wrapper around the state machine that consumes
//     pre-sampled challenges in order.
//
// =============================================================================
// Canonical layout (frozen)
// =============================================================================
//
// For each native size N == 2^sizeLog2 in DESCENDING order, within each
// size:
//
//	for shift s in ASCENDING order:
//	  for batch b in DECLARATION order:
//	    for the size-N SizedTable in batch b (skip if absent):
//	      for row r in g.Base then g.Ext (declaration order):
//	        if s appears in shifts[b][sizeLog2].Base|Ext[r]:
//	          emit a deepEntry; consume the next alpha_DEEP power.
//
// The alpha_DEEP power counter resets to 0 at each new size and is
// monotonic across all shifts within the size.
//
// Identical convention to the loom PCS at github.com/consensys/loom/
// internal/fri/. Decision matrix already pinned there:
//   - (i)   per-size reset.
//   - (ii)  per-(N, s) bundle batching (one accumulation per shift,
//     shift ascending, no reset inside a size).
//   - (iii) empty shift list is an error (every committed row is
//     opened at least once). OPEN QUESTION 1 below.
//   - (iv)  duplicate shifts inside a row's shift list is an error.
//   - (v)   no cross-batch dedup; caller is responsible.
//   - (vi)  caller picks batch order. Convention: setup batches at
//     the front, AIR-quotient batch at the back, witness rounds
//     in between -- though the PCS itself doesn't care, only
//     that prover and verifier agree on the order.
//
// =============================================================================
// Open questions for review
// =============================================================================
//
//  1. Empty shift lists. Loom rejects them; should this PCS too? An
//     empty shift list means "committed but not opened" -- the row's
//     value is still authenticated by the Merkle path, but it doesn't
//     contribute to the DEEP quotient. Allowing this is more flexible
//     but adds a dead-code-detection failure mode (typos in the shift
//     schedule become silent commitments). Default proposal: REJECT
//     empty shift lists, matching loom.
//
//  2. Multi-size conjugate-pair openings. The underlying [Tree] is
//     multi-size (smaller-size rows live in AuxSiblings), but
//     [Branch] opens only ONE position per level: the leaf + deepest
//     sibling form a conjugate pair AT THE BOTTOM SIZE only. The
//     conjugate at a smaller size N_small lives at a different aux
//     octuplet -- specifically, the sibling at aux-level for size
//     N_small -- which the current Branch does NOT include.
//
//     The DEEP bridge needs DQ_{N_small}(x) AND DQ_{N_small}(-x) at
//     every query position to verify the FRI level leaf at that size.
//     So an opening must surface BOTH conjugate values at every
//     present size.
//
//     Three concrete options to choose from:
//     (a) Open TWO branches per query per batch (one at position s,
//     one at s ^ 1 ... but that gives bottom-conjugate only).
//     Actually the right shape is "open the bottom and the
//     full-conjugate path" -- needs sketching.
//     (b) Extend the Branch structure to carry, at each aux level,
//     BOTH the path-aligned aux AND its sibling aux. Doubles
//     per-level aux data but keeps one Branch per (query, batch).
//     (c) Restrict batches to single-size only (each batch has a
//     single populated SizedTable). This degenerates to loom's
//     shape today; the multi-size tree's aux-leaf machinery is
//     unused.
//
//     Default proposal: option (b) -- a new "PairedBranch" type that
//     pairs the existing Branch with sibling-aux octuplets at every
//     level, plus raw row data at both s and s^1 for the bottom and at
//     both (s>>d) and (s>>d)^1 for each aux level. WMerkleOpening
//     below reflects this. Worth re-validating on cost vs option (c).
//
//  3. State-machine vs one-shot API. Both proposed; concrete decision
//     point for the team is: which one ships in the v1 implementation,
//     or do both ship simultaneously? Default proposal: ship the state-
//     machine variant (mirrors the existing fri.ProverState pattern);
//     add the one-shot wrapper if a caller asks for it.
//
//  4. Where does the canonical-name -> (batchIdx, sizeLog2, rowIdx, isExt)
//     mapping live? Caller-side, per the precedent set by loom (the PCS
//     doesn't know column names). Worth documenting an example caller
//     that builds this mapping at compile time.
//
//  5. Encoders + Params relationship. [NewPCS] currently takes both. We
//     could derive one set of encoders from Params (since Params knows
//     rate = N / D and the size schedule), but encoders also carry FFT
//     domains which Params already precomputes. Default proposal: take
//     both; document that pcs.Encoders[i] must have PlainTextSize =
//     2^i and inverse rate == pcs.Params.N / pcs.Params.D.
package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// =============================================================================
// PCS construction
// =============================================================================

// PCS bundles the FRI configuration and the per-size encoders into one
// receiver for Commit / Open / Verify. Built once at startup, reused
// across many proofs.
//
// Invariants (enforced by NewPCS):
//   - len(Encoders) == Params.numRounds + 1 (one encoder per size in
//     the multi-size schedule, sizes 2^0 .. 2^numRounds).
//   - Encoders[i].PlainTextSize == 1 << i.
//   - All Encoders share the same inverse rate, equal to Params.N /
//     Params.D.
type PCS struct {
	Params   Params
	Encoders []*RSEncoder
}

// NewPCS validates the encoder schedule against Params and returns a
// ready-to-use PCS. Wraps [assertValidMultiEncoder] for the
// cross-encoder consistency check.
func NewPCS(params Params, encoders []*RSEncoder) (*PCS, error) { //nolint:revive // design stub
	panic("TODO(pcs): NewPCS")
}

// =============================================================================
// Batch / Shape / Shifts -- caller-facing types
// =============================================================================

// Batch is one batch of polynomials committed via a single [Commit]
// call. It's an alias for MultiSizeTable so callers see the same
// witness shape they already build for commitment.
type Batch = MultiSizeTable

// Shape describes one Batch's per-size row counts WITHOUT the
// polynomial values -- the verifier-side input, since the verifier
// holds only roots and not the witness data. Indexed parallel to a
// MultiSizeTable: Shape[i] applies to size_log2 = i.
type Shape []SizedShape

// SizedShape carries the row counts of one SizedTable in a Batch. A
// SizedTable is "present" iff BaseWidth + ExtWidth > 0; otherwise that
// size_log2 index is empty for this batch.
type SizedShape struct {
	BaseWidth int
	ExtWidth  int
}

// BatchShifts describes which rotation shifts each row of each
// SizedTable in a Batch must be opened at. Indexed parallel to the
// Batch / Shape: BatchShifts[i] applies to size_log2 = i.
//
// A shift s is the integer such that the row is opened at zeta *
// omega_N^s, where omega_N is the generator of the size-N = 2^i
// domain. Shift lists must be non-empty (open question 1) and contain
// no duplicates.
type BatchShifts []SizedShifts

// SizedShifts is the per-row shift schedule for one SizedTable. The
// shape MUST align with the matching SizedTable / SizedShape (Base
// width × Ext width).
type SizedShifts struct {
	Base [][]int
	Ext  [][]int
}

// =============================================================================
// Layout -- internal canonical enumeration
// =============================================================================
//
// Mirrors loom's canonicalLayout. Producer of the alpha_DEEP power
// schedule consumed by both Open and Verify. Made package-internal;
// callers don't need to look inside.
type deepEntry struct { //nolint:unused // design stub
	BatchIdx int
	SizeLog2 int
	RowIdx   int
	IsExt    bool
}

type shiftBundle struct { //nolint:unused // design stub
	Shift   int
	Entries []deepEntry
}

type sizeBundle struct { //nolint:unused // design stub
	SizeLog2 int
	Bundles  []shiftBundle
}

type layout []sizeBundle //nolint:unused // design stub

// canonicalLayout walks shapes + shifts and produces the canonical
// enumeration. Validates shape alignment, per-row shift invariants
// (non-empty, no duplicates), and per-batch distinct sizes.
//
// Used by both Open (with shapes derived from witnesses) and Verify
// (with shapes passed in directly).
func canonicalLayout(shapes []Shape, shifts []BatchShifts) (layout, error) { //nolint:revive,unused // design stub
	panic("TODO(pcs): canonicalLayout")
}

// canonicalLayoutFromBatches is the prover-side entry point: shapes
// are inferred from witness row counts. Delegates to canonicalLayout.
func canonicalLayoutFromBatches(batches []Batch, shifts []BatchShifts) (layout, error) { //nolint:revive,unused
	panic("TODO(pcs): canonicalLayoutFromBatches")
}

// =============================================================================
// OpeningProof
// =============================================================================

// OpeningProof bundles everything Verify needs to check that every
// polynomial in every committed Batch evaluates to the listed values
// at zeta and at the rotation shifts in BatchShifts.
type OpeningProof struct {
	// DeepQuotientRoots: one Merkle root per distinct native size in
	// DESCENDING size order. The largest size becomes FRI level 0;
	// smaller sizes enter as multi-degree FRI levels at the round
	// where the running polynomial reaches their degree.
	DeepQuotientRoots []field.Octuplet

	// FRIProof is the underlying multi-degree FRI proof. Already
	// verifiable on its own (via [Verify]) under the same fold
	// challenges and query positions the PCS used.
	FRIProof Proof

	// PointSamplings[q][b] opens batch b's commitment tree at FRI
	// query q. Each WMerkleOpening carries raw row data at the
	// conjugate-pair positions for every present size in the batch,
	// plus the (extended) merkle path. See WMerkleOpening's doc and
	// Open Question 2 for the multi-size conjugate-pair concern.
	PointSamplings [][]WMerkleOpening
}

// BatchClaimedValues is one Batch's per-size claimed evaluations,
// indexed parallel to the matching BatchShifts.
type BatchClaimedValues []SizedClaimedValues

// SizedClaimedValues holds claimed evaluations for one SizedTable.
// Base[k][m] == row_k(zeta * omega_N^shifts.Base[k][m]) where N =
// 2^sizeLog2. Same for Ext[k][m].
type SizedClaimedValues struct {
	Base [][]field.Ext
	Ext  [][]field.Ext
}

// WMerkleOpening opens one Batch's commitment tree at a single FRI
// query position. The structure surfaces conjugate-pair raw row data
// at EVERY present size in the batch, so the verifier can recompute
// DQ_N(x) and DQ_N(-x) at the query for every size N participating in
// the multi-degree FRI.
//
// See Open Question 2 above; the shape here corresponds to the
// proposed "option (b)" -- an extended Branch carrying sibling-aux
// octuplets at every aux level. The exact field layout below is
// subject to refinement once option (b) vs (c) is decided.
type WMerkleOpening struct {
	// ConjugatePairsBySize[i] is the conjugate-pair row data at
	// size_log2 = i for this query position. Empty for absent sizes.
	// At the bottom (largest size), the pair is the leaf + deepest
	// sibling of the standard Branch. At each smaller size, the pair
	// is constructed from the path-aligned aux and the sibling aux at
	// that level (the sibling aux is NOT in the current Branch and is
	// added here -- see PairedBranch below).
	ConjugatePairsBySize []SizedConjugatePair

	// Path is the extended multi-size Merkle branch. See PairedBranch.
	Path PairedBranch
}

// SizedConjugatePair holds the conjugate-pair row values at one size
// for one query: {row(omega^j), row(omega^(j ^ 1))} where j is the
// folded query position at this size's tree level.
type SizedConjugatePair struct {
	Base [][2]field.Element
	Ext  [][2]field.Ext
}

// PairedBranch is an extended [Branch] that carries enough material
// to authenticate the conjugate row data at every present size, not
// just at the bottom. Concretely, it differs from a plain Branch by
// adding one extra octuplet per aux level -- the sibling aux at that
// level -- so the verifier can recompute the aux hash of the
// conjugate row data at every aux level.
//
// Layout (subject to refinement during implementation; this is the
// proposed shape):
//
//   - Leaf      : the path-aligned bottom-level octuplet.
//   - SibleafQ  : the conjugate bottom-level octuplet (= Branch.Siblings[last]).
//   - Siblings  : the chain of node-level siblings above the bottom
//     (= Branch.Siblings[:len-1]).
//   - AuxPathOctuplets[level]    : the path-aligned aux at this level
//     (= Branch.AuxSiblings[level]).
//   - AuxSiblingOctuplets[level] : the SIBLING aux at this level (new).
//
// Either nil if no aux at that level (no SizedTable of that size).
//
// The verifier first checks the standard Branch (Leaf + Siblings +
// AuxPathOctuplets, recovers the root). Then for each aux level with
// a present sized table, it independently hashes the conjugate raw
// row data and confirms it matches AuxSiblingOctuplets[level], which
// it does NOT need to chain into the root recovery (it's a leaf
// authentication, not a path authentication).
//
// (Caveat: this sketch is what we believe is sound; soundness
// analysis must validate that no path-injection attack is possible
// against AuxSiblingOctuplets. The conjugate aux is collision-free
// because the bottom-level Branch already binds the parent node which
// committed to BOTH children's aux via the 3-ary hashNode.)
type PairedBranch struct {
	Leaf                Branch // standard path-aligned branch
	AuxSiblingOctuplets []*field.Octuplet
}

// =============================================================================
// One-shot API
// =============================================================================
//
// For callers that have all challenges + query positions ready up-front
// (tests, externally-precomputed transcripts).

// OpenInputs bundles every parameter pcs.Open needs. Listed in a
// struct so the call site is self-documenting and so future fields
// can be added without breaking existing callers.
type OpenInputs struct {
	Witnesses []Batch
	Committed []CommitterState
	Shifts    []BatchShifts

	Zeta           field.Ext
	AlphaDeep      field.Ext
	FoldAlphas     []field.Ext // length == Params.numRounds
	QueryPositions []int       // length == Params.NumQueries
}

// Open produces an OpeningProof in one call.
//
// The caller is responsible for deriving all challenges via Fiat-
// Shamir from the appropriate prefix of the transcript. The
// documented derivation order (which Verify expects mirrored on its
// side) is:
//
//	for each batch b in declaration order:
//	    fs.absorb(Committed[b].Tree.Root())
//	fs.sample(Zeta)
//	for each (b, sizeLog2, rowIdx, isExt, shift) in canonical layout order:
//	    fs.absorb(claimed value at this entry)
//	fs.sample(AlphaDeep)
//	for j in 0..numRounds-1:
//	    fs.absorb(deepRoots OR running-layer root produced by fold j-1)
//	    fs.sample(FoldAlphas[j])
//	fs.absorb(final polynomial)
//	for k in 0..NumQueries-1:
//	    fs.sample(QueryPositions[k]) // mod N/2
//
// (The "absorb running-layer root" step at j=0 is actually
// absorbing the DEEP roots, since the running polynomial AT round 0
// is the largest DEEP codeword.)
func (pcs *PCS) Open(in OpenInputs) (OpeningProof, error) { //nolint:revive // design stub
	panic("TODO(pcs): Open")
}

// VerifyInputs bundles every parameter pcs.Verify needs. Mirrors
// OpenInputs without the witnesses and with per-batch roots / shapes
// in their place.
type VerifyInputs struct {
	Roots  []field.Octuplet
	Shapes []Shape
	Shifts []BatchShifts
	// ClaimedValues[b] mirrors shifts[b] exactly. The outer protocol
	// reads these to evaluate its constraints at zeta and to bind into
	// the alpha_DEEP transcript challenge.
	ClaimedValues []BatchClaimedValues

	Zeta           field.Ext
	AlphaDeep      field.Ext
	FoldAlphas     []field.Ext
	QueryPositions []int
}

// Verify checks an OpeningProof under the same challenges and query
// positions the prover used (see Open's doc for the derivation
// order).
//
// Performs in sequence:
//
//  1. Shape validation (Roots/Shapes/Shifts/ClaimedValues alignment).
//  2. Canonical-layout build from Shapes + Shifts (validates the
//     shift schedule too).
//  3. fri.Verify on the DEEP-quotient roots with FoldAlphas + query
//     positions.
//  4. Authenticate every (query, batch) PairedBranch against the
//     batch's root, including the per-size conjugate-aux check.
//  5. The bridge: for every query and every distinct size, recompute
//     DQ_N(X) and DQ_N(-X) in canonical layout order from raw row
//     data + claimed values, compare to the FRI level leaves at that
//     query.
func (pcs *PCS) Verify(in VerifyInputs, proof OpeningProof) error { //nolint:revive // design stub
	panic("TODO(pcs): Verify")
}

// =============================================================================
// State-machine API
// =============================================================================
//
// Mirrors the existing [ProverState] pattern: each call returns a
// digest the caller binds to its transcript and uses to derive the
// next challenge fed in.

// OpenerState drives Open as a coin-fed state machine. Construct via
// NewOpenerState, then call the four methods below in sequence:
//
//	os, _ := NewOpenerState(pcs, batches, committed, shifts)
//	values := os.ComputeClaimedValues(zeta)
//	// transcript absorbs values, derives alphaDEEP
//	deepRoots := os.CommitDeepQuotient(alphaDEEP)
//	// transcript absorbs deepRoots, derives alpha_0
//	for os.HasNext() {
//	    root := os.Fold(alpha_j)
//	    // transcript absorbs root, derives alpha_{j+1}
//	}
//	// transcript absorbs the final polynomial (read from os.FinalPoly())
//	// derives the query positions
//	proof := os.Open(queryPositions)
//
// Implementations of Open (one-shot) and Verify can be thin wrappers
// around this loop with pre-sampled challenges; that is the recommended
// internal layout. State-machine and one-shot variants share one body.
type OpenerState struct {
	// Implementation TBD. Holds the in-progress proof, the inner
	// [ProverState] for FRI, the layout, claimed values, and the
	// per-batch committer data. Exposed methods below.
}

// NewOpenerState validates inputs and seeds the state machine.
//
// Validates:
//   - batches and committed have matching length (each
//     committed[b] must be the result of Commit(pcs.Encoders, batches[b])).
//   - shifts mirrors batches/shapes.
//   - canonical layout produces a non-empty enumeration (open
//     question 1 may relax this).
func NewOpenerState(
	pcs *PCS, //nolint:revive // design stub
	committed []*CommitterState, //nolint:revive // design stub
	shifts []BatchShifts, //nolint:revive // design stub
	zeta field.Ext,
	claims []BatchClaimedValues,
) (*OpenerState, error) {
	panic("TODO(pcs): NewOpenerState")
}

// CommitDeepQuotient consumes the DEEP batching challenge and:
//
//  1. Builds the per-size DEEP-quotient codewords by accumulating, in
//     canonical layout order, alpha_DEEP-weighted (v - C(X)) /
//     (z_s - X) terms on the size-N Lagrange domain.
//  2. Encodes each codeword to size rate*N via the matching encoder.
//  3. Commits each codeword as a FRI level (a paired-leaf Merkle
//     tree built via buildTreeExt).
//  4. Returns the per-size roots in DESCENDING native-size order
//     (largest first), ready for the caller to bind and to use as
//     levelRoots in fri.Verify on the verifier side.
//
// The state machine transitions to the FRI commit phase: subsequent
// calls to Fold expect the FRI fold challenges in order.
func (os *OpenerState) CommitDeepQuotient(alphaDEEP field.Ext) []field.Octuplet { //nolint:revive // design stub
	panic("TODO(pcs): OpenerState.CommitDeepQuotient")
}

// HasNext reports whether another FRI fold challenge is expected.
func (os *OpenerState) HasNext() bool {
	panic("TODO(pcs): OpenerState.HasNext")
}

// Fold consumes one FRI fold challenge, folds the current running
// polynomial into the next layer (mixing in any multi-degree level
// scheduled at this round, batched with alpha^2 per the existing
// foldLayerInternally), and returns the new layer's Merkle root.
//
// On the final fold, the running polynomial collapses to the final
// polynomial (read via FinalPoly) and the returned octuplet is the
// zero-octuplet sentinel.
//
// Wraps the inner [fri.ProverState.Fold].
func (os *OpenerState) Fold(alpha field.Ext) field.Octuplet { //nolint:revive // design stub
	panic("TODO(pcs): OpenerState.Fold")
}

// FinalPoly returns the FRI final polynomial after all folds are
// done. Used by the caller to bind into the transcript before
// deriving the query positions.
func (os *OpenerState) FinalPoly() []field.Ext {
	panic("TODO(pcs): OpenerState.FinalPoly")
}

// Open consumes the FRI query positions and assembles the final
// OpeningProof. Must be called after all folds are done.
//
// For each (query, batch), the method opens the batch's commitment
// tree at the folded query position via PairedBranch, then packages
// the raw conjugate-pair row data at every present size in the batch.
func (os *OpenerState) Open(queryPositions []int) OpeningProof { //nolint:revive // design stub
	panic("TODO(pcs): OpenerState.Open")
}

// =============================================================================
// Verifier-side helpers
// =============================================================================
//
// A verifier-side VerifierState mirroring OpenerState is an option but
// is NOT proposed for v1. Reason: verification is more linear than
// proving (no Merkle commitments to produce mid-stream, just checks),
// so the one-shot pcs.Verify is sufficient. If a use case emerges for
// a coin-fed verifier (e.g. incremental verification), add it then.

// =============================================================================
// What's left untouched
// =============================================================================
//
// - [Commit], [MultiSizeTable], [SizedTable], [CommitterState],
//   [Tree], [Branch], [Params], [RSEncoder], [Proof], [ProverState]
//   are all reused as-is. The PCS layer doesn't replace any of them;
//   it sits on top.
//
// - [Verify] (the package-level FRI Verify) remains the multi-degree
//   FRI verifier and is called from pcs.Verify as one of its steps.
//
// - The transcript is the caller's. We never import a Fiat-Shamir
//   package from this file.
