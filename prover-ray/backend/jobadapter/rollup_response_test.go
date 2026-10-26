package jobadapter

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRollupResponse_MatchesReferenceShape verifies the placeholder rollup
// response has the exact V1 field shape (including the 22-field PI) and echoes
// the routing programVk. Values stay placeholder until the guest lands.
func TestNewRollupResponse_MatchesReferenceShape(t *testing.T) {
	vk := filledHash(0xbb)
	resp := newRollupResponse(backend.Result{ProofBytes: []byte{0xde, 0xad}}, 1000501, "test-version", vk[:])

	assert.Equal(t, "test-version", resp.ProverVersion)
	assert.Equal(t, repeatHex(0xbb), resp.ProgramVk)
	assert.Equal(t, "0xdead", resp.ProofHex)
	assert.Equal(t, uint64(1000501), resp.StartBlockNumber)
	assert.Empty(t, resp.PublicInputs.ProgramVks)
	assert.Empty(t, resp.PublicInputs.FilteredAddresses)
	assert.Empty(t, resp.PublicInputs.L2L1Messages)
	assert.Empty(t, resp.PublicInputs.L2MessagingBlocksOffsets)

	raw, err := jsonMarshalObject(resp)
	require.NoError(t, err)
	want := unmarshalObject(t, readSpecFixture(t, referenceRollupResponse))
	assertJSONShape(t, want, raw, "rollupResponse")
	assert.NotContains(t, raw, "status")
	assert.NotContains(t, raw, "jobId")
}

// TestNewRollupResponse_MatchesReferenceValues is the target test for the fully
// wired rollup response. It stays skipped until the rollup guest emits real
// public inputs / revealed arrays and proof serialization lands.
func TestNewRollupResponse_MatchesReferenceValues(t *testing.T) {
	t.Skip("enable after rollup public-input extraction, revealed arrays, and proof serialization are wired")
}
