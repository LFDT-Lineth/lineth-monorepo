package messagebus

import (
	"fmt"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// identity as [CheckHandleSumInShard.Check]: the handle's product cell equals
// the constant Expected.
func (h *CheckHandleSumInShard) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()
	api.AssertIsEqualExt(run.GetCellValue(h.Cell), api.ConstExt(h.Expected.AsExt()))
}

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// identity as [SharedRandomnessContributionChecker.Check]: every contribution
// public input equals the matching limb of the multiset hash of the current
// round's commitment, recomputed in-circuit with [multisethashing.HashGnark].
func (*SharedRandomnessContributionChecker) CheckGnark(api frontend.API, run *wiop.GnarkRuntime) {
	round := run.CurrentRound()
	if !round.HasCommitment {
		panic(fmt.Sprintf(
			"wiop/compilers/messagebus: SharedRandomnessContributionChecker: round %d has no commitment; "+
				"the system must be PCS-compiled", round.ID))
	}

	digest := multisethashing.HashGnark(api, run.GetCommitment(round.ID))
	for i := range digest {
		cell, _ := limbCell(run.System, SharedRandomnessSeedContributionPI, i)
		v, ok := run.GetCellBase(cell)
		if !ok {
			panic(fmt.Sprintf(
				"wiop/compilers/messagebus: contribution-%d is carried as an extension element", i))
		}
		run.API().AssertIsEqual(v, digest[i])
	}
}
