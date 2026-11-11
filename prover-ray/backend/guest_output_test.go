package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// referenceL2ExecutionResponse is the spec's reference l2-execution
	// response. Its public inputs are what the guest output frame encodes.
	referenceL2ExecutionResponse = "../../rollup_spec/src/rollup_spec/prover_io/testdata/" +
		"10-11-getZkL2ExecutionProofV1.response.json"
	// referenceGuestOutput is the guest output for those public inputs, made
	// by the spec encoder (utils/ssz/testdata/generate.py).
	referenceGuestOutput = "../utils/ssz/testdata/l2_execution_output.ssz"
)

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

// specPublicInputs is the publicInputs object of the spec's response.
type specPublicInputs struct {
	ParentBlockHash                       specHex   `json:"parentBlockHash"`
	EndBlockHash                          specHex   `json:"endBlockHash"`
	EndBlockNumber                        uint64    `json:"endBlockNumber"`
	EndBlockTimestamp                     uint64    `json:"endBlockTimestamp"`
	EndL1L2BridgeRollingHash              specHex   `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber uint64    `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                specHex   `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                  specHex   `json:"parentFtxRollingHash"`
	ParentFtxNumber                       uint64    `json:"parentFtxNumber"`
	EndFtxRollingHash                     specHex   `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                 uint64    `json:"endProcessedFtxNumber"`
	FilteredAddresses                     []specHex `json:"filteredAddresses"`
	TxFromsHash                           specHex   `json:"txFromsHash"`
	BlockCount                            uint64    `json:"blockCount"`
	L2MessagingBlocksOffsets              []uint16  `json:"l2MessagingBlocksOffsets"`
	L2L1Messages                          []specHex `json:"l2L1Messages"`
}

// referencePublicInputs reads the public inputs of the spec's reference
// response. An unknown field fails, so a field added to the spec shows up here.
func referencePublicInputs(t *testing.T) PublicInputs {
	t.Helper()
	data, err := os.ReadFile(referenceL2ExecutionResponse)
	require.NoError(t, err)
	var response struct {
		PublicInputs json.RawMessage `json:"publicInputs"`
	}
	require.NoError(t, json.Unmarshal(data, &response))
	dec := json.NewDecoder(bytes.NewReader(response.PublicInputs))
	dec.DisallowUnknownFields()
	var spec specPublicInputs
	require.NoError(t, dec.Decode(&spec), "the spec's public inputs should parse")

	hash := func(h specHex) [32]byte {
		require.Len(t, h, 32, "a hash should be 32 bytes")
		return [32]byte(h)
	}
	pi := PublicInputs{
		ParentBlockHash:                       hash(spec.ParentBlockHash),
		EndBlockHash:                          hash(spec.EndBlockHash),
		EndBlockNumber:                        spec.EndBlockNumber,
		EndBlockTimestamp:                     spec.EndBlockTimestamp,
		EndL1L2BridgeRollingHash:              hash(spec.EndL1L2BridgeRollingHash),
		EndL1L2BridgeRollingHashMessageNumber: spec.EndL1L2BridgeRollingHashMessageNumber,
		DynamicChainConfigHash:                hash(spec.DynamicChainConfigHash),
		ParentFtxRollingHash:                  hash(spec.ParentFtxRollingHash),
		ParentFtxNumber:                       spec.ParentFtxNumber,
		EndFtxRollingHash:                     hash(spec.EndFtxRollingHash),
		EndProcessedFtxNumber:                 spec.EndProcessedFtxNumber,
		FilteredAddresses:                     make([][20]byte, len(spec.FilteredAddresses)),
		TxFromsHash:                           hash(spec.TxFromsHash),
		BlockCount:                            spec.BlockCount,
		L2MessagingBlocksOffsets:              append([]uint16{}, spec.L2MessagingBlocksOffsets...),
		L2L1Messages:                          make([][32]byte, len(spec.L2L1Messages)),
	}
	for i, a := range spec.FilteredAddresses {
		require.Len(t, a, 20, "an address should be 20 bytes")
		pi.FilteredAddresses[i] = [20]byte(a)
	}
	for i, m := range spec.L2L1Messages {
		pi.L2L1Messages[i] = hash(m)
	}
	return pi
}

// referenceFrame returns the hash and the frame (schema id || SSZ public
// inputs) of the reference guest output.
func referenceFrame(t *testing.T) (hash, framed []byte) {
	t.Helper()
	out, err := os.ReadFile(referenceGuestOutput)
	require.NoError(t, err)
	require.Greater(t, len(out), guestOutputHashSize, "the guest output should hold a hash and a frame")
	return out[:guestOutputHashSize], out[guestOutputHashSize:]
}

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

func TestDecodeGuestPublicInputs(t *testing.T) {
	hash, framed := referenceFrame(t)
	require.NoError(t, checkGuestOutputFrame(hash, framed), "the reference output should pass the frame check")

	got, err := decodeGuestPublicInputs(framed)
	require.NoError(t, err)
	assert.Equal(t, referencePublicInputs(t), got, "every field should match the spec's public inputs")
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
			"offsets out of order"},
		"list past the end": {withOffset(framed, blocksOffsetPos, body+1),
			"offsets out of order"},
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
