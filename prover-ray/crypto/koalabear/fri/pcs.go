// Package fri's PCS layer wraps the existing multi-degree FRI primitives
// (Commit / RSEncoder / Tree / ProverState / Proof) into a batch
// polynomial-commitment scheme.
//
// This file defines the PCS-facing types and the canonical column layout.
//
// =============================================================================
// Design overview
// =============================================================================
//
// Fiat-Shamir is the caller's responsibility, matching the convention
// already established by [fri.ProverState]: every PCS method that "needs a
// challenge" takes that challenge as an explicit parameter. The PCS never
// reaches into a transcript.
//
// The PCS speaks the same data shapes as the underlying FRI primitives:
//   - One Batch == one MultiSizeTable. A batch's polynomials are
//     committed via [Commit] into a single CommitterState (Merkle tree
//     over the multi-size aux-leaf structure).
//   - The verifier sees only Shape (per-size row counts) for each
//     batch, since it doesn't hold the witness data.
//   - Shifts describe which rotation shifts each row must be opened at; the
//     canonical layout enumerates columns as (size desc, batch declaration order,
//     base-then-ext, row declaration order). All shifts of one column share the
//     same alpha_DEEP power.
//
// During the opening flow the prover:
//
//  1. Caller computes the claimed value of every (batch, size, row, shift) at
//     zeta * omega_N^shift and hands them to AddOpening. zeta is shared by
//     every batch of a single opening proof.
//  2. Caller absorbs the claimed values into its transcript.
//  3. PCS gathers, per distinct native size, the per-column DEEP-quotient data
//     and seeds the existing FRI ProverState with those levels. There is no
//     separate alpha_DEEP squeeze: each level's own alpha_DEEP is the square
//     of that level's own introduction-round fold challenge (round 0 for the
//     main degree-D polynomial), derived inside ProverState.Fold once that
//     challenge is sampled -- see [fri.Level.EvalsAt].
//  4. Caller derives the first FRI fold challenge alpha_0.
//  5. The FRI prover folds, returns the new layer's root; caller derives
//     alpha_{j+1} from it; repeat.
//  6. After the last fold, PCS reveals the final polynomial; caller
//     absorbs it and derives the query positions.
//  7. PCS opens every batch at every query position and produces the
//     final OpeningProof.
//
// Verify mirrors steps 2-7: same zeta and challenges in, authenticates the
// opened backing trees, and reconstructs the virtual quotients inside FRI.
//
// The prover-side staged API returns the existing [ProverState] rather than
// introduce a second PCS-specific opener state machine.
//
// Canonical prover call sequence:
//
//	pcs := NewPCS(params, encoders)
//
//	// Commit each batch; absorb each root into the transcript.
//	state0 := pcs.Commit(witness0)
//	transcript.Absorb(state0.Root())
//	state1 := pcs.Commit(witness1)
//	transcript.Absorb(state1.Root())
//
//	// Squeeze the single opening point shared by every batch. For each batch,
//	// compute its claimed evaluations, absorb them, and register the opening.
//	zeta       := transcript.Squeeze()
//	claims0    := computeClaims(witness0, shifts0, zeta) // caller-side evaluation
//	transcript.Absorb(claims0)
//	pcs.AddOpening(state0, zeta, shifts0, claims0)
//	claims1    := computeClaims(witness1, shifts1, zeta)
//	transcript.Absorb(claims1)
//	pcs.AddOpening(state1, zeta, shifts1, claims1)
//
//	// Seed the FRI prover: no alpha_DEEP squeeze, each level derives its own
//	// from its own introduction round's fold challenge.
//	friState, _ := pcs.NewProverState()
//
//	// FRI folding rounds: fold, absorb the new layer root, squeeze the next alpha.
//	for range numFoldingRounds {
//	    alpha := transcript.Squeeze()
//	    root  := friState.Fold(alpha) // zero octuplet on the final round
//	    transcript.Absorb(root)
//	}
//
//	// Absorb the final polynomial; squeeze query positions; open.
//	transcript.Absorb(friState.FinalPoly)
//	queries    := transcript.Squeeze()
//	proof      := pcs.Open(friState, queries)
//
// The verifier calls pcs.Verify with the same zeta, fold alphas, and query
// positions it derived from its own transcript replay.
//
// =============================================================================
// Canonical layout (frozen)
// =============================================================================
//
// For each native size N == 2^sizeLog2 in DESCENDING order, within each
// size:
//
//	for batch b in DECLARATION order:
//	  for the size-N SizedTable in batch b (skip if absent):
//	    for row r in g.Base then g.Ext (declaration order):
//	      emit a deepEntry; consume one alpha_DEEP power for the column.
//
// The alpha_DEEP power counter resets to 0 at each new size. All shifts on a
// column are carried by its one deepEntry and share that alpha_DEEP power.
//
// Decision matrix:
//   - (i)   per-size reset.
//   - (ii)  per-column batching, all shifts of a column sharing one alpha_DEEP power.
//   - (iii) empty shift list is an error (every committed row is
//     opened at least once).
//   - (iv)  duplicate shifts inside a row's shift list is an error.
//   - (v)   no cross-batch dedup; caller is responsible.
//   - (vi)  caller picks batch order. Convention: setup batches at
//     the front, AIR-quotient batch at the back, witness rounds
//     in between -- though the PCS itself doesn't care, only
//     that prover and verifier agree on the order.
package fri

import (
	"fmt"
	"math/big"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// =============================================================================
// PCS construction
// =============================================================================

// PCS bundles the FRI configuration and the per-size encoders into one
// receiver for Commit / AddOpening / Verify. Built once at startup, reused
// across many proofs.
//
// Invariants (enforced by NewPCS):
//   - len(Encoders) == Params.LogPlainTextSize + 1 (one encoder per size
//     in the multi-size schedule, sizes 2^0 .. 2^Params.LogPlainTextSize).
//   - Encoders[i].PlainTextSize == 1 << i.
//   - All Encoders share the same inverse rate, equal to
//     2^(Params.LogCodewordSize - Params.LogPlainTextSize).
type PCS struct {
	Params   Params
	Encoders []*RSEncoder
	openings []pcsOpening
	zeta     field.Ext
}

type pcsOpening struct {
	layout    layout
	committed CommitterState
	claimed   BatchClaimedValues
}

// Reset clears pending opening registrations and the shared opening point.
func (pcs *PCS) Reset() {
	pcs.openings = pcs.openings[:0]
	pcs.zeta = field.Ext{}
}

// NewPCS validates the encoder schedule against Params and returns a
// ready-to-use PCS.
func NewPCS(params Params, encoders []*RSEncoder) (*PCS, error) {
	wantEncoders := params.LogPlainTextSize + 1
	if len(encoders) != int(wantEncoders) {
		return nil, fmt.Errorf("fri: NewPCS: got %d encoders, want %d", len(encoders), wantEncoders)
	}
	if len(encoders) == 0 {
		return nil, fmt.Errorf("fri: NewPCS: no encoders")
	}

	for i, encoder := range encoders {
		if encoder == nil {
			return nil, fmt.Errorf("fri: NewPCS: encoders[%d] is nil", i)
		}
		if encoder.Domain == nil {
			return nil, fmt.Errorf("fri: NewPCS: encoders[%d].Domain is nil", i)
		}
		if encoder.PlainTextSize != 1<<i {
			return nil, fmt.Errorf("fri: NewPCS: encoders[%d].PlainTextSize=%d, want %d",
				i, encoder.PlainTextSize, 1<<i)
		}
	}

	inverseRate := encoders[0].InverseRate()
	wantInverseRate := 1 << (params.LogCodewordSize - params.LogPlainTextSize)
	if inverseRate != wantInverseRate {
		return nil, fmt.Errorf("fri: NewPCS: inverse rate %d, want %d", inverseRate, wantInverseRate)
	}

	for i, encoder := range encoders {
		if encoder.InverseRate() != inverseRate {
			return nil, fmt.Errorf("fri: NewPCS: encoders[%d] inverse rate %d, want %d",
				i, encoder.InverseRate(), inverseRate)
		}
	}

	return &PCS{Params: params, Encoders: encoders}, nil
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
// domain. Shift lists must be non-empty and contain no duplicates.
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
// Producer of the alpha_DEEP power schedule consumed by both AddOpening and
// Verify. Made package-internal; callers don't need to look inside.
type deepEntry struct {
	BatchIdx   int
	SizeLog2   uint8
	RowIdx     int
	IsExt      bool
	AlphaPower int
	Shifts     []int
}

type sizeBundle struct {
	SizeLog2 uint8
	Entries  []deepEntry
}

type layout []sizeBundle

// canonicalLayout walks shapes + shifts and produces the canonical
// enumeration. Validates shape alignment, per-row shift invariants
// (non-empty, no duplicates), and per-batch distinct sizes.
//
// Used by both AddOpening (with shapes derived from the committed encoded
// table) and Verify (with shapes passed in directly).
func canonicalLayout(shapes []Shape, shifts []BatchShifts) (layout, error) {
	if len(shapes) != len(shifts) {
		return nil, fmt.Errorf("fri: canonicalLayout: got %d shapes, %d shifts", len(shapes), len(shifts))
	}

	maxSizeLog2 := -1
	for b := range shapes {
		if len(shifts[b]) != len(shapes[b]) {
			return nil, fmt.Errorf("fri: canonicalLayout: batch %d has shape length %d, shifts length %d",
				b, len(shapes[b]), len(shifts[b]))
		}
		if len(shapes[b]) > maxSizeLog2+1 {
			maxSizeLog2 = len(shapes[b]) - 1
		}
	}

	res := make(layout, 0, maxSizeLog2+1)
	for sizeLog2 := maxSizeLog2; sizeLog2 >= 0; sizeLog2-- {
		bundle := sizeBundle{SizeLog2: uint8(sizeLog2)}
		alphaDeepPower := 0

		for batchIdx := range shapes {
			if sizeLog2 >= len(shapes[batchIdx]) {
				continue
			}

			sizedShape := shapes[batchIdx][sizeLog2]
			sizedShifts := shifts[batchIdx][sizeLog2]
			if err := validateSizedLayout(batchIdx, sizeLog2, sizedShape, sizedShifts); err != nil {
				return nil, err
			}

			for rowIdx := range sizedShape.BaseWidth {
				bundle.Entries = append(bundle.Entries, deepEntry{
					BatchIdx:   batchIdx,
					SizeLog2:   uint8(sizeLog2),
					RowIdx:     rowIdx,
					AlphaPower: alphaDeepPower,
					Shifts:     cloneInts(sizedShifts.Base[rowIdx]),
				})
				alphaDeepPower++
			}
			for rowIdx := range sizedShape.ExtWidth {
				bundle.Entries = append(bundle.Entries, deepEntry{
					BatchIdx:   batchIdx,
					SizeLog2:   uint8(sizeLog2),
					RowIdx:     rowIdx,
					IsExt:      true,
					AlphaPower: alphaDeepPower,
					Shifts:     cloneInts(sizedShifts.Ext[rowIdx]),
				})
				alphaDeepPower++
			}
		}

		if len(bundle.Entries) > 0 {
			res = append(res, bundle)
		}
	}

	return res, nil
}

func validateSizedLayout(batchIdx, sizeLog2 int, shape SizedShape, shifts SizedShifts) error {
	if shape.BaseWidth < 0 || shape.ExtWidth < 0 {
		return fmt.Errorf("fri: canonicalLayout: batch %d size %d has negative width", batchIdx, sizeLog2)
	}
	if len(shifts.Base) != shape.BaseWidth {
		return fmt.Errorf("fri: canonicalLayout: batch %d size %d has %d base shift rows, want %d",
			batchIdx, sizeLog2, len(shifts.Base), shape.BaseWidth)
	}
	if len(shifts.Ext) != shape.ExtWidth {
		return fmt.Errorf("fri: canonicalLayout: batch %d size %d has %d ext shift rows, want %d",
			batchIdx, sizeLog2, len(shifts.Ext), shape.ExtWidth)
	}
	if sizeLog2 > 255 {
		return fmt.Errorf("fri: canonicalLayout: batch %d size %d is too large", batchIdx, sizeLog2)
	}
	size := 1 << sizeLog2
	for rowIdx, rowShifts := range shifts.Base {
		if err := validateColumnShifts(rowShifts, size); err != nil {
			return fmt.Errorf("fri: canonicalLayout: batch %d size %d base row %d: %w",
				batchIdx, sizeLog2, rowIdx, err)
		}
	}
	for rowIdx, rowShifts := range shifts.Ext {
		if err := validateColumnShifts(rowShifts, size); err != nil {
			return fmt.Errorf("fri: canonicalLayout: batch %d size %d ext row %d: %w",
				batchIdx, sizeLog2, rowIdx, err)
		}
	}
	return nil
}

func validateColumnShifts(shifts []int, size int) error {
	if len(shifts) == 0 {
		return fmt.Errorf("empty shift list")
	}
	seen := make(map[int]struct{}, len(shifts))
	for _, shift := range shifts {
		if shift < 0 || shift >= size {
			return fmt.Errorf("shift %d outside [0,%d)", shift, size)
		}
		if _, ok := seen[shift]; ok {
			return fmt.Errorf("duplicate shift %d", shift)
		}
		seen[shift] = struct{}{}
	}
	return nil
}

func cloneInts(values []int) []int {
	cloned := make([]int, len(values))
	copy(cloned, values)
	return cloned
}

// =============================================================================
// Level reconstruction
// =============================================================================

type quotientClaim struct {
	Point field.Ext
	Value field.Ext
}

type quotientColumn struct {
	Evals  []field.Ext // Evals over the codeword domain
	Claims []quotientClaim
}

func reconstructDomainSize(domain domainLight) (int, error) {
	if domain.cardinality == 0 || domain.cardinality&(domain.cardinality-1) != 0 {
		return 0, fmt.Errorf("fri: reconstructLevels: domain cardinality %d is not a positive power of two",
			domain.cardinality)
	}
	return int(domain.cardinality), nil
}

func checkColumnClaimPoints(columnIdx int, claims []quotientClaim) error {
	seen := make(map[field.Ext]struct{}, len(claims))
	for claimIdx, claim := range claims {
		if _, ok := seen[claim.Point]; ok {
			return fmt.Errorf("fri: reconstructLevels: column %d has duplicate claim point at claim %d",
				columnIdx, claimIdx)
		}
		seen[claim.Point] = struct{}{}
	}
	return nil
}

func collectClaimPoints(columns []quotientColumn) (map[field.Ext]int, []field.Ext) {
	indexes := make(map[field.Ext]int)
	for _, column := range columns {
		for _, claim := range column.Claims {
			if _, ok := indexes[claim.Point]; ok {
				continue
			}
			indexes[claim.Point] = len(indexes)
		}
	}

	points := make([]field.Ext, len(indexes))
	for point, index := range indexes {
		points[index] = point
	}
	return indexes, points
}

func denominatorInverses(domainPoints, claimPoints []field.Ext) ([]field.Ext, error) {
	if len(claimPoints) == 0 {
		return nil, nil
	}

	denominators := make([]field.Ext, len(domainPoints)*len(claimPoints))
	for pos, x := range domainPoints {
		for pointIdx, point := range claimPoints {
			denominator := &denominators[pos*len(claimPoints)+pointIdx]
			denominator.Sub(&x, &point)
			if denominator.IsZero() {
				return nil, fmt.Errorf("fri: reconstructLevels: claim point %d lands on domain position %d",
					pointIdx, pos)
			}
		}
	}
	return field.BatchInvertExt(denominators), nil
}

// =============================================================================
// OpeningProof
// =============================================================================

// OpeningProof bundles everything PCS verification needs to check the claimed
// evaluations against the committed batches and the underlying FRI proof.
type OpeningProof struct {
	// InputQueries[k] contains one row-carrying opening per input tree for query k.
	InputQueries []InputQuery

	// FRIProof is checked through PCS verification, which reconstructs virtual
	// level values from ClaimedValues and InputQueries.
	FRIProof Proof
}

// InputQuery holds the PCS input-tree openings for one FRI query.
type InputQuery []InputTreeOpening

// InputTreeOpening is a Merkle branch whose path leaves are opened as row
// preimages.
type InputTreeOpening struct {
	Siblings []field.Octuplet
	Leaves   []*RowPair // Leaves along the way, when they exist, starting from the root down
}

// RowPair holds one level's conjugate row pair (see MultiSizeTable.Merkleize):
type RowPair [2]RowOpening

// RowOpening is one MultiSizeTable.Merkleize row preimage.
type RowOpening struct {
	Base []field.Element
	Ext  []field.Ext
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

// =============================================================================
// Prover API
// =============================================================================

// Challenges bundles the Fiat-Shamir values supplied by the caller. There is
// no separate DEEP-batching challenge (see the package doc's step 3).
type Challenges struct {
	FoldAlphas     []field.Ext // length == Params.numRounds
	QueryPositions []int       // length == Params.NumQueries
}

// Commit encodes and Merkle-commits witness using this PCS's encoder schedule.
// It is a convenience wrapper around the package-level Commit. It does not
// register an opening; pass the returned state to AddOpening when this
// commitment should contribute to the next opening proof.
func (pcs *PCS) Commit(witness MultiSizeTable) CommitterState {
	return Commit(pcs.Encoders, witness)
}

// AddOpening registers one committed batch to be opened at zeta with the given
// shift schedule and claimed evaluations. It records the commitment, layout,
// and claims as pending input to the virtual DEEP quotient later built by
// NewProverState.
//
// The PCS no longer computes the claimed evaluations: the caller (the outer
// protocol) evaluates every opened (size, row, shift) at zeta * omega^shift,
// absorbs the claims into its transcript before deriving alpha_DEEP, and passes
// them here. zeta is shared across every opening of a single proof; the first
// AddOpening fixes it and later calls must supply the same value.
//
// The plaintext witness is no longer needed: the column layout is derived from
// the committed encoded table, which carries the same per-size row widths.
func (pcs *PCS) AddOpening(
	committed CommitterState,
	zeta field.Ext,
	shifts BatchShifts,
	claimed BatchClaimedValues,
) error {
	if len(pcs.openings) > 0 && !pcs.zeta.Equal(&zeta) {
		return fmt.Errorf("fri: AddOpening: zeta mismatch")
	}
	if committed.Tree == nil {
		return fmt.Errorf("fri: AddOpening: commitment has nil tree")
	}
	shape := committed.EncodedTable.Shape()
	layout, err := canonicalLayout([]Shape{shape}, []BatchShifts{shifts})
	if err != nil {
		return err
	}
	if err = validateBatchClaimShape(claimed, shifts, zeta); err != nil {
		return fmt.Errorf("fri: AddOpening: %w", err)
	}

	pcs.zeta = zeta
	pcs.openings = append(pcs.openings, pcsOpening{
		layout:    layout,
		committed: committed,
		claimed:   claimed,
	})
	return nil
}

// validateBatchClaimShape checks that a caller-supplied BatchClaimedValues
// aligns exactly with the shift schedule it is meant to answer: one claimed
// value per (size, row, shift). It guards against empty or misshapen claims
// slipping through to NewProverState, where the mismatch would otherwise
// surface as an opaque reconstruction error.
func validateBatchClaimShape(claimed BatchClaimedValues, shifts BatchShifts, zeta field.Ext) error {
	if len(claimed) != len(shifts) {
		return fmt.Errorf("got %d claimed sizes, want %d", len(claimed), len(shifts))
	}
	for sizeLog2, sizedShifts := range shifts {
		sizedClaimed := claimed[sizeLog2]
		if len(sizedClaimed.Base) != len(sizedShifts.Base) {
			return fmt.Errorf("size %d has %d base claim rows, want %d",
				sizeLog2, len(sizedClaimed.Base), len(sizedShifts.Base))
		}
		if len(sizedClaimed.Ext) != len(sizedShifts.Ext) {
			return fmt.Errorf("size %d has %d ext claim rows, want %d",
				sizeLog2, len(sizedClaimed.Ext), len(sizedShifts.Ext))
		}
		for rowIdx, rowShifts := range sizedShifts.Base {
			if zeta.IsZero() && len(rowShifts) > 1 {
				return fmt.Errorf("size %d base row %d has %d shifts with zero zeta", sizeLog2, rowIdx, len(rowShifts))
			}
			if len(sizedClaimed.Base[rowIdx]) != len(rowShifts) {
				return fmt.Errorf("size %d base row %d has %d claims, want %d",
					sizeLog2, rowIdx, len(sizedClaimed.Base[rowIdx]), len(rowShifts))
			}
		}
		for rowIdx, rowShifts := range sizedShifts.Ext {
			if zeta.IsZero() && len(rowShifts) > 1 {
				return fmt.Errorf("size %d ext row %d has %d shifts with zero zeta", sizeLog2, rowIdx, len(rowShifts))
			}
			if len(sizedClaimed.Ext[rowIdx]) != len(rowShifts) {
				return fmt.Errorf("size %d ext row %d has %d claims, want %d",
					sizeLog2, rowIdx, len(sizedClaimed.Ext[rowIdx]), len(rowShifts))
			}
		}
	}
	return nil
}

// NewProverState gathers the virtual DEEP quotient levels' per-column data and
// returns the existing FRI prover state. It does not fold or open: each
// level's batched evaluations are computed later, at fold time (see
// Level.EvalsAt), once that level's own alphaDeep is known.
//
// The FRI schedule is restricted to the largest size actually opened, so the
// fold count follows the witness rather than the (possibly larger, static)
// Params: a size-2^k top level folds k times regardless of Params.LogPlainTextSize >= k.
func (pcs *PCS) NewProverState() (*ProverState, error) {
	if len(pcs.openings) == 0 {
		return nil, fmt.Errorf("fri: NewProverState: AddOpening must be called first")
	}

	pcs, err := pcs.restrictToOpenings()
	if err != nil {
		return nil, err
	}

	levels, err := pcs.reconstructLevels()
	if err != nil {
		return nil, err
	}

	state, err := NewProverState(pcs.Params, levels)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// maxSizeLog2 returns the largest size (log2) present in a canonical
// layout, i.e. the top of the FRI schedule the verifier needs.
func (l layout) maxSizeLog2() uint8 {
	maxIdx := uint8(0)
	for _, bundle := range l {
		if bundle.SizeLog2 > maxIdx {
			maxIdx = bundle.SizeLog2
		}
	}
	return maxIdx
}

// maxOpeningSizeLog2 returns the largest size (log2) opened across all pending
// openings, i.e. the top of the FRI schedule this proof actually needs.
func (pcs *PCS) maxOpeningSizeLog2() uint8 {
	maxIdx := uint8(0)
	for _, opening := range pcs.openings {
		for _, bundle := range opening.layout {
			if bundle.SizeLog2 > maxIdx {
				maxIdx = bundle.SizeLog2
			}
		}
	}
	return maxIdx
}

// restrictToOpenings returns a view of this PCS whose Params are restricted to
// the largest opened size, sharing the (immutable) encoders and pending
// openings. Prover-side entry points use it so the fold count tracks the witness.
func (pcs *PCS) restrictToOpenings() (*PCS, error) {
	return pcs.restrictTo(pcs.maxOpeningSizeLog2())
}

// restrictTo returns a view of this PCS with Params restricted to top size
// 2^topSizeLog2, sharing the encoders, openings, and zeta.
func (pcs *PCS) restrictTo(topSizeLog2 uint8) (*PCS, error) {
	restrictedParams, err := pcs.Params.restrictTo(topSizeLog2)
	if err != nil {
		return nil, err
	}
	return &PCS{
		Params:   restrictedParams,
		Encoders: pcs.Encoders,
		openings: pcs.openings,
		zeta:     pcs.zeta,
	}, nil
}

func (pcs *PCS) shiftedPoint(sizeLog2 uint8, shift int, zeta field.Ext) (field.Ext, error) {
	if int(sizeLog2) >= len(pcs.Encoders) {
		return field.Ext{}, fmt.Errorf("size %d has no encoder", sizeLog2)
	}
	encoder := pcs.Encoders[sizeLog2]
	if encoder == nil {
		return field.Ext{}, fmt.Errorf("encoder %d is nil", sizeLog2)
	}
	size := encoder.PlainTextSize
	if size <= 0 {
		return field.Ext{}, fmt.Errorf("encoder %d has invalid plaintext size %d", sizeLog2, size)
	}

	if shift < 0 || shift >= size {
		return field.Ext{}, fmt.Errorf("shift %d outside [0,%d)", shift, size)
	}

	var rotation field.Element
	rotation.Exp(encoder.smallDomain.Generator, big.NewInt(int64(shift)))

	var point field.Ext
	point.MulByElement(&zeta, &rotation)
	return point, nil
}

// reconstructLevels gathers, for each distinct committed size, the per-column
// DEEP-quotient data (columns and every distinct denominator inverse 1/(x-z),
// precomputed via one Montgomery batch inversion) that batches into
//
//	F(X) = Σ_i alpha_DEEP^i · Σ_j (f_i(X) - y_ij)/(X - z_ij)
//
// over input.Domain's bit-reversed evaluation order. alpha_DEEP is not known
// yet (it is the square of this level's own introduction round's fold
// challenge), so the batched evaluations themselves are computed later, by
// Level.EvalsAt.
func (pcs *PCS) reconstructLevels() ([]Level, error) {
	logD := pcs.Params.LogPlainTextSize
	levels := make([]Level, 0, logD+1)
	for sizeLog2 := int(logD); sizeLog2 >= 0; sizeLog2-- {
		var columns []quotientColumn
		var trees []*Tree
		for _, opening := range pcs.openings {
			for _, bundle := range opening.layout {
				if bundle.SizeLog2 != uint8(sizeLog2) {
					continue
				}
				trees = append(trees, opening.committed.Tree)
				for _, entry := range bundle.Entries {
					evals, err := encodedColumnEvals(opening.committed, entry)
					if err != nil {
						return nil, err
					}
					claims, err := pcs.openingClaimsForEntry(opening, entry)
					if err != nil {
						return nil, err
					}
					columns = append(columns, quotientColumn{
						Evals:  evals,
						Claims: claims,
					})
				}
			}
		}
		if len(columns) == 0 {
			continue
		}

		domain := pcs.Params.domainsLight[logD-uint8(sizeLog2)]
		size, err := reconstructDomainSize(domain)
		if err != nil {
			return nil, err
		}
		for columnIdx, column := range columns {
			if len(column.Evals) != size {
				return nil, fmt.Errorf("fri: reconstructLevels: column %d has %d evals, want %d",
					columnIdx, len(column.Evals), size)
			}
			if err = checkColumnClaimPoints(columnIdx, column.Claims); err != nil {
				return nil, err
			}
		}

		domainPoints := make([]field.Ext, size)
		for pos := range domainPoints {
			domainPoints[pos] = domainPointExt(domain, pos)
		}

		claimPointIndexes, claimPoints := collectClaimPoints(columns)
		denominatorInverses, err := denominatorInverses(domainPoints, claimPoints)
		if err != nil {
			return nil, err
		}

		levels = append(levels, Level{
			Trees:               trees,
			Columns:             columns,
			ClaimPointIndexes:   claimPointIndexes,
			ClaimPoints:         claimPoints,
			DenominatorInverses: denominatorInverses,
		})
	}
	return levels, nil
}

func (pcs *PCS) roundForSize(sizeLog2 uint8) (uint8, error) {
	logD := pcs.Params.LogPlainTextSize
	if sizeLog2 > logD {
		return 0, fmt.Errorf("fri: reconstructLevels: size %d is outside params schedule", sizeLog2)
	}
	return logD - sizeLog2, nil
}

func encodedColumnEvals(committed CommitterState, entry deepEntry) ([]field.Ext, error) {
	table := committed.EncodedTable
	if int(entry.SizeLog2) >= len(table) {
		return nil, fmt.Errorf("fri: reconstructLevels: missing encoded size %d", entry.SizeLog2)
	}
	sized := table[entry.SizeLog2]
	if entry.IsExt {
		if entry.RowIdx >= len(sized.Ext) {
			return nil, fmt.Errorf("fri: reconstructLevels: size %d missing ext row %d",
				entry.SizeLog2, entry.RowIdx)
		}
		evals := make([]field.Ext, len(sized.Ext[entry.RowIdx]))
		copy(evals, sized.Ext[entry.RowIdx])
		return evals, nil
	}

	if entry.RowIdx >= len(sized.Base) {
		return nil, fmt.Errorf("fri: reconstructLevels: size %d missing base row %d",
			entry.SizeLog2, entry.RowIdx)
	}
	base := sized.Base[entry.RowIdx]
	evals := make([]field.Ext, len(base))
	for i := range base {
		evals[i] = field.Lift(base[i])
	}
	return evals, nil
}

func (pcs *PCS) claimsForEntry(
	claimed []BatchClaimedValues,
	entry deepEntry,
	zeta field.Ext,
) ([]quotientClaim, error) {
	if entry.BatchIdx >= len(claimed) {
		return nil, fmt.Errorf("fri: reconstructLevels: missing claims for batch %d", entry.BatchIdx)
	}
	return pcs.claimsForBatchEntry(claimed[entry.BatchIdx], entry, zeta)
}

func (pcs *PCS) openingClaimsForEntry(opening pcsOpening, entry deepEntry) ([]quotientClaim, error) {
	return pcs.claimsForBatchEntry(opening.claimed, entry, pcs.zeta)
}

func (pcs *PCS) claimsForBatchEntry(
	claimed BatchClaimedValues,
	entry deepEntry,
	zeta field.Ext,
) ([]quotientClaim, error) {
	if int(entry.SizeLog2) >= len(claimed) {
		return nil, fmt.Errorf("fri: reconstructLevels: missing claims for size %d", entry.SizeLog2)
	}
	var values []field.Ext
	sized := claimed[entry.SizeLog2]
	if entry.IsExt {
		if entry.RowIdx >= len(sized.Ext) {
			return nil, fmt.Errorf("fri: reconstructLevels: size %d missing ext claims row %d",
				entry.SizeLog2, entry.RowIdx)
		}
		values = sized.Ext[entry.RowIdx]
	} else {
		if entry.RowIdx >= len(sized.Base) {
			return nil, fmt.Errorf("fri: reconstructLevels: size %d missing base claims row %d",
				entry.SizeLog2, entry.RowIdx)
		}
		values = sized.Base[entry.RowIdx]
	}

	if len(values) != len(entry.Shifts) {
		return nil, fmt.Errorf("fri: reconstructLevels: size %d row %d has %d claims, want %d",
			entry.SizeLog2, entry.RowIdx, len(values), len(entry.Shifts))
	}

	claims := make([]quotientClaim, len(entry.Shifts))
	for i, shift := range entry.Shifts {
		point, err := pcs.shiftedPoint(entry.SizeLog2, shift, zeta)
		if err != nil {
			return nil, err
		}
		claims[i] = quotientClaim{Point: point, Value: values[i]}
	}
	return claims, nil
}

// Open runs the PCS query phase for the already-folded FRI state.
func (pcs *PCS) Open(state *ProverState, queryPositions []int) OpeningProof {
	return OpeningProof{
		InputQueries: pcs.openInputQueries(queryPositions),
		FRIProof:     state.Open(queryPositions),
	}
}

func (pcs *PCS) openInputQueries(queryPositions []int) []InputQuery {
	restricted, err := pcs.restrictToOpenings()
	if err != nil {
		panic(err)
	}
	pcs = restricted

	inputs := pcs.inputOpeningCommitments()
	res := make([]InputQuery, len(queryPositions))
	for queryIdx, queryPosition := range queryPositions {
		res[queryIdx] = make(InputQuery, len(inputs))
		for inputIdx, committed := range inputs {
			res[queryIdx][inputIdx] = openInputTreeOpening(pcs.Params, committed, queryPosition)
		}
	}
	return res
}

func (pcs *PCS) inputOpeningCommitments() []CommitterState {
	seen := make(map[field.Octuplet]bool)
	var inputs []CommitterState
	for sizeLog2 := int(pcs.Params.LogPlainTextSize); sizeLog2 >= 0; sizeLog2-- {
		for _, opening := range pcs.openings {
			for _, bundle := range opening.layout {
				if bundle.SizeLog2 != uint8(sizeLog2) {
					continue
				}
				root := opening.committed.Tree.Root()
				if !seen[root] {
					seen[root] = true
					inputs = append(inputs, opening.committed)
				}
				break
			}
		}
	}
	return inputs
}

func openInputTreeOpening(p Params, committed CommitterState, queryPosition int) InputTreeOpening {
	tree := committed.Tree
	numLeaves := tree.NumLeaves()
	codewordSize := 1 << p.LogCodewordSize
	if numLeaves > codewordSize || codewordSize%numLeaves != 0 {
		panic("fri: openInputTreeOpening: tree size incompatible with domain size")
	}
	leafIndex := queryPosition / (codewordSize / numLeaves)
	branch := tree.OpenBranch(leafIndex)
	input := InputTreeOpening{
		// The bottom level's own sibling digest is derived from its pair
		// (below) rather than transmitted.
		Siblings: branch.Siblings[:len(branch.Siblings)-1],
		// The last slot is otherwise always vacant (see Merkleize: nothing
		// shifts to the depth just above the bottom level), so it is
		// repurposed for the bottom level's own mandatory pair.
		Leaves: make([]*RowPair, len(branch.AuxSiblings)),
	}
	input.Leaves[len(input.Leaves)-1] = &RowPair{
		openEncodedRowAtSize(committed.EncodedTable, numLeaves, leafIndex),
		openEncodedRowAtSize(committed.EncodedTable, numLeaves, leafIndex^1),
	}
	for sizeLog2 := range committed.EncodedTable {
		table := committed.EncodedTable[sizeLog2]
		if table.NumRows() == 0 || table.Size() == numLeaves {
			continue
		}
		levelSize := table.Size()
		if levelSize > numLeaves || numLeaves%levelSize != 0 {
			panic("fri: openInputTreeOpening: level size incompatible with tree size")
		}
		levelLog := bits.TrailingZeros(uint(levelSize)) - 1 // one depth shallower than levelSize (see Merkleize)
		if levelLog < 0 || levelLog >= len(input.Leaves)-1 {
			panic("fri: openInputTreeOpening: level size absent from branch")
		}
		base := leafIndex / (numLeaves / levelSize)
		input.Leaves[levelLog] = &RowPair{
			openEncodedRow(table, base),
			openEncodedRow(table, base^1),
		}
	}
	return input
}

func openEncodedRowAtSize(table MultiSizeTable, size, row int) RowOpening {
	for sizeLog2 := range table {
		if table[sizeLog2].NumRows() != 0 && table[sizeLog2].Size() == size {
			return openEncodedRow(table[sizeLog2], row)
		}
	}
	panic("fri: openEncodedRowAtSize: size absent from table")
}

func batchOrders(layout layout) [][]int {
	orders := make([][]int, len(layout))
	for i := range layout {
		prev := -1
		orders[i] = make([]int, 0, len(layout[i].Entries))
		for _, entry := range layout[i].Entries {
			if entry.BatchIdx == prev {
				continue
			}
			orders[i] = append(orders[i], entry.BatchIdx)
			prev = entry.BatchIdx
		}
	}
	return orders
}

func openEncodedRow(table SizedTable, row int) RowOpening {
	opening := RowOpening{
		Base: make([]field.Element, len(table.Base)),
		Ext:  make([]field.Ext, len(table.Ext)),
	}
	for i := range table.Base {
		opening.Base[i] = table.Base[i][row]
	}
	for i := range table.Ext {
		opening.Ext[i] = table.Ext[i][row]
	}
	return opening
}

func hashRowOpening(row RowOpening) field.Octuplet {
	hasher := poseidon2.NewMDHasher()
	absorbLeafHeader(hasher, len(row.Base), len(row.Ext))
	writeRowOpeningElements(hasher, row)
	return hasher.SumDigest()
}

// hashAuxPair must match Merkleize's even-before-odd hash order regardless of
// which one is Self, hence selfIsEven. The domain-separation header is written
// once per leaf (both rows share the same shape).
func hashAuxPair(pair RowPair, selfIsEven bool) field.Octuplet {
	hasher := poseidon2.NewMDHasher()
	absorbLeafHeader(hasher, len(pair[0].Base), len(pair[0].Ext))
	if selfIsEven {
		writeRowOpeningElements(hasher, pair[0])
		writeRowOpeningElements(hasher, pair[1])
	} else {
		writeRowOpeningElements(hasher, pair[1])
		writeRowOpeningElements(hasher, pair[0])
	}
	return hasher.SumDigest()
}

func writeRowOpeningElements(hasher *poseidon2.MDHasher, row RowOpening) {
	for _, base := range row.Base {
		hasher.WriteElements(base)
	}
	for _, ext := range row.Ext {
		hasher.WriteElements(ext.B0.A0, ext.B0.A1, ext.B1.A0, ext.B1.A1, ext.B2.A0, ext.B2.A1)
	}
}

// RecoverRoot folds this branch's rows up to the tree root. The bottom
// (deepest) level's own step combines its pair directly instead of reading a
// transmitted sibling digest, so Siblings holds one fewer entry than
// Leaves.
func (branch InputTreeOpening) RecoverRoot(idx int) (field.Octuplet, error) {
	numLevels := len(branch.Leaves)
	if numLevels == 0 || branch.Leaves[numLevels-1] == nil {
		return field.Octuplet{}, fmt.Errorf("malformed proof: missing bottom level")
	}
	if len(branch.Siblings) != numLevels-1 {
		return field.Octuplet{}, fmt.Errorf("malformed proof")
	}

	bottom := branch.Leaves[numLevels-1]
	ancestor := hashRowOpening(bottom[0])
	sibling := hashRowOpening(bottom[1])
	ancestor, currPos := foldOneLevel(ancestor, sibling, nil, idx)

	for i := numLevels - 2; i >= 0; i-- {
		ancestor, currPos = foldOneLevel(ancestor, branch.Siblings[i], branch.Leaves[i], currPos)
	}
	if currPos > 0 {
		return field.Octuplet{}, fmt.Errorf("all bits of currPos should have been bitshifted beyond LSb")
	}
	return ancestor, nil
}

func foldOneLevel(ancestor, sibling field.Octuplet, aux *RowPair, currPos int) (field.Octuplet, int) {
	selfIsEven := currPos&1 == 0
	left, right := ancestor, sibling
	if !selfIsEven {
		left, right = right, left
	}
	var auxDigest *field.Octuplet
	if aux != nil {
		hashed := hashAuxPair(*aux, selfIsEven)
		auxDigest = &hashed
	}
	return hashNode(left, right, auxDigest), currPos >> 1
}

// levelIndex resolves levelSize to its index into branch.Leaves. The bottom
// level keeps its own (unshifted) depth; every other level's pair attaches
// one depth shallower than its size (see Merkleize).
func (branch InputTreeOpening) levelIndex(levelSize int) (int, error) {
	if levelSize <= 0 || levelSize&(levelSize-1) != 0 {
		return 0, fmt.Errorf("levelSize must be a positive power of two")
	}

	treeLeaves := 1 << len(branch.Leaves)
	if levelSize > treeLeaves {
		return 0, fmt.Errorf("levelSize %d exceeds branch tree size %d", levelSize, treeLeaves)
	}

	if levelSize == treeLeaves {
		return len(branch.Leaves) - 1, nil
	}
	levelLog := bits.TrailingZeros(uint(levelSize)) - 1
	if levelLog < 0 {
		return 0, fmt.Errorf("levelSize %d has no aux sibling in branch", levelSize)
	}
	return levelLog, nil
}

func (branch InputTreeOpening) rowAtLevel(levelSize int) (RowOpening, error) {
	pair, err := branch.pairAtLevel(levelSize)
	if err != nil {
		return RowOpening{}, err
	}
	return pair[0], nil
}

// pairAtLevel returns the full conjugate pair at levelSize, so callers can
// validate (or read) both the on-path row and its conjugate uniformly,
// regardless of whether this level is the top one.
func (branch InputTreeOpening) pairAtLevel(levelSize int) (*RowPair, error) {
	levelLog, err := branch.levelIndex(levelSize)
	if err != nil {
		return nil, err
	}
	if branch.Leaves[levelLog] == nil {
		return nil, fmt.Errorf("levelSize %d is absent from branch", levelSize)
	}
	return branch.Leaves[levelLog], nil
}

// reconstructQueryValueAt combines bundle's columns with running (this
// level's own round's running-codeword value) at x, the same way EvalsAt
// combines a level's columns with the prover's running codeword. Entries are
// walked highest AlphaPower first (canonicalLayout assigns them
// 0..len(Entries)-1 in order, so that's simply the reverse index order).
func reconstructQueryValueAt(
	pcs *PCS,
	bundle sizeBundle,
	opening InputQuery,
	inputIndexByBatch []int,
	levelSize int,
	claimed []BatchClaimedValues,
	zeta field.Ext,
	alphaDeep field.Ext,
	x field.Ext,
	sibling bool,
	running field.Ext,
) (field.Ext, error) {
	value := running
	for i := len(bundle.Entries) - 1; i >= 0; i-- {
		entry := bundle.Entries[i]
		claims, err := pcs.claimsForEntry(claimed, entry, zeta)
		if err != nil {
			return field.Ext{}, err
		}
		branch := opening[inputIndexByBatch[entry.BatchIdx]]
		pair, err := branch.pairAtLevel(levelSize)
		if err != nil {
			return field.Ext{}, err
		}
		row := pair[0]
		if sibling {
			row = pair[1]
		}
		entryValue := rowValue(row, entry)
		term, err := quotientAtValue(entryValue, x, claims)
		if err != nil {
			return field.Ext{}, err
		}
		value.Mul(&value, &alphaDeep)
		value.Add(&value, &term)
	}
	return value, nil
}

func rowValue(row RowOpening, entry deepEntry) field.Ext {
	if entry.IsExt {
		return row.Ext[entry.RowIdx]
	}
	return field.Lift(row.Base[entry.RowIdx])
}

func quotientAtValue(value, x field.Ext, claims []quotientClaim) (field.Ext, error) {
	var res field.Ext
	for _, claim := range claims {
		var denominator field.Ext
		denominator.Sub(&x, &claim.Point)
		if denominator.IsZero() {
			return field.Ext{}, fmt.Errorf("claim point lands on query point")
		}
		denominator.Inverse(&denominator)

		var numerator, term field.Ext
		numerator.Sub(&value, &claim.Value)
		term.Mul(&numerator, &denominator)
		res.Add(&res, &term)
	}
	return res, nil
}

// VerifyInputs bundles every parameter pcs.Verify needs.
type VerifyInputs struct {
	Roots  []field.Octuplet
	Shapes []Shape
	Shifts []BatchShifts
	// ClaimedValues[b] mirrors shifts[b] exactly. The outer protocol
	// reads these to evaluate its constraints at zeta and to bind into
	// the alpha_DEEP transcript challenge.
	ClaimedValues []BatchClaimedValues
	Zeta          field.Ext

	Challenges Challenges
}

// Verify checks an OpeningProof under the same zeta, challenges, and query
// positions the prover used.
//
// Performs in sequence:
//
//  1. Shape validation for Roots/Shapes/Shifts and claimed values.
//  2. Canonical-layout build from Shapes + Shifts.
//  3. Per query: authenticate each deduped input opening once, bind top and
//     auxiliary rows from those branches, authenticate each running FRI layer
//     against its committed root, and resolve all fold values.
//  4. checkFolds: pure fold-recurrence arithmetic over the resolved values.
func (pcs *PCS) Verify(in VerifyInputs, proof OpeningProof) error {
	if len(in.Roots) != len(in.Shapes) {
		return fmt.Errorf("fri: pcs.Verify: got %d roots, %d shapes", len(in.Roots), len(in.Shapes))
	}
	layout, err := canonicalLayout(in.Shapes, in.Shifts)
	if err != nil {
		return err
	}
	if err = checkVerifyClaimShapes(in.ClaimedValues, in.Shifts, in.Zeta); err != nil {
		return err
	}
	// Restrict the FRI schedule to the largest opened size so the fold count
	// tracks the witness rather than the (static) Params; mirrors the prover.
	pcs, err = pcs.restrictTo(layout.maxSizeLog2())
	if err != nil {
		return err
	}
	if err = pcs.checkClaimPointsOutOfDomain(layout, in.Zeta); err != nil {
		return err
	}
	if len(proof.InputQueries) != int(pcs.Params.NumQueries) {
		return fmt.Errorf("fri: pcs.Verify: proof has %d input queries, want %d",
			len(proof.InputQueries), pcs.Params.NumQueries)
	}
	if len(in.Challenges.QueryPositions) < int(pcs.Params.NumQueries) {
		return fmt.Errorf("fri: pcs.Verify: %d query positions, need at least %d",
			len(in.Challenges.QueryPositions), pcs.Params.NumQueries)
	}
	orders := batchOrders(layout)
	positions := in.Challenges.QueryPositions[:pcs.Params.NumQueries]
	inputRoots, inputIndexByBatch := inputOpeningRoots(layout, orders, in.Roots)
	if err = checkOpeningProofShape(pcs.Params, proof.FRIProof, in.Challenges.FoldAlphas, positions); err != nil {
		return err
	}

	runningRoots := make([]QueryLayerRoots, pcs.Params.numRounds())
	for j := uint8(1); j < pcs.Params.numRounds(); j++ {
		runningRoots[j] = QueryLayerRoots{proof.FRIProof.RoundRoots[j-1]}
	}

	claimed := in.ClaimedValues
	zeta := in.Zeta
	foldAlphas := in.Challenges.FoldAlphas

	finalCodeword := make([]field.Ext, 1<<(pcs.Params.LogCodewordSize-pcs.Params.numRounds()))
	copy(finalCodeword, proof.FRIProof.FinalPoly)
	pcs.Params.domains[pcs.Params.numRounds()].FFTExt6(finalCodeword, fft.DIF)

	resolved := make([]resolvedQuery, pcs.Params.NumQueries)
	for queryIdx, queryPosition := range positions {
		rq := resolvedQuery{
			// Rounds[0..numRounds()-1] hold running-layer pairs; Rounds[0] is
			// always zero (no committed layer at round 0). An extra slot at
			// index numRounds() holds the zero seed for any level introduced
			// at that boundary round (e.g. a D=1 aux level at the final round).
			Rounds: make([]inputPair, pcs.Params.numRounds()+1),
			Aux:    make(map[uint8]inputPair, len(layout)),
			Final:  finalCodeword[queryPosition>>pcs.Params.numRounds()],
		}

		inputOpening := proof.InputQueries[queryIdx]
		if err = authenticateInputQuery(pcs.Params, inputOpening, inputRoots, queryPosition); err != nil {
			return fmt.Errorf("fri: pcs.Verify: query %d: %w", queryIdx, err)
		}

		// Running layers: authenticate and decode directly from the committed
		// codeword -- no PCS reconstruction involved. Computed before the level
		// loop below, since a level's own reconstruction seeds on this same
		// round's running pair (rq.Rounds[0] is left at its zero value, so
		// round 0 needs no special case).
		for j := uint8(1); j < pcs.Params.numRounds(); j++ {
			opening := proof.FRIProof.RunningQueries[queryIdx][j-1]
			if err = checkQueryLayerShape(
				opening, runningRoots[j], 1<<(pcs.Params.LogCodewordSize-j), true); err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d: %w", queryIdx, j, err)
			}
			branch, err := authenticateQueryLayer(j, opening, runningRoots[j], queryPosition>>j)
			if err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d: %w", queryIdx, err)
			}
			if len(branch.Siblings) == 0 {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d: branch carries no sibling", queryIdx, j)
			}
			self, err := octupletToExt(branch.Leaf)
			if err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d: decode leaf: %w", queryIdx, j, err)
			}
			sib, err := octupletToExt(branch.Siblings[len(branch.Siblings)-1])
			if err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d: decode sibling: %w", queryIdx, j, err)
			}
			rq.Rounds[j] = inputPair{Self: self, Sibling: sib}
		}

		// Every level -- including the main degree-D polynomial, introduced
		// at round 0 -- binds the same way: authenticate its rows against
		// their declared shape, then reconstruct its conjugate pair at
		// (position, position^1), using alphaDeep = FoldAlphas[round]²: the
		// square of that SAME round's own fold challenge, never an earlier
		// round's (see reconstructQueryValueAt).
		for levelIdx, bundle := range layout {
			round, err := pcs.roundForSize(bundle.SizeLog2)
			if err != nil {
				return err
			}
			if round > pcs.Params.numRounds() {
				return fmt.Errorf("fri: pcs.Verify: level %d introduced at round %d, must be <= %d",
					levelIdx, round, pcs.Params.numRounds())
			}
			domain := pcs.Params.domainsLight[round]
			levelSize, err := reconstructDomainSize(domain)
			if err != nil {
				return err
			}
			label := fmt.Sprintf("level %d", levelIdx)
			if round == 0 {
				label = "round 0"
			}
			if err = bindInputTreeOpenings(label, inputOpening, inputIndexByBatch,
				levelSize, orders[levelIdx], bundle, in.Shapes); err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d: %w", queryIdx, err)
			}
			var alphaDeep field.Ext
			if int(round) < len(foldAlphas) {
				alphaDeep.Square(&foldAlphas[round])
			}
			levelPos := queryPosition >> round
			self, err := reconstructQueryValueAt(pcs, bundle, inputOpening, inputIndexByBatch, levelSize,
				claimed, zeta, alphaDeep, domainPointExt(domain, levelPos), false, rq.Rounds[round].Self)
			if err != nil {
				return err
			}
			sib, err := reconstructQueryValueAt(pcs, bundle, inputOpening, inputIndexByBatch, levelSize,
				claimed, zeta, alphaDeep, domainPointExt(domain, levelPos^1), true, rq.Rounds[round].Sibling)
			if err != nil {
				return err
			}
			rq.Aux[round] = inputPair{Self: self, Sibling: sib}
		}

		// D=1: numRounds()==0 so checkFolds runs zero iterations and never
		// ties the top-level pair to the final polynomial. Do it explicitly:
		// the revealed FinalPoly IS the constant layer-0 codeword and both
		// conjugate positions must match it exactly.
		if pcs.Params.numRounds() == 0 {
			pair := rq.Aux[0]
			sibFinal := finalCodeword[queryPosition^1]
			if !pair.Self.Equal(&rq.Final) {
				return fmt.Errorf("fri: pcs.Verify: query %d: round-0 self does not match FinalPoly", queryIdx)
			}
			if !pair.Sibling.Equal(&sibFinal) {
				return fmt.Errorf("fri: pcs.Verify: query %d: round-0 sibling does not match FinalPoly", queryIdx)
			}
		}

		resolved[queryIdx] = rq
	}

	return checkFolds(pcs.Params, resolved, in.Challenges.FoldAlphas, positions)
}

func inputOpeningRoots(layout layout, orders [][]int, roots []field.Octuplet) (QueryLayerRoots, []int) {
	indexByBatch := make([]int, len(roots))
	for i := range indexByBatch {
		indexByBatch[i] = -1
	}
	indexByRoot := make(map[field.Octuplet]int)
	inputRoots := make(QueryLayerRoots, 0)
	for levelIdx := range layout {
		for _, batchIdx := range orders[levelIdx] {
			root := roots[batchIdx]
			if idx, ok := indexByRoot[root]; ok {
				indexByBatch[batchIdx] = idx
				continue
			}
			indexByBatch[batchIdx] = len(inputRoots)
			indexByRoot[root] = len(inputRoots)
			inputRoots = append(inputRoots, root)
		}
	}
	return inputRoots, indexByBatch
}

func authenticateInputQuery(p Params, opening InputQuery, roots QueryLayerRoots, queryPosition int) error {
	if len(opening) != len(roots) {
		return fmt.Errorf("input query has %d tree openings, want %d", len(opening), len(roots))
	}
	for i, branch := range opening {
		if len(branch.Leaves) == 0 || branch.Leaves[len(branch.Leaves)-1] == nil {
			return fmt.Errorf("input tree %d: missing bottom level", i)
		}
		numLeaves := 1 << len(branch.Leaves)
		codewordSize := 1 << p.LogCodewordSize
		if numLeaves > codewordSize || codewordSize%numLeaves != 0 {
			return fmt.Errorf("input tree %d: tree size %d incompatible with domain size %d", i, numLeaves, codewordSize)
		}
		root, err := branch.RecoverRoot(queryPosition / (codewordSize / numLeaves))
		if err != nil {
			return fmt.Errorf("input tree %d: recover root: %w", i, err)
		}
		if root != roots[i] {
			return fmt.Errorf("input tree %d: Merkle proof invalid", i)
		}
	}
	return nil
}

// bindInputTreeOpenings validates that each batch's authenticated branch carries
// a conjugate pair matching its declared shape, before reconstructQueryValueAt
// reads those same branches directly. Both rows of the pair are checked, not
// just the on-path one: the conjugate is unread by the fold today but is still
// transmitted, so an unvalidated conjugate would be a malleable proof.
func bindInputTreeOpenings(
	label string, opening InputQuery, inputIndexByBatch []int,
	levelSize int, order []int, bundle sizeBundle, shapes []Shape,
) error {
	for _, batchIdx := range order {
		branchIdx := inputIndexByBatch[batchIdx]
		if branchIdx < 0 || branchIdx >= len(opening) {
			return fmt.Errorf("%s: batch %d has no input opening", label, batchIdx)
		}
		branch := opening[branchIdx]
		pair, err := branch.pairAtLevel(levelSize)
		if err != nil {
			return fmt.Errorf("%s: tree %d level row: %w", label, branchIdx, err)
		}
		shape := shapes[batchIdx][bundle.SizeLog2]
		if !rowOpeningMatchesShape(pair[0], shape) {
			return fmt.Errorf("%s: tree %d row shape mismatch", label, branchIdx)
		}
		if !rowOpeningMatchesShape(pair[1], shape) {
			return fmt.Errorf("%s: tree %d conjugate row shape mismatch", label, branchIdx)
		}
	}
	return nil
}

func (pcs *PCS) checkClaimPointsOutOfDomain(layout layout, zeta field.Ext) error {
	// Every claim point of a size-N column is zeta * omega_N^shift, where omega_N
	// generates the size-N subgroup of that column's codeword domain. Because
	// omega_N^card == 1, we have (zeta * omega_N^shift)^card == zeta^card for any
	// shift, so a claim point lands in the codeword domain iff zeta itself does --
	// independent of the shift. It therefore suffices to test zeta once per
	// distinct size present in the layout.
	for _, bundle := range layout {
		if int(bundle.SizeLog2) >= len(pcs.Encoders) {
			return fmt.Errorf("fri: pcs.Verify: size %d is outside params schedule", bundle.SizeLog2)
		}
		encoder := pcs.Encoders[bundle.SizeLog2]
		if pointInDomain(zeta, encoder.Domain.Cardinality) {
			return fmt.Errorf("fri: pcs.Verify: size %d claim point on domain", bundle.SizeLog2)
		}
	}
	return nil
}

// checkVerifyClaimShapes fails fast when the caller-supplied ClaimedValues do
// not align with the shift schedule, one claimed value per (batch, size, row,
// shift). Without it an empty or truncated ClaimedValues would surface only
// deep inside the per-query reconstruction as an opaque error.
func checkVerifyClaimShapes(claimed []BatchClaimedValues, shifts []BatchShifts, zeta field.Ext) error {
	if len(claimed) != len(shifts) {
		return fmt.Errorf("fri: pcs.Verify: got %d claimed batches, want %d", len(claimed), len(shifts))
	}
	for batchIdx := range shifts {
		if err := validateBatchClaimShape(claimed[batchIdx], shifts[batchIdx], zeta); err != nil {
			return fmt.Errorf("fri: pcs.Verify: batch %d: %w", batchIdx, err)
		}
	}
	return nil
}

func pointInDomain(point field.Ext, size uint64) bool {
	base, ok := field.GetBase(&point)
	if !ok {
		return false
	}
	var powered field.Element
	powered.Exp(base, new(big.Int).SetUint64(size))
	one := field.One()
	return powered.Equal(&one)
}

func rowOpeningMatchesShape(row RowOpening, shape SizedShape) bool {
	return len(row.Base) == shape.BaseWidth && len(row.Ext) == shape.ExtWidth
}
