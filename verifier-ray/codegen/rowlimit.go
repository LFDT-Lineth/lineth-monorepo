package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
)

// RowLimitSystem is the compiled metadata for every lookup row-limit check in a
// wiop.System, in the form the Zig rowlimit sub-verifier consumes.
//
// prover-ray's lookuptologderivsum compiler bin-packs lookups sharing an
// includings table into subgroups, each of which shares a multiplicity
// column M and therefore drains its own MaxLookupRows budget independently
// (see lookuptologderivsum.RowLimitVerifierAction). Compile-time bin-packing
// keeps every subgroup under budget using static (maximum) module heights, but
// a dynamic module's RUNTIME size is prover-declared and PCS does not bind it
// to anything — nothing else in the transcript catches a prover that declares
// a far larger size than it compiled against. This sub-verifier re-checks the
// exact per-run heights for the concrete proof, exactly mirroring the runtime
// check prover-ray's own verifier performs.
type RowLimitSystem struct {
	SourceName string
	Checks     []RowLimitCheck
}

// RowLimitCheck is one subgroup's row-limit check: the included and
// includings module partitioning (one ModuleSize per fragment, mirroring
// RowLimitVerifierAction.IncludedModules/IncludingsModules one-to-one, so a
// repeated module appears once per fragment exactly as prover-ray's own sum
// does) and the shared per-side budget.
type RowLimitCheck struct {
	IncludedModules   []ModuleSize
	IncludingsModules []ModuleSize
	Limit             uint64
}

// BuildRowLimitSystem extracts every lookuptologderivsum.RowLimitVerifierAction
// registered on sys and records the module partitioning each one enforces.
// Checks are collected in round/registration order so the output is
// deterministic.
func BuildRowLimitSystem(sys *wiop.System) (RowLimitSystem, error) {
	out := RowLimitSystem{SourceName: sys.Context.Path()}
	dynamicIndices := DynamicModuleIndex(sys)

	moduleSizes := func(modules []*wiop.Module) ([]ModuleSize, error) {
		sizes := make([]ModuleSize, len(modules))
		for i, m := range modules {
			if m.IsDynamic() {
				idx, ok := dynamicIndices[m]
				if !ok {
					return nil, fmt.Errorf("codegen: dynamic module %q not found in sys.Modules order", m.Context.Path())
				}
				sizes[i] = ModuleSize{Dynamic: true, DynamicIndex: idx}
			} else {
				sizes[i] = ModuleSize{StaticSize: m.Size()}
			}
		}
		return sizes, nil
	}

	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			rl, ok := action.(*lookuptologderivsum.RowLimitVerifierAction)
			if !ok {
				continue
			}

			includedModules, err := moduleSizes(rl.IncludedModules)
			if err != nil {
				return RowLimitSystem{}, err
			}
			includingsModules, err := moduleSizes(rl.IncludingsModules)
			if err != nil {
				return RowLimitSystem{}, err
			}

			out.Checks = append(out.Checks, RowLimitCheck{
				IncludedModules:   includedModules,
				IncludingsModules: includingsModules,
				Limit:             wiop.MaxLookupRows,
			})
		}
	}

	return out, nil
}
