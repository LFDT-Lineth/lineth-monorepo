// Package fri's PCS layer wraps the existing multi-degree FRI primitives
// (Commit / RSEncoder / Tree / ProverState / Proof) into a batch
// polynomial-commitment scheme with an Open/Verify surface.
//
// This file defines the PCS-facing types and the canonical column layout. The
// Open / Verify bodies are intentionally staged behind later commits.
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
//   - Shifts describe which rotation shifts each row must be opened at; the
//     canonical layout enumerates columns as (size desc, batch declaration order,
//     base-then-ext, row declaration order). All shifts of one column share the
//     same alpha_DEEP power.
//
// At Open time the prover:
//
//  1. Computes the claimed value of every (batch, size, row, shift) at
//     zeta * omega_N^shift.
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
// Verify mirrors steps 2-7: same challenges in, authenticates the opened
// backing trees, and reconstructs the virtual quotients inside FRI.
//
// The one-shot API accepts all Fiat-Shamir challenges up front:
//
//	pcs.Open(in OpenInputs) (OpeningProof, error)
//	pcs.Verify(in VerifyInputs, proof OpeningProof) error
//
// The prover-side staged API will return the existing [ProverState] rather than
// introduce a second PCS-specific opener state machine.
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
//     contribute to the virtual quotient. Allowing this is more flexible
//     but adds a dead-code-detection failure mode (typos in the shift
//     schedule become silent commitments). Default proposal: REJECT
//     empty shift lists, matching loom.
//
//  2. Where does the canonical-name -> (batchIdx, sizeLog2, rowIdx, isExt)
//     mapping live? Caller-side, per the precedent set by loom (the PCS
//     doesn't know column names). Worth documenting an example caller
//     that builds this mapping at compile time.
//
//  3. Encoders + Params relationship. [NewPCS] currently takes both. We
//     could derive one set of encoders from Params (since Params knows
//     rate = N / D and the size schedule), but encoders also carry FFT
//     domains which Params already precomputes. Default proposal: take
//     both; document that pcs.Encoders[i] must have PlainTextSize =
//     2^i and inverse rate == pcs.Params.N / pcs.Params.D.
package fri

import (
	"fmt"
	"math/big"
	"math/bits"

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
// ready-to-use PCS.
func NewPCS(params Params, encoders []*RSEncoder) (*PCS, error) { //nolint:revive
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
// Used by both Open (with shapes derived from witnesses) and Verify
// (with shapes passed in directly).
func canonicalLayout(shapes []Shape, shifts []BatchShifts) (layout, error) { //nolint:revive
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
		alphaPower := 0

		for batchIdx := range shapes {
			if sizeLog2 >= len(shapes[batchIdx]) {
				continue
			}

			sizedShape := shapes[batchIdx][sizeLog2]
			sizedShifts := shifts[batchIdx][sizeLog2]
			if err := validateSizedLayout(batchIdx, sizeLog2, sizedShape, sizedShifts); err != nil {
				return nil, err
			}

			for rowIdx := 0; rowIdx < sizedShape.BaseWidth; rowIdx++ {
				bundle.Entries = append(bundle.Entries, deepEntry{
					BatchIdx:   batchIdx,
					SizeLog2:   sizeLog2,
					RowIdx:     rowIdx,
					AlphaPower: alphaPower,
					Shifts:     cloneInts(sizedShifts.Base[rowIdx]),
				})
				alphaPower++
			}
			for rowIdx := 0; rowIdx < sizedShape.ExtWidth; rowIdx++ {
				bundle.Entries = append(bundle.Entries, deepEntry{
					BatchIdx:   batchIdx,
					SizeLog2:   sizeLog2,
					RowIdx:     rowIdx,
					IsExt:      true,
					AlphaPower: alphaPower,
					Shifts:     cloneInts(sizedShifts.Ext[rowIdx]),
				})
				alphaPower++
			}
		}

		if len(bundle.Entries) > 0 {
			res = append(res, bundle)
		}
	}

	return res, nil
}

// canonicalLayoutFromBatches is the prover-side entry point: shapes
// are inferred from witness row counts. Delegates to canonicalLayout.
func canonicalLayoutFromBatches(batches []Batch, shifts []BatchShifts) (layout, error) { //nolint:revive,unused
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
	return canonicalLayout(shapes, shifts)
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
	for rowIdx, rowShifts := range shifts.Base {
		if err := validateColumnShifts(rowShifts); err != nil {
			return fmt.Errorf("fri: canonicalLayout: batch %d size %d base row %d: %w",
				batchIdx, sizeLog2, rowIdx, err)
		}
	}
	for rowIdx, rowShifts := range shifts.Ext {
		if err := validateColumnShifts(rowShifts); err != nil {
			return fmt.Errorf("fri: canonicalLayout: batch %d size %d ext row %d: %w",
				batchIdx, sizeLog2, rowIdx, err)
		}
	}
	return nil
}

func validateColumnShifts(shifts []int) error {
	if len(shifts) == 0 {
		return fmt.Errorf("empty shift list")
	}
	seen := make(map[int]struct{}, len(shifts))
	for _, shift := range shifts {
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
	// AlphaPower is copied from the canonical layout's deepEntry.AlphaPower. It
	// is explicit here so reconstruction does not silently depend on the caller
	// passing columns in canonical order.
	AlphaPower int
	Evals      []field.Ext
	Claims     []quotientClaim
}

// quotientLevelInput describes one virtual quotient level. Columns must all
// belong to the same size bundle; each column carries the AlphaPower assigned by
// canonicalLayout for that bundle.
type quotientLevelInput struct {
	D       int
	Domain  domainLight
	Columns []quotientColumn
	Trees   []*Tree
}

// reconstructLevel computes the virtual DEEP quotient level
//
//	F(X) = Σ_i alphaDeep^i · Σ_j (f_i(X) - y_ij)/(X - z_ij)
//
// over input.Domain's bit-reversed evaluation order and stores it in
// Level.Evals. It precomputes every distinct denominator inverse 1/(x-z) with
// one Montgomery batch inversion, then walks the columns in canonical order.
func reconstructLevel(input quotientLevelInput, alphaDeep field.Ext) (Level, error) {
	if input.D <= 0 || input.D&(input.D-1) != 0 {
		return Level{}, fmt.Errorf("fri: reconstructLevel: D=%d is not a positive power of two", input.D)
	}

	size, err := reconstructDomainSize(input.Domain)
	if err != nil {
		return Level{}, err
	}
	for columnIdx, column := range input.Columns {
		if len(column.Evals) != size {
			return Level{}, fmt.Errorf("fri: reconstructLevel: column %d has %d evals, want %d",
				columnIdx, len(column.Evals), size)
		}
		if column.AlphaPower < 0 {
			return Level{}, fmt.Errorf("fri: reconstructLevel: column %d has negative alpha power %d",
				columnIdx, column.AlphaPower)
		}
		if err := checkColumnClaimPoints(columnIdx, column.Claims); err != nil {
			return Level{}, err
		}
	}

	domainPoints := make([]field.Ext, size)
	for pos := range domainPoints {
		domainPoints[pos] = bitReversedDomainPoint(input.Domain, pos)
	}

	claimPointIndexes, claimPoints := collectClaimPoints(input.Columns)
	denominatorInverses, err := denominatorInverses(domainPoints, claimPoints)
	if err != nil {
		return Level{}, err
	}
	alphaPowers := alphaDeepPowers(alphaDeep, maxAlphaPower(input.Columns)+1)

	evals := make([]field.Ext, size)
	for pos := range evals {
		for _, column := range input.Columns {
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
			weighted.Mul(&columnSum, &alphaPowers[column.AlphaPower])
			evals[pos].Add(&evals[pos], &weighted)
		}
	}

	return Level{
		D:     input.D,
		Evals: evals,
		Trees: input.Trees,
	}, nil
}

func reconstructDomainSize(domain domainLight) (int, error) {
	if domain.cardinality == 0 || domain.cardinality&(domain.cardinality-1) != 0 {
		return 0, fmt.Errorf("fri: reconstructLevel: domain cardinality %d is not a positive power of two",
			domain.cardinality)
	}
	return int(domain.cardinality), nil
}

func checkColumnClaimPoints(columnIdx int, claims []quotientClaim) error {
	seen := make(map[field.Ext]struct{}, len(claims))
	for claimIdx, claim := range claims {
		if _, ok := seen[claim.Point]; ok {
			return fmt.Errorf("fri: reconstructLevel: column %d has duplicate claim point at claim %d",
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
				return nil, fmt.Errorf("fri: reconstructLevel: claim point %d lands on domain position %d",
					pointIdx, pos)
			}
		}
	}
	return field.BatchInvertExt(denominators), nil
}

func bitReversedDomainPoint(domain domainLight, position int) field.Ext {
	logSize := bits.TrailingZeros64(domain.cardinality)
	exponent := bits.Reverse64(uint64(position)) >> (64 - logSize)

	var x field.Element
	x.Exp(domain.generator, big.NewInt(int64(exponent)))
	return field.Lift(x)
}

func maxAlphaPower(columns []quotientColumn) int {
	maxPower := -1
	for _, column := range columns {
		if column.AlphaPower > maxPower {
			maxPower = column.AlphaPower
		}
	}
	return maxPower
}

func alphaDeepPowers(alphaDeep field.Ext, length int) []field.Ext {
	if length <= 0 {
		return nil
	}

	powers := make([]field.Ext, length)
	powers[0].SetOne()
	for i := 1; i < len(powers); i++ {
		powers[i].Mul(&powers[i-1], &alphaDeep)
	}
	return powers
}

// =============================================================================
// OpeningProof
// =============================================================================

// OpeningProof bundles everything Verify needs to check that every
// polynomial in every committed Batch evaluates to the listed values
// at zeta and at the rotation shifts in BatchShifts.
type OpeningProof struct {
	// ClaimedValues[b] mirrors shifts[b] exactly. The outer protocol
	// reads these to evaluate its constraints at zeta and to bind into the
	// alpha_DEEP transcript challenge.
	ClaimedValues []BatchClaimedValues

	// FRIProof is the underlying multi-degree FRI proof. Already
	// verifiable on its own (via [Verify]) under the same fold
	// challenges and query positions the PCS used.
	FRIProof Proof
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

	Challenges Challenges
}

// Challenges bundles the Fiat-Shamir values supplied by the caller.
type Challenges struct {
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
//	for each (b, sizeLog2, rowIdx, isExt) in canonical layout order:
//	    fs.absorb(claimed values at this entry's shifts)
//	fs.sample(AlphaDeep)
//	for j in 0..numRounds-1:
//	    fs.absorb(running-layer root produced by fold j-1, when present)
//	    fs.sample(FoldAlphas[j])
//	fs.absorb(final polynomial)
//	for k in 0..NumQueries-1:
//	    fs.sample(QueryPositions[k]) // mod N/2
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

	Challenges Challenges
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
//  3. fri.Verify over the virtual quotient levels with FoldAlphas + query
//     positions.
//  4. Reconstruct the virtual quotient values from authenticated FRI branches
//     and compare them to the FRI fold checks.
func (pcs *PCS) Verify(in VerifyInputs, proof OpeningProof) error { //nolint:revive // design stub
	panic("TODO(pcs): Verify")
}

// =============================================================================
// Verifier-side helpers
// =============================================================================
//
// A verifier-side VerifierState is an option but is NOT proposed for v1.
// Reason: verification is more linear than proving, so the one-shot pcs.Verify
// is sufficient. If a use case emerges for a coin-fed verifier (e.g.
// incremental verification), add it then.

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
