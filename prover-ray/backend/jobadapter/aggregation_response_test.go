package jobadapter

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceAggregationResponse = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/10-18-getZkRollupAggregationProofV1.response.json"

// TestNewAggregationResponse_MatchesReferenceShape verifies the placeholder
// aggregation response has the exact V1 field shape (no top-level programVk; the
// revealed arrays live in publicInputs). Values stay placeholder until the guest lands.
func TestNewAggregationResponse_MatchesReferenceShape(t *testing.T) {
	resp := newAggregationResponse(backend.Result{ProofBytes: []byte{0xde, 0xad}}, 1000501, "test-version")

	assert.Equal(t, "test-version", resp.ProverVersion)
	assert.Equal(t, "0xdead", resp.ProofHex)
	assert.Equal(t, uint64(1000501), resp.StartBlockNumber)
	assert.Empty(t, resp.PublicInputs.L2L1Roots)
	assert.Empty(t, resp.PublicInputs.FilteredAddresses)
	assert.Empty(t, resp.PublicInputs.ProgramIDs)
	assert.Empty(t, resp.PublicInputs.L2MessagingBlocksOffsets)

	raw, err := jsonMarshalObject(resp)
	require.NoError(t, err)
	want := unmarshalObject(t, readSpecFixture(t, referenceAggregationResponse))
	assertJSONShape(t, want, raw, "aggregationResponse")
	assert.NotContains(t, raw, "programVk")
	assert.NotContains(t, raw, "status")
}

// TestNewAggregationResponse_MatchesReferenceValues is the target test for the
// fully wired aggregation response, skipped until the guest and proof
// serialization land.
func TestNewAggregationResponse_MatchesReferenceValues(t *testing.T) {
	t.Skip("enable after aggregation public-input extraction, revealed arrays, and proof serialization are wired")
}
