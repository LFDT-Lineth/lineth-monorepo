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
// Columns are never carried in the proof, and their raw values are never
// absorbed into the Fiat-Shamir transcript. Binding them is the commitment
// scheme's job: a PCS-compiled protocol carries one commitment per committed
// round in Commitments, and [Runtime.AdvanceRound] absorbs that commitment
// instead. A protocol that was NOT PCS-compiled therefore has no witness
// binding at all -- the verifier holds no column data and cannot detect a
// tampered witness -- so running the PCS pass is what makes a protocol sound,
// not an optional final optimisation.
type Proof struct {
	Cells map[ObjectID]field.Gen
	// DynamicSizes maps module ID to their runtime size. The module ID
	// corresponds to the module's position in [System.Modules].
	DynamicSizes map[int]int
	// Commitments maps each interactive round ID to the coded Merkle commitment
	// of that round's committed columns, as produced by the PCS compiler. It is
	// empty for protocols that were not PCS-compiled. The verifier reloads it
	// into [Runtime.Commitments] so [Runtime.AdvanceRound] can replay the exact
	// Fiat-Shamir transcript, which absorbs each round's commitment in place of
	// that round's columns.
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

	// Sanity check: every column declared by the system must have been assigned
	// by the time the prover is done. Columns no longer take part in the
	// Fiat-Shamir transcript (binding them is the commitment scheme's job), so
	// this check has no bearing on the coins; it is kept purely to fail loudly
	// and precisely. A forgotten assignment is otherwise only discovered by
	// whichever consumer happens to read the column next -- a commitment action
	// or a constraint evaluation -- which reports the failure from deep inside
	// its own code without naming the round that left the hole. The check lives
	// here rather than in [Runtime.AdvanceRound] because it is prover-only:
	// [System.Verify] replays the same rounds but assigns no columns at all.
	for _, r := range sys.Rounds {
		for _, col := range r.Columns {
			if !rt.HasColumnAssignment(col) {
				utils.Panic("wiop: missing column in runtime: %v", col.Context.Path())
			}
		}
	}

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
// Verify also checks that the provided dynamic-module sizes are non-zero powers
// of two, and that the proof carries exactly one size per dynamic module.
//
// It returns an error if the proof contains a cell that the system does not
// declare, or that the replay never consumed.
func (sys *System) Verify(proof Proof, pub PublicInput) error {
	rt := NewRuntime(sys) // currentRound = r0, preloads precomputed columns

	// Restore the PCS artifacts before replaying the transcript: AdvanceRound
	// absorbs each committed round's commitment (for HasCommitment rounds) from
	// rt.Commitments, and the PCS verifier action reads the opening proof.
	for id, commitment := range proof.Commitments {
		rt.Commitments[id] = commitment
	}
	rt.PCSOpeningProof = proof.PCSOpeningProof

	// Dynamic-module sizes must be known before the transcript replay, because
	// AdvanceRound feeds them into Fiat-Shamir. The verifier holds no column
	// data, so it cannot derive them the way the prover does (as a side effect of
	// assignment) and has to take them from the proof. They are re-validated
	// (power of two, completeness) after the replay.
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

	// assignRound loads the proof's cells (and the public inputs) for r into the
	// runtime. AssignCell requires r to be the current round, so this is always
	// called on rt.CurrentRound().
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
	// exist in the system. Furthermore, we check that the size is a power of two.
	//
	// The sizes themselves are taken on trust: the verifier holds no column data,
	// so it has nothing to cross-check them against. Binding them to the witness
	// is the commitment scheme's job.
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

		rt.dynamicSizes[k] = v
	}

	// Reject sizes for modules that are not dynamic modules of the system. This
	// is checked against sys.Modules and not rt.dynamicSizes, because the latter
	// was seeded from proof.DynamicSizes above and would accept anything.
	for k := range proof.DynamicSizes {
		if k < 0 || k >= len(sys.Modules) || !sys.Modules[k].IsDynamic() {
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
