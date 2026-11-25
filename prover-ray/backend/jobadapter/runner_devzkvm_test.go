package jobadapter

import (
	"context"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests shared with dev-partial are in runner_extended_test.go.

// The native runner is optional in dev-zkvm: without it, the job passes on
// the guest's own output.
func TestRunner_DevZkVM_WithoutNativeRunner(t *testing.T) {
	mode := backend.ProverModeDevZkVM
	runner, err := NewRunner(extendedProver(mode, guestOutput, nil), testProverVersion, WithMode(mode))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dz-3", readFixture(t, "request_single_block.json")))

	require.Equal(t, RunStatusSuccess, result.Status, "run err: %v", result.Err)
	resp, ok := result.ResponseBody.(executionResponse)
	require.True(t, ok)
	assert.Equal(t, hexBytes(extendedModes[mode]), resp.ProofHex)
}
