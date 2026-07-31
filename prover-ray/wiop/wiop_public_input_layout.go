package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// PublicInputLayout defines the semantic structure of public
// inputs (mainly used for inter-shard consistency).
// The canonical flat cell ordering — used for the [PublicInput] wire format —
// is given by [PublicInputLayout.Cells].
type PublicInputLayout struct {
	// MessageBus holds one cell per registered message-bus handle, in
	// alphabetical handle order (messagebus.Compile sorts handles before
	// processing them, so the ordering is deterministic across all shards).
	MessageBus []*Cell
	// ProgramVK holds cells encoding the program verification key.
	ProgramVK []*Cell
	// SharedRandomness holds cells encoding the shared initial Fiat-Shamir state.
	SharedRandomness []*Cell
	// SharedRandomnessContribution holds cells encoding this shard's additive
	// contribution to the sharedRandomness.
	SharedRandomnessContribution []*Cell
	// GuestPublicOutputs holds cells encoding the guest program's public outputs.
	GuestPublicOutputs []*Cell
	// IsLastShard is a cell whose runtime value is field.One for the last
	// shard and field.Zero otherwise. Nil if not used.
	IsLastShard *Cell

	// cachedCells is set once by Register (which seals the layout) and
	// returned by Cells on all subsequent calls, avoiding repeated allocation.
	cachedCells []*Cell
	sealed      bool
}

// RegisterMessageBus appends a cell to [MessageBus]. Each call corresponds to
// one handle; messagebus.Compile calls this once per handle in alphabetical
// order, so positions are deterministic across shards.
func (l *PublicInputLayout) RegisterMessageBus(_ string, cell *Cell) {
	l.MessageBus = append(l.MessageBus, cell)
}

// Cells returns all cells in a canonical, deterministic order:
//  1. MessageBus (in registration order)
//  2. ProgramVK
//  3. SharedRandomness
//  4. SharedRandomnessContribution
//  5. GuestPublicOutputs
//  6. IsLastShard (if non-nil)
//
// This ordering defines the index-to-cell mapping in the flat [PublicInput]
// wire format produced by [System.Prove].
//
// After [Register] is called the result is cached; all subsequent calls return
// the same slice.
func (l *PublicInputLayout) Cells() []*Cell {
	if l.sealed {
		return l.cachedCells
	}
	var cells []*Cell
	cells = append(cells, l.MessageBus...)
	cells = append(cells, l.ProgramVK...)
	cells = append(cells, l.SharedRandomness...)
	cells = append(cells, l.SharedRandomnessContribution...)
	cells = append(cells, l.GuestPublicOutputs...)
	if l.IsLastShard != nil {
		cells = append(cells, l.IsLastShard)
	}
	return cells
}

// Register sets sys.PublicInputs to this layout. It first validates that no
// cell appears more than once across all fields. Panics on nil cells,
// duplicates.
func (l *PublicInputLayout) Register(sys *System) {
	cells := l.Cells()
	seen := make(map[ObjectID]struct{}, len(cells))
	for _, c := range cells {
		if c == nil {
			panic("wiop: PublicInputLayout.Register: nil cell in layout")
		}
		if _, dup := seen[c.Context.ID]; dup {
			panic(fmt.Sprintf("wiop: PublicInputLayout.Register: cell %q appears more than once in layout", c.Context.Path()))
		}
		seen[c.Context.ID] = struct{}{}
	}
	l.cachedCells = cells
	l.sealed = true
	sys.PublicInputs = l
}

// PublicInputValues carries the evaluated field-element values of
// public inputs, organised by semantic role. It is produced by
// [PublicInputLayout.Unpack] from the flat [PublicInput] returned by
// [System.Prove].
type PublicInputValues struct {
	// MessageBus is indexed identically to [PublicInputLayout.MessageBus].
	MessageBus                   []field.Gen
	ProgramVK                    []field.Gen
	SharedRandomness             []field.Gen
	SharedRandomnessContribution []field.Gen
	GuestPublicOutputs           []field.Gen
	// IsLastShard is the zero value of field.Gen when
	// [PublicInputLayout.IsLastShard] was nil.
	IsLastShard field.Gen
}

// Unpack splits the flat wire-format pub into the layout format.
func (l *PublicInputLayout) Unpack(pub PublicInput) (*PublicInputValues, error) {
	cells := l.Cells()
	if len(pub) != len(cells) {
		return nil, fmt.Errorf("wiop: PublicInputLayout.Unpack: length mismatch: got %d, want %d", len(pub), len(cells))
	}

	unpackSlice := func(n, i int) ([]field.Gen, int) {
		s := make([]field.Gen, n)
		copy(s, pub[i:i+n])
		return s, i + n
	}

	v := &PublicInputValues{}
	i := 0
	v.MessageBus, i = unpackSlice(len(l.MessageBus), i)
	v.ProgramVK, i = unpackSlice(len(l.ProgramVK), i)
	v.SharedRandomness, i = unpackSlice(len(l.SharedRandomness), i)
	v.SharedRandomnessContribution, i = unpackSlice(len(l.SharedRandomnessContribution), i)
	v.GuestPublicOutputs, i = unpackSlice(len(l.GuestPublicOutputs), i)
	if l.IsLastShard != nil {
		v.IsLastShard = pub[i]
		i++
	}
	if i != len(pub) {
		return nil, fmt.Errorf("wiop: PublicInputLayout.Unpack: internal error: consumed %d of %d elements", i, len(pub))
	}

	return v, nil
}
