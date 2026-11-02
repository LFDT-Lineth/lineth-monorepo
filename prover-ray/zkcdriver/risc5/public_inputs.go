package risc5

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
)

// The base [wiop.PublicInputTag] values of the RISCV-ZKC public inputs. A role
// spanning several cells is registered one cell at a time under a numeric
// suffix, so its tags are Role_0 … Role_{n-1} (see
// [wiop.System.RegisterPublicInputs]); a single-cell role is registered
// unsuffixed.
const (
	MessageBusPI                   wiop.PublicInputTag = messagebus.PublicInputTag
	ProgramVKPI                    wiop.PublicInputTag = "ProgramVerificationKey"
	SharedRandomnessPI                                 = messagebus.SharedRandomnessSeedPI
	SharedRandomnessContributionPI                     = messagebus.SharedRandomnessSeedContributionPI
	GuestPublicOutputsPI           wiop.PublicInputTag = "GuestPublicOutputs"

	// NumGuestPublicOutputs = 32 because only the first 32 bytes of the guest
	// output are bound as public inputs, one byte per row.
	NumGuestPublicOutputs = 32

	// NumSharedRandomness is the number of shared randomness cells. 8 because
	// this is the state of a poseidon2 hasher.
	NumSharedRandomness = messagebus.NumSharedRandomness

	// NumSharedRandomnessContributions is not decided yet and is to be taken from
	// another place. The negative value is an unset sentinel: it is deliberately
	// unusable as a count or loop bound until it is given its real value.
	NumSharedRandomnessContributions = messagebus.NumSharedRandomnessContribution
)

// RegisterPublicInputs registers the public inputs of the RISCV-ZKC proof
// system. It must run before the compiler passes.
func RegisterPublicInputs(sys *wiop.System) {
	RegisterGuestPublicOutputs(sys)
	RegisterProgramVK(sys)
}

// CheckPublicInputs panics unless every public input of the RISCV-ZKC proof
// system is registered. It must run after the compiler passes.
func CheckPublicInputs(sys *wiop.System) {

	if len(sys.MessageBuses) < 1 {
		panic("RISCV-ZKC requires at least one message bus")
	}

	// One public input per bus, so the bound is the number of distinct handles —
	// sys.MessageBuses counts participations (a handle has at least one Send and
	// one Receive) and would overcount.
	checkTag(sys, MessageBusPI, len(sys.MessageBusHandles()))
	checkTag(sys, GuestPublicOutputsPI, NumGuestPublicOutputs)
	checkTag(sys, ProgramVKPI, NumProgramVK)

	if messagebus.HasSharedRandomness(sys) {
		checkTag(sys, SharedRandomnessPI, NumSharedRandomness)
		checkTag(sys, SharedRandomnessContributionPI, NumSharedRandomnessContributions)
	}
}

// checkTag panics unless the public inputs tag_0 … tag_{n-1} are registered.
func checkTag(sys *wiop.System, tag wiop.PublicInputTag, n int) {
	for i := range n {
		if _, pos := sys.LookupPublicInputByTag(tag, i); pos < 0 {
			panic(fmt.Sprintf("RISCV-ZKC requires the public input %s_%d to be registered", tag, i))
		}
	}
}
