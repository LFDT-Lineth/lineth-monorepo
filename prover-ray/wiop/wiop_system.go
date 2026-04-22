package wiop

import "github.com/consensys/linea-monorepo/prover-ray/utils/arena"

// System is the top-level container for an abstract cryptographic protocol.
// It owns all rounds, modules, and the single precomputed round. It is also
// the primary entry-point for constructing protocol objects: modules are
// created via [System.NewModule] / [System.NewSizedModule], and rounds via
// [System.NewRound].
type System struct {
	// Context is the root ContextFrame of the protocol hierarchy. All
	// sub-objects derive their identity from this root.
	Context *ContextFrame
	// PrecomputedRound is the special round for offline precomputations. It is
	// created automatically by [NewSystemf] and is always non-nil.
	PrecomputedRound *PrecomputedRound
	// Rounds holds the interactive rounds of the protocol in declaration order.
	// Each round's ID equals its index in this slice.
	Rounds []*Round
	// Modules holds all modules registered with this system in declaration order.
	Modules []*Module
	// LagrangeEvals holds all [LagrangeEval] queries registered with this
	// system via [System.NewLagrangeEval] and [System.NewLagrangeEvalFrom],
	// in declaration order.
	LagrangeEvals []*LagrangeEval
	// TableRelations holds all [TableRelation] queries registered with this
	// system via [System.NewPermutation] and [System.NewInclusion], in
	// declaration order.
	TableRelations []*TableRelation
	// LogDerivativeSums holds all [LogDerivativeSum] queries registered with
	// this system via [System.NewLogDerivativeSum], in declaration order.
	LogDerivativeSums []*LogDerivativeSum
	// scratchArena backs the [PlanningContext] used by [Materialize]. It is
	// nil until Materialize is called.
	scratchArena *arena.VectorArena
}

// NewSystemf constructs an empty System. It creates a root [ContextFrame]
// using the formatted name as its label, then initialises the PrecomputedRound.
// msg and args follow [fmt.Sprintf] conventions.
func NewSystemf(msg string, args ...any) *System {
	ctx := NewRootFramef(msg, args...)
	sys := &System{
		Context:          ctx,
		PrecomputedRound: &PrecomputedRound{Round: Round{system: nil}},
	}
	// Wire the back-reference after the System pointer is stable.
	sys.PrecomputedRound.system = sys
	return sys
}

// Free releases the scratch memory arena allocated by [Materialize]. Safe to
// call on a System that was never materialized.
func (sys *System) Free() {
	if sys.scratchArena != nil {
		sys.scratchArena.Free()
		sys.scratchArena = nil
	}
}

// NewRound creates a new interactive round, appends it to [System.Rounds],
// and returns it. The round's ID is set to its index in the slice.
func (sys *System) NewRound() *Round {
	r := &Round{
		ID:     len(sys.Rounds),
		system: sys,
	}
	sys.Rounds = append(sys.Rounds, r)
	return r
}

// NewModule creates an unsized module, registers it with the system, and
// returns it. The module's size must be fixed later via [Module.SetSize].
//
// Panics if ctx is nil.
func (sys *System) NewModule(ctx *ContextFrame, pd PaddingDirection) *Module {
	if ctx == nil {
		panic("wiop: System.NewModule requires a non-nil ContextFrame")
	}
	m := &Module{
		Context:     ctx,
		Padding:     pd,
		Annotations: make(Annotations),
		index:       len(sys.Modules),
		system:      sys,
	}
	sys.Modules = append(sys.Modules, m)
	return m
}

// NewSizedModule creates a module with a fixed size, registers it with the
// system, and returns it. It is a shorthand for [System.NewModule] followed by
// [Module.SetSize].
//
// Panics if ctx is nil or size is not positive.
func (sys *System) NewSizedModule(ctx *ContextFrame, size int, pd PaddingDirection) *Module {
	m := sys.NewModule(ctx, pd)
	m.SetSize(size)
	return m
}
