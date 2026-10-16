package lookuptologderivsum

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. The circuit only supports
// statically sized modules (see [wiop.AllocateVerifierCircuit]), so the row
// counts [RowLimitVerifierAction.Check] sums are fixed when the circuit is
// built: an over-limit subgroup is a structural problem and panics, and a
// within-limit one needs no constraint.
func (a *RowLimitVerifierAction) CheckGnark(_ frontend.API, _ *wiop.GnarkRuntime) {
	if rows := staticRows(a.IncludedModules); rows >= wiop.MaxLookupRows {
		panic(a.overLimitError("A", rows))
	}
	if rows := staticRows(a.IncludingsModules); rows >= wiop.MaxLookupRows {
		panic(a.overLimitError("B", rows))
	}
}

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// identity as [ResultIsZeroVerifierAction.Check]: the aggregated lookup Result
// is zero.
func (a *ResultIsZeroVerifierAction) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()
	api.AssertIsEqualExt(run.GetCellValue(a.LogDerivativeSum.Result), api.ZeroExt())
}

// staticRows sums the static heights of modules, one entry per fragment, so a
// module listed twice is counted twice exactly as in
// [RowLimitVerifierAction.Check].
func staticRows(modules []*wiop.Module) uint64 {
	var rows uint64
	for _, m := range modules {
		if m.IsDynamic() {
			panic(fmt.Sprintf("wiop/compilers: lookuptologderivsum.RowLimitVerifierAction.CheckGnark: dynamic module %q is not supported",
				m.Context.Path()))
		}
		rows += uint64(m.Size())
	}
	return rows
}
