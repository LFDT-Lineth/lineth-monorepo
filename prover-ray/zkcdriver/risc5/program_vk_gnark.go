package risc5

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It enforces the same
// predicate as [ProgramVKChecker.Check]: each [ProgramVKPI] cell equals the
// matching limb of the commitment to [zkcdriver.ProgramRound], or zero when
// that round carries no commitment (as [programCommitment] returns natively).
//
// Whether a cell is base or extension is fixed when the circuit is built, so
// an extension-valued cell is a structural error and panics instead of being
// constrained.
func (*ProgramVKChecker) CheckGnark(_ frontend.API, run *wiop.GnarkRuntime) {
	api := run.API()

	var commitment circuit.Octuplet
	if run.System.Rounds[zkcdriver.ProgramRound].HasCommitment {
		commitment = run.GetCommitment(zkcdriver.ProgramRound)
	} else {
		// No PCS: the program round was never committed, so the native check
		// compares against the zero commitment (see programCommitment). Mirror
		// that instead of calling GetCommitment, which would panic. In the
		// backend only the dev-partial mode compiles without a PCS, and its
		// proof is not wrapped, so this branch only serves tests.
		// TODO: consider replacing it with an error, since a wrapped proof
		// with a zero program VK is meaningless.
		for i := range commitment {
			commitment[i] = api.Zero()
		}
	}

	for i := range commitment {
		cell := programVKCell(run.System, i)
		v, ok := run.GetCellBase(cell)
		if !ok {
			utils.Panic("risc5: ProgramVKChecker.CheckGnark: program-vk-%d is carried as an extension element", i)
		}
		api.AssertIsEqual(v, commitment[i])
	}
}
