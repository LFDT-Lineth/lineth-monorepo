package wiop

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// PublicInputField identifies a semantic group within [PublicInputLayout].
// Pass one to [PublicInputLayout.Register] to add cells to that group.
type PublicInputField string

const (
	MessageBusPI                   PublicInputField = "MessageBus"
	ProgramVKPI                    PublicInputField = "ProgramVK"
	SharedRandomnessPI             PublicInputField = "SharedRandomness"
	SharedRandomnessContributionPI PublicInputField = "SharedRandomnessContribution"
	GuestPublicOutputsPI           PublicInputField = "GuestPublicOutputs"
	// IsLastShardPI sets the single IsLastShard cell. Passing more than one
	// cell, or registering it twice, panics.
	IsLastShardPI PublicInputField = "IsLastShard"
)

// PublicInputLayout defines the semantic structure of public
// inputs (mainly used for inter-shard consistency).
// The canonical flat cell ordering — used for the [PublicInput] wire format —
// is given by [PublicInputLayout.Cells].
//
// Typical usage:
//
//	sys.PublicInputs = &PublicInputLayout{}
//	sys.PublicInputs.Register(GuestPublicOutputsPI, cellA, cellB)
//
// Struct-literal construction is also valid for simple cases:
//
//	sys.PublicInputs = &PublicInputLayout{GuestPublicOutputs: []*Cell{cell}}
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

	// cachedCells and sealed are set once by the first publicInputCells call
	// inside Prove or Verify.
	cachedCells []*Cell
	sealed      bool
}

// Register adds cells to the layout group identified by f. For slice groups
// (every field except [IsLastShardPI]) it appends all of them in order; for
// [IsLastShardPI] exactly one cell must be given and it fills the single slot.
// Registering zero cells is a no-op. Panics if:
//   - any cell is nil
//   - any cell is already registered in any group (duplicate)
//   - [IsLastShardPI] receives more than one cell, or is set a second time
//   - f is not a known [PublicInputField] constant
//   - the layout has already been sealed (first [System.Prove] or
//     [System.Verify] was already called)
func (l *PublicInputLayout) Register(f PublicInputField, cells ...*Cell) {
	if l.sealed {
		panic("wiop: PublicInputLayout.Register: layout is sealed — cannot register cells after first Prove/Verify")
	}
	if len(cells) == 0 {
		return
	}
	if f == IsLastShardPI && len(cells) > 1 {
		panic(fmt.Sprintf(
			"wiop: PublicInputLayout.Register: %s accepts exactly one cell, got %d",
			f, len(cells),
		))
	}

	// Check each incoming cell against those already in the layout AND against
	// the earlier cells of this same call, so a duplicate within cells is caught.
	seen := l.Cells()
	for _, cell := range cells {
		if cell == nil {
			panic(fmt.Sprintf("wiop: PublicInputLayout.Register: nil cell for field %s", f))
		}
		for _, c := range seen {
			if c.Context.ID == cell.Context.ID {
				panic(fmt.Sprintf(
					"wiop: PublicInputLayout.Register: cell %q is already registered in the layout",
					cell.Context.Path(),
				))
			}
		}
		seen = append(seen, cell)
	}

	switch f {
	case MessageBusPI:
		l.MessageBus = append(l.MessageBus, cells...)
	case ProgramVKPI:
		l.ProgramVK = append(l.ProgramVK, cells...)
	case SharedRandomnessPI:
		l.SharedRandomness = append(l.SharedRandomness, cells...)
	case SharedRandomnessContributionPI:
		l.SharedRandomnessContribution = append(l.SharedRandomnessContribution, cells...)
	case GuestPublicOutputsPI:
		l.GuestPublicOutputs = append(l.GuestPublicOutputs, cells...)
	case IsLastShardPI:
		if l.IsLastShard != nil {
			panic(fmt.Sprintf(
				"wiop: PublicInputLayout.Register: IsLastShard already set to %q",
				l.IsLastShard.Context.Path(),
			))
		}
		l.IsLastShard = cells[0]
	default:
		panic(fmt.Sprintf("wiop: PublicInputLayout.Register: unknown field %q", f))
	}
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

// seal caches the result of Cells and marks the layout as sealed. Called once
// by publicInputCells before the first Prove or Verify. Also validates nil
// cells and duplicates as a safety net for layouts built via struct literals
// (Register already prevents these when used). Idempotent.
func (l *PublicInputLayout) seal() {
	if l.sealed {
		return
	}
	cells := l.Cells()
	seen := make(map[ObjectID]struct{}, len(cells))
	for _, c := range cells {
		if c == nil {
			panic("wiop: PublicInputLayout: nil cell in layout")
		}
		if _, dup := seen[c.Context.ID]; dup {
			panic(fmt.Sprintf("wiop: PublicInputLayout: cell %q appears more than once in layout", c.Context.Path()))
		}
		seen[c.Context.ID] = struct{}{}
	}
	l.cachedCells = cells
	l.sealed = true
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
