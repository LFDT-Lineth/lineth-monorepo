package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// Proof is the transcript produced by [System.Prove] and consumed by
// [System.Verify]. It carries:
//
//   - cells;
//   - the per-Runtime size of each dynamic module, so the verifier can
//     reconstruct module domains.
//
// It deliberately does NOT carry the verifier coins: [System.Verify] re-derives
// every Fiat-Shamir challenge itself by replaying the transcript, so a prover
// cannot influence the challenges by supplying forged coin values.
//
// Columns are not carried in the proof and are not absorbed into the
// Fiat-Shamir transcript. Binding the columns into the transcript is the job of
// the commitment scheme (the verifier will absorb each column's commitment, not
// its raw values); until that lands, the verifier has no column data and cannot
// detect a tampered witness.
type Proof struct {
	Cells map[ObjectID]field.Gen
	// DynamicSizes maps module ID to their runtime size. The module ID
	// corresponds to the module's position in [System.Modules].
	DynamicSizes map[int]int
	// Commitments maps each interactive round ID to the coded Merkle commitment
	// of that round's committed columns, as produced by the PCS compiler. It is
	// empty for protocols that were not PCS-compiled. The verifier reloads it
	// into [Runtime.Commitments] so [Runtime.AdvanceRound] can replay the exact
	// Fiat-Shamir transcript (which absorbs each commitment instead of the raw
	// oracle columns once they have been made internal).
	Commitments map[int]field.Octuplet
	// PCSOpeningProof is the FRI opening proof binding every claimed evaluation
	// to the committed columns. It is a *fri.OpeningProof, held opaquely so the
	// core wiop package does not depend on the FRI package. Nil when the protocol
	// was not PCS-compiled.
	PCSOpeningProof *fri.OpeningProof
}

// ProveOptions are options for [System.Prove].
type ProveOptions struct {
	// CheckUnreducedQueries prompt the prover to run [Query.Check] on every
	// query that has not yet been consumed by a compiler pass (i.e.
	// [Query.IsReduced] returns false). This is helpful when debugging. Not
	// needed in production.
	CheckUnreducedQueries bool
}

// Prove runs the prover over every interactive round of sys and returns the
// resulting [Proof].
//
// assign is the witness hook: it is called once on a fresh [Runtime] before any
// round is processed and is responsible for assigning the first round's
// columns (and any other prover inputs). This is the seam used by the zkcdriver
// (driver.AssignWithPreRead) and by the test scenarios (AssignHonest /
// AssignWitness).
//
// Prove drives the same prover loop as [wioptest.RunAndVerify]: it runs each
// round's [ProverAction]s, advancing the Fiat-Shamir transcript between rounds,
// then captures the cells into the returned Proof. The verifier coins are not
// captured; [System.Verify] re-derives them.
//
// The caller is responsible for running the compiler passes (and, optionally,
// [Materialize]) on sys before calling Prove.
func (sys *System) Prove(assign func(rt *Runtime), proveOpts ...ProveOptions) (Proof, PublicInput) {

	proveOpt := ProveOptions{}
	if len(proveOpts) > 0 {
		proveOpt = proveOpts[0]
	}

	rt := NewRuntime(sys)
	assign(rt)

	// Runs all the prover action and advances the Fiat-Shamir transcript
	for rt.currentRound.ID < len(sys.Rounds) {
		for _, a := range rt.CurrentRound().ProverActions {
			a.Run(rt)
		}

		if rt.currentRound.ID == len(sys.Rounds)-1 {
			break
		}

		rt.AdvanceRound()
	}

	proof := Proof{
		Cells:        make(map[ObjectID]field.Gen),
		DynamicSizes: make(map[int]int),
		Commitments:  make(map[int]field.Octuplet),
	}

	// Carry the PCS artifacts (per-round commitments and the FRI opening proof)
	// produced by the PCS compiler's actions. Both are empty/nil for protocols
	// that were not PCS-compiled.
	for id, commitment := range rt.Commitments {
		proof.Commitments[id] = commitment
	}
	proof.PCSOpeningProof = rt.PCSOpeningProof

	// Membership index for the per-cell loop below. Their values are captured
	// into the returned PublicInput, not into the proof, so the two structures
	// never overlap.
	piIdx := sys.publicInputIndex()

	for _, r := range sys.Rounds {
		for _, cell := range r.Cells {
			// Public inputs are carried separately in PublicInput.
			if _, isPI := piIdx[cell.Context.ID]; isPI {
				continue
			}

			// GetCellValue resolves lazily-assigned openings (e.g. endpoint and
			// quotient/evaluation claims) so their values are captured.
			proof.Cells[cell.Context.ID] = rt.GetCellValue(cell)
		}
	}

	for k, v := range rt.dynamicSizes {
		proof.DynamicSizes[k] = v
	}

	// Capture the registered public-input cells into a separate PublicInput, in
	// their registration order. GetCellValue resolves lazily-assigned openings,
	// so a cell opened from a column resolves to that column's value.
	pub := make(PublicInput, len(sys.PublicInputs))
	for i, cell := range sys.PublicInputs {
		pub[i] = rt.GetCellValue(cell)
	}

	if proveOpt.CheckUnreducedQueries {
		if err := sys.checkUnreducedQueries(rt); err != nil {
			panic(fmt.Sprintf("wiop: unreduced query check failed: %v", err))
		}
	}

	return proof, pub
}

// Verify reconstructs a [Runtime] from proof and runs every verifier action
// registered on sys. It returns the first failing check, or nil if all checks
// pass.
//
// Crucially, Verify does not trust the coins: it replays the transcript round by
// round, re-deriving every Fiat-Shamir challenge with [Runtime.AdvanceRound]
// from the cells. The prover therefore cannot forge a challenge. Verifier
// actions read the re-derived coins and the cells.
//
// Verify also checks that the provided sizes are non-zero powers of two.
//
// The function also panics if the proof contains any unexpected columns or
// cells. For the column it will check that their visibility is correct.
func (sys *System) Verify(proof Proof, pub PublicInput) error {
	rt := NewRuntime(sys) // currentRound = r0, preloads precomputed columns

	// Restore the PCS artifacts before replaying the transcript: AdvanceRound
	// absorbs each committed round's commitment (for HasCommitment rounds) from
	// rt.Commitments, and the PCS verifier action reads the opening proof.
	for id, commitment := range proof.Commitments {
		rt.Commitments[id] = commitment
	}
	rt.PCSOpeningProof = proof.PCSOpeningProof

	// Dynamic-module sizes must be known before the transcript replay:
	// AdvanceRound feeds them into Fiat-Shamir, and a PCS-compiled protocol hides
	// the oracle columns that would otherwise set them as a side effect of
	// assignment. Seed them up-front; they are re-validated (power of two,
	// completeness, consistency with any visible column) after the replay.
	for k, v := range proof.DynamicSizes {
		rt.dynamicSizes[k] = v
	}

	// piIdx maps each registered public-input cell to its position in pub. Their
	// values are read from pub rather than proof, enforcing the no-overlap
	// invariant between the two structures.
	piIdx := sys.publicInputIndex()
	if len(pub) != len(sys.PublicInputs) {
		return fmt.Errorf("wiop: public inputs length mismatch: got %d, want %d", len(pub), len(sys.PublicInputs))
	}

	// assignRound loads the proof's committed columns and cells (and the public
	// inputs) for r into the runtime. AssignColumn / AssignCell require r to be
	// the current round, so this is always called on rt.CurrentRound().
	assignRound := func(r *Round) error {

		for _, cell := range r.Cells {
			if pos, isPI := piIdx[cell.Context.ID]; isPI {
				rt.AssignCell(cell, pub[pos])
				continue
			}

			v, ok := proof.Cells[cell.Context.ID]
			if !ok {
				return fmt.Errorf("cell %q not found in proof", cell.Context.Path())
			}

			rt.AssignCell(cell, v)
		}

		return nil
	}

	// Replay the transcript: assign each round's committed data, then advance
	// (which feeds that data into Fiat-Shamir and re-derives the next round's
	// coins). This reproduces the prover's exact challenge sequence.
	for rt.currentRound.ID < len(sys.Rounds) {
		round := rt.CurrentRound()
		if err := assignRound(round); err != nil {
			return err
		}

		if rt.currentRound.ID == len(sys.Rounds)-1 {
			break
		}

		rt.AdvanceRound()
	}

	if rt.currentRound.ID != len(sys.Rounds)-1 {
		return fmt.Errorf("wiop: proof contains too many rounds: %v", rt.currentRound.ID)
	}

	for id := range proof.Cells {
		if pos, isPI := piIdx[id]; isPI {
			return fmt.Errorf("cell %q is a public input and must not appear in the proof", sys.PublicInputs[pos].Context.Path())
		}

		cell := sys.LookupCell(id)
		if cell == nil {
			return fmt.Errorf("cell %q not found in system", id)
		}

		if !rt.HasCellValue(cell) {
			return fmt.Errorf("cell %q not used in proof", cell.Context.Path())
		}
	}

	// Every registered public-input cell must have been consumed during the
	// transcript replay (the length of pub was checked above).
	for _, cell := range sys.PublicInputs {
		if !rt.HasCellValue(cell) {
			return fmt.Errorf("public-input cell %q not used", cell.Context.Path())
		}
	}

	// Restore dynamic-module sizes so Module.RuntimeSize resolves during the
	// verifier actions and the subsequent verifier checks. We check that the
	// proof contains all the dynamic module sizes and only module sizes that
	// exists in the system. Furthermore, we check that the size is a power of
	//two.
	for k := range sys.Modules {
		if !sys.Modules[k].IsDynamic() {
			continue
		}

		v, ok := proof.DynamicSizes[k]
		if !ok {
			return fmt.Errorf("dynamic module %d not found in proof", k)
		}

		if v == 0 || !utils.IsPowerOfTwo(v) {
			return fmt.Errorf("wiop: dynamic module %d size must be a power of two: %v", k, v)
		}

		// If the system contains dynamic-size visible columns, then the column
		// assignment function will set the dynamic size for the corresponding
		// modules. We check that these sizes are consistents in that case.
		if n, ok := rt.dynamicSizes[k]; ok && n != v {
			return fmt.Errorf("wiop: dynamic module %d size mismatch: %v vs %v", k, n, v)
		}

		rt.dynamicSizes[k] = v
	}

	for k := range proof.DynamicSizes {
		if _, ok := rt.dynamicSizes[k]; !ok {
			return fmt.Errorf("wiop: dynamic module does not exist in the system: %d", k)
		}
	}

	// This runs all the verifier actions.
	for _, r := range sys.Rounds {
		for _, va := range r.VerifierActions {
			if err := va.Check(rt); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkUnreducedQueries calls [Query.Check] on every query that has not yet
// been consumed by a compiler pass (i.e. [Query.IsReduced] returns false). It
// is intended for pre-compilation testing: it directly validates that the
// current [Runtime] assignment satisfies every raw query still pending.
//
// Returns the first error encountered, or nil if all unreduced queries pass.
func (sys *System) checkUnreducedQueries(rt *Runtime) error {
	check := func(q Query) error {
		if q.IsReduced() {
			return nil
		}
		return q.Check(rt)
	}

	for _, m := range sys.Modules {
		for _, q := range m.Vanishings {
			if err := check(q); err != nil {
				return err
			}
		}
		for _, q := range m.RangeChecks {
			if err := check(q); err != nil {
				return err
			}
		}
		for _, q := range m.NonNatives {
			if err := check(q); err != nil {
				return err
			}
		}
	}
	for _, q := range sys.LagrangeEvals {
		if err := check(q); err != nil {
			return err
		}
	}
	for _, q := range sys.TableRelations {
		if err := check(q); err != nil {
			return err
		}
	}
	for _, q := range sys.LogDerivativeSums {
		if err := check(q); err != nil {
			return err
		}
	}
	for _, q := range sys.GrandProducts {
		if err := check(q); err != nil {
			return err
		}
	}
	for _, q := range sys.MessageBuses {
		if err := check(q); err != nil {
			return err
		}
	}

	return nil
}
