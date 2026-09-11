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

// registerSharedRandomness appends the round carrying α and β — the coins the
// bus uses — and returns them.
//
// With [CompileOptions.SharedRandomness] set it also declares two types of public inputs: γ, the seed
// every shard shares, and this shard's contribution to it.
// Round 0 is
// where  γ has to live: being absorbed into Fiat-Shamir on the way out of that
// round is what lets the challenges drawn later depend on it.
func registerSharedRandomness(sys *wiop.System, opt CompileOptions) (alpha, beta *wiop.CoinField) {
	compCtx := sys.Context.Childf("message-bus")

	// The coins for the message bus land on the round immediately after the seed — which is also
	// where the preflight data lands. This draws the bus coins via standard fiat-shamir, as far as the round 0 (and precomputed round) carries the same data over shards, all shards samples the same bus coins.
	coinRound := sys.NewRound()
	alpha = coinRound.NewCoinField(compCtx.Childf("alpha"))
	beta = coinRound.NewCoinField(compCtx.Childf("beta"))

	if opt.SharedRandomness {
		ctx := sys.Context.Childf("shared-randomness")
		seedRound := sys.Rounds[0]

		for i := range NumSharedRandomness {
			cell := seedRound.NewCell(ctx.Childf("gamma-%d", i), false)
			sys.RegisterPublicInputs(SharedRandomnessSeedPI, cell, i)
		}

		// the shard specific preflight data lands on the same round as bus coins, this allows the shard to generate its contribution in the shared randomness  γ.
		for i := range NumSharedRandomnessContribution {
			cell := coinRound.NewCell(ctx.Childf("contribution-%d", i), false)
			sys.RegisterPublicInputs(SharedRandomnessSeedContributionPI, cell, i)
		}
		// register prover and verifier actions of the preflight round
		coinRound.RegisterAction(&SharedRandomnessContributionAssigner{})
		coinRound.RegisterVerifierAction(&SharedRandomnessContributionChecker{})
	}

	return alpha, beta
}

// GetSharedRandomnessSeed returns the god-given value of the shared randomness
// that was provided by [AssignSharedRandomness].
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
// there is no γ cell, so the shard derives α and β from its own transcript as an
// unsharded protocol should.
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

// sharedRandomnessContribution returns this shard's contribution to the shared
// randomness.
// The preflight
// columns are assumed isolated all landed on the same round — an assumption the backend enforces.
func sharedRandomnessContribution(rt *wiop.Runtime) multisethashing.MSetHash {
	if !rt.CurrentRound().HasCommitment {
		logrus.Warnf(
			"No commitment found for round: %v. Did you use a message bus? "+
				"And did you reduce the current system using a PCS?", rt.CurrentRound().ID)
	}

	com := rt.Commitments[rt.CurrentRound().ID]

	return multisethashing.Hash(com)
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
