package global

import (
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// BuiltPrograms lists the quotient programs of sys built so far, by module,
// bucket and increasing size. Elements are only compared for identity.
func BuiltPrograms(sys *wiop.System) []any {
	var out []any
	for _, r := range sys.Rounds {
		for _, act := range r.ProverActions {
			a, ok := act.(*QuotientProverAction)
			if !ok {
				continue
			}
			for _, q := range a.modules {
				for i := range q.buckets {
					c := q.buckets[i].programs
					c.mu.Lock()
					sizes := make([]int, 0, len(c.byN))
					for n := range c.byN {
						sizes = append(sizes, n)
					}
					slices.Sort(sizes)
					for _, n := range sizes {
						out = append(out, c.byN[n])
					}
					c.mu.Unlock()
				}
			}
		}
	}
	return out
}
