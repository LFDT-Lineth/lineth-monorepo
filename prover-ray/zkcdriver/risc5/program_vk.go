package risc5

import (
	"errors"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/sirupsen/logrus"
)

// NumProgramVK is the number of [ProgramVKPI] cells: one per limb of the
// commitment to [zkcdriver.ProgramRound].
const NumProgramVK = len(field.Octuplet{})

// ErrProgramVKMismatch is returned by [ProgramVKChecker] when the public
// program VK differs from the commitment to [zkcdriver.ProgramRound].
var ErrProgramVKMismatch = errors.New("risc5: program VK mismatch")

// RegisterProgramVK registers the commitment to [zkcdriver.ProgramRound], which
// holds the decoded program only, as the [ProgramVKPI] public inputs of sys.
// The cells are lazy so that they are resolved after the PCS commit action.
//
// Panics if the trace is in [zkcdriver.ProgramRound] too.
func RegisterProgramVK(sys *wiop.System) {

	// DynamicSizeRound is the trace round (see [zkcdriver.Define]).
	if sys.DynamicSizeRound == zkcdriver.ProgramRound {
		utils.Panic(
			"risc5: RegisterProgramVK: the trace is in the program round %d",
			zkcdriver.ProgramRound,
		)
	}

	var (
		ctx   = sys.Context.Childf("program-vk")
		round = sys.Rounds[zkcdriver.ProgramRound]
	)

	for i := range NumProgramVK {
		cell := round.NewLazyCell(ctx.Childf("vk-%d", i), false,
			func(rt *wiop.Runtime) field.Gen {
				return field.ElemFromBase(programCommitment(rt)[i])
			})
		sys.RegisterPublicInputs(ProgramVKPI, cell, i)
	}

	round.RegisterVerifierAction(&ProgramVKChecker{})
}

// programCommitment returns the commitment to [zkcdriver.ProgramRound].
func programCommitment(rt *wiop.Runtime) field.Octuplet {
	if !rt.System.Rounds[zkcdriver.ProgramRound].HasCommitment {
		logrus.Warnf(
			"No commitment found for round: %v. Did you reduce the current system using a PCS?",
			zkcdriver.ProgramRound)
	}
	return rt.Commitments[zkcdriver.ProgramRound]
}

// programVKCell returns the public-input cell carrying limb i of the program
// verification key.
func programVKCell(sys *wiop.System, i int) *wiop.Cell {
	cell, pos := sys.LookupPublicInputByTag(ProgramVKPI, i)
	if pos < 0 {
		utils.Panic("risc5: missing program-vk-%d", i)
	}
	if cell.IsExtension() {
		utils.Panic("risc5: program-vk-%d must not be an extension-field value", i)
	}
	return cell
}

// ProgramVKChecker is a verifier action checking that the [ProgramVKPI] cells
// equal the commitment to [zkcdriver.ProgramRound].
type ProgramVKChecker struct{}

// Check implements the [wiop.VerifierAction] interface for [ProgramVKChecker].
func (*ProgramVKChecker) Check(rt *wiop.Runtime) error {
	commitment := programCommitment(rt)
	for i := range commitment {
		//an extension value is rejected
		v := rt.GetCellValue(programVKCell(rt.System, i))
		if !v.IsBase() || commitment[i] != v.AsBase() {
			return fmt.Errorf("%w: program-vk-%d", ErrProgramVKMismatch, i)
		}
	}
	return nil
}

// GetProgramVK returns the program verification key read from the public
// inputs pub of a proof over sys.
//
// Panics if a public input is missing or is an extension-field value.
func GetProgramVK(sys *wiop.System, pub wiop.PublicInput) field.Octuplet {
	var vk field.Octuplet
	for i := range vk {
		_, pos := sys.LookupPublicInputByTag(ProgramVKPI, i)
		if pos < 0 {
			utils.Panic("risc5: GetProgramVK: no public input registered for program-vk-%d", i)
		}
		if !pub[pos].IsBase() {
			utils.Panic("risc5: GetProgramVK: program-vk-%d must not be an extension-field value", i)
		}
		vk[i] = pub[pos].AsBase()
	}
	return vk
}
