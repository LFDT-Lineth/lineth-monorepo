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
	// DynamicSizeSlots is the Fiat-Shamir absorb schedule for dynamic module
	// sizes: the `module_sizes` indices absorbed at the start of every round, in
	// ascending `System.Modules` order. This is the exact order prover-ray's
	// `AdvanceRound` feeds each dynamic module's size into the transcript. The
	// runtime `module_sizes` slice is laid out so slot j is the j-th dynamic
	// module in that order, so this is the dense identity list `0..n`; it is
	// emitted explicitly so the Zig `Spec` is self-describing and the layout
	// contract is visible rather than implicit.
	DynamicSizeSlots []int
}

// DynamicModuleRanks maps each dynamic module to its dense rank in ascending
// `System.Modules` order. This single source of truth aligns two consumers:
//   - the Fiat-Shamir absorb schedule (CoinRouting.DynamicSizeSlots), which must
//     match prover-ray's `AdvanceRound` module-index iteration order; and
//   - the vanishing sub-verifier's `DynamicIndex`, which indexes the same
//     runtime `module_sizes` slice.
// Keeping both on this ordering means `module_sizes[rank]` is unambiguous.
func DynamicModuleRanks(sys *wiop.System) map[*wiop.Module]int {
	ranks := map[*wiop.Module]int{}
	for _, mod := range sys.Modules {
		if mod.IsDynamic() {
			ranks[mod] = len(ranks)
		}
	}
	return ranks
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
	// The absorb schedule is the dense set of dynamic-module ranks in
	// System.Modules order, i.e. 0..DynamicModuleCount. Emitting it explicitly
	// keeps the module_sizes layout contract in the generated Spec.
	dynamicCount := len(DynamicModuleRanks(sys))
	out.DynamicSizeSlots = make([]int, dynamicCount)
	for i := range out.DynamicSizeSlots {
		out.DynamicSizeSlots[i] = i
	}
	return out, nil
}
