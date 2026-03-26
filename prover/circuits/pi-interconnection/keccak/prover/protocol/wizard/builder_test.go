package wizard_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestRound(t *testing.T) {
	define := func(b *wizard.Builder) {
		_ = b.RegisterCommit("P", 16)
		x := b.RegisterRandomCoin("X", coin.Field)
		require.Equal(t, 1, x.Round)
		y := b.RegisterRandomCoin("Y", coin.Field)
		require.Equal(t, 1, y.Round)
	}

	wizard.Compile(define)
}
