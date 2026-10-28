package jobadapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceAggregationRequest = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/10-18-getZkRollupAggregationProofV1.request.json"

// TestDecodeAggregationRequest_Golden decodes the reference fixture and checks
// the envelope and the embedded rollup proofs with their 22-field PI.
func TestDecodeAggregationRequest_Golden(t *testing.T) {
	req, err := DecodeAggregationRequest(readSpecFixture(t, referenceAggregationRequest))
	require.NoError(t, err)

	vk := filledHash(0x33)
	assert.Equal(t, vk[:], req.ProgramVk)

	require.Len(t, req.RollupProofs, 2)
	p0 := req.RollupProofs[0]
	assert.Equal(t, mustHex(t, "0xabcdef"), p0.Proof)
	assert.Equal(t, uint64(10), p0.StartBlockNumber)
	assert.Equal(t, mustHex(t, "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), p0.ProgramVk)
	assert.Equal(t, uint64(11), p0.PublicInputs.EndBlockNumber)
	assert.Equal(t, filledHash(0x8d), p0.PublicInputs.EndDataRollingHash)
	assert.Equal(t, uint64(0), p0.PublicInputs.ParentDataTailTakeBytes)
	assert.Equal(t, uint64(0), p0.PublicInputs.FinalDataTailDiscardBytes)
	assert.Equal(t, uint64(2), p0.PublicInputs.BlockCount)
	require.Len(t, p0.PublicInputs.ProgramVks, 1)
	assert.Equal(t, filledHash(0xaa), p0.PublicInputs.ProgramVks[0])
	assert.Len(t, p0.PublicInputs.L2L1Messages, 2)
	assert.Len(t, p0.PublicInputs.FilteredAddresses, 1)
	assert.Empty(t, p0.PublicInputs.L2MessagingBlocksOffsets)

	assert.Equal(t, uint64(12), req.RollupProofs[1].StartBlockNumber)
	assert.Equal(t, uint64(18), req.RollupProofs[1].PublicInputs.EndBlockNumber)
}

func TestDecodeAggregationRequest_InvalidJSON(t *testing.T) {
	_, err := DecodeAggregationRequest([]byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing JSON")
}

// TestDecodeAggregationRequest_InvalidShape sweeps the aggregation envelope error paths.
func TestDecodeAggregationRequest_InvalidShape(t *testing.T) {
	pr := func(o map[string]any) map[string]any { return o[proofRequestKey].(map[string]any) }
	proof0 := func(o map[string]any) map[string]any {
		return pr(o)[rollupProofsKey].([]any)[0].(map[string]any)
	}
	pi0 := func(o map[string]any) map[string]any { return proof0(o)[publicInputsKey].(map[string]any) }

	//nolint:goconst
	cases := []struct {
		name    string
		mutate  func(o map[string]any)
		wantErr string
	}{
		{"MissingGuestProgramId", func(o map[string]any) { delete(o, guestProgramIDKey) }, guestProgramIDKey},
		{"GuestProgramIdWrongLength", func(o map[string]any) { o[guestProgramIDKey] = "0x1234" }, guestProgramIDKey},
		{"MissingProofRequest", func(o map[string]any) { delete(o, proofRequestKey) }, proofRequestKey},
		{"MissingRollupProofs", func(o map[string]any) { delete(pr(o), rollupProofsKey) }, rollupProofsKey},
		{"EmptyRollupProofs", func(o map[string]any) { pr(o)[rollupProofsKey] = []any{} }, rollupProofsKey},
		{"ProofMissingProgramVk", func(o map[string]any) { delete(proof0(o), programVkKey) }, programVkKey},
		{"ProofMissingProof", func(o map[string]any) { delete(proof0(o), proofKey) }, proofKey},
		{"ProofMissingPublicInputs", func(o map[string]any) { delete(proof0(o), publicInputsKey) }, publicInputsKey},
		{"PublicInputsMissingProgramVks", func(o map[string]any) { delete(pi0(o), programVksKey) }, programVksKey},
		{"PublicInputsMissingHashField", func(o map[string]any) { delete(pi0(o), "endDataRollingHash") }, "endDataRollingHash"},
		{"PublicInputsMissingParentDataTailTakeBytes",
			func(o map[string]any) { delete(pi0(o), parentDataTailTakeBytesKey) }, parentDataTailTakeBytesKey},
		{"PublicInputsMissingL2L1Messages",
			func(o map[string]any) { delete(pi0(o), l2L1MessagesKey) }, l2L1MessagesKey},
		{"PublicInputsMissingFilteredAddresses",
			func(o map[string]any) { delete(pi0(o), filteredAddressesKey) }, filteredAddressesKey},
		{"PublicInputsMissingBlockCount",
			func(o map[string]any) { delete(pi0(o), blockCountKey) }, blockCountKey},
		{"PublicInputsMissingOffsets",
			func(o map[string]any) { delete(pi0(o), l2MessagingBlocksOffsetsKey) }, l2MessagingBlocksOffsetsKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := unmarshalObject(t, readSpecFixture(t, referenceAggregationRequest))
			tc.mutate(obj)
			_, err := DecodeAggregationRequest(marshal(t, obj))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
