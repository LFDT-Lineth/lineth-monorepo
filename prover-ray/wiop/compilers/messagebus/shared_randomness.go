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

// registerSharedRandomness declares the two inputs a sharded protocol takes
// from the orchestrator as public inputs: γ, the seed every shard shares, and
// this shard's contribution to it. Both are supplied from outside the proof by
// [AssignSharedRandomness] — the prover writes them, the verifier reads them
// back from the public-input vector — and neither is derived in-shard.
//
// γ gets [NumSharedRandomness] cells on round 0, each registered under
// [SharedRandomnessSeedPI] with its limb index as numeric suffix. Round 0 is
// where it has to live: being absorbed into Fiat-Shamir on the way out of that
// round is what lets the challenges drawn later depend on it.
//
// The contribution gets [NumSharedRandomnessContribution] cells on the round
// this function appends, under [SharedRandomnessSeedContributionPI]. That round
// also carries [SharedRandomnessChecker], which checks both groups
// of cells are declared and base-field.
func registerSharedRandomness(sys *wiop.System, opt CompileOptions) (alpha, beta *wiop.CoinField) {

	compCtx := sys.Context.Childf("message-bus")
	sys.NewRound()
	coinRound := sys.CurrentRound() // coins for the bus message are generated in the round imidiatly after the seed, this is also  where preflight dtata lands
	alpha = coinRound.NewCoinField(compCtx.Childf("alpha"))
	// Declare β on the same round, drawn from the same Fiat–Shamir state as α.
	beta = coinRound.NewCoinField(compCtx.Childf("beta"))

	if opt.SharedRandomness {

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

// HasSharedRandomness reports whether [registerSharedRandomness] ran on sys and
// it therefore carries a γ to assign.

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

// sharedRandomnessContribution generates the contribution of the shard in the shardrandomness,
// the assumtion is that preflight columns are isolated and would land on the same round, alowing to calculate the contribution from this round.
// the assumption is inforced in the backend level
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
