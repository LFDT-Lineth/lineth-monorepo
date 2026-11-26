package jobadapter

import (
	"encoding/json"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRollupResponse_MatchesReferenceShape verifies that the response of a
// result with no public inputs has the exact V1 field shape, with empty lists,
// and carries the programVk of the result.
func TestNewRollupResponse_MatchesReferenceShape(t *testing.T) {
	result := backend.Result{ProofBytes: []byte{0xde, 0xad}, ProgramVK: filledHash(0xbb)}
	resp := newRollupResponse(result, 1000501, "test-version")

	assert.Equal(t, "test-version", resp.ProverVersion)
	assert.Equal(t, repeatHex(0xbb), resp.ProgramVk, "programVk should be the VK of the result")
	assert.Equal(t, "0xdead", resp.ProofHex)
	assert.Equal(t, uint64(1000501), resp.StartBlockNumber)
	assert.Empty(t, resp.PublicInputs.ProgramVks)
	assert.Empty(t, resp.PublicInputs.FilteredAddresses)
	assert.Empty(t, resp.PublicInputs.L2L1Messages)
	assert.Empty(t, resp.PublicInputs.L2MessagingBlocksOffsets)

	raw, err := jsonMarshalObject(resp)
	require.NoError(t, err)
	require.Contains(t, raw, publicInputsKey)
	delete(raw, publicInputsKey)
	assertJSONShape(t, unmarshalObject(t, readSpecFixture(t, referenceRollupResponse)), raw, "rollupResponse")
	assert.NotContains(t, raw, "status")
	assert.NotContains(t, raw, "jobId")
}

// TestNewRollupResponse_MatchesReferenceValues checks that a result holding the
// guest output of the spec's reference response gives that response, plus the
// publicInputs of the result.
func TestNewRollupResponse_MatchesReferenceValues(t *testing.T) {
	want := unmarshalObject(t, readSpecFixture(t, referenceRollupResponse))
	result := backend.Result{
		ProofBytes:         mustHex(t, want[proofKey].(string)),
		ProgramVK:          [32]byte(mustHex(t, want[programVkKey].(string))),
		GuestOutput:        mustHex(t, want[guestOutputKey].(string)),
		RollupPublicInputs: testRollupPublicInputs,
	}
	resp := newRollupResponse(result, uint64(want[startBlockNumberKey].(float64)), want["proverVersion"].(string))

	got, err := jsonMarshalObject(resp)
	require.NoError(t, err)
	var piObj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(marshal(t, got[publicInputsKey]), &piObj))
	pi, err := decodeRollupProofPublicInputs(piObj, decodeAggregationOp, "")
	require.NoError(t, err)
	assert.Equal(t, testRollupPublicInputs, backend.RollupPublicInputs(pi), "publicInputs should read back to the result's")

	delete(got, publicInputsKey)
	assert.Equal(t, want, got)
}
