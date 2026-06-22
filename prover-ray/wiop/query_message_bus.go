package wiop

import "fmt"

// BusDirection is the sign of a [MessageBus] entry's contribution to its
// (OriginShard, Handle) accumulator.
type BusDirection int

const (
	// BusSend marks an entry that ADDS its rows to the (OriginShard, Handle)
	// accumulator. Built by [System.NewMessageBusSend].
	BusSend BusDirection = iota
	// BusReceive marks an entry that SUBTRACTS its rows (weighted by an optional
	// multiplicity) from the (OriginShard, Handle) accumulator. Built by
	// [System.NewMessageBusReceive].
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
// (OriginShard, Handle)-keyed log-up accumulator. Each instance is the unit of
// participation: one Send entry adds its rows to the accumulator; one Receive
// entry subtracts them.
//
// Semantics. Two coins α and β (extension field, shared across every
// participant of every Handle reduced together by the [messagebus] compiler)
// are drawn after every participant column is committed. For an entry with
// column views (c_0, …, c_{w-1}), define the row-folding
//
//	d(row) = β + α^{w-1}·c_0(row) + … + α·c_{w-2}(row) + c_{w-1}(row)
//
// and a per-row filter equal to Tab.Selector when present and 1 otherwise.
// The entry contributes
//
//	Send:    +Σ_row filter(row) · 1 / d(row)
//	Receive: -Σ_row filter(row) · Multiplicity(row) / d(row)         (Multiplicity = 1 if nil)
//
// to the (OriginShard, Handle) accumulator. A single [messagebus.Compile]
// invocation runs inside exactly one shard, so every entry it sees must agree
// on OriginShard. The compiler enforces that invariant and lowers all entries
// of a Handle into a single [LogDerivativeSum] holding this shard's running
// sum (the shard's "residual" on that Handle), plus a verifier action that
// asserts the residual equals an expected value (zero in the unsharded case).
// The OriginShard tag is preserved on the query so a downstream cross-shard
// layer can collect residuals from sibling shards and join them.
//
// MessageBus does not implement [GnarkCheckableQuery] nor [AssignableQuery]:
// its semantics span multiple queries within a system and are discharged by
// the dedicated compiler pass. [MessageBus.Check] is a no-op for that reason —
// the in-shard residual identity is enforced after compilation by the
// LogDerivativeSum and the verifier action.
//
// Use [System.NewMessageBusSend] and [System.NewMessageBusReceive] to
// construct instances.
type MessageBus struct {
	baseQuery
	// OriginShard names the shard whose [messagebus.Compile] call this entry
	// belongs to. Within a single Compile invocation every entry must share
	// the same OriginShard — the compiler panics on a mismatch. Across
	// shards the field lets a downstream cross-shard layer identify which
	// shard contributed which residual to the per-Handle accumulator.
	// Always non-empty.
	OriginShard string
	// Handle is the bus name. Entries with the same Handle in the same
	// Compile invocation are summed together into a single LogDerivativeSum
	// representing this shard's residual on that Handle. Always non-empty.
	Handle string
	// Direction selects the sign of the contribution. See [BusDirection].
	Direction BusDirection
	// Tab is the column tuple (with an optional selector) being sent or
	// received. The Selector field of Tab acts as a per-row filter and may be
	// nil.
	Tab Table
	// Multiplicity is an optional row-weighting expression applied on the
	// Receive side only. Always nil when Direction == BusSend; nil is also
	// permitted on the Receive side and is treated as the constant 1.
	//
	// If non-nil and vector-valued, Multiplicity must share the same module as
	// Tab. A scalar Multiplicity is allowed and applies uniformly to every row.
	Multiplicity Expression
}

// Round implements [Query]. Returns the latest [Round] across every column in
// Tab (including Tab.Selector) and, when present, Multiplicity. The bus's
// semantic check cannot be performed before this round — both α/β need every
// participant column to be committed before being sampled.
func (mb *MessageBus) Round() *Round {
	var best *Round
	if r := mb.Tab.Round(); r != nil {
		best = r
	}
	if mb.Multiplicity != nil {
		if r := maxRoundInExpr(mb.Multiplicity); r != nil && (best == nil || r.ID > best.ID) {
			best = r
		}
	}
	return best
}

// Check implements [Query]. Always returns nil. The per-Handle residual
// identity is inherently cross-query and is discharged by the [messagebus]
// compiler pass together with the [logderivativesum] compiler that follows.
func (mb *MessageBus) Check(_ Runtime) error { return nil }

// NewMessageBusSend constructs and registers a Send entry on
// (originShard, handle). The entry contributes +Σ_row filter(row) / d(row)
// to the (OriginShard, Handle) accumulator. There is no multiplicity on the
// Send side (per the message-bus spec).
//
// Invariants enforced at construction:
//   - ctx is non-nil.
//   - originShard and handle are non-empty.
//   - tab has at least one column (already enforced by [NewTable]).
//
// Panics on any invariant violation.
func (sys *System) NewMessageBusSend(ctx *ContextFrame, originShard, handle string, tab Table) *MessageBus {
	return sys.newMessageBus(ctx, originShard, handle, BusSend, tab, nil)
}

// NewMessageBusReceive constructs and registers a Receive entry on
// (originShard, handle). The entry contributes
// -Σ_row filter(row) · multiplicity(row) / d(row) to the (OriginShard,
// Handle) accumulator. multiplicity may be nil, in which case it is treated
// as the constant 1.
//
// Invariants enforced at construction:
//   - ctx is non-nil.
//   - originShard and handle are non-empty.
//   - tab has at least one column.
//   - If multiplicity is non-nil and vector-valued, its module matches
//     tab.Module().
//
// Panics on any invariant violation.
func (sys *System) NewMessageBusReceive(
	ctx *ContextFrame,
	originShard, handle string,
	tab Table,
	multiplicity Expression,
) *MessageBus {
	return sys.newMessageBus(ctx, originShard, handle, BusReceive, tab, multiplicity)
}

// newMessageBus is the shared constructor for [System.NewMessageBusSend] and
// [System.NewMessageBusReceive]. It validates the invariants and appends the
// query to [System.MessageBuses].
func (sys *System) newMessageBus(
	ctx *ContextFrame,
	originShard, handle string,
	dir BusDirection,
	tab Table,
	multiplicity Expression,
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
	if dir == BusSend && multiplicity != nil {
		panic("wiop: System.NewMessageBusSend: multiplicity must be nil on the Send side")
	}
	if multiplicity != nil {
		if m := multiplicity.Module(); m != nil && m != tab.Module() {
			panic(fmt.Sprintf(
				"wiop: System.NewMessageBusReceive: multiplicity module %q differs from tab module %q; "+
					"a vector-valued multiplicity must share the tab's module",
				m.Context.Path(), tab.Module().Context.Path(),
			))
		}
	}

	mb := &MessageBus{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		OriginShard:  originShard,
		Handle:       handle,
		Direction:    dir,
		Tab:          tab,
		Multiplicity: multiplicity,
	}
	sys.MessageBuses = append(sys.MessageBuses, mb)
	return mb
}
