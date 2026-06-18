// Package messagebus implements the LogUp message-bus compiler pass for the
// wiop protocol framework.
//
// A single [Compile] invocation runs inside exactly one shard, so every
// unreduced [wiop.MessageBus] entry it sees is expected to share the same
// [wiop.MessageBus.OriginShard] (the compiler panics on a mismatch). The
// pass consumes those entries and emits, for each Handle, a single
// [wiop.LogDerivativeSum] holding this shard's running sum on that Handle —
// i.e. the shard's "residual". By Schwartz–Zippel over two extension-field
// coins α, β (shared across every participant of every Handle reduced by
// this pass), the residual is zero, for each Handle h, iff
//
//	∑_{Send entries on h}  Σ_row filter(row) ·  1               / d_h(row)
//	    =
//	∑_{Recv entries on h}  Σ_row filter(row) ·  Multiplicity(row) / d_h(row)
//
// where d_h(row) = β + α^{w_h-1}·c_0(row) + … + c_{w_h-1}(row). Equivalently,
// the multiset of rows sent into h equals the multiset of rows received from
// h, weighted by the receiver-side multiplicities. The same α, β are reused
// across handles (each handle just folds with α raised to powers up to its
// own width); handles remain independent residuals because each is asserted
// by its own verifier action. See [wiop.MessageBus] for the per-entry
// semantics.
//
// The pass allocates α and β itself, via [Round.NewCoinField] on a fresh
// (or reused) coin round immediately after the latest participant round.
// In a sharded protocol the caller is expected to pre-allocate that coin
// round and register a [Round.RegisterPreSamplingHook] entry on it that
// calls [Runtime.SetFSState] with shared randomness derived from a
// cross-shard handoff. The compiler's ensureRoundAfter reuses any
// pre-existing tail round at the right position, so messagebus's coin
// allocation lands on the same round the hook is registered on — and every
// shard's α, β therefore derive from the seeded FS state instead of the
// local transcript.
//
// Caller order: invoke messagebus.Compile(sys) BEFORE
// logderivativesum.Compile(sys); the latter discharges the LogDerivativeSums
// this pass emits.
package messagebus

import (
	"fmt"
	"sort"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
)

// Compile reduces every unreduced [wiop.MessageBus] entry in sys to a
// collection of [wiop.LogDerivativeSum] queries (one per handle) plus one
// [wiop.VerifierAction] per handle that asserts the shard's residual equals
// the expected value (zero in the unsharded case). See the package
// documentation for the full reduction.
//
// The pass appends up to two fresh interactive rounds to sys.Rounds: a
// coin round where the shared α and β are declared, and a result round
// where the [wiop.LogDerivativeSum] result cells and the per-handle
// verifier action live. Either round may already exist at the right
// position (e.g. when a sharded protocol pre-allocates the coin round to
// attach a [Round.RegisterPreSamplingHook]); ensureRoundAfter reuses
// existing tail rounds rather than appending duplicates.
//
// Panics if the unreduced entries do not all share the same
// [wiop.MessageBus.OriginShard] — Compile is a per-shard operation and
// mixing shards in one call is a misuse.
//
// Already-reduced entries are skipped; remaining unreduced entries are marked
// reduced on return.
func Compile(sys *wiop.System) {
	// Collect every unreduced MessageBus entry in declaration order, indexed by
	// handle. Sort the handles for deterministic round/coin/cell ordering
	// across runs.
	byHandle := map[string][]*wiop.MessageBus{}
	var anyEntry *wiop.MessageBus
	for _, mb := range sys.MessageBuses {
		if mb.IsReduced() {
			continue
		}
		if anyEntry == nil {
			anyEntry = mb
		} else if mb.OriginShard != anyEntry.OriginShard {
			panic(fmt.Sprintf(
				"wiop/compilers/messagebus: Compile is a per-shard operation but the system contains entries "+
					"from different shards: %q at %q vs %q at %q",
				anyEntry.OriginShard, anyEntry.Context().Path(),
				mb.OriginShard, mb.Context().Path(),
			))
		}
		byHandle[mb.Handle] = append(byHandle[mb.Handle], mb)
	}
	if len(byHandle) == 0 {
		return
	}
	handles := make([]string, 0, len(byHandle))
	for h := range byHandle {
		handles = append(handles, h)
	}
	sort.Strings(handles)

	compCtx := sys.Context.Childf("message-bus")

	// Allocate the shared (α, β) coins on a fresh — or pre-existing — coin
	// round immediately after the latest participant round. A sharded
	// protocol typically pre-allocates this round so it can register a
	// PreSamplingHook that seeds FS with cross-shard shared randomness;
	// ensureRoundAfter reuses any tail round already at this position
	// rather than appending a duplicate.

	// Find the highest-ID round any participant column or multiplicity touches.
	maxParticipantRound := latestParticipantRound(byHandle)
	// Pick the slot directly after the participants — allocate a fresh round
	// if empty, reuse any round already sitting there. The reuse path is what
	// lands α/β on the *same* round a sharded caller pre-allocated for a
	// PreSamplingHook, so the hook's SetFSState fires immediately before this
	// round's coin sampling.
	coinRound := ensureRoundAfter(sys, maxParticipantRound)
	// Declare α on that round — sampled by AdvanceRound, after any pre-sampling hook fires.
	alpha := coinRound.NewCoinField(compCtx.Childf("alpha"))
	// Declare β on the same round, drawn from the same Fiat–Shamir state as α.
	beta := coinRound.NewCoinField(compCtx.Childf("beta"))

	// The result round (where LDS cells and the verifier action live) sits
	// strictly after the coin round so the LDS prover action sees α and β
	// already sampled.
	resultRound := ensureRoundAfter(sys, coinRound)

	// Per-handle: validate uniform width within a handle. Widths may differ
	// across handles (each handle just raises the shared α to powers up to
	// its own width).
	for _, h := range handles {
		entries := byHandle[h]
		width := entries[0].Tab.Width()
		for _, mb := range entries[1:] {
			if mb.Tab.Width() != width {
				panic(fmt.Sprintf(
					"wiop/compilers/messagebus: handle %q has a participant of width %d at %q "+
						"but expected %d (set by the first participant at %q); all participants of a handle must share a width",
					h, mb.Tab.Width(), mb.Context().Path(), width, entries[0].Context().Path(),
				))
			}
		}
	}

	// Per handle: aggregate every entry's contribution into one
	// LogDerivativeSum holding this shard's residual on that handle.
	cellByHandle := make(map[string]*wiop.Cell, len(handles))
	for _, h := range handles {
		fractions := buildFractions(alpha, beta, byHandle[h])
		ld := sys.NewLogDerivativeSum(compCtx.Childf("handle-%s", h), fractions)
		cellByHandle[h] = ld.Result
	}

	// One in-shard verifier action per handle: this shard's residual on the
	// handle must equal Expected (zero in the unsharded case). Suppressed
	// when System.MessageBusSkipInShardCheck is set, so a downstream
	// cross-shard layer can own the consistency check instead.
	if !sys.MessageBusSkipInShardCheck {
		for _, h := range handles {
			resultRound.RegisterVerifierAction(&CheckHandleSumInShard{
				Handle:   h,
				Cell:     cellByHandle[h],
				Path:     compCtx.Childf("handle-%s", h).Childf("residual").Path(),
				Expected: field.ElemZero(),
			})
		}
	}

	// Mark every consumed entry as reduced.
	for _, h := range handles {
		for _, mb := range byHandle[h] {
			mb.MarkAsReduced()
		}
	}
}

// latestParticipantRound returns the [wiop.Round] with the highest ID among
// the participant columns and multiplicity expressions of every unreduced
// MessageBus entry, or nil if no entry references a round-bearing leaf.
func latestParticipantRound(byHandle map[string][]*wiop.MessageBus) *wiop.Round {
	var best *wiop.Round
	update := func(r *wiop.Round) {
		if r != nil && (best == nil || r.ID > best.ID) {
			best = r
		}
	}
	for _, entries := range byHandle {
		for _, mb := range entries {
			update(mb.Round())
		}
	}
	return best
}

// ensureRoundAfter returns a round with ID > after.ID, reusing the existing
// tail round when one already sits in that slot; otherwise appending a fresh
// round via sys.NewRound. after may be nil, in which case the returned round
// is sys.Rounds[0] (allocated if absent).
func ensureRoundAfter(sys *wiop.System, after *wiop.Round) *wiop.Round {
	startID := -1
	if after != nil {
		startID = after.ID
	}
	for len(sys.Rounds) <= startID+1 {
		sys.NewRound()
	}
	return sys.Rounds[startID+1]
}

// buildFractions turns every MessageBus entry on one handle into a
// [wiop.Fraction] suitable for [wiop.System.NewLogDerivativeSum].
//
// Each entry contributes one fraction:
//
//	Send:    Filter = Tab.Selector, Numerator = +1,             Denominator = d_h(row)
//	Receive: Filter = Tab.Selector, Numerator = -Multiplicity,  Denominator = d_h(row)
//
// where d_h(row) = β + α^{w-1}·c_0(row) + … + c_{w-1}(row). For width-1 tabs
// the Horner loop is empty and the fold reduces to β + c_0(row); α is still
// passed in but never multiplied. A nil Multiplicity on the Receive side
// becomes the constant 1 (so the numerator is just -1).
func buildFractions(
	alpha *wiop.CoinField,
	beta *wiop.CoinField,
	entries []*wiop.MessageBus,
) []wiop.Fraction {
	one := wiop.NewConstantField(field.NewFromString("1"))

	fractions := make([]wiop.Fraction, 0, len(entries))
	for _, mb := range entries {
		den := foldDenominator(alpha, beta, mb.Tab.Columns)

		var num wiop.Expression
		switch mb.Direction {
		case wiop.BusSend:
			num = one
		case wiop.BusReceive:
			weight := mb.Multiplicity
			if weight == nil {
				weight = one
			}
			num = wiop.Negate(weight)
		default:
			panic(fmt.Sprintf(
				"wiop/compilers/messagebus: unknown BusDirection %v at %q",
				mb.Direction, mb.Context().Path(),
			))
		}

		var filter wiop.Expression
		if mb.Tab.Selector != nil {
			filter = mb.Tab.Selector
		}

		fractions = append(fractions, wiop.Fraction{
			Filter:      filter,
			Numerator:   num,
			Denominator: den,
		})
	}
	return fractions
}

// foldDenominator returns the expression β + α^{w-1}·c_0 + … + α·c_{w-2} +
// c_{w-1}, evaluated as a Horner pass over cols. For width-1 tabs the loop
// is empty and the result is β + c_0; α is not consulted in that case but
// must still be non-nil so callers don't need to special-case it.
func foldDenominator(alpha, beta *wiop.CoinField, cols []*wiop.ColumnView) wiop.Expression {
	acc := wiop.Expression(cols[0])
	for _, c := range cols[1:] {
		acc = wiop.Add(wiop.Mul(acc, alpha), c)
	}
	return wiop.Add(beta, acc)
}

// CheckHandleSumInShard is the verifier action that closes the in-shard half
// of the message-bus reduction: the LogDerivativeSum cell produced for one
// handle on this shard — the shard's residual on that handle — must equal
// [CheckHandleSumInShard.Expected]. For a single-shard protocol the expected
// value is always zero; the field exists so a sharded protocol can
// instantiate this action with the residual the cross-shard layer expects
// to see on this shard.
type CheckHandleSumInShard struct {
	// Handle names the bus this check belongs to. Diagnostic-only.
	Handle string
	// Cell is the LogDerivativeSum result holding this shard's residual on
	// Handle. A single Compile call produces exactly one cell per handle —
	// the action is therefore a single-cell equality check, not a sum.
	Cell *wiop.Cell
	// Path is the qualified ContextFrame path of the check, used in error
	// messages.
	Path string
	// Expected is the value Cell must hold on this shard. Constant — fixed
	// at action-construction time, not derived from any other runtime
	// state. [Compile] sets this to [field.ElemZero] for the single-shard
	// case; sharded callers that bypass [Compile]'s built-in registration
	// may construct the action directly with a non-zero value.
	Expected field.Gen
}

// Check implements [wiop.VerifierAction]. Reads the residual cell and
// returns an error if it differs from [CheckHandleSumInShard.Expected].
func (h *CheckHandleSumInShard) Check(rt wiop.Runtime) error {
	got := rt.GetCellValue(h.Cell)
	diff := got.Sub(h.Expected)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/messagebus: handle %q (%s): residual is %v, expected %v",
			h.Handle, h.Path, got, h.Expected,
		)
	}
	return nil
}
