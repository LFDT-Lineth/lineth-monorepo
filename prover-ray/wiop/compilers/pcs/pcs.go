// Package pcs implements the polynomial-commitment part of the proof system as
// a wiop compilation pass.
//
// After the arithmetization passes (range check, lookup, log-derivative, local
// vanishing, global quotient) have reduced every constraint to a set of
// [wiop.LagrangeEval] claims — "column C evaluated at the point zeta equals the
// value in this cell" — nothing yet ties those claims to an actual witness: the
// columns never travel in the [wiop.Proof] and never enter the Fiat-Shamir
// transcript on their own. This pass closes that gap: it commits to every
// committed column with a FRI Merkle commitment and produces a single FRI
// opening proof that binds every LagrangeEval claim cell to its committed
// column at zeta.
//
// Compile wires three kinds of actions:
//
//   - one commit prover-action per interactive round that owns columns: it FRI-
//     commits that round's columns and records the Merkle root in
//     [Runtime.Commitments]. [Runtime.AdvanceRound] then absorbs that root into
//     Fiat-Shamir in place of that round's raw columns.
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
	// FRILogInverseRate is the log2 of the FRI blow-up factor (codeword size /
	// plaintext size).
	FRILogInverseRate = 1
)

// friNumQueries is the number of FRI query openings. This is obtained from
// https://github.com/ethereum/soundcalc
//
// To match 128 bits of security, we determined that the following number of
// queries is required. It is a variable (rather than a constant) so tests
// exercising the full compilation pipeline can lower it via
// [SetFRINumQueriesForTest]; production callers must never mutate it.
var friNumQueries = 229

var (
	// maxCommittableSizeLog2 is the fixed capacity of the static FRI parameters:
	// the largest committed column size the PCS supports, 2^22 — matching the
	// wiop column-size ceiling. Every proof folds only as many rounds as its own
	// witness needs (see [fri.Params] restriction); this is just the ceiling.
	maxCommittableSizeLog2 = uint8(utils.Log2Ceil(wiop.ColumnSizeMaxSupported))
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
		params, err := fri.NewParams(FRILogInverseRate+maxCommittableSizeLog2, maxCommittableSizeLog2, uint(friNumQueries))
		if err != nil {
			panic(fmt.Errorf("pcs: staticFRI: %w", err))
		}
		staticFRIParams = params
		staticFRIEncoders = buildEncoders(1<<FRILogInverseRate, maxCommittableSizeLog2)
	})
	return staticFRIParams, staticFRIEncoders
}

// FRINumQueries returns the number of FRI query openings currently configured.
// It tracks [SetFRINumQueriesForTest]; production callers must never use it to
// mutate query behaviour.
func FRINumQueries() int { return friNumQueries }

// FRIMaxCommittableSizeLog2 is the log2 of the largest committed column size the
// PCS supports — the fixed capacity of the static FRI envelope (2^22).
func FRIMaxCommittableSizeLog2() uint8 { return maxCommittableSizeLog2 }

// FRIStaticParams returns the process-wide FRI envelope parameters (sized to
// FRIMaxCommittableSizeLog2).
func FRIStaticParams() fri.Params {
	params, _ := staticFRI()
	return params
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
// size of the largest committed (Present) column. Query positions are drawn
// from [0, N), so this must match the size the PCS restricts its schedule to
// (both derive it from the same committed columns).
func effectiveN(rt *wiop.Runtime, batches []BatchRef, manifests []ColumnManifest) int {
	maxSizeIndex := 0
	for i, b := range batches {
		if idx := roundMaxSizeIndex(b.Round, rt, manifests[i]); idx > maxSizeIndex {
			maxSizeIndex = idx
		}
	}
	return 1 << (maxSizeIndex + FRILogInverseRate)
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
	// manifestCells maps each committed interactive round ID to its column
	// manifest cells, one per column in round.Columns order (see manifest.go).
	manifestCells map[int][]*wiop.Cell
	// colShifts maps each committed column to the raw shifting offsets it is
	// opened at (from every LagrangeEval), collected once at compile time. The
	// alias rule normalizes them at the runtime padded size.
	colShifts map[wiop.ObjectID][]int
	// elisionDisabled forces every manifest to all-Present.
	elisionDisabled bool
}

// BatchRef identifies one FRI batch: an interactive round, or the precomputed
// round when IsPrecomp is set.
type BatchRef struct {
	Round     *wiop.Round
	IsPrecomp bool
}

// CompileOptions tunes [Compile].
type CompileOptions struct {
	// DisableColumnElision commits every column as Present. The manifest cells
	// are still declared and transported (all zero), so the protocol shape does
	// not depend on the option; only the committed rows do. Useful to measure
	// the effect of elision or to isolate a regression.
	DisableColumnElision bool
}

// Compile wires the polynomial-commitment scheme onto sys. It must run last, after
// every arithmetization pass has registered its columns and [wiop.LagrangeEval]
// queries. It is a no-op when no columns are committed.
func Compile(sys *wiop.System, opts ...CompileOptions) {
	batches := CommittedBatches(sys)
	if len(batches) == 0 {
		return
	}

	c := &compiled{
		manifestCells: make(map[int][]*wiop.Cell),
		colShifts:     collectColumnShifts(sys),
	}
	if len(opts) > 0 {
		c.elisionDisabled = opts[0].DisableColumnElision
	}

	// Commit the static precomputed round once, if it owns columns. A throwaway
	// runtime exposes the (static) precomputed assignments; its encoders are a
	// prefix of the static schedule so the root is stable across proof runs.
	// The precomputed round is never elided.
	if len(sys.PrecomputedRound.Columns) > 0 {
		pr := &sys.PrecomputedRound.Round
		st := commitToRound(1<<FRILogInverseRate, pr, wiop.NewRuntime(sys), allPresentManifest(len(pr.Columns)))
		c.precomputed = st
		c.precomputedRoot = st.Tree.Root()
		sys.PrecomputedCommitment = c.precomputedRoot
	}

	// For each committed interactive round: flag the round as carrying a
	// commitment (so AdvanceRound absorbs the root), declare its column manifest
	// cells, and register the commit action that computes the manifest and the
	// root at prove time.
	ctx := sys.Context.Childf("pcs")
	for _, b := range batches {
		if b.IsPrecomp {
			continue
		}
		b.Round.HasCommitment = true
		cells := make([]*wiop.Cell, len(b.Round.Columns))
		for i := range cells {
			cells[i] = b.Round.NewCell(ctx.Childf("manifest-r%d-c%d", b.Round.ID, i), false)
		}
		c.manifestCells[b.Round.ID] = cells
		b.Round.RegisterAction(&commitRoundAction{c: c, round: b.Round})
	}

	// A fresh final round hosts the opening: putting it after every committed
	// round guarantees AdvanceRound absorbs the last committed round's root
	// before the opening squeezes alpha_DEEP.
	openingRound := sys.NewRound()
	openingRound.RegisterAction(&openingProverAction{c: c})
	openingRound.RegisterVerifierAction(&OpeningVerifierAction{c: c})
}

// CommittedBatches returns the canonical batch ordering: every interactive round
// that owns columns (in round order), then the precomputed round if it owns
// columns. Deterministic from the System alone, so prover and verifier agree.
func CommittedBatches(sys *wiop.System) []BatchRef {
	var refs []BatchRef
	for _, r := range sys.Rounds {
		if len(r.Columns) > 0 {
			refs = append(refs, BatchRef{Round: r})
		}
	}
	if len(sys.PrecomputedRound.Columns) > 0 {
		refs = append(refs, BatchRef{Round: &sys.PrecomputedRound.Round, IsPrecomp: true})
	}
	return refs
}

// collectColumnShifts gathers, per committed column, the raw shifting offsets of
// every LagrangeEval opening of it. Compile runs last, so the LagrangeEval set
// is complete.
func collectColumnShifts(sys *wiop.System) map[wiop.ObjectID][]int {
	out := make(map[wiop.ObjectID][]int)
	for _, eval := range sys.LagrangeEvals {
		for _, cv := range eval.Polynomials {
			id := cv.Column.Context.ID
			out[id] = append(out[id], cv.ShiftingOffset)
		}
	}
	return out
}

// =============================================================================
// Actions
// =============================================================================

// commitRoundAction decides one interactive round's column manifest, writes it
// into the round's manifest cells, FRI-commits the Present columns and records
// the Merkle root in the runtime (for the Fiat-Shamir transcript) and the full
// committed state in the runtime state bag (for the opening action).
type commitRoundAction struct {
	c     *compiled
	round *wiop.Round
}

func (a *commitRoundAction) Run(rt *wiop.Runtime) {
	vectors := materializeColumns(a.round, rt)
	manifest := a.c.buildManifest(a.round, vectors)
	if err := a.c.validateManifest(a.round, rt, manifest); err != nil {
		panic(fmt.Errorf("pcs: commit: %w", err))
	}
	a.c.assignManifest(a.round, rt, manifest)
	st := commitVectors(1<<FRILogInverseRate, vectors, manifest)
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

// OpeningVerifierAction replays the opening transcript and checks the proof.
type OpeningVerifierAction struct{ c *compiled }

func (a *OpeningVerifierAction) Check(rt *wiop.Runtime) error {
	return a.c.verify(rt, *rt.PCSOpeningProof)
}

// =============================================================================
// Prove / Verify cores
// =============================================================================

// open runs the full prover-side opening: register every batch on a fresh FRI
// PCS, seed the DEEP quotient with alpha_DEEP, fold, and open the queries.
func (c *compiled) open(rt *wiop.Runtime) fri.OpeningProof {
	batches := CommittedBatches(rt.System)
	manifests, err := c.readManifests(rt, batches)
	if err != nil {
		panic(fmt.Errorf("pcs: open: %w", err))
	}
	batchShifts, batchClaims, _, evalPoint, err := RecoverBatchClaims(rt, batches, manifests)
	if err != nil {
		panic(fmt.Errorf("pcs: open: %w", err))
	}

	pcs := newStaticPCS()
	states := c.collectCommittedStates(rt, batches)
	for i := range states {
		if err := pcs.AddOpening(*states[i], evalPoint, batchShifts[i], batchClaims[i]); err != nil {
			panic(fmt.Errorf("pcs: open: AddOpening batch %d: %w", i, err))
		}
	}

	fs := rt.GetFS()

	// The DEEP quotient is virtual: it is reconstructed by the verifier from
	// the opened committed rows, so there is no separate quotient commitment
	// to absorb, and no separate alpha_DEEP challenge either -- each level's
	// own alpha_DEEP is the square of its own introduction round's fold
	// challenge (see fri.Level.EvalsAt).
	state, err := pcs.NewProverState()
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

	fs.UpdateExt(state.FinalPoly...)
	positions := fs.RandomManyIntegers(int(pcs.Params.NumQueries), effectiveN(rt, batches, manifests))
	return pcs.Open(state, positions)
}

// verify replays the opening transcript exactly as the prover produced it and
// checks the opening proof against the transported commitments.
func (c *compiled) verify(rt *wiop.Runtime, proof fri.OpeningProof) error {
	batches := CommittedBatches(rt.System)
	manifests, err := c.readManifests(rt, batches)
	if err != nil {
		return err
	}
	batchShifts, batchClaims, shapes, evalPoint, err := RecoverBatchClaims(rt, batches, manifests)
	if err != nil {
		return err
	}

	pcs := newStaticPCS()

	fs := rt.GetFS()

	// Mirror the prover's Fiat-Shamir transcript: one fold challenge per round,
	// absorbing each intermediate layer root. The final round reveals the final
	// polynomial and commits no root, so its challenge is squeezed without a
	// matching absorption.
	foldAlphas := make([]field.Ext, 0, len(proof.FRIProof.RoundRoots)+1)
	for _, friRoot := range proof.FRIProof.RoundRoots {
		foldAlphas = append(foldAlphas, fs.RandomFext())
		fs.Update(friRoot[:]...)
	}
	foldAlphas = append(foldAlphas, fs.RandomFext())

	fs.UpdateExt(proof.FRIProof.FinalPoly...)
	queryPositions := fs.RandomManyIntegers(int(pcs.Params.NumQueries), effectiveN(rt, batches, manifests))

	return pcs.Verify(fri.VerifyInputs{
		Roots:         c.collectRoots(rt, batches),
		ClaimedValues: batchClaims,
		Shapes:        shapes,
		Shifts:        batchShifts,
		Zeta:          evalPoint,
		Challenges: fri.Challenges{
			FoldAlphas:     foldAlphas,
			QueryPositions: queryPositions,
		},
	}, proof)
}

// collectCommittedStates returns the committed states in batch order. Interactive
// states were stashed on the runtime by the commit actions; the precomputed
// state was committed once at compile time.
func (c *compiled) collectCommittedStates(rt *wiop.Runtime, batches []BatchRef) []*fri.CommitterState {
	states := make([]*fri.CommitterState, len(batches))
	for i, b := range batches {
		if b.IsPrecomp {
			states[i] = c.precomputed
			continue
		}
		v, ok := rt.GetState(committedStateKey(b.Round.ID))
		if !ok {
			panic(fmt.Sprintf("pcs: missing committed state for round %d", b.Round.ID))
		}
		states[i] = v.(*fri.CommitterState)
	}
	return states
}

// collectRoots returns the commitment roots in batch order: interactive roots
// from the transported [Runtime.Commitments], the precomputed root from compile.
func (c *compiled) collectRoots(rt *wiop.Runtime, batches []BatchRef) []field.Octuplet {
	roots := make([]field.Octuplet, len(batches))
	for i, b := range batches {
		if b.IsPrecomp {
			roots[i] = c.precomputedRoot
			continue
		}
		root, ok := rt.Commitments[b.Round.ID]
		if !ok {
			panic(fmt.Sprintf("pcs: missing commitment for round %d", b.Round.ID))
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
func buildEncoders(inverseRate, maxSizeIndex uint8) []*fri.RSEncoder {
	encoders := make([]*fri.RSEncoder, int(maxSizeIndex)+1)
	for i := range encoders {
		enc := fri.NewEncoder(uint64(inverseRate)*(1<<i), 1<<i)
		encoders[i] = &enc
	}
	return encoders
}

// roundMaxSizeIndex returns the largest log2 padded size among a round's
// Present columns, or 0 when the round owns no committed columns.
func roundMaxSizeIndex(round *wiop.Round, rt *wiop.Runtime, manifest ColumnManifest) int {
	maxSizeIndex := 0
	for i, col := range round.Columns {
		if manifest[i] != ManifestPresent {
			continue
		}
		if idx := columnSizeIndex(col, rt); idx > maxSizeIndex {
			maxSizeIndex = idx
		}
	}
	return maxSizeIndex
}

// commitToRound materializes a round's columns and commits the Present ones
// (see [commitVectors]).
func commitToRound(inverseRate uint8, round *wiop.Round, rt *wiop.Runtime, manifest ColumnManifest) *fri.CommitterState {
	return commitVectors(inverseRate, materializeColumns(round, rt), manifest)
}

// commitVectors sorts a round's Present columns into a [fri.MultiSizeTable] by
// padded size (base then extension within each size, in column-declaration
// order) and FRI-commits it with a freshly-built per-round encoder schedule (a
// prefix of the global schedule). The column ordering matches [GetLayout]
// exactly; elided columns are absent from the committed rows.
func commitVectors(inverseRate uint8, vectors []paddedColumn, manifest ColumnManifest) *fri.CommitterState {

	var (
		sortedColumns = make(fri.MultiSizeTable, 64)
		maxSizeIndex  = 0
	)

	for i := range vectors {
		if manifest[i] != ManifestPresent {
			continue
		}
		v := &vectors[i]
		maxSizeIndex = max(maxSizeIndex, v.sizeIndex)

		if v.isExt {
			sortedColumns[v.sizeIndex].Ext = append(sortedColumns[v.sizeIndex].Ext, v.ext)
		} else {
			sortedColumns[v.sizeIndex].Base = append(sortedColumns[v.sizeIndex].Base, v.base)
		}
	}
	if maxSizeIndex > 255 {
		panic("pcs: maxSizeIndex too big")
	}
	committerState := fri.Commit(buildEncoders(inverseRate, uint8(maxSizeIndex)), sortedColumns[:maxSizeIndex+1])
	return &committerState
}

// GetLayout maps each of a round's Present columns to its [ColumnLocation] and
// returns the round's [fri.Shape] (per-size base/extension widths). The shape
// length is maxSizeIndex+1 over the Present columns, matching the committed
// table produced by [commitVectors], and positions are assigned in
// column-declaration order so both agree. Elided columns have no location.
func GetLayout(round *wiop.Round, rt *wiop.Runtime, manifest ColumnManifest) (map[wiop.ObjectID]ColumnLocation, fri.Shape) {

	var (
		cols   = round.Columns
		layout = make(map[wiop.ObjectID]ColumnLocation, len(cols))
		shape  = make(fri.Shape, 0, 8)
	)

	for i, col := range cols {
		if manifest[i] != ManifestPresent {
			continue
		}

		sizeIndex := columnSizeIndex(col, rt)

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

// elidedClaim is one LagrangeEval opening of an elided column, checked after
// every Present column's schedule has been built.
type elidedClaim struct {
	batchIdx int
	colIdx   int // index in round.Columns
	code     uint32
	rawShift int
	value    field.Ext
	path     string
}

// RecoverBatchClaims walks every [wiop.LagrangeEval] and collects, per batch, the
// shift schedule and claimed evaluations of each opened Present column at the
// (single, shared) evaluation point zeta. Shifts are normalized into [0, size);
// repeated (column, shift) openings are deduplicated and cross-checked for a
// consistent claimed value. It returns the per-batch shifts, claims and shapes
// aligned with batches, plus zeta.
//
// Elided columns (see manifest.go) have no FRI claim. Their claim cells are
// pinned instead: a Zero column's claims must be zero and an aliased column's
// claims must equal the target column's claim at the same normalized shift,
// which must be part of the target's schedule. Any violation, or a malformed
// manifest, is returned as an error.
func RecoverBatchClaims(rt *wiop.Runtime, batches []BatchRef, manifests []ColumnManifest) (
	[]fri.BatchShifts,
	[]fri.BatchClaimedValues,
	[]fri.Shape,
	field.Ext,
	error,
) {
	var (
		sys       = rt.System
		layouts   = make([]map[wiop.ObjectID]ColumnLocation, len(batches))
		colIdx    = make([]map[wiop.ObjectID]int, len(batches))
		shapes    = make([]fri.Shape, len(batches))
		shifts    = make([]fri.BatchShifts, len(batches))
		claims    = make([]fri.BatchClaimedValues, len(batches))
		batchOf   = make(map[*wiop.Round]int, len(batches))
		seen      = make(map[claimKey]field.Ext)
		elided    []elidedClaim
		evalPoint *field.Ext
	)

	if len(manifests) != len(batches) {
		return nil, nil, nil, field.Ext{}, fmt.Errorf("pcs: %d manifests for %d batches", len(manifests), len(batches))
	}

	for i, b := range batches {
		if len(manifests[i]) != len(b.Round.Columns) {
			return nil, nil, nil, field.Ext{}, fmt.Errorf("pcs: round %d manifest has %d entries for %d columns",
				b.Round.ID, len(manifests[i]), len(b.Round.Columns))
		}
		layouts[i], shapes[i] = GetLayout(b.Round, rt, manifests[i])
		shifts[i] = initializeBatchShift(shapes[i])
		claims[i] = initializeBatchClaims(shapes[i])
		batchOf[b.Round] = i
		colIdx[i] = make(map[wiop.ObjectID]int, len(b.Round.Columns))
		for j, col := range b.Round.Columns {
			colIdx[i][col.Context.ID] = j
		}
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

			value := rt.GetCellValue(eval.EvaluationClaims[k]).AsExt()
			ci := colIdx[batchIdx][colView.Column.Context.ID]
			if code := manifests[batchIdx][ci]; code != ManifestPresent {
				elided = append(elided, elidedClaim{
					batchIdx: batchIdx,
					colIdx:   ci,
					code:     code,
					rawShift: colView.ShiftingOffset,
					value:    value,
					path:     colView.Column.Context.Path(),
				})
				continue
			}

			loc := layouts[batchIdx][colView.Column.Context.ID]
			size := 1 << loc.SizeID
			shift := ((colView.ShiftingOffset % size) + size) % size

			key := claimKey{batchIdx, loc.SizeID, loc.IsExt, loc.Position, shift}
			if prev, dup := seen[key]; dup {
				if !prev.Equal(&value) {
					return nil, nil, nil, field.Ext{}, fmt.Errorf(
						"pcs: inconsistent claimed values for column %q at shift %d", colView.Column.Context.Path(), shift)
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

	// Pin the elided columns' claims against the Present schedule built above.
	for _, e := range elided {
		if e.code == ManifestZero {
			if !e.value.IsZero() {
				return nil, nil, nil, field.Ext{}, fmt.Errorf(
					"pcs: column %q is elided as zero but claims a non-zero evaluation", e.path)
			}
			continue
		}
		target, ok := ManifestAliasTarget(e.code)
		if !ok {
			return nil, nil, nil, field.Ext{}, fmt.Errorf("pcs: column %q has invalid manifest code %d", e.path, e.code)
		}
		round := batches[e.batchIdx].Round
		if target >= len(round.Columns) || manifests[e.batchIdx][target] != ManifestPresent {
			return nil, nil, nil, field.Ext{}, fmt.Errorf("pcs: column %q aliases a column that is not present", e.path)
		}
		loc := layouts[e.batchIdx][round.Columns[target].Context.ID]
		size := 1 << loc.SizeID
		shift := ((e.rawShift % size) + size) % size
		key := claimKey{e.batchIdx, loc.SizeID, loc.IsExt, loc.Position, shift}
		prev, opened := seen[key]
		if !opened {
			return nil, nil, nil, field.Ext{}, fmt.Errorf(
				"pcs: column %q is opened at shift %d but its alias target is not", e.path, shift)
		}
		if !prev.Equal(&e.value) {
			return nil, nil, nil, field.Ext{}, fmt.Errorf(
				"pcs: column %q claims a different evaluation than its alias target at shift %d", e.path, shift)
		}
	}

	return shifts, claims, shapes, *evalPoint, nil
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
// size, respecting the module's padding direction so the committed polynomial
// matches the one evalLagrangePadded evaluates.
func writeDownVectorBase(concrete *wiop.ConcreteVector, size int, padding wiop.PaddingDirection) []field.Element {

	if !concrete.Plain.IsBase() {
		panic("is not base")
	}

	plainBase := concrete.Plain.AsBase()
	plain := make([]field.Element, size)

	if padding == wiop.PaddingDirectionLeft {
		gap := size - len(plainBase)
		for i := range gap {
			plain[i] = concrete.Padding
		}
		copy(plain[gap:], plainBase)
	} else {
		copy(plain, plainBase)
		for i := len(plainBase); i < size; i++ {
			plain[i] = concrete.Padding
		}
	}

	return plain
}

// writeDownVectorExt materializes an extension-field column assignment padded up
// to size, respecting the module's padding direction so the committed polynomial
// matches the one evalLagrangePadded evaluates.
func writeDownVectorExt(concrete *wiop.ConcreteVector, size int, padding wiop.PaddingDirection) []field.Ext {

	plainExt := concrete.Plain.AsExt()
	plain := make([]field.Ext, size)
	padExt := field.Lift(concrete.Padding)

	if padding == wiop.PaddingDirectionLeft {
		gap := size - len(plainExt)
		for i := range gap {
			plain[i] = padExt
		}
		copy(plain[gap:], plainExt)
	} else {
		copy(plain, plainExt)
		for i := len(plainExt); i < size; i++ {
			plain[i] = padExt
		}
	}

	return plain
}
