package jobadapter

import (
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The goldens come from rollup_spec, see utils/ssz/testdata/generate.py.
func TestBuildRollupInput_MatchesSpec(t *testing.T) {
	reference := readSpecFixture(t, referenceRollupRequest)

	// No parent blob data rolling hash (the spec needs no carry-over tail then) and a calldata chunk.
	obj := unmarshalObject(t, reference)
	pr := obj[proofRequestKey].(map[string]any)
	pr[parentSharedBlobCarryOverTailBytesKey] = 0
	delete(pr, parentBlobDataRollingHashKey)
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
			in, err := buildRollupInput(req)
			require.NoError(t, err)
			got, err := ssz.EncodeRollupInput(in)
			require.NoError(t, err)
			assert.Equal(t, want, got, "the input should equal the spec's encoding of the request")
		})
	}
}
