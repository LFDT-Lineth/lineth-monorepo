package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/require"
)

// TestConfigure_ConsumedByCompile checks that a preconfigured opt-out reaches
// an option-less Compile on the unsharded round-0 layout.
func TestConfigure_ConsumedByCompile(t *testing.T) {
	sys := buildShardWithBusOnRound(t, 1, 0)
	require.NoError(t, messagebus.Configure(sys, messagebus.WithoutSharedRandomness()))
	messagebus.Compile(sys)
	require.False(t, messagebus.HasSharedRandomness(sys), "Compile must use the preconfigured opt-out")
}

// TestConfigure_ConflictsWithConsumedDefault checks that the shared-randomness
// default consumed by Compile cannot be changed afterwards.
func TestConfigure_ConflictsWithConsumedDefault(t *testing.T) {
	sys := buildShardWithBusOnRound(t, 2, 1)
	messagebus.Compile(sys)
	require.True(t, messagebus.HasSharedRandomness(sys), "shared randomness is the default")
	require.ErrorContains(t, messagebus.Configure(sys, messagebus.WithoutSharedRandomness()),
		"shared randomness false conflicts with preconfigured true",
		"the consumed default must be frozen")
}

// TestConfigure_RequiresOption checks that an empty configuration is rejected.
func TestConfigure_RequiresOption(t *testing.T) {
	sys := buildShardWithBusOnRound(t, 1, 0)
	require.ErrorContains(t, messagebus.Configure(sys), "at least one option")
}
