package wiop

import (
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils/arena"
)

// ProverAction represents a prover-side computation to be performed during protocol
// execution.
type ProverAction interface {
	// Run executes the action against the given [Runtime].
	Run(Runtime)
}

// VerifierAction represents a verifier-side check to be performed during
// protocol execution.
type VerifierAction interface {
	// Check executes the verification step against the given [Runtime] and
	// returns an error if the check fails.
	Check(Runtime) error
}

// Planner is an optional extension of [ProverAction] for actions that
// pre-allocate scratch memory. [Materialize] calls Plan once on every action
// that implements this interface, after all compiler passes complete. The arena
// backing [PlanningContext] persists for the lifetime of the [System], so
// slices allocated during Plan remain valid across all subsequent proof runs.
type Planner interface {
	Plan(ctx *PlanningContext)
}

// PlanningContext is the allocation surface exposed to [Planner] implementations.
// Slices obtained from it are backed by a [arena.VectorArena] owned by the
// [System] and survive the lifetime of the proof system. If the arena is
// exhausted, allocations fall back to the heap automatically.
type PlanningContext struct {
	arena *arena.VectorArena
}

// AllocField returns a slice of n base field elements backed by the planning
// arena. The slice is zeroed on first access (mmap lazy pages).
func (c *PlanningContext) AllocField(n int) []field.Element {
	return arena.Get[field.Element](c.arena, n)
}

// AllocExt returns a slice of n extension field elements backed by the
// planning arena. The slice is zeroed on first access (mmap lazy pages).
func (c *PlanningContext) AllocExt(n int) []field.Ext {
	return arena.Get[field.Ext](c.arena, n)
}
