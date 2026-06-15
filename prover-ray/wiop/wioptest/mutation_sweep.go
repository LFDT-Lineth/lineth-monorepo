package wioptest

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// MutationOutcome records the result of corrupting one trace position and
// re-running verification.
type MutationOutcome struct {
	// Mutator is the perturbation that was applied.
	Mutator Mutator
	// Err is the verifier's verdict on the corrupted witness. A non-nil Err
	// means the mutation was rejected — the desired outcome. A panic during the
	// run is captured here as an error too, since it likewise denies the
	// cheating prover an accepting transcript.
	Err error
}

// Caught reports whether the mutation was rejected.
func (o MutationOutcome) Caught() bool { return o.Err != nil }

// MutationReport is the result of a full single-position sweep, one entry per
// corrupted position.
type MutationReport []MutationOutcome

// AnyCaught reports whether at least one mutation was rejected.
func (r MutationReport) AnyCaught() bool {
	for _, o := range r {
		if o.Caught() {
			return true
		}
	}
	return false
}

// Escaped returns the mutators whose corruption the verifier accepted. Each is
// a trace position the protocol does not constrain: an intentionally free value
// or a soundness gap to investigate.
func (r MutationReport) Escaped() []Mutator {
	var out []Mutator
	for _, o := range r {
		if !o.Caught() {
			out = append(out, o.Mutator)
		}
	}
	return out
}

// TraceTargets enumerates one Mutator per corruptible position assigned in rt:
// every Plain entry of every assigned column, the padding fill of each padded
// column, and every assigned cell. rt must already hold a transcript so that
// column lengths are known; when rt is the output of the full honest prover the
// targets span every committed value (witness, commitments, opening cells, and
// claimed aggregates), not just the prover's inputs. The returned mutators
// carry no Tweak; callers set one or rely on the default.
func TraceTargets(rt wiop.Runtime) []Mutator {
	var out []Mutator
	for _, r := range rt.System.Rounds {
		for _, c := range r.Columns {
			if !rt.HasColumnAssignment(c) {
				continue
			}
			n := rt.GetColumnAssignment(c).Plain.Len()
			for row := 0; row < n; row++ {
				out = append(out, Mutator{Column: c, Row: row})
			}
			if c.Module.Padding != wiop.PaddingDirectionNone {
				out = append(out, Mutator{Column: c, Padding: true})
			}
		}
		for _, cell := range r.Cells {
			if rt.HasCellValue(cell) {
				out = append(out, Mutator{Cell: cell})
			}
		}
	}
	return out
}

// SweepMutations corrupts committed transcript positions of sys one at a time
// and reports the verifier's verdict for each. It is the entry point for
// mutation-based soundness testing: feed in a wizard IOP and an honest prover,
// and learn which single-value corruptions slip past verification.
//
// prepare must run the honest prover to completion — installing the witness and
// every value it derives (commitments, opening cells, claimed aggregates) — so
// that tampering happens against a fully committed transcript. verify must run
// only the verifier and return a non-nil error exactly when it rejects.
// Splitting the two is essential: a flipped column must contradict the cells and
// commitments already pinned from the honest trace, not be silently re-derived.
// Typical wiring is prepare = assign-witness + [RunProver] with verify =
// [RunVerifier] for a compiled system, or prepare = the scenario's honest run
// with verify = a query's Check.
//
// For each position it builds a fresh Runtime, runs prepare, applies tweak to
// that one position, then runs verify. A nil tweak uses the Mutator default
// ([RandomValue]). prepare must populate the same positions on every call.
//
// maxAttempts caps the number of positions visited, in [TraceTargets] order;
// a value <= 0 sweeps every position. Use it to bound the cost on large traces,
// where an exhaustive sweep is one verify run per committed value.
//
// sys is reused across positions (only the Runtime is rebuilt), so it must
// already be compiled when prepare/verify rely on compiled actions.
func SweepMutations(
	sys *wiop.System,
	prepare func(*wiop.Runtime),
	verify func(wiop.Runtime) error,
	tweak Tweak,
	maxAttempts int,
) MutationReport {
	honest := wiop.NewRuntime(sys)
	prepare(&honest)

	targets := TraceTargets(honest)
	if maxAttempts > 0 && maxAttempts < len(targets) {
		targets = targets[:maxAttempts]
	}

	report := make(MutationReport, len(targets))
	for i, m := range targets {
		m.Tweak = tweak
		report[i] = MutationOutcome{
			Mutator: m,
			Err:     runOneMutation(sys, prepare, verify, m),
		}
	}
	return report
}

// runOneMutation performs a single prepare-tamper-verify cycle on a fresh
// Runtime. A panic is recovered and reported as a rejection: a corruption that
// derails the run has not produced an accepting transcript.
func runOneMutation(
	sys *wiop.System,
	prepare func(*wiop.Runtime),
	verify func(wiop.Runtime) error,
	m Mutator,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("run panicked (treated as rejection): %v", r)
		}
	}()

	rt := wiop.NewRuntime(sys)
	prepare(&rt)
	m.Apply(rt)
	return verify(rt)
}
