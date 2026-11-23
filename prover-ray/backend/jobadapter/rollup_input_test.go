package jobadapter

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildRollupInput_MatchesSpec checks that the rollup guest input built from
// a request equals the bytes rollup_spec encodes for the same request
// (utils/ssz/testdata/generate.py).
func TestBuildRollupInput_MatchesSpec(t *testing.T) {
	reference := readSpecFixture(t, referenceRollupRequest)

	// No boundary hash, which the spec only allows with no tail bytes, and a
	// calldata chunk in place of the blob.
	obj := unmarshalObject(t, reference)
	pr := obj[proofRequestKey].(map[string]any)
	pr[parentDataTailTakeBytesKey] = 0
	delete(pr, boundaryPrevDataRollingHashKey)
	chunk := pr[chunksKey].([]any)[0].(map[string]any)
	chunk[isCalldataKey] = true
	chunk[blobBytesKey] = "0x"
	chunk[calldataBytesKey] = "0xdead"
	chunk[kzgCommitmentKey] = "0x"
	chunk[kzgProofKey] = "0x"

	for name, tc := range map[string]struct {
		request []byte
		golden  string
	}{
		"reference":            {reference, "rollup_input.ssz"},
		"calldata no boundary": {marshal(t, obj), "rollup_input_calldata.ssz"},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := DecodeRollupRequest(tc.request)
			require.NoError(t, err)
			want, err := os.ReadFile("../../utils/ssz/testdata/" + tc.golden)
			require.NoError(t, err)
			assert.Equal(t, want, ssz.EncodeRollupInput(buildRollupInput(req)),
				"the input should equal the spec's encoding of the request")
		})
	}
}
