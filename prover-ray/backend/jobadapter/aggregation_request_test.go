package jobadapter

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceAggregationRequest = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/10-18-getZkRollupAggregationProofV1.request.json"

// testRollupPublicInputs are the public inputs that the publicInputs-format
// tests put in each embedded rollup proof.
var testRollupPublicInputs = backend.RollupPublicInputs{
	EndBlockNumber:                        18,
	EndBlockTimestamp:                     1763000457,
	EndL1L2BridgeRollingHash:              filledHash(0x03),
	EndL1L2BridgeRollingHashMessageNumber: 7,
	DynamicChainConfigHash:                filledHash(0xc0),
	ParentFtxRollingHash:                  filledHash(0x04),
	ParentFtxNumber:                       15,
	EndFtxRollingHash:                     filledHash(0x05),
	EndProcessedFtxNumber:                 18,
	ParentDataRollingHash:                 filledHash(0x47),
	EndDataRollingHash:                    filledHash(0x8d),
	ParentBlockHash:                       filledHash(0x0a),
	EndBlockHash:                          filledHash(0x0b),
	ParentSharedBlobCarryOverTailBytes:    4,
	FinalSharedBlobCarryOverTailBytes:     2,
	L2L1Messages:                          [][32]byte{filledHash(0x77), filledHash(0x88)},
	FilteredAddresses:                     [][20]byte{{0x01}},
	ProgramVks:                            [][32]byte{filledHash(0xaa)},
	BlockCount:                            9,
	L2MessagingBlocksOffsets:              []uint16{3},
}

// TestDecodeAggregationRequest_Golden decodes the reference fixture and checks
// the envelope and the embedded rollup proofs with their guestOutput.
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
	assert.Equal(t, uint64(12), req.RollupProofs[1].StartBlockNumber)

	outputs := embeddedGuestOutputs(t, unmarshalObject(t, readSpecFixture(t, referenceAggregationRequest)), rollupProofsKey)
	for i, p := range req.RollupProofs {
		assert.Equal(t, mustHex(t, outputs[i]), p.GuestOutput, "proof %d", i)
		assert.Zero(t, p.PublicInputs, "proof %d", i)
	}
}

// TestDecodeAggregationRequest_PublicInputs checks that the embedded proofs of a
// request in the publicInputs format decode to their public inputs.
func TestDecodeAggregationRequest_PublicInputs(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceAggregationRequest))
	withPublicInputs(t, obj, rollupProofsKey, rollupPublicInputs(testRollupPublicInputs))

	req, err := DecodeAggregationRequest(marshal(t, obj))
	require.NoError(t, err)
	require.Len(t, req.RollupProofs, 2)
	for i, p := range req.RollupProofs {
		assert.Equal(t, testRollupPublicInputs, backend.RollupPublicInputs(p.PublicInputs), "proof %d", i)
		assert.Nil(t, p.GuestOutput, "proof %d", i)
	}
}

// TestDecodeAggregationRequest_GuestOutputOverPublicInputs checks that an
// embedded rollup proof carrying both guestOutput and publicInputs is read from
// guestOutput.
func TestDecodeAggregationRequest_GuestOutputOverPublicInputs(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceAggregationRequest))
	outputs := embeddedGuestOutputs(t, obj, rollupProofsKey)
	withPublicInputs(t, obj, rollupProofsKey, rollupPublicInputs(testRollupPublicInputs))
	for i, p := range obj[proofRequestKey].(map[string]any)[rollupProofsKey].([]any) {
		p.(map[string]any)[guestOutputKey] = outputs[i]
	}

	req, err := DecodeAggregationRequest(marshal(t, obj))
	require.NoError(t, err)
	for i, p := range req.RollupProofs {
		assert.Equal(t, mustHex(t, outputs[i]), p.GuestOutput, "proof %d", i)
		assert.Zero(t, p.PublicInputs, "proof %d", i)
	}
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
		{"ProofWithoutPublicInputsOrGuestOutput",
			func(o map[string]any) { delete(proof0(o), publicInputsKey) }, guestOutputKey},
		{"ProofGuestOutputNotHex",
			func(o map[string]any) {
				delete(proof0(o), publicInputsKey)
				proof0(o)[guestOutputKey] = "0xzz"
			},
			guestOutputKey},
		{"PublicInputsMissingProgramVks", func(o map[string]any) { delete(pi0(o), programVksKey) }, programVksKey},
		{"PublicInputsMissingHashField", func(o map[string]any) { delete(pi0(o), "endDataRollingHash") }, "endDataRollingHash"},
		{"PublicInputsMissingParentSharedBlobCarryOverTailBytes",
			func(o map[string]any) { delete(pi0(o), parentSharedBlobCarryOverTailBytesKey) }, parentSharedBlobCarryOverTailBytesKey},
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
			withPublicInputs(t, obj, rollupProofsKey, rollupPublicInputs(testRollupPublicInputs))
			tc.mutate(obj)
			_, err := DecodeAggregationRequest(marshal(t, obj))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
