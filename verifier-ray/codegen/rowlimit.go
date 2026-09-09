package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
)

// RowLimitSystem is the compiled metadata for every lookup and permutation
// row-limit check in a wiop.System, in the form the Zig rowlimit sub-verifier
// consumes.
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
//
// The same mechanism, and therefore the same Zig sub-verifier, also guards
// prover-ray's grandproduct-compiled permutation queries: each unreduced
// permutation query gets its own [grandproduct.RowLimitAction] enforcing
// MaxPermutationRows on its A/B sides (see grandproduct.compilePermutations).
// The check is structurally identical — sum runtime module heights on each
// side, reject at the limit — just against a different budget constant, so it
// is folded into this same RowLimitCheck shape rather than growing a parallel
// type.
type RowLimitSystem struct {
	SourceName string
	Checks     []RowLimitCheck
}

// RowLimitCheck is one row-limit check — either a lookuptologderivsum
// subgroup or a single grandproduct-compiled permutation query — expressed as
// an included and includings module partitioning (one ModuleSize per
// fragment, mirroring RowLimitVerifierAction.IncludedModules/IncludingsModules
// one-to-one, so a repeated module appears once per fragment exactly as
// prover-ray's own sum does) and the shared per-side budget.
//
// For a permutation check the "included"/"includings" naming is lookup
// terminology that does not literally apply, but the Zig rowlimit
// sub-verifier is symmetric/side-agnostic (it just sums runtime module rows
// on each side against a limit), so the fields are reused as-is rather than
// renamed for this one caller: IncludedModules holds the permutation's A-side
// modules and IncludingsModules its B-side modules.
type RowLimitCheck struct {
	IncludedModules   []ModuleSize
	IncludingsModules []ModuleSize
	Limit             uint64
}

// BuildRowLimitSystem extracts every lookuptologderivsum.RowLimitVerifierAction
// and every grandproduct.RowLimitAction registered on sys and records the
// module partitioning each one enforces. Checks are collected in
// round/registration order so the output is deterministic.
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

	// tablesModules extracts one *wiop.Module per fragment of a permutation
	// side, one-to-one with the fragments, mirroring how
	// lookuptologderivsum.registerRowLimitChecks derives IncludedModules from
	// g.included: each Table's Module() is its shared module (see
	// wiop.Table.Module).
	tablesModules := func(tables []wiop.Table) []*wiop.Module {
		modules := make([]*wiop.Module, len(tables))
		for i, tab := range tables {
			modules[i] = tab.Module()
		}
		return modules
	}

	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			switch a := action.(type) {
			case *lookuptologderivsum.RowLimitVerifierAction:
				includedModules, err := moduleSizes(a.IncludedModules)
				if err != nil {
					return RowLimitSystem{}, err
				}
				includingsModules, err := moduleSizes(a.IncludingsModules)
				if err != nil {
					return RowLimitSystem{}, err
				}

				out.Checks = append(out.Checks, RowLimitCheck{
					IncludedModules:   includedModules,
					IncludingsModules: includingsModules,
					Limit:             wiop.MaxLookupRows,
				})

			case *grandproduct.RowLimitAction:
				includedModules, err := moduleSizes(tablesModules(a.Query.A))
				if err != nil {
					return RowLimitSystem{}, err
				}
				includingsModules, err := moduleSizes(tablesModules(a.Query.B))
				if err != nil {
					return RowLimitSystem{}, err
				}

				out.Checks = append(out.Checks, RowLimitCheck{
					IncludedModules:   includedModules,
					IncludingsModules: includingsModules,
					Limit:             a.Limit,
				})
			}
		}
	}

	return out, nil
}
