// Package pcs implements the polynomial-commitment part of the proof system as
// a wiop compilation pass.
//
// After the arithmetization passes (range check, lookup, log-derivative, local
// vanishing, global quotient) have reduced every constraint to a set of
// [wiop.LagrangeEval] claims — "column C evaluated at the point zeta equals the
// value in this cell" — the columns themselves are still transported in the
// clear inside the [wiop.Proof]. This pass closes that gap: it commits to every
// committed column with a FRI Merkle commitment, hides the columns
// ([wiop.VisibilityInternal]), and produces a single FRI opening proof that
// binds every LagrangeEval claim cell to its committed column at zeta.
//
// Compile wires three kinds of actions:
//
//   - one commit prover-action per interactive round that owns columns: it FRI-
//     commits that round's columns and records the Merkle root in
//     [Runtime.Commitments]. [Runtime.AdvanceRound] then absorbs that root into
//     Fiat-Shamir in place of the (now internal) raw columns.
//   - one opening prover-action, in a fresh final round, that batches every
//     committed round (plus the static precomputed round) into the FRI opener,
//     folds, and stores the resulting opening proof on the runtime.
//   - one verifier-action, in the same final round, that replays the same
//     Fiat-Shamir transcript and checks the opening proof.
//
// The precomputed round is static, so its commitment is computed once at compile
// time; the interactive rounds are witness-dependent, so their commitments are
// computed at prove time and transported in the [wiop.Proof].
//
// Batches are enumerated canonically as: every interactive round that owns
// columns, in round order, followed by the precomputed round if it owns columns.
// The prover's opening and the verifier's inputs share this exact ordering so
// batch b's root, shape, shifts and claims all line up.
package pcs

import (
	"fmt"
	"sync"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const (
	// friInverseRate is the FRI blow-up factor (codeword size / plaintext size).
	friInverseRate = 2
	// friNumQueries is the number of FRI query openings. This is obtained from
	// https://github.com/ethereum/soundcalc
	//
	// To match 128 bits of security, we determined that the following number of
	// queries is required.
	friNumQueries = 229
)

var (
	// maxCommittableSizeLog2 is the fixed capacity of the static FRI parameters:
	// the largest committed column size the PCS supports, 2^22 — matching the
	// wiop column-size ceiling. Every proof folds only as many rounds as its own
	// witness needs (see [fri.Params] restriction); this is just the ceiling.
	maxCommittableSizeLog2 = utils.Log2Ceil(wiop.ColumnSizeMaxSupported)
)

// The FRI parameters and encoder schedule are a pure function of the fixed
// capacity, so they are built once per process and shared across every compiled
// System. Each proof wraps them in a fresh [fri.PCS] (cheap) and folds only as
// many rounds as its witness requires.
var (
	staticFRIOnce     sync.Once
	staticFRIParams   fri.Params
	staticFRIEncoders []*fri.RSEncoder
)

// staticFRI returns the process-wide FRI parameters and encoders sized to the
// fixed maximum capacity.
func staticFRI() (fri.Params, []*fri.RSEncoder) {
	staticFRIOnce.Do(func() {
		d := 1 << maxCommittableSizeLog2
		params, err := fri.NewParams(friInverseRate*d, d, friNumQueries)
		if err != nil {
			panic(fmt.Errorf("pcs: staticFRI: %w", err))
		}
		staticFRIParams = params
		staticFRIEncoders = buildEncoders(friInverseRate, maxCommittableSizeLog2)
	})
	return staticFRIParams, staticFRIEncoders
}

// newStaticPCS wraps the shared static parameters in a fresh, per-proof [fri.PCS]
// (which carries the mutable opening state). Wrapping is cheap — no domains are
// rebuilt — and each proof restricts the fold schedule to its own witness size.
func newStaticPCS() *fri.PCS {
	params, encoders := staticFRI()
	pcs, err := fri.NewPCS(params, encoders)
	if err != nil {
		panic(fmt.Errorf("pcs: newStaticPCS: %w", err))
	}
	return pcs
}

// effectiveN is the FRI top-domain size for this proof's witness: the codeword
// size of the largest committed column. Query positions are drawn from [0, N),
// so this must match the size the PCS restricts its schedule to (both derive it
// from the same committed columns).
func effectiveN(rt *wiop.Runtime, batches []batchRef) int {
	maxSizeIndex := 0
	for _, b := range batches {
		if idx := roundMaxSizeIndex(b.round, rt); idx > maxSizeIndex {
			maxSizeIndex = idx
		}
	}
	return friInverseRate * (1 << maxSizeIndex)
}

// ColumnLocation records where a column sits inside its round's committed batch:
// the size bucket (SizeID = log2 of the padded column size), the position within
// that bucket's base or extension list, and whether it is an extension column.
type ColumnLocation struct {
	RoundID  int
	SizeID   int
	Position int
	IsExt    bool
}

// compiled is the immutable, per-Compile state captured by every action. The FRI
// parameters and encoders live in the process-wide static schedule (see
// [staticFRI]); each proof folds only as many rounds as its witness needs.
type compiled struct {
	// precomputed is the committed state of the (static) precomputed round; nil
	// when the precomputed round owns no columns. Committed once at compile time:
	// its columns are static and its encoders are a prefix of the static
	// schedule, so the root is stable across proof runs.
	precomputed     *fri.CommitterState
	precomputedRoot field.Octuplet
}

// batchRef identifies one FRI batch: an interactive round, or the precomputed
// round when isPrecomp is set.
type batchRef struct {
	round     *wiop.Round
	isPrecomp bool
}

// Compile wires the polynomial-commitment scheme onto sys. It must run last, after
// every arithmetization pass has registered its columns and [wiop.LagrangeEval]
// queries. It is a no-op when no columns are committed.
func Compile(sys *wiop.System) {
	batches := committedBatches(sys)
	if len(batches) == 0 {
		return
	}

	c := &compiled{}

	// Commit the static precomputed round once, if it owns columns. A throwaway
	// runtime exposes the (static) precomputed assignments; its encoders are a
	// prefix of the static schedule so the root is stable across proof runs.
	if len(sys.PrecomputedRound.Columns) > 0 {
		st := commitToRound(friInverseRate, &sys.PrecomputedRound.Round, wiop.NewRuntime(sys))
		c.precomputed = st
		c.precomputedRoot = st.Tree.Root()
		sys.PrecomputedCommitment = c.precomputedRoot
	}

	// For each committed interactive round: hide its columns, flag the round as
	// carrying a commitment (so AdvanceRound absorbs the root), and register the
	// commit action that computes that root at prove time.
	for _, b := range batches {
		if b.isPrecomp {
			continue
		}
		hideCommittedColumns(b.round)
		b.round.HasCommitment = true
		b.round.RegisterAction(&commitRoundAction{c: c, round: b.round})
	}

	// A fresh final round hosts the opening: putting it after every committed
	// round guarantees AdvanceRound absorbs the last committed round's root
	// before the opening squeezes alpha_DEEP.
	openingRound := sys.NewRound()
	openingRound.RegisterAction(&openingProverAction{c: c})
	openingRound.RegisterVerifierAction(&openingVerifierAction{c: c})
}

// committedBatches returns the canonical batch ordering: every interactive round
// that owns columns (in round order), then the precomputed round if it owns
// columns. Deterministic from the System alone, so prover and verifier agree.
func committedBatches(sys *wiop.System) []batchRef {
	var refs []batchRef
	for _, r := range sys.Rounds {
		if len(r.Columns) > 0 {
			refs = append(refs, batchRef{round: r})
		}
	}
	if len(sys.PrecomputedRound.Columns) > 0 {
		refs = append(refs, batchRef{round: &sys.PrecomputedRound.Round, isPrecomp: true})
	}
	return refs
}

// hideCommittedColumns turns every column of a committed round internal so it is
// neither absorbed as raw data into Fiat-Shamir nor carried in the proof; the
// commitment stands in for it. Verifier-visible (public) columns cannot be
// replaced by a commitment, so they are rejected explicitly.
func hideCommittedColumns(round *wiop.Round) {
	for i := range round.Columns {
		col := round.Columns[i]
		if col.Visibility == wiop.VisibilityPublic {
			panic(fmt.Sprintf(
				"pcs: committed round %d holds a verifier-visible (public) column %q; "+
					"public columns cannot be hidden behind a commitment",
				round.ID, col.Context.Path(),
			))
		}
		round.Columns[i].Visibility = wiop.VisibilityInternal
	}
}

// =============================================================================
// Actions
// =============================================================================

// commitRoundAction FRI-commits one interactive round's columns and records the
// Merkle root in the runtime (for the Fiat-Shamir transcript) and the full
// committed state in the runtime state bag (for the opening action).
type commitRoundAction struct {
	c     *compiled
	round *wiop.Round
}

func (a *commitRoundAction) Run(rt *wiop.Runtime) {
	st := commitToRound(friInverseRate, a.round, rt)
	rt.Commitments[a.round.ID] = st.Tree.Root()
	rt.SetState(committedStateKey(a.round.ID), st)
}

// openingProverAction batches every committed round and produces the FRI opening
// proof, storing it on the runtime for [System.Prove] to carry into the proof.
type openingProverAction struct{ c *compiled }

func (a *openingProverAction) Run(rt *wiop.Runtime) {
	proof := a.c.open(rt)
	rt.PCSOpeningProof = &proof
}

// openingVerifierAction replays the opening transcript and checks the proof.
type openingVerifierAction struct{ c *compiled }

func (a *openingVerifierAction) Check(rt *wiop.Runtime) error {
	proof, ok := rt.PCSOpeningProof.(*fri.OpeningProof)
	if !ok || proof == nil {
		return fmt.Errorf("pcs: verify: missing opening proof")
	}
	return a.c.verify(rt, *proof)
}

// =============================================================================
// Prove / Verify cores
// =============================================================================

// open runs the full prover-side opening: register every batch on a fresh FRI
// PCS, seed the DEEP quotient with alpha_DEEP, fold, and open the queries.
func (c *compiled) open(rt *wiop.Runtime) fri.OpeningProof {
	batches := committedBatches(rt.System)
	batchShifts, batchClaims, _, evalPoint := recoverBatchClaims(rt, batches)

	pcs := newStaticPCS()
	states := c.collectCommittedStates(rt, batches)
	for i := range states {
		if err := pcs.AddOpening(*states[i], evalPoint, batchShifts[i], batchClaims[i]); err != nil {
			panic(fmt.Errorf("pcs: open: AddOpening batch %d: %w", i, err))
		}
	}

	fs := rt.GetFS()
	alphaDeep := fs.RandomFext()

	// The DEEP quotient is virtual: it is reconstructed by the verifier from the
	// opened committed rows, so there is no separate quotient commitment to
	// absorb. Seeding the FRI prover with alpha_DEEP is enough.
	state, err := pcs.NewProverState(alphaDeep)
	if err != nil {
		panic(fmt.Errorf("pcs: open: %w", err))
	}

	for state.HasNext() {
		alphaFold := fs.RandomFext()
		root := state.Fold(alphaFold)
		// The last fold reveals the final polynomial and commits no root
		// (state.Fold returns the zero octuplet), so only intermediate layer
		// roots are absorbed.
		if state.HasNext() {
			fs.Update(root[:]...)
		}
	}

	fs.UpdateExt(state.FinalPolyExt...)
	positions := fs.RandomManyIntegers(pcs.Params.NumQueries, effectiveN(rt, batches))
	return fri.OpeningProof{
		RowOpenings: pcs.OpenedRows(positions),
		FRIProof:    state.Open(positions),
	}
}

// verify replays the opening transcript exactly as the prover produced it and
// checks the opening proof against the transported commitments.
func (c *compiled) verify(rt *wiop.Runtime, proof fri.OpeningProof) error {
	batches := committedBatches(rt.System)
	batchShifts, batchClaims, shapes, evalPoint := recoverBatchClaims(rt, batches)

	pcs := newStaticPCS()

	fs := rt.GetFS()
	alphaDeep := fs.RandomFext()

	// Mirror the prover's Fiat-Shamir transcript: one fold challenge per round,
	// absorbing each intermediate layer root. The final round reveals the final
	// polynomial and commits no root, so its challenge is squeezed without a
	// matching absorption.
	foldAlphas := make([]field.Ext, 0, len(proof.FRIProof.FRIRoots)+1)
	for _, friRoot := range proof.FRIProof.FRIRoots {
		foldAlphas = append(foldAlphas, fs.RandomFext())
		fs.Update(friRoot[:]...)
	}
	foldAlphas = append(foldAlphas, fs.RandomFext())

	fs.UpdateExt(proof.FRIProof.FinalPolyExt...)
	queryPositions := fs.RandomManyIntegers(pcs.Params.NumQueries, effectiveN(rt, batches))

	return pcs.Verify(fri.VerifyInputs{
		Roots:         c.collectRoots(rt, batches),
		ClaimedValues: batchClaims,
		Shapes:        shapes,
		Shifts:        batchShifts,
		Zeta:          evalPoint,
		Challenges: fri.Challenges{
			AlphaDeep:      alphaDeep,
			FoldAlphas:     foldAlphas,
			QueryPositions: queryPositions,
		},
	}, proof)
}

// collectCommittedStates returns the committed states in batch order. Interactive
// states were stashed on the runtime by the commit actions; the precomputed
// state was committed once at compile time.
func (c *compiled) collectCommittedStates(rt *wiop.Runtime, batches []batchRef) []*fri.CommitterState {
	states := make([]*fri.CommitterState, len(batches))
	for i, b := range batches {
		if b.isPrecomp {
			states[i] = c.precomputed
			continue
		}
		v, ok := rt.GetState(committedStateKey(b.round.ID))
		if !ok {
			panic(fmt.Sprintf("pcs: missing committed state for round %d", b.round.ID))
		}
		states[i] = v.(*fri.CommitterState)
	}
	return states
}

// collectRoots returns the commitment roots in batch order: interactive roots
// from the transported [Runtime.Commitments], the precomputed root from compile.
func (c *compiled) collectRoots(rt *wiop.Runtime, batches []batchRef) []field.Octuplet {
	roots := make([]field.Octuplet, len(batches))
	for i, b := range batches {
		if b.isPrecomp {
			roots[i] = c.precomputedRoot
			continue
		}
		root, ok := rt.Commitments[b.round.ID]
		if !ok {
			panic(fmt.Sprintf("pcs: missing commitment for round %d", b.round.ID))
		}
		roots[i] = root
	}
	return roots
}

func committedStateKey(roundID int) string {
	return fmt.Sprintf("pcs/committedState/%d", roundID)
}

// =============================================================================
// Layout / commitment helpers
// =============================================================================

// buildEncoders builds the encoder schedule for sizes 2^0 .. 2^maxSizeIndex at
// the given inverse rate. The schedule is a deterministic function of (rate,
// index), so a per-round schedule is always a prefix of the global one.
func buildEncoders(inverseRate, maxSizeIndex int) []*fri.RSEncoder {
	encoders := make([]*fri.RSEncoder, maxSizeIndex+1)
	for i := range encoders {
		enc := fri.NewEncoder(uint64(inverseRate)*(1<<i), 1<<i)
		encoders[i] = &enc
	}
	return encoders
}

// roundMaxSizeIndex returns the largest log2 padded size among a round's columns,
// or 0 when the round owns no columns.
func roundMaxSizeIndex(round *wiop.Round, rt *wiop.Runtime) int {
	maxSizeIndex := 0
	for _, col := range round.Columns {
		size := utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))
		if idx := utils.Log2Ceil(size); idx > maxSizeIndex {
			maxSizeIndex = idx
		}
	}
	return maxSizeIndex
}

// commitToRound sorts a round's columns into a [fri.MultiSizeTable] by padded
// size (base then extension within each size, in column-declaration order) and
// FRI-commits it with a freshly-built per-round encoder schedule (a prefix of
// the global schedule). The column ordering matches [getLayout] exactly.
func commitToRound(inverseRate int, round *wiop.Round, rt *wiop.Runtime) *fri.CommitterState {

	var (
		cols          = round.Columns
		sortedColumns = make(fri.MultiSizeTable, 64)
		maxSizeIndex  = 0
	)

	for _, col := range cols {

		size := utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))
		sizeIndex := utils.Log2Ceil(size)
		assignment := rt.GetColumnAssignment(col)

		if size != 1<<sizeIndex {
			panic("wiop: only powers of 2 are supported")
		}

		maxSizeIndex = max(maxSizeIndex, sizeIndex)

		if col.IsExtension {
			sortedColumns[sizeIndex].Ext = append(
				sortedColumns[sizeIndex].Ext,
				writeDownVectorExt(assignment, size),
			)
		} else {
			sortedColumns[sizeIndex].Base = append(
				sortedColumns[sizeIndex].Base,
				writeDownVectorBase(assignment, size),
			)
		}
	}

	committerState := fri.Commit(buildEncoders(inverseRate, maxSizeIndex), sortedColumns[:maxSizeIndex+1])
	return &committerState
}

// getLayout maps each of a round's columns to its [ColumnLocation] and returns
// the round's [fri.Shape] (per-size base/extension widths). The shape length is
// maxSizeIndex+1, matching the committed table produced by [commitToRound], and
// positions are assigned in column-declaration order so both agree.
func getLayout(round *wiop.Round, rt *wiop.Runtime) (map[wiop.ObjectID]ColumnLocation, fri.Shape) {

	var (
		cols   = round.Columns
		layout = make(map[wiop.ObjectID]ColumnLocation, len(cols))
		shape  = make(fri.Shape, 0, 8)
	)

	for _, col := range cols {

		size := utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))
		sizeIndex := utils.Log2Ceil(size)

		if size != 1<<sizeIndex {
			panic("wiop: only powers of 2 are supported")
		}

		for len(shape) <= sizeIndex {
			shape = append(shape, fri.SizedShape{})
		}

		var position int
		if col.IsExtension {
			position = shape[sizeIndex].ExtWidth
			shape[sizeIndex].ExtWidth++
		} else {
			position = shape[sizeIndex].BaseWidth
			shape[sizeIndex].BaseWidth++
		}

		layout[col.Context.ID] = ColumnLocation{
			SizeID:   sizeIndex,
			Position: position,
			IsExt:    col.IsExtension,
			RoundID:  round.ID,
		}
	}

	return layout, shape
}

// claimKey identifies a single (batch, column, shift) opening for deduplication.
type claimKey struct {
	batch    int
	sizeID   int
	isExt    bool
	position int
	shift    int
}

// recoverBatchClaims walks every [wiop.LagrangeEval] and collects, per batch, the
// shift schedule and claimed evaluations of each opened column at the (single,
// shared) evaluation point zeta. Shifts are normalized into [0, size); repeated
// (column, shift) openings are deduplicated and cross-checked for a consistent
// claimed value. It returns the per-batch shifts, claims and shapes aligned with
// batches, plus zeta.
func recoverBatchClaims(rt *wiop.Runtime, batches []batchRef) (
	[]fri.BatchShifts,
	[]fri.BatchClaimedValues,
	[]fri.Shape,
	field.Ext,
) {
	var (
		sys       = rt.System
		layouts   = make([]map[wiop.ObjectID]ColumnLocation, len(batches))
		shapes    = make([]fri.Shape, len(batches))
		shifts    = make([]fri.BatchShifts, len(batches))
		claims    = make([]fri.BatchClaimedValues, len(batches))
		batchOf   = make(map[*wiop.Round]int, len(batches))
		seen      = make(map[claimKey]field.Ext)
		evalPoint *field.Ext
	)

	for i, b := range batches {
		layouts[i], shapes[i] = getLayout(b.round, rt)
		shifts[i] = initializeBatchShift(shapes[i])
		claims[i] = initializeBatchClaims(shapes[i])
		batchOf[b.round] = i
	}

	for _, eval := range sys.LagrangeEvals {

		xExt := eval.EvaluationPoint.EvaluateSingle(rt).Value.AsExt()
		if evalPoint == nil {
			evalPoint = &xExt
		}
		if !evalPoint.Equal(&xExt) {
			panic("pcs: every LagrangeEval must share the same evaluation point")
		}

		for k, colView := range eval.Polynomials {

			round := colView.Column.Round()
			batchIdx, ok := batchOf[round]
			if !ok {
				panic(fmt.Sprintf("pcs: column %q is in a round that owns no committed batch",
					colView.Column.Context.Path()))
			}

			loc := layouts[batchIdx][colView.Column.Context.ID]
			size := 1 << loc.SizeID
			shift := ((colView.ShiftingOffset % size) + size) % size
			value := rt.GetCellValue(eval.EvaluationClaims[k]).AsExt()

			key := claimKey{batchIdx, loc.SizeID, loc.IsExt, loc.Position, shift}
			if prev, dup := seen[key]; dup {
				if !prev.Equal(&value) {
					panic("pcs: inconsistent claimed values for the same column and shift")
				}
				continue
			}
			seen[key] = value

			sizedShift := &shifts[batchIdx][loc.SizeID]
			sizedClaim := &claims[batchIdx][loc.SizeID]
			if loc.IsExt {
				sizedShift.Ext[loc.Position] = append(sizedShift.Ext[loc.Position], shift)
				sizedClaim.Ext[loc.Position] = append(sizedClaim.Ext[loc.Position], value)
			} else {
				sizedShift.Base[loc.Position] = append(sizedShift.Base[loc.Position], shift)
				sizedClaim.Base[loc.Position] = append(sizedClaim.Base[loc.Position], value)
			}
		}
	}

	if evalPoint == nil {
		panic("pcs: no LagrangeEval queries to open")
	}

	return shifts, claims, shapes, *evalPoint
}

func initializeBatchShift(shape fri.Shape) fri.BatchShifts {
	batchShifts := make(fri.BatchShifts, len(shape))
	for i, sizedShape := range shape {
		batchShifts[i] = fri.SizedShifts{
			Base: make([][]int, sizedShape.BaseWidth),
			Ext:  make([][]int, sizedShape.ExtWidth),
		}
	}
	return batchShifts
}

func initializeBatchClaims(shape fri.Shape) fri.BatchClaimedValues {
	batchClaims := make(fri.BatchClaimedValues, len(shape))
	for i, sizedShape := range shape {
		batchClaims[i] = fri.SizedClaimedValues{
			Base: make([][]field.Ext, sizedShape.BaseWidth),
			Ext:  make([][]field.Ext, sizedShape.ExtWidth),
		}
	}
	return batchClaims
}

// writeDownVectorBase materializes a base-field column assignment padded up to
// size with its constant padding value.
func writeDownVectorBase(concrete *wiop.ConcreteVector, size int) []field.Element {

	if !concrete.Plain.IsBase() {
		panic("is not base")
	}

	plainBase := concrete.Plain.AsBase()
	plain := make([]field.Element, size)
	copy(plain, plainBase)
	for i := len(plainBase); i < size; i++ {
		plain[i] = concrete.Padding
	}

	return plain
}

// writeDownVectorExt materializes an extension-field column assignment padded up
// to size with its constant (lifted) padding value.
func writeDownVectorExt(concrete *wiop.ConcreteVector, size int) []field.Ext {

	plainExt := concrete.Plain.AsExt()
	plain := make([]field.Ext, size)
	copy(plain, plainExt)
	padExt := field.Lift(concrete.Padding)
	for i := len(plainExt); i < size; i++ {
		plain[i] = padExt
	}

	return plain
}
