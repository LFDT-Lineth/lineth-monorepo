package grandproduct

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. The circuit only supports
// statically sized modules (see [wiop.AllocateVerifierCircuit]), so every
// runtime size equals the static one and the row budget is decided entirely
// when the circuit is built: an over-limit query is a structural problem and
// panics, and a within-limit one needs no constraint.
func (a *RowLimitAction) CheckGnark(_ frontend.API, _ *wiop.GnarkRuntime) {
	a.Query.PrecheckRowLimit(a.Limit)
}

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// boundary identity as [FinalProductCheck.Check]: the product of the final Z
// openings equals the claimed Result.
func (f *FinalProductCheck) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()
	zFinals := make([]circuit.Ext, len(f.Entries))
	for i, e := range f.Entries {
		zFinals[i] = run.GetCellValue(e.ZFinal)
	}
	claimed := run.GetCellValue(f.GrandProduct.Result)
	api.AssertIsEqualExt(productExt(api, zFinals), claimed)
}

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// identity as [CheckResultIsOne.Check]: the aggregated Result equals one.
func (c *CheckResultIsOne) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()
	api.AssertIsEqualExt(run.GetCellValue(c.GrandProduct.Result), api.OneExt())
}

// productExt returns the product of xs, or one when xs is empty, matching the
// native accumulator that starts from one.
func productExt(api *circuit.KoalaBearAPI, xs []circuit.Ext) circuit.Ext {
	if len(xs) == 0 {
		return api.OneExt()
	}
	prod := xs[0]
	for _, x := range xs[1:] {
		prod = api.MulExt(prod, x)
	}
	return prod
}
