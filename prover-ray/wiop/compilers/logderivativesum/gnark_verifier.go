package logderivativesum

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// final-sum identity as [VerifierAction.Check]: Σ_i n_i · C_i equals the
// claimed Result, n_i being the size of entry i's module.
//
// Only statically sized modules are supported, so every n_i is a circuit
// constant and each term costs a constant multiplication, not an extension
// one.
func (a *VerifierAction) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()
	terms := make([]circuit.Ext, len(a.Entries))
	for i, e := range a.Entries {
		m := e.Module()
		if m.IsDynamic() {
			panic(fmt.Sprintf("wiop/compilers: logderivativesum.VerifierAction.CheckGnark: dynamic module %q is not supported",
				m.Context.Path()))
		}
		n := big.NewInt(int64(m.Size()))
		terms[i] = api.MulConstExt(run.GetCellValue(e.Average), n)
	}
	sum := api.ZeroExt()
	if len(terms) > 0 {
		sum = api.SumExt(terms...)
	}
	api.AssertIsEqualExt(sum, run.GetCellValue(a.LogDerivativeSum.Result))
}
