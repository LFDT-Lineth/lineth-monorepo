package wiop

import (
	"fmt"
	"sort"
)

// ModuleSizes maps a dynamic module's path ([Module.Context] Path) to a size.
// It is the stable, reviewable form of the sizes passed to
// [System.FixDynamicSizes]: unlike a module index, a path does not silently
// move to another module when modules are added or reordered, so a size map
// can be checked into the repository next to the system it pins.
type ModuleSizes map[string]int

// MeasureDynamicSizes returns, for every dynamic module of sys, the largest
// size any of the given honest proofs of sys used ([Proof.DynamicSizes]). It
// is how the sizes of the terminal recursion layer are chosen: prove the
// dynamic system on representative inputs (the worst case included), measure,
// then pin the result with [System.FixDynamicSizesByPath], after adding
// headroom if desired (see [ModuleSizes.Scaled]).
//
// It returns an error if sys has no dynamic module, if no proof is given, or
// if a proof lacks the size of a dynamic module or carries a size for a module
// that is not dynamic.
func (sys *System) MeasureDynamicSizes(proofs ...Proof) (ModuleSizes, error) {
	if len(proofs) == 0 {
		return nil, fmt.Errorf("wiop: MeasureDynamicSizes: no proof given")
	}
	paths, err := sys.dynamicModulePaths()
	if err != nil {
		return nil, fmt.Errorf("wiop: MeasureDynamicSizes: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("wiop: MeasureDynamicSizes: system %q has no dynamic module", sys.Context.Path())
	}
	measured := make(ModuleSizes, len(paths))
	for i, proof := range proofs {
		for k := range proof.DynamicSizes {
			if _, ok := paths[k]; !ok {
				return nil, fmt.Errorf("wiop: MeasureDynamicSizes: proof %d has a size for module index %d, "+
					"which is not a dynamic module of this system", i, k)
			}
		}
		for k, path := range paths {
			size, ok := proof.DynamicSizes[k]
			if !ok {
				return nil, fmt.Errorf("wiop: MeasureDynamicSizes: proof %d has no size for dynamic module %q", i, path)
			}
			measured[path] = max(measured[path], size)
		}
	}
	return measured, nil
}

// Scaled returns a copy of s with every size multiplied by factor, a power of
// two (so sizes stay powers of two). It adds headroom to measured sizes.
//
// Panics if factor is not a positive power of two.
func (s ModuleSizes) Scaled(factor int) ModuleSizes {
	if factor <= 0 || factor&(factor-1) != 0 {
		panic(fmt.Sprintf("wiop: ModuleSizes.Scaled: factor %d is not a positive power of two", factor))
	}
	res := make(ModuleSizes, len(s))
	for path, size := range s {
		res[path] = size * factor
	}
	return res
}

// CheckSizesCover reports an error unless pinned is a valid size plan for the
// dynamic modules of sys that covers measured: both must name exactly the
// dynamic modules of sys, and every pinned size must be at least the measured
// one. It is meant for a CI test that re-measures the sizes of the terminal
// layer and compares them with the checked-in ones, so that a change that
// grows the trace, or adds, removes or renames a module, fails a test rather
// than a production prover. All violations are reported, sorted by path.
//
// sys must still be dynamic (not yet passed to [System.FixDynamicSizes]).
func (sys *System) CheckSizesCover(pinned, measured ModuleSizes) error {
	paths, err := sys.dynamicModulePaths()
	if err != nil {
		return fmt.Errorf("wiop: CheckSizesCover: %w", err)
	}
	want := make(map[string]bool, len(paths))
	for _, path := range paths {
		want[path] = true
	}

	var problems []string
	for _, named := range []struct {
		what  string
		sizes ModuleSizes
	}{{"pinned", pinned}, {"measured", measured}} {
		for path := range named.sizes {
			if !want[path] {
				problems = append(problems, fmt.Sprintf("%s size for %q, which is not a dynamic module", named.what, path))
			}
		}
	}
	for path := range want {
		p, hasPinned := pinned[path]
		m, hasMeasured := measured[path]
		switch {
		case !hasPinned:
			problems = append(problems, fmt.Sprintf("dynamic module %q has no pinned size", path))
		case !hasMeasured:
			problems = append(problems, fmt.Sprintf("dynamic module %q has no measured size", path))
		case m > p:
			problems = append(problems, fmt.Sprintf("dynamic module %q needs size %d, pinned to %d", path, m, p))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	msg := fmt.Sprintf("wiop: CheckSizesCover: %d problem(s):", len(problems))
	for _, p := range problems {
		msg += "\n  - " + p
	}
	return fmt.Errorf("%s", msg)
}

// FixDynamicSizesByPath is [System.FixDynamicSizes] with the sizes keyed by
// module path instead of module index. It returns an error, and changes
// nothing, if a path names no module or two modules share a path, in addition
// to the errors of [System.FixDynamicSizes].
func (sys *System) FixDynamicSizesByPath(sizes ModuleSizes) error {
	index := make(map[string]int, len(sys.Modules))
	for k, mod := range sys.Modules {
		path := mod.Context.Path()
		if prev, dup := index[path]; dup {
			return fmt.Errorf("wiop: FixDynamicSizesByPath: modules %d and %d share the path %q", prev, k, path)
		}
		index[path] = k
	}
	byIndex := make(map[int]int, len(sizes))
	paths := make([]string, 0, len(sizes))
	for path := range sizes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		k, ok := index[path]
		if !ok {
			return fmt.Errorf("wiop: FixDynamicSizesByPath: no module has the path %q", path)
		}
		byIndex[k] = sizes[path]
	}
	return sys.FixDynamicSizes(byIndex)
}

// dynamicModulePaths returns the path of every dynamic module, by module
// index. It returns an error if two dynamic modules share a path, which would
// make a [ModuleSizes] ambiguous.
func (sys *System) dynamicModulePaths() (map[int]string, error) {
	paths := make(map[int]string)
	seen := make(map[string]int)
	for k, mod := range sys.Modules {
		if !mod.isDynamic {
			continue
		}
		path := mod.Context.Path()
		if prev, dup := seen[path]; dup {
			return nil, fmt.Errorf("dynamic modules %d and %d share the path %q", prev, k, path)
		}
		seen[path] = k
		paths[k] = path
	}
	return paths, nil
}
