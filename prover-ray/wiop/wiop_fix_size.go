package wiop

import (
	"fmt"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// FixDynamicSizes turns every dynamic module of the system into a statically
// sized module, using sizes[k] as the size of module k (indexed like
// [System.Modules], [Proof.DynamicSizes] and [System.MinRuntimeSizes]).
//
// It exists for the terminal recursion layer, whose proof is verified by a
// gnark circuit: a dynamic size changes the circuit shape, so the system is
// pinned to fixed sizes and every proof of it has one shape, known before any
// proof exists. Padding a dynamic module is already sound: a dynamic module is
// valid at any power-of-two size covering its data and at least its
// [Module.MinRuntimeSize], and its columns are padded per the module's
// [PaddingDirection]. A fixed module simply drops the choice.
//
// The fixed system is a different protocol from the dynamic one: sizes are
// no longer absorbed into the Fiat-Shamir transcript and no longer carried in
// [Proof.DynamicSizes]. Provers and verifiers of the terminal layer must both
// use the fixed system.
//
// It must be called after the system is defined and before any compiler pass
// runs, since passes read the module sizes. It is all or nothing: it returns
// an error, and changes nothing, if
//   - a dynamic module has no entry in sizes, or an entry names a module that
//     is out of range or not dynamic;
//   - a size is not a power of two, is below the module's
//     [Module.MinRuntimeSize], or exceeds [ColumnSizeMaxSupported];
//   - a compiler pass has already run (a query is reduced or a round carries
//     a commitment).
//
// A prover whose data overflows a fixed size fails at column assignment, as for
// any sized module.
func (sys *System) FixDynamicSizes(sizes map[int]int) error {
	if err := sys.checkNotCompiled(); err != nil {
		return fmt.Errorf("wiop: FixDynamicSizes: %w", err)
	}

	keys := make([]int, 0, len(sizes))
	for k := range sizes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		if k < 0 || k >= len(sys.Modules) {
			return fmt.Errorf("wiop: FixDynamicSizes: module index %d out of range [0, %d)", k, len(sys.Modules))
		}
		if !sys.Modules[k].isDynamic {
			return fmt.Errorf("wiop: FixDynamicSizes: module %q is not dynamic", sys.Modules[k].Context.Path())
		}
	}

	minSizes := sys.MinRuntimeSizes()
	for k, mod := range sys.Modules {
		if !mod.isDynamic {
			continue
		}
		size, ok := sizes[k]
		if !ok {
			return fmt.Errorf("wiop: FixDynamicSizes: dynamic module %q has no size", mod.Context.Path())
		}
		if !utils.IsPowerOfTwo(size) {
			return fmt.Errorf("wiop: FixDynamicSizes: module %q: size %d is not a power of two",
				mod.Context.Path(), size)
		}
		if size > ColumnSizeMaxSupported {
			return fmt.Errorf("wiop: FixDynamicSizes: module %q: size %d exceeds the maximum supported %d",
				mod.Context.Path(), size, ColumnSizeMaxSupported)
		}
		if size < minSizes[k] {
			return fmt.Errorf("wiop: FixDynamicSizes: module %q: size %d is below its minimum %d",
				mod.Context.Path(), size, minSizes[k])
		}
	}

	for k, mod := range sys.Modules {
		if !mod.isDynamic {
			continue
		}
		mod.isDynamic = false
		mod.size = sizes[k]
	}
	return nil
}

// checkNotCompiled reports an error if a compiler pass has already run on the
// system: a pass reduces the queries it consumes, and the PCS pass flags the
// committed rounds.
func (sys *System) checkNotCompiled() error {
	var reduced Query
	sys.forEachQuery(func(q Query) {
		if reduced == nil && q.IsReduced() {
			reduced = q
		}
	})
	if reduced != nil {
		return fmt.Errorf("system %q is already compiled: query %T is reduced", sys.Context.Path(), reduced)
	}
	for _, r := range sys.Rounds {
		if r.HasCommitment {
			return fmt.Errorf("system %q is already compiled: round %d carries a commitment", sys.Context.Path(), r.ID)
		}
	}
	return nil
}
