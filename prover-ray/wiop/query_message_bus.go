package wiop

import "fmt"

// BusDirection is the sign of a [MessageBus] entry's contribution to its
// (OriginShard, Handle) accumulator.
type BusDirection int

const (
	// BusSend marks an entry that ADDS its rows to the (OriginShard, Handle)
	// accumulator. Built by [System.NewMessageBusSend].
	BusSend BusDirection = iota
	// BusReceive marks an entry that SUBTRACTS its rows from the
	// (OriginShard, Handle) accumulator. Built by [System.NewMessageBusReceive].
	BusReceive
)

// String returns a human-readable label for d, used in diagnostics.
func (d BusDirection) String() string {
	switch d {
	case BusSend:
		return "Send"
	case BusReceive:
		return "Receive"
	default:
		return fmt.Sprintf("BusDirection(%d)", int(d))
	}
}

// MessageBus is a [Query] declaring that one [Table] participates in an
// (OriginShard, Handle)-keyed grand-product (permutation) accumulator. Each
// instance is the unit of participation: one Send entry multiplies its rows
// into the accumulator's numerator; one Receive entry multiplies them into the
// denominator.
//
// Semantics. Two coins α and β (extension field, shared across every
// participant of every Handle reduced together by the [messagebus] compiler)
// are drawn after every participant column is committed. For an entry with
// column views (c_0, …, c_{w-1}), define the row-folding
//
//	d(row) = β + α^w + α^{w-1}·c_0(row) + … + α·c_{w-2}(row) + c_{w-1}(row)
//
// where the leading α^w is a length sentinel binding the entry's own width w,
// so participants of a Handle may differ in width. A per-row filter equal to
// Tab.Selector when present and 1 otherwise gates each row: a selected row
// contributes its fold and an unselected row contributes the neutral factor 1.
// The entry contributes its selected rows to the (OriginShard, Handle)
// accumulator as
//
//	Send:    ∏_selected-row d(row)   (numerator)
//	Receive: ∏_selected-row d(row)   (denominator)
//
// The grand-product argument proves the selected-send and selected-receive row
// multisets are equal and therefore carries no multiplicities (every selected
// row counts exactly once). A single [messagebus.Compile] invocation runs
// inside exactly one shard, so every entry it sees must agree on OriginShard.
// The compiler enforces that invariant and lowers all entries of a Handle into
// a single [GrandProduct] holding this shard's running product (the shard's
// accumulator on that Handle), plus a verifier action that asserts the product
// equals an expected value (one in the unsharded case). The OriginShard tag is
// preserved on the query so a downstream cross-shard layer can collect products
// from sibling shards and join them.
//
// MessageBus does not implement [GnarkCheckableQuery] nor [AssignableQuery]:
// its semantics span multiple queries within a system and are discharged by
// the dedicated compiler pass. [MessageBus.Check] is a no-op for that reason —
// the in-shard product identity is enforced after compilation by the
// GrandProduct and the verifier action.
//
// Use [System.NewMessageBusSend] and [System.NewMessageBusReceive] to
// construct instances.
type MessageBus struct {
	baseQuery
	// OriginShard names the shard whose [messagebus.Compile] call this entry
	// belongs to. Within a single Compile invocation every entry must share
	// the same OriginShard — the compiler panics on a mismatch. Across
	// shards the field lets a downstream cross-shard layer identify which
	// shard contributed which product to the per-Handle accumulator.
	// Always non-empty.
	OriginShard string
	// Handle is the bus name. Entries with the same Handle in the same
	// Compile invocation are combined into a single GrandProduct representing
	// this shard's product on that Handle. Always non-empty.
	Handle string
	// Direction selects the sign of the contribution. See [BusDirection].
	Direction BusDirection
	// Tab is the column tuple (with an optional selector) being sent or
	// received. The Selector field of Tab acts as a per-row filter and may be
	// nil.
	Tab Table
	// SkipInShardCheck controls whether the messagebus compiler registers a
	// per-handle in-shard verifier action ([messagebus.CheckHandleSumInShard])
	// for the handle this entry belongs to. When false (the default) the
	// compiler registers the action, asserting the per-handle GrandProduct
	// equals one on this shard. Set to true when a downstream cross-shard layer
	// owns the consistency check and the in-shard product must remain unasserted
	// so it can be carried over to the cross-shard identity.
	//
	// All entries that share the same Handle must agree on this field — the
	// compiler panics on a mismatch.
	SkipInShardCheck bool
}

// Round implements [Query]. Returns the latest [Round] across every column in
// Tab (including Tab.Selector). The bus's semantic check cannot be performed
// before this round — both α/β need every participant column to be committed
// before being sampled.
func (mb *MessageBus) Round() *Round {
	return mb.Tab.Round()
}

// Check implements [Query]. Always returns nil. The per-Handle product
// identity is inherently cross-query and is discharged by the [messagebus]
// compiler pass together with the [grandproduct] compiler that follows.
func (mb *MessageBus) Check(_ *Runtime) error { return nil }

// NewMessageBusSend constructs and registers a grand-product Send entry on
// (originShard, handle). It multiplies the entry's selected rows into the
// accumulator's numerator. A selector on tab restricts participation to the
// selected rows; there is no multiplicity (every selected row counts exactly
// once).
//
// Invariants enforced at construction:
//   - ctx is non-nil.
//   - originShard and handle are non-empty.
//   - tab has at least one column (already enforced by [NewTable]).
//
// Panics on any invariant violation.
func (sys *System) NewMessageBusSend(ctx *ContextFrame, originShard, handle string, tab Table) *MessageBus {
	return sys.newMessageBus(ctx, originShard, handle, BusSend, tab)
}

// NewMessageBusReceive constructs and registers a grand-product Receive entry
// on (originShard, handle). It multiplies the entry's selected rows into the
// accumulator's denominator. The grand-product argument proves multiset
// equality and so carries no multiplicity — hence the signature has no
// multiplicity parameter (multiplicity-freeness is structural, not a runtime
// check).
//
// Invariants enforced at construction:
//   - ctx is non-nil.
//   - originShard and handle are non-empty.
//   - tab has at least one column.
//
// Panics on any invariant violation.
func (sys *System) NewMessageBusReceive(ctx *ContextFrame, originShard, handle string, tab Table) *MessageBus {
	return sys.newMessageBus(ctx, originShard, handle, BusReceive, tab)
}

// newMessageBus is the shared constructor for the message-bus entry builders.
// It validates the invariants and appends the query to [System.MessageBuses].
func (sys *System) newMessageBus(
	ctx *ContextFrame,
	originShard, handle string,
	dir BusDirection,
	tab Table,
) *MessageBus {
	if ctx == nil {
		panic("wiop: System.NewMessageBus* requires a non-nil ContextFrame")
	}
	if originShard == "" {
		panic("wiop: System.NewMessageBus*: originShard must be non-empty")
	}
	if handle == "" {
		panic("wiop: System.NewMessageBus*: handle must be non-empty")
	}
	if len(tab.Columns) == 0 {
		panic("wiop: System.NewMessageBus*: tab must have at least one column")
	}

	mb := &MessageBus{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		OriginShard: originShard,
		Handle:      handle,
		Direction:   dir,
		Tab:         tab,
	}
	sys.MessageBuses = append(sys.MessageBuses, mb)
	return mb
}
