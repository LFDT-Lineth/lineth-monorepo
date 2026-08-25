package messagebus

import (
	"fmt"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
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

// registerSharedRandomness declares γ and this shard's contribution to it as
// public inputs and wires γ into coinRound. It is the implementation of
// [CompileOptions.SharedRandomness] and is called by [Compile] once coinRound is
// fixed; it is deliberately not exported, because a caller invoking it around
// [Compile] rather than through it could attach the hook to a round that does not
// end up carrying α and β.
//
// γ gets [NumSharedRandomness] cells on round 0, each registered under
// [SharedRandomnessSeedPI] with its limb index as numeric suffix, plus a
// [wiop.Round.RegisterPreSamplingHook] on coinRound that replaces the
// Fiat-Shamir state with γ just before α and β are drawn. Because
// [wiop.Runtime.AdvanceRound] fires pre-sampling hooks on the prover and the
// verifier alike, both sides derive the same challenges with no extra verifier
// action. Round 0 is where γ has to live: it is assigned there — by the prover
// through [AssignSharedRandomnessSeed], by the verifier from the public-input
// vector — before the runtime ever advances into coinRound and the hook reads it.
//
// seededCoins are marked with [wiop.CoinField.MarkSeeded] so the seed reaches
// them and nothing else, the transcript being restored once they are drawn.
// coinRound is not this pass's private property — the lookup and permutation
// passes anchor their coin rounds the way [ensureCoinRound] does and regularly
// land on it — and a permanently replaced transcript would strip every later
// challenge of its binding to the preceding rounds.
//
// The contribution gets [NumSharedRandomnessContribution] cells on coinRound,
// under [SharedRandomnessSeedContributionPI], written by
// [SharedRandomnessContributionAssigner] and checked by
// [SharedRandomnessContributionChecker].
//
// Panics if coinRound is round 0, since there is then no earlier round to carry
// γ. That happens when no message-bus entry references a round-bearing column.
func registerSharedRandomness(sys *wiop.System, coinRound *wiop.Round, seededCoins ...*wiop.CoinField) {
	if coinRound.ID == 0 {
		panic(
			"wiop/compilers/messagebus: the message-bus coin round is round 0, " +
				"so there is no earlier round to carry γ. This means no message-bus " +
				"entry references a round-bearing column.",
		)
	}

	ctx := sys.Context.Childf("shared-randomness")
	seedRound := sys.Rounds[0]

	for i := range NumSharedRandomness {
		cell := seedRound.NewCell(ctx.Childf("gamma-%d", i), false)
		sys.RegisterPublicInputs(SharedRandomnessSeedPI, cell, i)
	}

	// The contribution cells sit on coinRound rather than beside γ: their value is
	// a function of the commitments of every round before coinRound, which do not
	// exist until the runtime has advanced past those rounds. On round 0 they would
	// be demanded by AdvanceRound long before anything could compute them.
	for i := range NumSharedRandomnessContribution {
		cell := coinRound.NewCell(ctx.Childf("contribution-%d", i), false)
		sys.RegisterPublicInputs(SharedRandomnessSeedContributionPI, cell, i)
	}

	coinRound.RegisterPreSamplingHook(&SharedRandomnessSeedHook{})
	coinRound.RegisterAction(&SharedRandomnessContributionAssigner{})
	coinRound.RegisterVerifierAction(&SharedRandomnessContributionChecker{})

	// Confine the seed to the caller's own coins.
	for _, coin := range seededCoins {
		coin.MarkSeeded()
	}
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
		// was correctly registered by [registerSharedRandomness], it must be.
		gamma[i] = rt.GetCellValue(c).AsBase()
	}
	return gamma
}

// HasSharedRandomness reports whether sys was compiled with
// [CompileOptions.SharedRandomness] and therefore carries a γ to assign.
//
// An assignment path that does not itself choose the compiler options — the zkc
// driver, say, which is handed a system somebody else compiled — uses this to
// decide whether [AssignSharedRandomnessSeed] applies. Skipping the assignment
// when this is false is safe rather than silently degrading: with the option off
// there is no γ cell, and no hook reading one, so the shard derives α and β from
// its own transcript as an unsharded protocol should.
func HasSharedRandomness(sys *wiop.System) bool {
	_, pos := sys.LookupPublicInputByTag(SharedRandomnessSeedPI, 0)
	return pos >= 0
}

// AssignSharedRandomnessSeed writes γ into the public-input cells declared by
// [CompileOptions.SharedRandomness]. The orchestrator computes γ during the
// preflight phase — outside any proof, from every shard's data — and hands the
// same value to each shard; feeding two shards different values silently
// desynchronizes their α and β and breaks the cross-shard permutation.
//
// It must be called while the runtime is on round 0, which is where the cells
// live ([wiop.Runtime.AssignCell] rejects a cell from any other round).
//
// Panics if sys was compiled without the option, since there is then no cell to
// write to; [HasSharedRandomness] answers that question in advance.
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

// sharedRandomnessContribution maps every commitment preceding the round the
// caller is running on into the multiset-hash group and sums them. The prover
// action and its verifier analog both go through here, so neither can drift from
// the other's preimage: [wiop.Runtime.CurrentRound] is the round the running
// action was registered on on both sides.
//
// The value returned here is exactly the
// Hash(root) that [github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight.Run]
// accumulates for that shard. Combining several rounds with
// [multisethashing.Combine] keeps that correspondence while staying
// order-independent, so no round ordering has to be agreed.
func sharedRandomnessContribution(rt *wiop.Runtime) multisethashing.MSetHash {
	acc := multisethashing.Identity()
	for i := range rt.CurrentRound().ID {
		if !rt.System.Rounds[i].HasCommitment {
			logrus.Warnf(
				"No commitment found for round: %v. Did you use a message bus? "+
					"And did you reduce the current system using a PCS?", i)
			continue
		}

		acc = multisethashing.Combine(acc, multisethashing.Hash(rt.Commitments[i]))
	}

	return acc
}

// contributionCell returns the public-input cell carrying limb i of the shared
// randomness contribution. A missing or extension-field cell means the wizard is
// ill-defined rather than the proof invalid, which is why both are panics.
func contributionCell(sys *wiop.System, i int) *wiop.Cell {
	cell, pos := sys.LookupPublicInputByTag(SharedRandomnessSeedContributionPI, i)
	if pos < 0 {
		panic(fmt.Sprintf("wiop/compilers/messagebus: missing contribution-%d", i))
	}
	if cell.IsExtension() {
		panic(fmt.Sprintf("wiop/compilers/messagebus: contribution-%d must not be an extension-field value", i))
	}
	return cell
}

// Run implements [wiop.ProverAction] on behalf of [SharedRandomnessContributionAssigner].
func (*SharedRandomnessContributionAssigner) Run(rt *wiop.Runtime) {
	contribution := sharedRandomnessContribution(rt)
	for i := range contribution {
		rt.AssignCell(contributionCell(rt.System, i), field.ElemFromBase(contribution[i]))
	}
}

// Check implements the [VerifierAction] interface for
// [SharedRandomnessContributionChecker].
func (*SharedRandomnessContributionChecker) Check(rt *wiop.Runtime) error {
	contribution := sharedRandomnessContribution(rt)
	for i := range contribution {
		if contribution[i] != rt.GetCellValue(contributionCell(rt.System, i)).AsBase() {
			return fmt.Errorf(
				"wiop/compilers/messagebus: SharedRandomnessContributionChecker: mismatch in contribution-%d", i)
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
