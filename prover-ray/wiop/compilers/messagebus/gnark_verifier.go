package messagebus

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// identity as [CheckHandleSumInShard.Check]: the handle's product cell equals
// the constant Expected.
//
// [SharedRandomnessContributionChecker] has no in-circuit counterpart yet: it
// would need an in-circuit multiset hash of a round commitment. Systems
// compiled with [CompileOptions.SharedRandomness] are therefore rejected by
// [wiop.AllocateVerifierCircuit].
func (h *CheckHandleSumInShard) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()
	api.AssertIsEqualExt(run.GetCellValue(h.Cell), api.ConstExt(h.Expected.AsExt()))
}
