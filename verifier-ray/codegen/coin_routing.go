package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
)

// CoinRouting is the protocol-level Fiat-Shamir coin layout shared by every
// sub-verifier. It is the source for the standalone protocol.Spec constant and
// is built once per system rather than duplicated inside each sub-verifier's
// System.
type CoinRouting struct {
	// RoundCoinCounts[i] is the number of coins squeezed after round i is
	// absorbed. Index 0 is always 0: no coins precede the first round message.
	RoundCoinCounts []int
	// RoundCoinOffsets[i] is the start index of round i's coins in the flat
	// all_coins array consumed by the Zig verifier.
	RoundCoinOffsets []int
	// TotalRoundCoins is the total number of coins across all rounds; the
	// length of the Zig verifier's all_coins array.
	TotalRoundCoins int
	// DynamicModuleCount is the number of distinct dynamically-sized modules
	// whose runtime sizes the prover absorbs into the transcript at each round
	// advance (prover-ray's AdvanceRound). The Zig verifier's replay absorbs the
	// first DynamicModuleCount entries of module_sizes at every round to stay
	// byte-exact with the prover. Counted in the same order as the proof's
	// module_sizes (VanishingSystem dynamic-module order).
	DynamicModuleCount int
	// SharedRandomnessCoinRound is the round index whose coins must be derived
	// from γ (messagebus.CompileOptions.SharedRandomness) rather than from this
	// shard's own transcript state, or -1 if the system was not compiled with
	// that option. Mirrors prover-ray's Runtime.AdvanceRound, which runs every
	// Round.PreSamplingHooks entry — here, exactly the
	// messagebus.SharedRandomnessSeedHook — before deriving the round's coins:
	// without replaying that override, the Zig verifier's replay derives a
	// different α/β than the prover did and every downstream check fails.
	SharedRandomnessCoinRound int
	// SharedRandomnessGammaRefs locates γ's NumSharedRandomness base-field limbs
	// in the replayed transcript, in limb order, so the Zig verifier can rebuild
	// the octuplet and install it as the Fiat-Shamir state before
	// SharedRandomnessCoinRound's coins are squeezed. Empty when
	// SharedRandomnessCoinRound is -1.
	SharedRandomnessGammaRefs []PublicInputRef
}

// BuildCoinRouting extracts the protocol-level coin layout from a compiled
// system. The layout is shared across all sub-verifiers, so it is emitted as a
// single protocol.Spec rather than recomputed per sub-verifier.
//
// It enforces the spec invariant that round 0 squeezes no coins: coins are
// always derived after a round message is absorbed, so the first round cannot
// carry any. Catching this here fails generation loudly instead of at Zig
// compile time.
func BuildCoinRouting(sys *wiop.System) (CoinRouting, error) {
	out := CoinRouting{
		RoundCoinCounts:  make([]int, len(sys.Rounds)),
		RoundCoinOffsets: make([]int, len(sys.Rounds)),
	}
	for i, round := range sys.Rounds {
		out.RoundCoinOffsets[i] = out.TotalRoundCoins
		out.RoundCoinCounts[i] = len(round.Coins)
		out.TotalRoundCoins += len(round.Coins)
	}
	if len(out.RoundCoinCounts) > 0 && out.RoundCoinCounts[0] != 0 {
		return CoinRouting{}, fmt.Errorf(
			"codegen: round 0 has %d coins; protocol.Spec requires round_coin_counts[0] == 0",
			out.RoundCoinCounts[0],
		)
	}

	// The number of dynamically-sized modules whose sizes the transcript absorbs
	// once per round advance. This MUST match the count and order prover-ray's
	// `Runtime.AdvanceRound` uses, which iterates `sys.Modules` in module-index
	// order (NOT verifier-action-registration order). See DynamicModuleOrder.
	out.DynamicModuleCount = len(DynamicModuleOrder(sys))

	sharedRandomnessRound, err := sharedRandomnessCoinRound(sys)
	if err != nil {
		return CoinRouting{}, err
	}
	out.SharedRandomnessCoinRound = -1
	if sharedRandomnessRound != nil {
		out.SharedRandomnessCoinRound = sharedRandomnessRound.ID
		out.SharedRandomnessGammaRefs = make([]PublicInputRef, messagebus.NumSharedRandomness)
		for i := range out.SharedRandomnessGammaRefs {
			cell, pos := sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedPI, i)
			if pos < 0 {
				return CoinRouting{}, fmt.Errorf(
					"codegen: BuildCoinRouting: HasSharedRandomness is true but gamma limb %d is missing", i)
			}
			out.SharedRandomnessGammaRefs[i] = PublicInputRef{
				StatementIndex: pos,
				Round:          cell.Context.ID.Slot(),
				Index:          cell.Context.ID.Position(),
			}
		}
	}

	return out, nil
}

// sharedRandomnessCoinRound returns the round messagebus.registerSharedRandomness
// attached its Fiat-Shamir-overriding pre-sampling hook to, or nil if sys was not
// compiled with messagebus.CompileOptions.SharedRandomness. Detected structurally
// (any round carrying a pre-sampling hook) rather than by re-deriving
// messagebus's own coin-round choice, so this stays correct even if that choice
// changes.
//
// Errors if more than one round carries a hook: [wiop.Round.RegisterPreSamplingHook]
// allows stacking hooks on one round (last one wins) but never spreads them
// across rounds in any wiop compiler today, so more than one hook round would
// mean either a new compiler this function does not know about yet, or that the
// single-hook assumption baked into the Zig replay no longer holds.
func sharedRandomnessCoinRound(sys *wiop.System) (*wiop.Round, error) {
	if !messagebus.HasSharedRandomness(sys) {
		return nil, nil
	}
	var found *wiop.Round
	for _, r := range sys.Rounds {
		if len(r.PreSamplingHooks) == 0 {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf(
				"codegen: BuildCoinRouting: rounds %d and %d both carry pre-sampling hooks; "+
					"the Zig replay only supports a single shared-randomness override round",
				found.ID, r.ID)
		}
		found = r
	}
	if found == nil {
		return nil, fmt.Errorf(
			"codegen: BuildCoinRouting: HasSharedRandomness is true but no round carries a pre-sampling hook")
	}
	return found, nil
}

// DynamicModuleOrder returns the dynamically-sized modules in the canonical
// order the verifier must use for `module_sizes`: prover-ray's
// `Runtime.AdvanceRound` absorbs one size per dynamic module by iterating
// `sys.Modules` in module-index order, so the verifier's `module_sizes` slice
// (and every `DynamicIndex` into it) must follow that same order to reproduce
// the transcript. This is the single source of truth for dynamic-module
// ordering, shared by BuildCoinRouting (count), BuildVanishingSystem
// (DynamicIndex), and the fixture generator (module_sizes slice).
//
// Returns modules in `sys.Modules` order (dynamic ones only). `out[i]` is the
// module whose runtime size occupies `module_sizes[i]`.
func DynamicModuleOrder(sys *wiop.System) []*wiop.Module {
	var out []*wiop.Module
	for _, m := range sys.Modules {
		if m.IsDynamic() {
			out = append(out, m)
		}
	}
	return out
}

// DynamicModuleIndex returns a map from each dynamic module to its index in the
// canonical DynamicModuleOrder — i.e. its slot in the verifier's `module_sizes`.
func DynamicModuleIndex(sys *wiop.System) map[*wiop.Module]int {
	order := DynamicModuleOrder(sys)
	idx := make(map[*wiop.Module]int, len(order))
	for i, m := range order {
		idx[m] = i
	}
	return idx
}

// DynamicModuleSizes returns the runtime size of every dynamic module in sys,
// in DynamicModuleOrder — the same order PcsColumnDesc.DynamicIndex and the
// proof's `module_sizes` slice both reference.
func DynamicModuleSizes(sys *wiop.System, rt *wiop.Runtime) []int {
	order := DynamicModuleOrder(sys)
	sizes := make([]int, len(order))
	for i, m := range order {
		sizes[i] = m.RuntimeSize(rt)
	}
	return sizes
}
