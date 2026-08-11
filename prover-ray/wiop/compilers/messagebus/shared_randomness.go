package messagebus

import (
	"fmt"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/sirupsen/logrus"
)

const (
	NumSharedRandomness                                    = len(field.Octuplet{})
	NumSharedRandomnessContribution                        = len(multisethashing.MSetHash{})
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

	for i := range NumSharedRandomnessContribution {
		cell := seedRound.NewCell(ctx.Childf("contribution-%d", i), false)
		sys.RegisterPublicInputs(SharedRandomnessSeedContributionPI, cell, i)
	}

	coinRound.RegisterPreSamplingHook(&SharedRandomnessSeedHook{})
	coinRound.RegisterAction(&SharedRandomnessContributionAssigner{})
	coinRound.RegisterVerifierAction(&SharedRandomnessContributionChecker{})
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

// SharedRandomnessContributionAssigner is a prover action that takes all the
// PCS commitment preceding the shared randomness seed Fiat-Shamir override and
// hash them into a multiset hash that is then exposed to the verifier as a
// public-input.
//
// This function is meant to be run as a prover action at the round where the
// the message BUS randomness is sampled.
//
// In case this function is called over a system that is not using a PCS, the
// function will unsoundly assign a multiset-hash derived from 0. If no message
// bus is called in this system, this function will also assign a dummy multiset
// hash.
type SharedRandomnessContributionAssigner struct{}

// SharedRandomnessContributionChecker is a verifier action that checks that the
// public-input cells of the shared randomness contribution are correctly
// computed against the commitment cell values. It is the verifier analog to
// [SharedRandomnessContributionAssigner].
type SharedRandomnessContributionChecker struct{}

// Run implements [wiop.ProverAction] on behalf of [SharedRandomnessContributionAssigner].
func (_ *SharedRandomnessContributionAssigner) Run(rt *wiop.Runtime) {
	round := rt.CurrentRound()
	hasher := poseidon2.NewMDHasher()
	for i := range round.ID {
		if !rt.System.Rounds[i].HasCommitment {
			logrus.Warnf(
				"No commitment found for round: %v. Did you use a message bus? "+
					"And did you reduce the current system using a PCS?", i)
			continue
		}

		com := rt.Commitments[i]
		hasher.WriteElements(com[:]...)
	}

	digest := hasher.SumDigest()
	digestMSet := multisethashing.Hash(digest)

	for i := range digestMSet {

		cell, pos := rt.System.LookupPublicInputByTag(SharedRandomnessSeedPI, i)
		if pos < 0 {
			panic(fmt.Sprintf("wiop/compilers/messagebus: SharedRandomnessContributionProverAction: missing contribution-%d", i))
		}
		if cell.IsExtension() {
			panic("the shared randomness cell should not be an extension-field value")
		}
		rt.AssignCell(cell, field.ElemFromBase(digestMSet[i]))
	}
}

// Check implements the [VerifierAction] interface for
// [SharedRandomnessContributionChecker].
func (_ *SharedRandomnessContributionChecker) Check(rt *wiop.Runtime) error {

	round := rt.CurrentRound()
	hasher := poseidon2.NewMDHasher()
	for i := range round.ID {
		if !rt.System.Rounds[i].HasCommitment {
			logrus.Warnf(
				"No commitment found for round: %v. Did you use a message bus? "+
					"And did you reduce the current system using a PCS?", i)
			continue
		}

		com := rt.Commitments[i]
		hasher.WriteElements(com[:]...)
	}

	digest := hasher.SumDigest()
	digestMSet := multisethashing.Hash(digest)

	for i := range digestMSet {

		cell, pos := rt.System.LookupPublicInputByTag(SharedRandomnessSeedPI, i)
		if pos < 0 {
			// If this fails, this indicates the wizard is ill-defined, not the
			// proof. That's why it is a panic.
			panic(fmt.Sprintf("wiop/compilers/messagebus: SharedRandomnessContributionProverAction: missing contribution-%d", i))
		}

		if cell.IsExtension() {
			// If this fails, this indicates the wizard is ill-defined, not the
			// proof. That's why it is a panic.
			panic("the shared randomness cell should not be an extension-field value")
		}

		if digestMSet[i] != rt.GetCellValue(cell).AsBase() {
			return fmt.Errorf("wiop/compilers/messagebus: SharedRandomnessContributionChecker: mismatch in contribution-%d", i)
		}
	}

	return nil
}

// SharedRandomnessSeedHook is the [wiop.ProverAction] registered as a
// pre-sampling hook on the message-bus coin round. It reads γ from the
// public-input cells and installs it as the Fiat-Shamir state, so that the α
// and β sampled immediately afterwards are a function of γ alone and therefore
// identical on every shard that was given the same γ.
type SharedRandomnessSeedHook struct{}

// Run implements [wiop.ProverAction]. It also runs on the verifier, which
// reaches it through [wiop.System.Verify]'s transcript replay.
func (h *SharedRandomnessSeedHook) Run(rt *wiop.Runtime) {
	seed := GetSharedRandomnessSeed(rt)
	rt.SetFSState(seed)
}
