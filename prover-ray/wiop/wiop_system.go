package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/arena"
)

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
	// TableRelations holds all [LookupQuery] queries registered with this
	// system via [System.NewInclusion] and [System.NewPermutation], in
	// declaration order.
	TableRelations []*LookupQuery
	// LogDerivativeSums holds all [LogDerivativeSum] queries registered with
	// this system via [System.NewLogDerivativeSum], in declaration order.
	LogDerivativeSums []*LogDerivativeSum
	// GrandProducts holds all [GrandProduct] queries registered with this
	// system via [System.NewGrandProduct], in declaration order. The
	// grandproduct compiler also creates these when reducing permutation
	// [LookupQuery] queries.
	GrandProducts []*GrandProduct
	// MessageBuses holds all [MessageBus] queries registered with this system
	// via [System.NewMessageBusSend] and [System.NewMessageBusReceive], in
	// declaration order.
	MessageBuses []*MessageBus
	// PublicInputs is the ordered list of cells whose values form the protocol
	// "statement". It is populated, in registration order, via
	// [System.RegisterPublicInputs]. Public inputs are always cells (a
	// column is exposed by opening positions into cells, see
	// [ColumnPosition.Open]); a human-readable label, if desired, lives in the
	// cell's own Context.Lable. Their values are carried separately from the
	// [Proof] (in a [PublicInput], aligned to this order) and are received by
	// [System.Verify] alongside the proof.
	PublicInputs []*Cell
	// scratchArena backs the [PlanningContext] used by [Materialize]. It is
	// nil until Materialize is called.
	scratchArena *arena.VectorArena
	// Annotation is some user-defined information that can be attached to the
	// System.
	Annotations Annotations
	// PrecomputedCommitment commitment bears the commitment to the precomputed
	// values.
	PrecomputedCommitment field.Octuplet
}

// NewSystemf constructs an empty System. It creates a root [ContextFrame]
// using the formatted name as its label, then initialises the PrecomputedRound.
// msg and args follow [fmt.Sprintf] conventions.
//
// The system is initialized with one round
func NewSystemf(msg string, args ...any) *System {
	ctx := NewRootFramef(msg, args...)
	sys := &System{
		Context:          ctx,
		PrecomputedRound: &PrecomputedRound{Round: Round{system: nil}},
	}

	// Wire the back-reference after the System pointer is stable.
	sys.PrecomputedRound.system = sys
	sys.Annotations = make(Annotations)

	return sys
}

// RegisterPublicInputs appends cells to the ordered public-input registry.
// Public inputs are always cells: a column from the arithmetization is exposed
// by opening the desired position into a cell (see [ColumnPosition.Open]), and
// that cell is registered here. A human-readable label, if desired, is stored
// in the cell's own [Context.Lable]. Their values form the protocol statement,
// carried in a [PublicInput] separately from the [Proof] and in this
// registration order.
//
// Panics if a cell is nil, or is already
// registered.
func (sys *System) RegisterPublicInputs(cells ...*Cell) {
	// Seed the seen-set from the cells already registered so a duplicate is
	// caught across calls as well as within this one.
	seen := sys.publicInputIndex()
	for _, cell := range cells {
		if cell == nil {
			panic("wiop: RegisterPublicInputs requires non-nil cells")
		}
		id := cell.Context.ID
		if _, dup := seen[id]; dup {
			panic(fmt.Sprintf("wiop: RegisterPublicInputs: cell %q already registered as a public input", cell.Context.Path()))
		}
		seen[id] = len(sys.PublicInputs) // prevents duplication on the same call
		sys.PublicInputs = append(sys.PublicInputs, cell)
	}
}

// publicInputIndex maps each registered public-input cell's [ObjectID] to its
// position in [System.PublicInputs], for fast membership tests and for aligning
// a [PublicInput] to the registration order during [System.Prove] and
// [System.Verify].
func (sys *System) publicInputIndex() map[ObjectID]int {
	idx := make(map[ObjectID]int, len(sys.PublicInputs))
	for i, cell := range sys.PublicInputs {
		idx[cell.Context.ID] = i
	}
	return idx
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

// NewDynamicModule creates a module whose domain size is provided per-Runtime
// via [WithModuleSize] rather than fixed once via [Module.SetSize]. The same
// System can therefore be reused across proving sessions that differ in trace
// length.
//
// Panics if ctx is nil or if pd is [PaddingDirectionNone] (dynamic modules
// require a padding direction so that shorter columns can be padded to the
// module's runtime size).
func (sys *System) NewDynamicModule(ctx *ContextFrame, pd PaddingDirection) *Module {
	if pd == PaddingDirectionNone {
		panic("wiop: NewDynamicModule: dynamic modules require a padding direction (PaddingDirectionLeft or PaddingDirectionRight)")
	}
	m := sys.NewModule(ctx, pd)
	m.isDynamic = true
	return m
}
