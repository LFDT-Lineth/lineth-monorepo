package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/global"
)

func boolModuleSystem(name string) *wiop.System {
	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewVanishing(sys.Context.Childf("bool"), wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()))
	global.Compile(sys)
	return sys
}

func TestBuildCoinRouting(t *testing.T) {
	// global.Compile appends a quotient round (merge coin) and an eval round
	// (eval coin); the witness round 0 carries no coins.
	routing, err := BuildCoinRouting(boolModuleSystem("routing"))
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}

	if got := routing.RoundCoinCounts[0]; got != 0 {
		t.Fatalf("round 0 coin count = %d, want 0", got)
	}
	if routing.TotalRoundCoins == 0 {
		t.Fatalf("total round coins = 0, want > 0")
	}

	// Offsets must be the running prefix sum of counts, and the final sum must
	// equal TotalRoundCoins.
	sum := 0
	for i, count := range routing.RoundCoinCounts {
		if routing.RoundCoinOffsets[i] != sum {
			t.Fatalf("offset[%d] = %d, want %d", i, routing.RoundCoinOffsets[i], sum)
		}
		sum += count
	}
	if sum != routing.TotalRoundCoins {
		t.Fatalf("sum of counts = %d, want total %d", sum, routing.TotalRoundCoins)
	}
}

func TestBuildCoinRoutingRejectsRoundZeroCoins(t *testing.T) {
	sys := wiop.NewSystemf("round-zero-coin")
	r0 := sys.NewRound()
	r0.NewCoinField(sys.Context.Childf("illegal-r0-coin"))

	if _, err := BuildCoinRouting(sys); err == nil {
		t.Fatalf("BuildCoinRouting() error = nil, want round-0-coin error")
	}
}

func TestWriteSpecZig(t *testing.T) {
	routing := CoinRouting{
		RoundCoinCounts:  []int{0, 1, 1},
		RoundCoinOffsets: []int{0, 0, 1},
		TotalRoundCoins:  2,
	}

	var buf bytes.Buffer
	if err := WriteSpecZig(&buf, routing); err != nil {
		t.Fatalf("WriteSpecZig() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		`const protocol = @import("../protocol/root.zig");`,
		"pub const spec = protocol.Spec{",
		".round_coin_counts = &[_]usize{ 0, 1, 1 },",
		".round_coin_offsets = &[_]usize{ 0, 0, 1 },",
		".total_round_coins = 2,",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("generated spec missing %q\n--- got ---\n%s", want, out)
		}
	}
}
