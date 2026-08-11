package messagebus

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const (
	NumSharedRandomness                                    = len(field.Octuplet{})
	SharedRandomnessSeedPI             wiop.PublicInputTag = "SharedRandomnessSeed"
	SharedRandomnessSeedContributionPI wiop.PublicInputTag = "SharedRandomnessSeedContribution"
)

// RegisterSharedRandomness declares γ as a public input and wires it into the
// message-bus coin round.
//
// It declares [NumSharedRandomness] cells on round 0, registers each under
// [SharedRandomnessSeedPI] with its limb index as numeric suffix, and registers a
// [wiop.Round.RegisterPreSamplingHook] on the message-bus coin round that
// replaces the Fiat-Shamir state with γ just before α and β are drawn. Because
// [wiop.Runtime.AdvanceRound] fires pre-sampling hooks on the prover and the
// verifier alike, both sides derive the same challenges with no extra verifier
// action.
//
// The cells live on round 0 so that they are assigned — by the prover through
// [AssignSharedRandomnessSeed], by the verifier from the public-input vector —
// before the runtime ever advances into the coin round and the hook reads them.
//
// Ordering: call it after every [wiop.MessageBus] entry has been declared and
// before [Compile]. See [ensureCoinRound] for why the window matters.
//
// Panics if sys declares no message-bus entry on a round-bearing column, since
// the coin round would then be round 0 itself and the hook would run before γ
// could be assigned.
func RegisterSharedRandomness(sys *wiop.System) {
	coinRound := ensureCoinRound(sys)
	if coinRound.ID == 0 {
		panic(
			"wiop/compilers/messagebus: RegisterSharedRandomness: the message-bus coin round is round 0, " +
				"so there is no earlier round to carry γ. This means no message-bus " +
				"entry references a round-bearing column — declare the bus entries " +
				"before calling RegisterSharedRandomness.",
		)
	}

	ctx := sys.Context.Childf("shared-randomness")
	seedRound := sys.Rounds[0]

	for i := range NumSharedRandomness {
		cell := seedRound.NewCell(ctx.Childf("gamma-%d", i), false)
		sys.RegisterPublicInputs(SharedRandomnessSeedPI, cell, i)
	}

	coinRound.RegisterPreSamplingHook(&sharedRandomnessSeedHook{})
}

// GetSharedRandomnessSeed returns the god-given value of the shared randomness
// that was provided by [AssignSharedRandomnessSeed].
func GetSharedRandomnessSeed(rt *wiop.Runtime) field.Octuplet {
	var gamma field.Octuplet
	for i := range gamma {
		c, pos := rt.System.LookupPublicInputByTag(SharedRandomnessSeedPI, i)
		if pos < 0 {
			panic(fmt.Sprintf("wiop/compilers/messagebus: GetSharedRandomnessSeed: missing gamma-%d", i))
		}
		// This calls panics if the cell is not a base-field element. But if it
		// was correctly registered by [RegisterSharedRandomness], it must be.
		gamma[i] = rt.GetCellValue(c).AsBase()
	}
	return gamma
}

// AssignSharedRandomnessSeed writes γ into the public-input cells declared by
// [RegisterSharedRandomness]. The orchestrator computes γ during the preflight
// phase — outside any proof, from every shard's data — and hands the same value
// to each shard; feeding two shards different values silently desynchronizes
// their α and β and breaks the cross-shard permutation.
//
// It must be called while the runtime is on round 0, which is where the cells
// live ([wiop.Runtime.AssignCell] rejects a cell from any other round).
func AssignSharedRandomnessSeed(rt *wiop.Runtime, gamma field.Octuplet) {
	for i := range NumSharedRandomness {
		cell, pos := rt.System.LookupPublicInputByTag(SharedRandomnessSeedPI, i)
		if pos < 0 {
			panic(fmt.Sprintf("wiop/compilers/messagebus: AssignSharedRandomnessSeed: missing gamma-%d", i))
		}
		if cell.IsExtension() {
			panic("the shared randomness cell should not be an extension-field value")
		}
		rt.AssignCell(cell, field.ElemFromBase(gamma[i]))
	}
}

// sharedRandomnessSeedHook is the [wiop.ProverAction] registered as a
// pre-sampling hook on the message-bus coin round. It reads γ from the
// public-input cells and installs it as the Fiat-Shamir state, so that the α
// and β sampled immediately afterwards are a function of γ alone and therefore
// identical on every shard that was given the same γ.
type sharedRandomnessSeedHook struct{}

// Run implements [wiop.ProverAction]. It also runs on the verifier, which
// reaches it through [wiop.System.Verify]'s transcript replay.
func (h *sharedRandomnessSeedHook) Run(rt *wiop.Runtime) {
	seed := GetSharedRandomnessSeed(rt)
	rt.SetFSState(seed)
}
