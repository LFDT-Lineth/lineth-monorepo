package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// referenceRollupResponse is the spec's reference rollup response. Its
	// guestOutput is the reference rollup guest output.
	referenceRollupResponse = "../../rollup_spec/src/rollup_spec/prover_io/testdata/" +
		"10-14-getZkRollupProofV1.response.json"
	// referenceRollupRequest is the request of that response.
	referenceRollupRequest = "../../rollup_spec/src/rollup_spec/prover_io/testdata/" +
		"10-14-getZkRollupProofV1.request.json"
)

// referenceRollupFrame returns the hash and the frame of the reference output.
func referenceRollupFrame(t *testing.T) (hash, framed []byte) {
	t.Helper()
	out := specGuestOutput(t, referenceRollupResponse)
	return out[:guestOutputHashSize], out[guestOutputHashSize:]
}

// referenceRollupPublicInputs returns the public inputs of the reference rollup
// output, as decodeRollupPublicInputs decodes them (see
// TestDecodeRollupPublicInputs).
func referenceRollupPublicInputs(t *testing.T) RollupPublicInputs {
	t.Helper()
	_, framed := referenceRollupFrame(t)
	pi, err := decodeRollupPublicInputs(framed)
	require.NoError(t, err)
	return pi
}

// Positions of the list offsets in the fixed head of the rollup public inputs.
const (
	rollupMessagesOffsetPos   = 312
	rollupFilteredOffsetPos   = 316
	rollupProgramVksOffsetPos = 320
	rollupBlocksOffsetPos     = 332
)

// TestDecodeRollupPublicInputs checks the decoder on the spec's reference rollup
// guest output, against the fields that the reference request fixes. The other
// fields of the reference are independent sample values.
func TestDecodeRollupPublicInputs(t *testing.T) {
	hash, framed := referenceRollupFrame(t)
	require.NoError(t, checkGuestOutputFrame(hash, framed, rollupOutputSchemaID),
		"the reference output should pass the frame check")

	got, err := decodeRollupPublicInputs(framed)
	require.NoError(t, err)

	data, err := os.ReadFile(referenceRollupRequest)
	require.NoError(t, err)
	var request struct {
		ProofRequest struct {
			ParentDataRollingHash specHex `json:"parentDataRollingHash"`
			L2ExecutionProofs     []struct {
				GuestOutput specHex `json:"guestOutput"`
				ProgramVk   specHex `json:"programVk"`
			} `json:"l2ExecutionProofs"`
		} `json:"proofRequest"`
	}
	require.NoError(t, json.Unmarshal(data, &request))
	proofs := request.ProofRequest.L2ExecutionProofs
	require.NotEmpty(t, proofs)
	inner := make([]PublicInputs, len(proofs))
	var messages, vks [][32]byte
	for i, p := range proofs {
		require.Greater(t, len(p.GuestOutput), guestOutputHashSize)
		inner[i], err = decodeGuestPublicInputs(p.GuestOutput[guestOutputHashSize:])
		require.NoError(t, err)
		messages = append(messages, inner[i].L2L1Messages...)
		if !slices.Contains(vks, [32]byte(p.ProgramVk)) {
			vks = append(vks, [32]byte(p.ProgramVk))
		}
	}
	slices.SortFunc(vks, func(a, b [32]byte) int { return bytes.Compare(a[:], b[:]) })

	assert.Equal(t, [32]byte(request.ProofRequest.ParentDataRollingHash), got.ParentDataRollingHash)
	assert.Equal(t, vks, got.ProgramVks, "the distinct VKs of the inner proofs, sorted")
	assert.Equal(t, inner[0].ParentBlockHash, got.ParentBlockHash, "the parent block of the first inner proof")
	assert.Equal(t, inner[len(inner)-1].EndBlockHash, got.EndBlockHash, "the end block of the last inner proof")
	assert.Equal(t, messages, got.L2L1Messages, "the messages of the inner proofs, in order")
}

func TestDecodeRollupPublicInputs_Rejects(t *testing.T) {
	_, framed := referenceRollupFrame(t)
	programVksStart := offsetAt(framed, rollupProgramVksOffsetPos)

	cases := map[string]struct {
		framed  []byte
		wantErr string
	}{
		"l2-execution schema id": {append([]byte{0x00, 0x03}, framed[2:]...),
			"schema id is 0x0003, want 0x0005"},
		"shorter than the fixed head": {framed[:guestOutputSchemaIDSize+rollupPublicInputsFixedSize-1],
			"shorter than the 336-byte fixed head"},
		"first list not behind the head": {withOffset(framed, rollupMessagesOffsetPos, rollupPublicInputsFixedSize+1),
			"l2L1Messages offset is 337, want 336"},
		"lists out of order": {withOffset(framed, rollupFilteredOffsetPos, rollupPublicInputsFixedSize-1),
			errOffsetsOutOfOrder},
		"partial program VK": {withOffset(framed, rollupBlocksOffsetPos, programVksStart+31),
			"programVks is 31 bytes, not a multiple of 32"},
		"partial messaging block offset": {append(bytes.Clone(framed), 0),
			"l2MessagingBlocksOffsets is 1 bytes, not a multiple of 2"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeRollupPublicInputs(tc.framed)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
