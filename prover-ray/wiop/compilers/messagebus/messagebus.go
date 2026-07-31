// Package messagebus implements the grand-product message-bus compiler pass
// for the wiop protocol framework.
//
// A single [Compile] invocation runs inside exactly one shard, so every
// unreduced [wiop.MessageBus] entry it sees is expected to share the same
// [wiop.MessageBus.OriginShard] (the compiler panics on a mismatch). The
// pass consumes those entries and emits, for each Handle, a single
// [wiop.GrandProduct] holding this shard's running product on that Handle —
// i.e. the shard's accumulator. By Schwartz–Zippel over two extension-field
// coins α, β (shared across every participant of every Handle reduced by
// this pass), the product is one, for each Handle h, iff
//
//	∏_{Send entries on h}    ∏_selected-row d_h(row)
//	    =
//	∏_{Recv entries on h}    ∏_selected-row d_h(row)
//
// where d_h(row) = β + α^w + α^{w-1}·c_0(row) + … + c_{w-1}(row) and w is the
// row's own participant width. The leading α^w is a length sentinel: it makes
// the fold injective across widths, so participants of a single handle may
// differ in width (see foldDenominator) without a short tuple aliasing a
// longer zero-padded one. Equivalently, the multiset of rows sent into h
// equals the multiset of rows received from h. The same α, β are reused across
// handles and across widths; handles remain independent products because each
// is asserted by its own verifier action. See [wiop.MessageBus] for the
// per-entry semantics.
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
// grandproduct.Compile(sys); the latter discharges the GrandProducts this
// pass emits.
package messagebus

import (
	"fmt"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Compile reduces every unreduced [wiop.MessageBus] entry in sys to a
// collection of [wiop.GrandProduct] queries (one per handle) plus one
// [wiop.VerifierAction] per handle that asserts the shard's product equals
// the expected value (one in the unsharded case). See the package
// documentation for the full reduction.
//
// The pass appends up to two fresh interactive rounds to sys.Rounds: a
// coin round where the shared α and β are declared, and a result round
// where the [wiop.GrandProduct] result cells and the per-handle verifier
// action live. Either round may already exist at the right position (e.g.
// when a sharded protocol pre-allocates the coin round to attach a
// [Round.RegisterPreSamplingHook]); ensureRoundAfter reuses existing tail
// rounds rather than appending duplicates.
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

	// Find the highest-ID round any participant column touches.
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

	// The result round (where GrandProduct cells and the verifier action live)
	// sits strictly after the coin round so the GrandProduct prover action sees
	// α and β already sampled.
	resultRound := ensureRoundAfter(sys, coinRound)

	// No cross-participant width check: foldDenominator binds each row's width
	// into its fold via an α^w length sentinel, so participants of one handle
	// may differ in width without a short tuple aliasing a zero-padded longer
	// one. (Widths naturally differ across handles too.)

	// Per handle: combine every entry's contribution into one GrandProduct
	// accumulator holding this shard's product on that handle (expected 1),
	// discharged later by grandproduct.Compile.
	cellByHandle := make(map[string]*wiop.Cell, len(handles))
	for _, h := range handles {
		entries := byHandle[h]
		nums, dens := buildPermutationFactors(alpha, beta, entries)
		gp := sys.NewGrandProduct(compCtx.Childf("handle-%s", h), nums, dens)
		cellByHandle[h] = gp.Result

		sys.PublicInputs.Register(wiop.MessageBusPI, gp.Result)

	}

	// One in-shard verifier action per handle: this shard's product on the
	// handle must equal one (in the unsharded case). Suppressed when every
	// entry for the handle has SkipInShardCheck set, so a downstream
	// cross-shard layer can own the consistency check instead. All entries for
	// a handle must agree on SkipInShardCheck — the compiler panics on a mismatch.
	for _, h := range handles {
		entries := byHandle[h]
		skip := entries[0].SkipInShardCheck
		for _, e := range entries[1:] {
			if e.SkipInShardCheck != skip {
				panic(fmt.Sprintf(
					"wiop/compilers/messagebus: entries for handle %q disagree on SkipInShardCheck: "+
						"%q has %v but %q has %v",
					h,
					entries[0].Context().Path(), skip,
					e.Context().Path(), e.SkipInShardCheck,
				))
			}
		}
		if !skip {
			resultRound.RegisterVerifierAction(&CheckHandleSumInShard{
				Handle:   h,
				Cell:     cellByHandle[h],
				Path:     compCtx.Childf("handle-%s", h).Childf("residual").Path(),
				Expected: field.ElemOne(),
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
// the participant columns of every unreduced MessageBus entry, or nil if no
// entry references a round-bearing leaf.
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

// buildPermutationFactors turns the entries of a handle into the grand-product
// factor lists: each Send contributes one numerator factor and each Receive
// one denominator factor. The shard's accumulator is then
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
// is assumed {0,1}-valued and zero on padding rows.
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
// of the message-bus reduction: the GrandProduct cell produced for one handle
// on this shard must equal [CheckHandleSumInShard.Expected] (a multiplicative
// product, expected one). For a single-shard protocol the expected value is
// one; the field exists so a sharded protocol can instantiate this action with
// the value the cross-shard layer expects to see on this shard.
type CheckHandleSumInShard struct {
	// Handle names the bus this check belongs to. Diagnostic-only.
	Handle string
	// Cell is the accumulator result holding this shard's product on Handle. A
	// single Compile call produces exactly one cell per handle — the action is
	// therefore a single-cell equality check.
	Cell *wiop.Cell
	// Path is the qualified ContextFrame path of the check, used in error
	// messages.
	Path string
	// Expected is the value Cell must hold on this shard. Constant — fixed
	// at action-construction time, not derived from any other runtime
	// state. [Compile] sets this to [field.ElemOne]; sharded callers that
	// bypass [Compile]'s built-in registration may construct the action
	// directly with a different value.
	Expected field.Gen
}

// Check implements [wiop.VerifierAction]. Reads the product cell and
// returns an error if it differs from [CheckHandleSumInShard.Expected].
func (h *CheckHandleSumInShard) Check(rt *wiop.Runtime) error {
	got := rt.GetCellValue(h.Cell)
	diff := got.Sub(h.Expected)
	if !diff.IsZero() {
		return fmt.Errorf(
			"wiop/compilers/messagebus: handle %q (%s): product is %v, expected %v",
			h.Handle, h.Path, got, h.Expected,
		)
	}
	return nil
}
