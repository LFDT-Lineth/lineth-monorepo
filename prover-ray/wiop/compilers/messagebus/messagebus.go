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
// where d_h(row) = β + α^w + α^{w-1}·c_0(row) + … + c_{w-1}(row) and w is the
// row's own participant width. The leading α^w is a length sentinel: it makes
// the fold injective across widths, so participants of a single handle may
// differ in width (see foldDenominator) without a short tuple aliasing a
// longer zero-padded one. Equivalently, the multiset of rows sent into h
// equals the multiset of rows received from h, weighted by the receiver-side
// multiplicities. The same α, β are reused across handles and across widths;
// handles remain independent residuals because each is asserted by its own
// verifier action. See [wiop.MessageBus] for the per-entry semantics.
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

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
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

	// No cross-participant width check: foldDenominator binds each row's width
	// into its fold via an α^w length sentinel, so participants of one handle
	// may differ in width without a short tuple aliasing a zero-padded longer
	// one. (Widths naturally differ across handles too.)

	// Per handle: aggregate every entry's contribution into one accumulator
	// holding this shard's residual on that handle. The reduction is declared
	// per entry and must be uniform within a handle:
	//   - ReduceLogUp:      a LogDerivativeSum residual (additive, expected 0).
	//   - ReducePermutation: a GrandProduct accumulator (multiplicative,
	//     expected 1), discharged later by grandproduct.Compile.
	cellByHandle := make(map[string]*wiop.Cell, len(handles))
	reductionByHandle := make(map[string]wiop.BusReduction, len(handles))
	for _, h := range handles {
		entries := byHandle[h]
		red := handleReduction(h, entries)
		reductionByHandle[h] = red
		switch red {
		case wiop.ReducePermutation:
			nums, dens := buildPermutationFactors(alpha, beta, entries)
			gp := sys.NewGrandProduct(compCtx.Childf("handle-%s", h), nums, dens)
			cellByHandle[h] = gp.Result
		default:
			fractions := buildFractions(alpha, beta, entries)
			ld := sys.NewLogDerivativeSum(compCtx.Childf("handle-%s", h), fractions)
			cellByHandle[h] = ld.Result
		}
	}

	// One in-shard verifier action per handle: this shard's accumulator on the
	// handle must equal Expected — zero for a LogUp residual, one for a
	// permutation product (both in the unsharded case). Suppressed when
	// System.MessageBusSkipInShardCheck is set, so a downstream cross-shard
	// layer can own the consistency check instead; it reads the Reduction tag
	// to decide whether to sum residuals to zero or multiply products to one.
	if !sys.MessageBusSkipInShardCheck {
		for _, h := range handles {
			expected := field.ElemZero()
			if reductionByHandle[h] == wiop.ReducePermutation {
				expected = field.ElemOne()
			}
			resultRound.RegisterVerifierAction(&CheckHandleSumInShard{
				Handle:    h,
				Cell:      cellByHandle[h],
				Path:      compCtx.Childf("handle-%s", h).Childf("residual").Path(),
				Expected:  expected,
				Reduction: reductionByHandle[h],
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
// where d_h(row) = β + α^w + α^{w-1}·c_0(row) + … + c_{w-1}(row) is the
// width-binding fold from foldDenominator (the α^w sentinel lets participants
// of a handle differ in width). A nil Multiplicity on the Receive side becomes
// the constant 1 (so the numerator is just -1).
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

// handleReduction returns the reduction shared by every entry of a handle,
// panicking if the entries disagree. A handle is discharged by exactly one
// argument, so mixing log-derivative and permutation entries under one handle
// is a misuse.
func handleReduction(handle string, entries []*wiop.MessageBus) wiop.BusReduction {
	red := entries[0].Reduction
	for _, mb := range entries[1:] {
		if mb.Reduction != red {
			panic(fmt.Sprintf(
				"wiop/compilers/messagebus: handle %q mixes reductions: %v at %q vs %v at %q; "+
					"all entries of a handle must declare the same reduction",
				handle, red, entries[0].Context().Path(), mb.Reduction, mb.Context().Path(),
			))
		}
	}
	return red
}

// buildPermutationFactors turns the entries of a permutation handle into the
// grand-product factor lists: each Send contributes one numerator factor and
// each Receive one denominator factor. The shard's accumulator is then
// ∏send factor / ∏recv factor, equal to one iff the selected-send and
// selected-receive row multisets coincide. There are no multiplicities on this
// path (enforced at construction).
func buildPermutationFactors(
	alpha, beta *wiop.CoinField,
	entries []*wiop.MessageBus,
) (nums, dens []wiop.Expression) {
	for _, mb := range entries {
		factor := permutationFold(alpha, beta, mb.Tab)
		switch mb.Direction {
		case wiop.BusSend:
			nums = append(nums, factor)
		case wiop.BusReceive:
			dens = append(dens, factor)
		default:
			panic(fmt.Sprintf(
				"wiop/compilers/messagebus: unknown BusDirection %v at %q",
				mb.Direction, mb.Context().Path(),
			))
		}
	}
	return nums, dens
}

// permutationFold returns the per-row grand-product factor for one entry:
//
//	selector·(β + fold(row)) + (1 − selector)
//
// where β + fold(row) is the width-binding fold from foldDenominator
// (β + α^w + α^{w-1}·c_0 + … + c_{w-1}, the α^w sentinel letting participants
// of a handle differ in width). A selected row contributes β + fold(row) and
// an unselected row contributes the neutral factor 1 (dropping out of the
// product). With no selector the factor is simply β + fold(row). The selector
// is assumed {0,1}-valued and zero on padding rows — the same assumption the
// log-derivative path makes of its filters.
func permutationFold(alpha, beta *wiop.CoinField, tab wiop.Table) wiop.Expression {
	fold := foldDenominator(alpha, beta, tab.Columns)
	if tab.Selector == nil {
		return fold
	}
	sel := wiop.Expression(tab.Selector)
	one := wiop.NewConstantField(field.NewFromString("1"))
	return wiop.Add(wiop.Mul(sel, fold), wiop.Sub(one, sel))
}

// foldDenominator returns the width-binding row fold
//
//	β + α^w + α^{w-1}·c_0 + … + α·c_{w-2} + c_{w-1}
//
// where w = len(cols). The α^w "length sentinel" makes the encoding injective
// across widths: two rows fold to the same polynomial in α only if they have
// the same width AND the same entries, so participants of a handle may safely
// differ in width — a shorter tuple can no longer alias a longer one with
// leading zeros. Same-width participants get the same sentinel, so a balanced
// bus stays balanced.
//
// The sentinel is folded in for free. Evaluating the coefficient sequence
// [1, c_0, …, c_{w-1}] at α by Horner is exactly α^w + α^{w-1}·c_0 + … +
// c_{w-1}; seeding acc = α + c_0 collapses the leading 1·α + c_0 step, so the
// sentinel costs one extra addition and NO extra multiplication over the plain
// RLC. α is always consulted, including the width-1 case (β + α + c_0).
func foldDenominator(alpha, beta *wiop.CoinField, cols []*wiop.ColumnView) wiop.Expression {
	// acc = α + c_0 fuses the first two Horner steps (1·α + c_0), seeding the
	// coefficient sequence [1, c_0, …] that carries the α^w length sentinel.
	acc := wiop.Add(alpha, cols[0])
	for _, c := range cols[1:] {
		acc = wiop.Add(wiop.Mul(acc, alpha), c)
	}
	return wiop.Add(beta, acc)
}

// CheckHandleSumInShard is the verifier action that closes the in-shard half
// of the message-bus reduction: the accumulator cell produced for one handle on
// this shard must equal [CheckHandleSumInShard.Expected]. The cell is a
// LogDerivativeSum residual (additive, expected zero) or a GrandProduct product
// (multiplicative, expected one), per [CheckHandleSumInShard.Reduction]. For a
// single-shard protocol the expected value is zero / one respectively; the
// field exists so a sharded protocol can instantiate this action with the value
// the cross-shard layer expects to see on this shard.
type CheckHandleSumInShard struct {
	// Handle names the bus this check belongs to. Diagnostic-only.
	Handle string
	// Cell is the accumulator result holding this shard's residual/product on
	// Handle. A single Compile call produces exactly one cell per handle —
	// the action is therefore a single-cell equality check.
	Cell *wiop.Cell
	// Path is the qualified ContextFrame path of the check, used in error
	// messages.
	Path string
	// Expected is the value Cell must hold on this shard. Constant — fixed
	// at action-construction time, not derived from any other runtime
	// state. [Compile] sets this to [field.ElemZero] for a LogUp handle and
	// [field.ElemOne] for a permutation handle; sharded callers that bypass
	// [Compile]'s built-in registration may construct the action directly with
	// a different value.
	Expected field.Gen
	// Reduction records how this handle's accumulator composes across shards:
	// [wiop.ReduceLogUp] residuals sum to zero, [wiop.ReducePermutation]
	// products multiply to one. A cross-shard aggregation layer reads it to
	// pick the right global identity.
	Reduction wiop.BusReduction
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
