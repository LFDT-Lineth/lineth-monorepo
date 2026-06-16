package codegen

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
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
	return out, nil
}
