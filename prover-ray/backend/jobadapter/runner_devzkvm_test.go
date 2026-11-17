package jobadapter

import (
	"context"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests shared with dev-partial are in runner_extended_test.go.

func TestRunner_DevZkVM_RequiresBinary(t *testing.T) {
	runner, err := NewRunner(&mockProver{}, testProverVersion, WithMode(backend.ProverModeDevZkVM))
	require.NoError(t, err)

	result := runner.Run(context.Background(),
		l2ExecutionRunRequest("dz-3", readFixture(t, "request_single_block.json")))

	assert.Equal(t, RunStatusFailed, result.Status)
	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "native runner binary")
}
