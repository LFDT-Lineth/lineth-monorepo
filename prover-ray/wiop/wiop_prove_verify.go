package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// Proof is the transcript produced by [System.Prove] and consumed by
// [System.Verify]. It carries:
//
//   - every committed column (Visibility >= VisibilityOracle, i.e. oracle and
//     public); internal columns are prover-only and omitted;
//   - cells (always public — see [Cell.Visibility]);
//   - the per-Runtime size of each dynamic module, so the verifier can
//     reconstruct module domains.
//
// It deliberately does NOT carry the verifier coins: [System.Verify] re-derives
// every Fiat-Shamir challenge itself by replaying the transcript, so a prover
// cannot influence the challenges by supplying forged coin values.
//
// Including the oracle columns is a testing convenience: it lets the verifier
// reconstruct the exact Fiat-Shamir state without a commitment scheme (the raw
// oracle values stand in for their commitments). In a production proof an oracle
// column is sent only as its commitment, never in full; the pipeline should
// ultimately guarantee that no VisibilityOracle column survives compilation.
type Proof struct {
	Columns map[ObjectID]*ConcreteVector
	Cells   map[ObjectID]field.Gen
	// DynamicSizes maps module ID to their runtime size. The module ID
	// corresponds to the module's position in [System.Modules].
	DynamicSizes map[int]int
}

// Prove runs the prover over every interactive round of sys and returns the
// resulting [Proof].
//
// assign is the witness hook: it is called once on a fresh [Runtime] before any
// round is processed and is responsible for assigning the first round's oracle
// columns (and any other prover inputs). This is the seam used by the zkcdriver
// (driver.AssignWithPreRead) and by the test scenarios (AssignHonest /
// AssignWitness).
//
// Prove drives the same prover loop as [wioptest.RunAndVerify]: it runs each
// round's [ProverAction]s, advancing the Fiat-Shamir transcript between rounds,
// then captures the committed columns and cells into the returned Proof. The
// verifier coins are not captured; [System.Verify] re-derives them.
//
// The caller is responsible for running the compiler passes (and, optionally,
// [Materialize]) on sys before calling Prove.
func (sys *System) Prove(assign func(rt *Runtime)) Proof {
	rt := NewRuntime(sys)
	assign(&rt)

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
		Columns:      make(map[ObjectID]*ConcreteVector),
		Cells:        make(map[ObjectID]field.Gen),
		DynamicSizes: make(map[int]int),
	}

	for _, r := range sys.Rounds {
		for _, col := range r.Columns {
			// Capture every committed column (oracle + public). These are
			// exactly the columns AdvanceRound feeds into Fiat-Shamir, so the
			// verifier needs them to re-derive the coins.
			if !rt.HasColumnAssignment(col) {
				utils.Panic("wiop: missing column in runtime: %v", col.Context.Path())
			}

			// Skip the internal columns.
			if col.Visibility < VisibilityOracle {
				continue
			}

			proof.Columns[col.Context.ID] = rt.GetColumnAssignment(col)
		}

		for _, cell := range r.Cells {
			// GetCellValue resolves lazily-assigned openings (e.g. endpoint and
			// quotient/evaluation claims) so their values are captured.
			proof.Cells[cell.Context.ID] = rt.GetCellValue(cell)
		}
	}

	for k, v := range rt.dynamicSizes {
		proof.DynamicSizes[k] = v
	}

	return proof
}

// Verify reconstructs a [Runtime] from proof and runs every verifier action
// registered on sys. It returns the first failing check, or nil if all checks
// pass.
//
// Crucially, Verify does not trust the coins: it replays the transcript round by
// round, re-deriving every Fiat-Shamir challenge with [Runtime.AdvanceRound]
// from the committed columns and cells. The prover therefore cannot forge a
// challenge. Verifier actions read the re-derived coins, the cells, and the
// public columns.
//
// Verify also checks that the provided sizes are non-zero powers of two. It
// also checks that the sizes are compatible with the oracle-visible columns.
//
// The function also panics if the proof contains any unexpected columns or
// cells. For the column it will check that their visibility is correct.
func (sys *System) Verify(proof Proof) error {
	rt := NewRuntime(sys) // currentRound = r0, preloads precomputed columns

	// assignRound loads the proof's committed columns and cells for r into the
	// runtime. AssignColumn / AssignCell require r to be the current round, so
	// this is always called on rt.CurrentRound().
	assignRound := func(r *Round) error {

		for _, col := range r.Columns {
			if col.Visibility < VisibilityOracle {
				continue
			}

			v, found := proof.Columns[col.Context.ID]
			if !found {
				return fmt.Errorf("column %q not found in proof", col.Context.Path())
			}

			rt.AssignColumn(col, v)
		}

		for _, cell := range r.Cells {
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

	// This checks that all the items of the proof have been used. Meaning all
	// cells and all columns of the proof are read.
	for id := range proof.Columns {
		col := sys.LookupColumn(id)
		if col == nil {
			return fmt.Errorf("column %q not found in system", id)
		}

		if !rt.HasColumnAssignment(col) {
			return fmt.Errorf("column %q not used in proof, its visibility is %s", col.Context.Path(), col.Visibility)
		}
	}

	for id := range proof.Cells {
		cell := sys.LookupCell(id)
		if cell == nil {
			return fmt.Errorf("cell %q not found in system", id)
		}

		if !rt.HasCellValue(cell) {
			return fmt.Errorf("cell %q not used in proof", cell.Context.Path())
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
