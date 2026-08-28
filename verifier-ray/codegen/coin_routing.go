package codegen

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
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

	return out, nil
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
