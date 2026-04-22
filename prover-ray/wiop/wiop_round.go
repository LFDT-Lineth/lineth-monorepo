package wiop

import "fmt"

// Round represents a single interaction round between the prover and the
// verifier. At the start of a round the verifier draws random coin challenges;
// the prover then responds by committing to columns and providing cell values.
// The first round of a protocol typically contains no coins.
//
// A Round is identified by its zero-based position [Round.ID] within
// [System.Rounds]. It carries no [ContextFrame]: identity is purely positional.
//
// Columns are registered in a Round indirectly through [Module.NewColumn] and
// [Module.NewExtensionColumn]; coins and cells are registered directly via
// [Round.NewCoinField] and [Round.NewCell].
type Round struct {
	// ID is the zero-based index of this round in [System.Rounds]. Set once
	// at registration time by [System.NewRound], never mutated.
	ID int
	// Coins holds the random challenges the verifier draws at the start of
	// this round, in declaration order.
	Coins []*CoinField
	// Columns holds the columns committed by the prover during this round, in
	// declaration order. Columns are appended here by [Module.NewColumn] and
	// [Module.NewExtensionColumn].
	Columns []*Column
	// Cells holds the scalar values committed by the prover during this round,
	// in declaration order.
	Cells []*Cell
	// ProverActions holds the prover-side computations registered for this round, in
	// declaration order. Each action is run by the prover when the runtime
	// enters this round, after coins have been derived.
	ProverActions []ProverAction
	// VerifierActions holds the verifier-side checks registered for this
	// round, in declaration order. Each check is run by the verifier when the
	// runtime enters this round, after coins have been derived.
	VerifierActions []VerifierAction
	// system is the owning System. Set once at registration time, never nil
	// for a well-formed Round.
	system *System
}

// RegisterAction appends a to the round's action list. Actions are run by the
// prover in declaration order when the runtime enters this round.
func (r *Round) RegisterAction(a ProverAction) {
	r.ProverActions = append(r.ProverActions, a)
}

// RegisterVerifierAction appends a to the round's verifier-action list.
// Verifier actions are run in declaration order when the runtime enters this
// round.
func (r *Round) RegisterVerifierAction(a VerifierAction) {
	r.VerifierActions = append(r.VerifierActions, a)
}

// System returns the owning System. It is always non-nil for a well-formed
// Round.
func (r *Round) System() *System { return r.system }

// Prev returns the round that immediately precedes this one in [System.Rounds]
// and true. Returns nil and false if this is the first round (ID == 0).
func (r *Round) Prev() (*Round, bool) {
	if r.ID == 0 {
		return nil, false
	}
	return r.system.Rounds[r.ID-1], true
}

// Next returns the round that immediately follows this one in [System.Rounds]
// and true. Returns nil and false if this is the last round.
func (r *Round) Next() (*Round, bool) {
	if r.ID == len(r.system.Rounds)-1 {
		return nil, false
	}
	return r.system.Rounds[r.ID+1], true
}

// NewCoinField declares a new random-coin challenge in this round, registers
// it, and returns it. Coins are always extension-field elements.
//
// Panics if ctx is nil.
func (r *Round) NewCoinField(ctx *ContextFrame) *CoinField {
	if ctx == nil {
		panic("wiop: Round.NewCoinField requires a non-nil ContextFrame")
	}
	if ctx.ID != 0 {
		panic(fmt.Sprintf("wiop: ContextFrame %q is already registered (id=%d)", ctx.Path(), ctx.ID))
	}
	ctx.ID = newCoinID(r.ID, len(r.Coins))
	coin := &CoinField{
		Context:     ctx,
		Annotations: make(Annotations),
		round:       r,
	}
	r.Coins = append(r.Coins, coin)
	return coin
}

// NewCell declares a new scalar cell in this round, registers it, and returns
// it. isExtension marks whether the cell is evaluated over an extended domain.
//
// Panics if ctx is nil.
func (r *Round) NewCell(ctx *ContextFrame, isExtension bool) *Cell {
	if ctx == nil {
		panic("wiop: Round.NewCell requires a non-nil ContextFrame")
	}
	if ctx.ID != 0 {
		panic(fmt.Sprintf("wiop: ContextFrame %q is already registered (id=%d)", ctx.Path(), ctx.ID))
	}
	ctx.ID = newCellID(r.ID, len(r.Cells))
	c := &Cell{
		Context:           ctx,
		Annotations:       make(Annotations),
		isExtensionCached: isExtension,
		round:             r,
	}
	r.Cells = append(r.Cells, c)
	return c
}

// PrecomputedRound is a special round representing offline precomputations
// that take place before the interactive protocol begins. It extends [Round]
// with two additional invariants:
//
//  1. Coins is always empty — no verifier challenges precede offline work.
//  2. Every Column declared in it has a corresponding static ConcreteVector
//     assignment. PrecomputedValues is parallel to Round.Columns: entry i is
//     the assignment for Round.Columns[i].
//
// Precomputed columns are declared via [Module.NewPrecomputedColumn].
// PrecomputedRound is created automatically by [NewSystemf]; there is exactly
// one per System and it is not part of [System.Rounds].
type PrecomputedRound struct {
	Round
	// PrecomputedValues holds the static field assignments for columns declared
	// in this round. Entry i is the assignment for Round.Columns[i].
	PrecomputedValues []*ConcreteVector
}

// Prev panics unconditionally: the precomputed round has no predecessor in
// the interactive protocol sequence.
func (pr *PrecomputedRound) Prev() (*Round, bool) {
	panic("wiop: Prev() cannot be called on PrecomputedRound")
}

// Next panics unconditionally: the precomputed round is not part of the
// interactive sequence and has no successor reachable via this method.
func (pr *PrecomputedRound) Next() (*Round, bool) {
	panic("wiop: Next() cannot be called on PrecomputedRound")
}

// addPrecomputedColumn registers col and its static assignment with this
// precomputed round. It appends col to Round.Columns and assignment to
// PrecomputedValues in lock-step, preserving the parallel-slice invariant.
//
// Panics if assignment is nil.
func (pr *PrecomputedRound) addPrecomputedColumn(col *Column, assignment *ConcreteVector) {
	if assignment == nil {
		panic(fmt.Sprintf(
			"wiop: PrecomputedRound.addPrecomputedColumn: nil assignment for column %q",
			col.Context.Path(),
		))
	}
	pr.Columns = append(pr.Columns, col)
	pr.PrecomputedValues = append(pr.PrecomputedValues, assignment)
}
