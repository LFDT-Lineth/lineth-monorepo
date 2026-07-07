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
//  2. Caller absorbs the claimed values into its transcript, derives
//     alpha_DEEP, hands it back.
//  3. PCS builds one virtual DEEP-quotient codeword per distinct native size and
//     seeds the existing FRI ProverState with those levels.
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
//	// Squeeze alphaDeep; seed the FRI prover.
//	alphaDeep  := transcript.Squeeze()
//	friState, _ := pcs.NewProverState(alphaDeep)
//
//	// FRI folding rounds: absorb each new root, squeeze the next alpha.
//	for range numFoldingRounds {
//	    alpha := transcript.Squeeze()
//	    friState.Fold(alpha)
//	    transcript.Absorb(friState.LastRoot())
//	}
//
//	// Absorb the final polynomial; squeeze query positions; open.
//	transcript.Absorb(friState.FinalPoly())
//	queries    := transcript.Squeeze()
//	proof, _   := friState.Open(queries)
//
// The verifier calls pcs.Verify with the same zeta, alphaDeep, fold alphas,
// and query positions it derived from its own transcript replay.
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
// Identical convention to the loom PCS at github.com/consensys/loom/
// internal/fri/. Decision matrix already pinned there:
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

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// =============================================================================
// PCS construction
// =============================================================================

// PCS bundles the FRI configuration and the per-size encoders into one
// receiver for Commit / AddOpening / Verify. Built once at startup, reused
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
	if len(encoders) != params.numRounds+1 {
		return nil, fmt.Errorf("fri: NewPCS: got %d encoders, want %d", len(encoders), params.numRounds+1)
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
	wantInverseRate := params.N / params.D
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
// Mirrors loom's canonicalLayout. Producer of the alpha_DEEP power
// schedule consumed by both AddOpening and Verify. Made package-internal;
// callers don't need to look inside.
type deepEntry struct {
	BatchIdx   int
	SizeLog2   int
	RowIdx     int
	IsExt      bool
	AlphaPower int
	Shifts     []int
}

type sizeBundle struct {
	SizeLog2 int
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
		bundle := sizeBundle{SizeLog2: sizeLog2}
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
					SizeLog2:   sizeLog2,
					RowIdx:     rowIdx,
					AlphaPower: alphaDeepPower,
					Shifts:     cloneInts(sizedShifts.Base[rowIdx]),
				})
				alphaDeepPower++
			}
			for rowIdx := range sizedShape.ExtWidth {
				bundle.Entries = append(bundle.Entries, deepEntry{
					BatchIdx:   batchIdx,
					SizeLog2:   sizeLog2,
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

func shapesFromBatches(batches []Batch) []Shape {
	shapes := make([]Shape, len(batches))
	for batchIdx := range batches {
		shapes[batchIdx] = make(Shape, len(batches[batchIdx]))
		for sizeLog2 := range batches[batchIdx] {
			shapes[batchIdx][sizeLog2] = SizedShape{
				BaseWidth: len(batches[batchIdx][sizeLog2].Base),
				ExtWidth:  len(batches[batchIdx][sizeLog2].Ext),
			}
		}
	}
	return shapes
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

func powers(x field.Ext, length int) []field.Ext {
	if length <= 0 {
		return nil
	}

	powers := make([]field.Ext, length)
	powers[0].SetOne()
	for i := 1; i < len(powers); i++ {
		powers[i].Mul(&powers[i-1], &x)
	}
	return powers
}

// =============================================================================
// OpeningProof
// =============================================================================

// OpeningProof bundles everything PCS verification needs to check the claimed
// evaluations against the committed batches and the underlying FRI proof.
type OpeningProof struct {
	// RowOpenings[k][b][i] contains the opened encoded row for query k, batch b, size 2^i.
	RowOpenings []QueryRowOpenings

	// FRIProof is checked through PCS verification, which reconstructs virtual
	// level values from ClaimedValues and RowOpenings.
	FRIProof Proof
}

// QueryRowOpenings holds committed-row preimages opened for one FRI query.
type QueryRowOpenings []BatchRowOpenings

// BatchRowOpenings holds one committed batch's opened rows by size_log2.
type BatchRowOpenings []SizedRowOpening

// SizedRowOpening holds one row and, for the top level, its conjugate sibling.
type SizedRowOpening struct {
	Leaf       RowOpening
	Sibling    RowOpening
	HasSibling bool
}

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

// Challenges bundles the Fiat-Shamir values supplied by the caller.
type Challenges struct {
	AlphaDeep      field.Ext
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
	shape := shapesFromBatches([]Batch{committed.EncodedTable})[0]
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

// NewProverState reconstructs the virtual DEEP quotient levels and returns the
// existing FRI prover state. It does not fold or open.
//
// The FRI schedule is restricted to the largest size actually opened, so the
// fold count follows the witness rather than the (possibly larger, static)
// Params: a size-2^k top level folds k times regardless of Params.D >= 2^k.
func (pcs *PCS) NewProverState(alphaDeep field.Ext) (*ProverState, error) {
	if len(pcs.openings) == 0 {
		return nil, fmt.Errorf("fri: NewProverState: AddOpening must be called first")
	}

	pcs, err := pcs.restrictToOpenings()
	if err != nil {
		return nil, err
	}

	levels, err := pcs.reconstructLevels(alphaDeep)
	if err != nil {
		return nil, err
	}

	state, err := NewProverState(pcs.Params, levels)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// layoutMaxSizeLog2 returns the largest size (log2) present in a canonical
// layout, i.e. the top of the FRI schedule the verifier needs.
func layoutMaxSizeLog2(l layout) int {
	maxIdx := 0
	for _, bundle := range l {
		if bundle.SizeLog2 > maxIdx {
			maxIdx = bundle.SizeLog2
		}
	}
	return maxIdx
}

// maxOpeningSizeLog2 returns the largest size (log2) opened across all pending
// openings, i.e. the top of the FRI schedule this proof actually needs.
func (pcs *PCS) maxOpeningSizeLog2() int {
	maxIdx := 0
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
func (pcs *PCS) restrictTo(topSizeLog2 int) (*PCS, error) {
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

func (pcs *PCS) shiftedPoint(sizeLog2, shift int, zeta field.Ext) (field.Ext, error) {
	if sizeLog2 < 0 || sizeLog2 >= len(pcs.Encoders) {
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

// reconstructLevels computes the virtual DEEP quotient levels
//
//	F(X) = Σ_i alpha_DEEP^i · Σ_j (f_i(X) - y_ij)/(X - z_ij)
//
// over input.Domain's bit-reversed evaluation order and stores it in
// Level.Evals. For each level, it precomputes every distinct denominator inverse 1/(x-z) with
// one Montgomery batch inversion, then walks the columns in canonical order.
func (pcs *PCS) reconstructLevels(alphaDeepChallenge field.Ext) ([]Level, error) {
	levels := make([]Level, 0, pcs.Params.numRounds+1)
	for sizeLog2 := pcs.Params.numRounds; sizeLog2 >= 0; sizeLog2-- {
		var columns []quotientColumn
		var trees []*Tree
		for _, opening := range pcs.openings {
			for _, bundle := range opening.layout {
				if bundle.SizeLog2 != sizeLog2 {
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

		domain := pcs.Params.domainsLight[pcs.Params.numRounds-sizeLog2]
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
		alphaDeepPowers := powers(alphaDeepChallenge, len(columns))

		evals := make([]field.Ext, size)
		for pos := range evals {
			for columnIdx, column := range columns {
				var columnSum field.Ext
				for _, claim := range column.Claims {
					pointIdx := claimPointIndexes[claim.Point]
					inv := denominatorInverses[pos*len(claimPoints)+pointIdx]

					var numerator, term field.Ext
					numerator.Sub(&column.Evals[pos], &claim.Value)
					term.Mul(&numerator, &inv)
					columnSum.Add(&columnSum, &term)
				}

				var weighted field.Ext
				weighted.Mul(&columnSum, &alphaDeepPowers[columnIdx])
				evals[pos].Add(&evals[pos], &weighted)
			}
		}

		levels = append(levels, Level{D: 1 << sizeLog2, Evals: evals, Trees: trees})
	}
	return levels, nil
}

func (pcs *PCS) roundForSize(sizeLog2 int) (int, error) {
	if sizeLog2 < 0 || sizeLog2 > pcs.Params.numRounds {
		return 0, fmt.Errorf("fri: reconstructLevels: size %d is outside params schedule", sizeLog2)
	}
	return pcs.Params.numRounds - sizeLog2, nil
}

func encodedColumnEvals(committed CommitterState, entry deepEntry) ([]field.Ext, error) {
	table := committed.EncodedTable
	if entry.SizeLog2 >= len(table) {
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
	if entry.SizeLog2 >= len(claimed) {
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

// OpenedRows opens, at each query position, the committed row (and, at the top
// level, its conjugate sibling) of every registered opening. Callers pair the
// result with the FRI Proof from [ProverState.Open] to assemble an OpeningProof.
func (pcs *PCS) OpenedRows(queryPositions []int) []QueryRowOpenings {
	restricted, err := pcs.restrictToOpenings()
	if err != nil {
		panic(err)
	}
	pcs = restricted

	topSizeLog2 := -1
	for _, opening := range pcs.openings {
		if len(opening.layout) > 0 && opening.layout[0].SizeLog2 > topSizeLog2 {
			topSizeLog2 = opening.layout[0].SizeLog2
		}
	}

	res := make([]QueryRowOpenings, len(queryPositions))
	for queryIdx, queryPosition := range queryPositions {
		res[queryIdx] = make(QueryRowOpenings, len(pcs.openings))
		for openingIdx, opening := range pcs.openings {
			res[queryIdx][openingIdx] = make(BatchRowOpenings, len(opening.committed.EncodedTable))
			for _, bundle := range opening.layout {
				round, _ := pcs.roundForSize(bundle.SizeLog2)
				base := queryPosition >> round

				sized := &res[queryIdx][openingIdx][bundle.SizeLog2]
				sized.Leaf = openEncodedRow(opening.committed.EncodedTable[bundle.SizeLog2], base)
				if bundle.SizeLog2 == topSizeLog2 {
					sized.Sibling = openEncodedRow(opening.committed.EncodedTable[bundle.SizeLog2], base^1)
					sized.HasSibling = true
				}
			}
		}
	}
	return res
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
	for _, base := range row.Base {
		hasher.WriteElements(base)
	}
	for _, ext := range row.Ext {
		hasher.WriteElements(ext.B0.A0, ext.B0.A1, ext.B1.A0, ext.B1.A1, ext.B2.A0, ext.B2.A1)
	}
	return hasher.SumDigest()
}

func reconstructQueryValueAt(
	pcs *PCS,
	bundle sizeBundle,
	rows QueryRowOpenings,
	claimed []BatchClaimedValues,
	zeta field.Ext,
	alphaDeep field.Ext,
	x field.Ext,
	sibling bool,
) (field.Ext, error) {
	numPowers := 0
	for _, entry := range bundle.Entries {
		if entry.AlphaPower >= numPowers {
			numPowers = entry.AlphaPower + 1
		}
	}
	alphaDeepPowers := powers(alphaDeep, numPowers)

	var value field.Ext
	for _, entry := range bundle.Entries {
		claims, err := pcs.claimsForEntry(claimed, entry, zeta)
		if err != nil {
			return field.Ext{}, err
		}
		sizedRow := rows[entry.BatchIdx][entry.SizeLog2]
		row := sizedRow.Leaf
		if sibling {
			if !sizedRow.HasSibling {
				return field.Ext{}, fmt.Errorf("fri: reconstructQueryValueAt: batch %d size %d missing sibling row",
					entry.BatchIdx, entry.SizeLog2)
			}
			row = sizedRow.Sibling
		}
		entryValue := rowValue(row, entry)
		term, err := quotientAtValue(entryValue, x, claims)
		if err != nil {
			return field.Ext{}, err
		}
		term.Mul(&term, &alphaDeepPowers[entry.AlphaPower])
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

// Verify checks an OpeningProof under the same zetas, challenges, and query
// positions the prover used.
//
// Performs in sequence:
//
//  1. Shape validation for Roots/Shapes/Shifts and row openings.
//  2. Canonical-layout build from Shapes + Shifts.
//  3. Per query: authenticate every opening -- the top level and each
//     auxiliary level against the caller-supplied rows, and every running
//     FRI layer against its committed root -- and resolve its fold values.
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
	pcs, err = pcs.restrictTo(layoutMaxSizeLog2(layout))
	if err != nil {
		return err
	}
	if err = pcs.checkClaimPointsOutOfDomain(layout, in.Zeta); err != nil {
		return err
	}
	if len(proof.RowOpenings) != pcs.Params.NumQueries {
		return fmt.Errorf("fri: pcs.Verify: proof has %d row openings, want %d",
			len(proof.RowOpenings), pcs.Params.NumQueries)
	}
	if len(in.Challenges.QueryPositions) < pcs.Params.NumQueries {
		return fmt.Errorf("fri: pcs.Verify: %d query positions, need at least %d",
			len(in.Challenges.QueryPositions), pcs.Params.NumQueries)
	}
	orders := batchOrders(layout)
	rows := proof.RowOpenings
	if err = checkRowOpenings(rows, in.Shapes, layout); err != nil {
		return err
	}
	positions := in.Challenges.QueryPositions[:pcs.Params.NumQueries]
	if err = checkOpeningProofShape(pcs.Params, len(layout)-1, proof.FRIProof, in.Challenges.FoldAlphas, positions); err != nil {
		return err
	}

	levelRoots := make([]QueryLayerRoots, len(layout))
	for i := range layout {
		levelRoots[i] = make(QueryLayerRoots, len(orders[i]))
		for j, batchIdx := range orders[i] {
			levelRoots[i][j] = in.Roots[batchIdx]
		}
	}
	runningRoots := make([]QueryLayerRoots, pcs.Params.numRounds)
	for j := 1; j < pcs.Params.numRounds; j++ {
		runningRoots[j] = QueryLayerRoots{proof.FRIProof.FRIRoots[j-1]}
	}

	topDomain := pcs.Params.domainsLight[0]
	if _, err = reconstructDomainSize(topDomain); err != nil {
		return err
	}
	claimed := in.ClaimedValues
	zeta := in.Zeta
	alphaDeep := in.Challenges.AlphaDeep

	resolved := make([]resolvedQuery, pcs.Params.NumQueries)
	for queryIdx, queryPosition := range positions {
		queryRows := rows[queryIdx]
		rq := resolvedQuery{
			Rounds: make([]inputPair, pcs.Params.numRounds),
			Aux:    make(map[int]field.Ext, len(layout)-1),
			Final:  proof.FRIProof.FinalPolyExt[queryPosition>>pcs.Params.numRounds],
		}

		// Round 0: the top level's input opening, authenticated against the
		// caller-supplied rows and reconstructed into the conjugate pair.
		topOpening := proof.FRIProof.FRIQueries[queryIdx][0]
		if err = authenticateInputBranches("round 0", topOpening, levelRoots[0],
			pcs.Params.N, queryPosition, orders[0], layout[0], queryRows, true); err != nil {
			return fmt.Errorf("fri: pcs.Verify: query %d: %w", queryIdx, err)
		}
		self, err := reconstructQueryValueAt(pcs, layout[0], queryRows, claimed, zeta, alphaDeep,
			domainPointExt(topDomain, queryPosition), false)
		if err != nil {
			return err
		}
		sib, err := reconstructQueryValueAt(pcs, layout[0], queryRows, claimed, zeta, alphaDeep,
			domainPointExt(topDomain, queryPosition^1), true)
		if err != nil {
			return err
		}
		rq.Rounds[0] = inputPair{Self: self, Sibling: sib}

		// Auxiliary levels: one authenticated value per level, mixed into the
		// fold at that level's introduction round.
		for levelIdx := 1; levelIdx < len(layout); levelIdx++ {
			bundle := layout[levelIdx]
			round, err := pcs.roundForSize(bundle.SizeLog2)
			if err != nil {
				return err
			}
			if round >= pcs.Params.numRounds {
				return fmt.Errorf("fri: pcs.Verify: level %d introduced at round %d, must be < %d",
					levelIdx, round, pcs.Params.numRounds)
			}
			domain := pcs.Params.domainsLight[round]
			levelSize, err := reconstructDomainSize(domain)
			if err != nil {
				return err
			}
			opening := proof.FRIProof.LevelQueries[levelIdx-1][queryIdx]
			if err = authenticateInputBranches(fmt.Sprintf("level %d", levelIdx), opening, levelRoots[levelIdx],
				levelSize, queryPosition>>round, orders[levelIdx], bundle, queryRows, false); err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d: %w", queryIdx, err)
			}
			value, err := reconstructQueryValueAt(pcs, bundle, queryRows, claimed, zeta, alphaDeep,
				domainPointExt(domain, queryPosition>>round), false)
			if err != nil {
				return err
			}
			rq.Aux[round] = value
		}

		// Running layers: authenticate and decode directly from the
		// committed codeword -- no PCS reconstruction involved.
		for j := 1; j < pcs.Params.numRounds; j++ {
			opening := proof.FRIProof.FRIQueries[queryIdx][j]
			if err = checkQueryLayerShape(opening, runningRoots[j], pcs.Params.N>>j, true); err != nil {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d: %w", queryIdx, j, err)
			}
			branch, err := authenticateQueryLayer(fmt.Sprintf("round %d", j), opening, runningRoots[j], queryPosition>>j)
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

		resolved[queryIdx] = rq
	}

	return checkFolds(pcs.Params, resolved, in.Challenges.FoldAlphas, positions)
}

// authenticateInputBranches authenticates opening (one branch per tree
// backing this bundle) against roots, then binds each branch's leaf -- and,
// for the top level, its deepest sibling -- to the hash of the row the
// caller opened for that tree. The row is the branch's authenticated
// preimage: this is what ties a caller-supplied row to the committed column.
func authenticateInputBranches(
	label string, opening QueryLayer, roots QueryLayerRoots,
	levelSize, base int, order []int, bundle sizeBundle, rows QueryRowOpenings, top bool,
) error {
	if err := checkQueryLayerShape(opening, roots, levelSize, false); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if err := authenticateInputOpening(label, opening, roots, levelSize, base); err != nil {
		return err
	}
	for branchIdx, batchIdx := range order {
		row := rows[batchIdx][bundle.SizeLog2]
		leaf, err := branchLeafAtLevel(opening[branchIdx], levelSize)
		if err != nil {
			return fmt.Errorf("%s: tree %d level leaf: %w", label, branchIdx, err)
		}
		if leaf != hashRowOpening(row.Leaf) {
			return fmt.Errorf("%s: tree %d row digest mismatch", label, branchIdx)
		}
		if !top {
			continue
		}
		siblings := opening[branchIdx].Siblings
		if len(siblings) == 0 || siblings[len(siblings)-1] != hashRowOpening(row.Sibling) {
			return fmt.Errorf("%s: tree %d sibling digest mismatch", label, branchIdx)
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
		if bundle.SizeLog2 < 0 || bundle.SizeLog2 >= len(pcs.Encoders) {
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

func checkRowOpenings(rows []QueryRowOpenings, shapes []Shape, layout layout) error {
	topSizeLog2 := -1
	if len(layout) > 0 {
		topSizeLog2 = layout[0].SizeLog2
	}

	for queryIdx, queryRows := range rows {
		if len(queryRows) != len(shapes) {
			return fmt.Errorf("fri: pcs.Verify: query %d has %d row-opening batches, want %d",
				queryIdx, len(queryRows), len(shapes))
		}
		for batchIdx, batchRows := range queryRows {
			if len(batchRows) != len(shapes[batchIdx]) {
				return fmt.Errorf("fri: pcs.Verify: query %d batch %d has %d row-opening size slots, want %d",
					queryIdx, batchIdx, len(batchRows), len(shapes[batchIdx]))
			}

			for sizeLog2, row := range batchRows {
				shape := shapes[batchIdx][sizeLog2]
				if !rowOpeningMatchesShape(row.Leaf, shape) {
					return fmt.Errorf("fri: pcs.Verify: query %d batch %d size %d row shape mismatch",
						queryIdx, batchIdx, sizeLog2)
				}
				if sizeLog2 == topSizeLog2 && (shape.BaseWidth != 0 || shape.ExtWidth != 0) {
					if !row.HasSibling || !rowOpeningMatchesShape(row.Sibling, shape) {
						return fmt.Errorf("fri: pcs.Verify: query %d batch %d size %d sibling shape mismatch",
							queryIdx, batchIdx, sizeLog2)
					}
				} else if row.HasSibling || !rowOpeningMatchesShape(row.Sibling, SizedShape{}) {
					return fmt.Errorf("fri: pcs.Verify: query %d batch %d size %d sibling shape mismatch",
						queryIdx, batchIdx, sizeLog2)
				}
			}
		}
	}
	return nil
}

func rowOpeningMatchesShape(row RowOpening, shape SizedShape) bool {
	return len(row.Base) == shape.BaseWidth && len(row.Ext) == shape.ExtWidth
}
