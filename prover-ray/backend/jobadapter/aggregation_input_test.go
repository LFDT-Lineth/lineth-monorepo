package jobadapter

import (
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
			got, err := ssz.EncodeAggregationInput(buildAggregationInput(req))
			require.NoError(t, err)
			assert.Equal(t, want, got, "the input should equal the spec's encoding of the request")
		})
	}
}
