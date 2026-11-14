package jobadapter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/guests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestGuestProgramID reads guestProgramId from a request body.
func requestGuestProgramID(t *testing.T, body []byte) [32]byte {
	t.Helper()
	var env struct {
		GuestProgramID string `json:"guestProgramId"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	b, err := hex.DecodeString(strings.TrimPrefix(env.GuestProgramID, "0x"))
	require.NoError(t, err)
	return [32]byte(b)
}

const guestJobID = "guest-job"

// guestRequests is one request of each proof type.
func guestRequests(t *testing.T) map[string]RunRequest {
	t.Helper()
	return map[string]RunRequest{
		"l2-execution": {ID: guestJobID, Type: backend.ProofTypeL2Execution, Body: readFixture(t, "request_single_block.json")},
		"rollup":       {ID: guestJobID, Type: backend.ProofTypeRollup, Body: readSpecFixture(t, referenceRollupRequest)},
		"aggregation":  {ID: guestJobID, Type: backend.ProofTypeRollupAggregation, Body: readSpecFixture(t, referenceAggregationRequest)},
	}
}

func TestRunner_PassesGuestProgramID(t *testing.T) {
	for name, req := range guestRequests(t) {
		t.Run(name, func(t *testing.T) {
			mock := &mockProver{}
			runner, err := NewRunner(mock, testProverVersion)
			require.NoError(t, err)

			result := runner.Run(context.Background(), req)
			require.Equal(t, RunStatusSuccess, result.Status, "run error: %v", result.Err)
			require.Len(t, mock.jobs, 1)
			assert.Equal(t, requestGuestProgramID(t, req.Body), mock.jobs[0].GuestProgramID,
				"the job must carry the request's guestProgramId")
		})
	}
}

func TestRunner_UnknownGuestIsInvalidInput(t *testing.T) {
	unknown := func(job backend.Job) backend.Result {
		return backend.Result{JobID: job.ID, Status: backend.ResultStatusFailed,
			Err: fmt.Errorf("loading guest: %w", guests.ErrUnknownGuest)}
	}
	for name, req := range guestRequests(t) {
		t.Run(name, func(t *testing.T) {
			runner, err := NewRunner(&mockProver{result: unknown}, testProverVersion)
			require.NoError(t, err)

			result := runner.Run(context.Background(), req)
			assert.Equal(t, RunStatusFailed, result.Status)
			assert.Equal(t, FailureCodeInvalidInput, result.FailureCode,
				"a guestProgramId with no guest ELF is a bad request, not a prover bug")
		})
	}

	t.Run("dev-zkvm", func(t *testing.T) {
		runner, err := NewRunner(&mockProver{result: unknown}, testProverVersion,
			WithMode(backend.ProverModeDevZkVM),
			WithNativeRunnerBin(fakeNativeRunner(t, guestOutput)))
		require.NoError(t, err)

		result := runner.Run(context.Background(),
			l2ExecutionRunRequest(guestJobID, readFixture(t, "request_single_block.json")))
		assert.Equal(t, FailureCodeInvalidInput, result.FailureCode)
	})
}
