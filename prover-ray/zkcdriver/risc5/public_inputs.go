package risc5

import (
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
	SharedRandomnessPI             wiop.PublicInputTag = "SharedRandomness"
	SharedRandomnessContributionPI wiop.PublicInputTag = "SharedRandomnessContribution"
	GuestPublicOutputsPI           wiop.PublicInputTag = "GuestPublicOutputs"

	// IsLastShardPI tags the single cell whose runtime value is one for the last
	// shard and zero otherwise. Being a single cell, it is registered unsuffixed.
	IsLastShardPI wiop.PublicInputTag = "IsLastShard"

	// NumGuestPublicOutputs = 8 because this is a poseidon hash of the other
	// public outputs.
	NumGuestPublicOutputs = 8

	// NumSharedRandomness is the number of shared randomness cells. 8 because
	// this is the state of a poseidon2 hasher.
	NumSharedRandomness = 8

	// NumSharedRandomnessContributions is not decided yet and is to be taken from
	// another place. The negative value is an unset sentinel: it is deliberately
	// unusable as a count or loop bound until it is given its real value.
	NumSharedRandomnessContributions = -1
)

// RegisterPublicInputs registers the public inputs of the RISCV-ZKC proof
// system. It is incomplete: only the message-bus public inputs are handled,
// and those are merely checked rather than registered, since the messagebus
// compiler registers them itself. Registering the remaining roles needs
// information this package does not have yet — which columns and positions of
// RISCV-ZKC.bin carry each value — so the call panics once the message-bus check
// passes.
func RegisterPublicInputs(sys *wiop.System) {

	if len(sys.MessageBuses) < 1 {
		panic("RISCV-ZKC requires at least one message bus")
	}

	// One public input per bus, so the bound is the number of distinct handles —
	// sys.MessageBuses counts participations (a handle has at least one Send and
	// one Receive) and would overcount.
	for i := range len(sys.MessageBusHandles()) {
		if _, pos := sys.LookupPublicInputByTag(MessageBusPI, i); pos < 0 {
			panic("RISCV-ZKC requires all message buses to be registered as public inputs")
		}
	}

	panic("not implemented: registering and checking the other public-inputs")
}
