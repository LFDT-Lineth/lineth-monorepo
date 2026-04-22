package wiop

import "github.com/consensys/linea-monorepo/prover-ray/utils/arena"

// scratchArenaCap is the mmap arena capacity reserved for prover scratch
// buffers. Pages are lazily allocated by the kernel, so over-provisioning is
// free until actually accessed.
const scratchArenaCap = 512 << 20 // 512 MiB

// Materialize pre-allocates scratch memory for all [Planner] actions in sys.
// Call once after all compiler passes have run and before creating the first
// [Runtime]. Subsequent calls are no-ops.
func Materialize(sys *System) {
	if sys.scratchArena != nil {
		return
	}

	a := arena.NewVectorArenaMmap[byte](scratchArenaCap)
	sys.scratchArena = a

	ctx := &PlanningContext{arena: a}
	for _, r := range sys.Rounds {
		for _, act := range r.ProverActions {
			if p, ok := act.(Planner); ok {
				p.Plan(ctx)
			}
		}
	}
}
