package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// referenceL2ExecutionResponse is the spec's reference l2-execution response.
// Its guestOutput is the reference guest output.
const referenceL2ExecutionResponse = "../../rollup_spec/src/rollup_spec/prover_io/testdata/" +
	"10-11-getZkL2ExecutionProofV1.response.json"

// specHex is a 0x-prefixed hex string of the spec's JSON.
type specHex []byte

func (h *specHex) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	*h = b
	return err
}

// specGuestOutput returns the guestOutput of the spec response at path.
func specGuestOutput(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: test-controlled path
	require.NoError(t, err)
	var response struct {
		GuestOutput specHex `json:"guestOutput"`
	}
	require.NoError(t, json.Unmarshal(data, &response))
	require.Greater(t, len(response.GuestOutput), guestOutputHashSize, "the guest output should hold a hash and a frame")
	return response.GuestOutput
}

// referenceOutput returns the guestOutput of the spec's reference l2-execution
// response.
func referenceOutput(t *testing.T) []byte {
	t.Helper()
	return specGuestOutput(t, referenceL2ExecutionResponse)
}

// referenceFrame returns the hash and the frame (schema id || SSZ public
// inputs) of the reference guest output.
func referenceFrame(t *testing.T) (hash, framed []byte) {
	t.Helper()
	out := referenceOutput(t)
	return out[:guestOutputHashSize], out[guestOutputHashSize:]
}

// referencePublicInputs returns the public inputs of the reference guest
// output, as decodeGuestPublicInputs decodes them (see
// TestDecodeGuestPublicInputs).
func referencePublicInputs(t *testing.T) PublicInputs {
	t.Helper()
	_, framed := referenceFrame(t)
	pi, err := decodeGuestPublicInputs(framed)
	require.NoError(t, err)
	return pi
}

const errOffsetsOutOfOrder = "offsets out of order"

// Positions of the three list offsets in the SSZ fixed head.
const (
	messagesOffsetPos = 80
	filteredOffsetPos = 236
	blocksOffsetPos   = 280
)

// offsetAt returns the list offset at pos (in the SSZ body) of framed.
func offsetAt(framed []byte, pos int) uint32 {
	return binary.LittleEndian.Uint32(framed[guestOutputSchemaIDSize+pos:])
}

// withOffset returns a copy of framed with the list offset at pos (in the SSZ
// body) set to v.
func withOffset(framed []byte, pos int, v uint32) []byte {
	out := bytes.Clone(framed)
	binary.LittleEndian.PutUint32(out[guestOutputSchemaIDSize+pos:], v)
	return out
}

// TestDecodeGuestPublicInputs checks the decoder on the spec's reference guest
// output: utils/ssz, a separate encoder, must give back the same bytes from the
// decoded public inputs.
func TestDecodeGuestPublicInputs(t *testing.T) {
	hash, framed := referenceFrame(t)
	require.NoError(t, checkGuestOutputFrame(hash, framed, l2ExecutionOutputSchemaID),
		"the reference output should pass the frame check")

	got, err := decodeGuestPublicInputs(framed)
	require.NoError(t, err)

	input := func(p ssz.L2ExecutionProof) []byte {
		b, err := ssz.EncodeRollupInput(ssz.RollupInput{L2ExecutionProofs: []ssz.L2ExecutionProof{p}})
		require.NoError(t, err)
		return b
	}
	assert.Equal(t,
		input(ssz.L2ExecutionProof{PublicInputsSSZ: framed[guestOutputSchemaIDSize:]}),
		input(ssz.L2ExecutionProof{PublicInputs: ssz.L2ExecutionPublicInputs(got)}),
		"the decoded public inputs should encode back to the spec's bytes")
}

func TestDecodeGuestPublicInputs_EmptyLists(t *testing.T) {
	_, framed := referenceFrame(t)
	// Keep the fixed head only, every list starting and ending at its end.
	head := bytes.Clone(framed[:guestOutputSchemaIDSize+publicInputsFixedSize])
	for _, pos := range []int{messagesOffsetPos, filteredOffsetPos, blocksOffsetPos} {
		head = withOffset(head, pos, publicInputsFixedSize)
	}

	got, err := decodeGuestPublicInputs(head)
	require.NoError(t, err)
	// Empty, not nil, as the native runner's output parses.
	assert.Equal(t, [][32]byte{}, got.L2L1Messages, "no L2 to L1 message should be decoded")
	assert.Equal(t, [][20]byte{}, got.FilteredAddresses, "no filtered address should be decoded")
	assert.Equal(t, []uint16{}, got.L2MessagingBlocksOffsets, "no messaging block offset should be decoded")
	assert.Equal(t, referencePublicInputs(t).EndBlockHash, got.EndBlockHash, "the fixed fields should still be decoded")
}

func TestDecodeGuestPublicInputs_Rejects(t *testing.T) {
	_, framed := referenceFrame(t)
	var (
		body          = uint32(len(framed) - guestOutputSchemaIDSize)
		messagesStart = offsetAt(framed, messagesOffsetPos)
		filteredStart = offsetAt(framed, filteredOffsetPos)
		oneMoreByte   = append(bytes.Clone(framed), 0)
	)

	cases := map[string]struct {
		framed  []byte
		wantErr string
	}{
		"wrong schema id": {append([]byte{0x00, 0x02}, framed[2:]...),
			"schema id is 0x0002, want 0x0003"},
		"shorter than the fixed head": {framed[:guestOutputSchemaIDSize+publicInputsFixedSize-1],
			"shorter than the 284-byte fixed head"},
		"first list not behind the head": {withOffset(framed, messagesOffsetPos, publicInputsFixedSize+1),
			"l2L1Messages offset is 285, want 284"},
		"lists out of order": {withOffset(framed, filteredOffsetPos, publicInputsFixedSize-1),
			errOffsetsOutOfOrder},
		"list past the end": {withOffset(framed, blocksOffsetPos, body+1),
			errOffsetsOutOfOrder},
		"partial message": {withOffset(framed, filteredOffsetPos, messagesStart+31),
			"l2L1Messages is 31 bytes, not a multiple of 32"},
		"partial filtered address": {withOffset(oneMoreByte, blocksOffsetPos, filteredStart+21),
			"filteredAddresses is 21 bytes, not a multiple of 20"},
		"partial messaging block offset": {oneMoreByte,
			"l2MessagingBlocksOffsets is 1 bytes, not a multiple of 2"},
		"too many messaging block offsets": {append(bytes.Clone(framed), make([]byte, 2*(maxMessagingBlocks+1))...),
			"elements, more than 65536"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeGuestPublicInputs(tc.framed)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestGuestOutputPublicInputs checks that the public inputs of a guest output
// are returned only when its hash and the schema id of the proof type match.
func TestGuestOutputPublicInputs(t *testing.T) {
	out := referenceOutput(t)

	got, err := GuestOutputPublicInputs(out, ProofTypeL2Execution)
	require.NoError(t, err)
	assert.Equal(t, out[guestOutputHashSize+guestOutputSchemaIDSize:], got)

	badHash := bytes.Clone(out)
	badHash[0] ^= 1
	for name, tc := range map[string]struct {
		out     []byte
		typ     ProofType
		wantErr string
	}{
		"empty":        {nil, ProofTypeL2Execution, "shorter than the hash"},
		"no schema id": {out[:guestOutputHashSize+1], ProofTypeL2Execution, "shorter than the schema id"},
		"wrong hash":   {badHash, ProofTypeL2Execution, "hash does not match"},
		"wrong schema": {out, ProofTypeRollup, "schema id is 0x0003, want 0x0005"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := GuestOutputPublicInputs(tc.out, tc.typ)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
