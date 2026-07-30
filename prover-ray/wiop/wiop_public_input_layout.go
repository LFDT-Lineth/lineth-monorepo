package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// PublicInputLayout defines the semantic structure of public
// inputs (mainly used for inter-shard consistancy).
// The canonical flat cell ordering — used for the [PublicInput] wire format -
// that [System.Prove] returns and [System.Verify] consumes
type PublicInputLayout struct {
	// MessageBusMemory holds cells for the "memory" message-bus grand-product
	// endpoint.
	MessageBusMemory []*Cell
	// MessageBusRegister holds cells for the "register" message-bus
	// grand-product endpoint.
	MessageBusRegister []*Cell
	// more MessageBuses might be add later.
	// ProgramVK holds cells encoding the program verification key.
	ProgramVK []*Cell
	// SharedRandomness holds cells encoding the shared initial Fiat-Shamir state.
	SharedRandomness []*Cell
	// SharedRandomnessContribution holds cells encoding this shard's additive
	// contribution to the sharedRandomness
	SharedRandomnessContribution []*Cell
	// GuestPublicOutputs holds cells encoding the guest program's public outputs.
	GuestPublicOutputs []*Cell
	// IsLastShard is a cell whose runtime value is field.One for the last
	// shard and field.Zero otherwise.
	IsLastShard *Cell
}

// Cells returns all cells in a canonical, deterministic order:
//  1. MessageBusMemory
//  2. MessageBusRegister
//  3. ProgramVK
//  4. SharedRandomness
//  5. SharedRandomnessContribution
//  6. GuestPublicOutputs
//  7. IsLastShard (if non-nil)
//
// This ordering defines the index-to-cell mapping in the flat [PublicInput]
// wire format produced by [System.Prove].
func (l *PublicInputLayout) Cells() []*Cell {
	var cells []*Cell
	cells = append(cells, l.MessageBusMemory...)
	cells = append(cells, l.MessageBusRegister...)
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
// cell appears more than once across all fields. Panics on nil cells or
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
	sys.PublicInputs = l
}

// PublicInputValues carries the evaluated field-element values of
// public inputs, organised by semantic role. It is produced by
// [PublicInputLayout.Unpack] from the flat [PublicInput] returned by
// [System.Prove].
type PublicInputValues struct {
	MessageBusMemory             []field.Gen
	MessageBusRegister           []field.Gen
	ProgramVK                    []field.Gen
	SharedRandomness             []field.Gen
	SharedRandomnessContribution []field.Gen
	GuestPublicOutputs           []field.Gen
	IsLastShard                  field.Gen
}

// Unpack splits the flat wire-format pub into  the layout formate.
func (l *PublicInputLayout) Unpack(pub PublicInput) (*PublicInputValues, error) {
	cells := l.Cells()
	if len(pub) != len(cells) {
		return nil, fmt.Errorf("wiop: PublicInputLayout.Unpack: length mismatch: got %d, want %d", len(pub), len(cells))
	}

	unpackSlice := func(n int, i int) ([]field.Gen, int) {
		s := make([]field.Gen, n)
		for j := 0; j < n; j++ {
			s[j] = pub[i]
			i++
		}
		return s, i
	}

	v := &PublicInputValues{}
	i := 0
	v.MessageBusMemory, i = unpackSlice(len(l.MessageBusMemory), i)
	v.MessageBusRegister, i = unpackSlice(len(l.MessageBusRegister), i)
	v.ProgramVK, i = unpackSlice(len(l.ProgramVK), i)
	v.SharedRandomness, i = unpackSlice(len(l.SharedRandomness), i)
	v.SharedRandomnessContribution, i = unpackSlice(len(l.SharedRandomnessContribution), i)
	v.GuestPublicOutputs, i = unpackSlice(len(l.GuestPublicOutputs), i)
	if l.IsLastShard != nil {
		v.IsLastShard = pub[i]
	}

	return v, nil
}
