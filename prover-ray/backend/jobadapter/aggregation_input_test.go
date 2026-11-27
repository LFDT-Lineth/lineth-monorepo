package jobadapter

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The goldens come from rollup_spec, see utils/ssz/testdata/generate.py.
func TestBuildAggregationInput_MatchesSpec(t *testing.T) {
	reference := readSpecFixture(t, referenceAggregationRequest)

	obj := unmarshalObject(t, reference)
	proof := obj[proofRequestKey].(map[string]any)[rollupProofsKey].([]any)[0].(map[string]any)
	proof[publicInputsKey].(map[string]any)[l2MessagingBlocksOffsetsKey] = []any{1, 258}

	for name, tc := range map[string]struct {
		request []byte
		golden  string
	}{
		"reference": {reference, "aggregation_input.ssz"},
		"offsets":   {marshal(t, obj), "aggregation_input_offsets.ssz"},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := DecodeAggregationRequest(tc.request)
			require.NoError(t, err)
			want, err := os.ReadFile("../../utils/ssz/testdata/" + tc.golden)
			require.NoError(t, err)
			in, err := buildAggregationInput(req)
			require.NoError(t, err)
			got, err := ssz.EncodeAggregationInput(in)
			require.NoError(t, err)
			assert.Equal(t, want, got, "the input should equal the spec's encoding of the request")
		})
	}
}

// The first rollup proof carries the public inputs of the spec's rollup
// response, as publicInputs or as the guest output the spec encodes for them
// (utils/ssz/testdata/generate.py).
func TestBuildAggregationInput_GuestOutput(t *testing.T) {
	response := unmarshalObject(t, readSpecFixture(t, referenceRollupResponse))
	rollupOutput, err := os.ReadFile("../../utils/ssz/testdata/rollup_output.ssz")
	require.NoError(t, err)
	l2ExecutionOutput, err := os.ReadFile("../../utils/ssz/testdata/l2_execution_output.ssz")
	require.NoError(t, err)

	build := func(set func(proof map[string]any)) (ssz.AggregationInput, error) {
		obj := unmarshalObject(t, readSpecFixture(t, referenceAggregationRequest))
		set(obj[proofRequestKey].(map[string]any)[rollupProofsKey].([]any)[0].(map[string]any))
		req, err := DecodeAggregationRequest(marshal(t, obj))
		require.NoError(t, err)
		return buildAggregationInput(req)
	}
	withGuestOutput := func(out []byte) func(map[string]any) {
		return func(proof map[string]any) {
			delete(proof, publicInputsKey)
			proof[guestOutputKey] = "0x" + hex.EncodeToString(out)
		}
	}

	fromPublicInputs, err := build(func(proof map[string]any) { proof[publicInputsKey] = response[publicInputsKey] })
	require.NoError(t, err)
	fromGuestOutput, err := build(withGuestOutput(rollupOutput))
	require.NoError(t, err)
	want, err := ssz.EncodeAggregationInput(fromPublicInputs)
	require.NoError(t, err)
	got, err := ssz.EncodeAggregationInput(fromGuestOutput)
	require.NoError(t, err)
	assert.Equal(t, want, got, "guestOutput and publicInputs should build the same input")

	_, err = build(withGuestOutput(l2ExecutionOutput))
	assert.ErrorContains(t, err, "rollupProofs[0].guestOutput", "an l2-execution guest output should be rejected")
}
