package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// referenceRollupResponse is the spec's reference rollup response. Its
	// public inputs are what the rollup guest output frame encodes.
	referenceRollupResponse = "../../rollup_spec/src/rollup_spec/prover_io/testdata/" +
		"10-14-getZkRollupProofV1.response.json"
	// referenceRollupOutput is the rollup guest output for those public
	// inputs, made by the spec encoder (utils/ssz/testdata/generate.py).
	referenceRollupOutput = "../utils/ssz/testdata/rollup_output.ssz"
)

// specRollupPublicInputs is the publicInputs object of the spec's rollup
// response.
type specRollupPublicInputs struct {
	EndBlockNumber                        uint64    `json:"endBlockNumber"`
	EndBlockTimestamp                     uint64    `json:"endBlockTimestamp"`
	EndL1L2BridgeRollingHash              specHex   `json:"endL1L2BridgeRollingHash"`
	EndL1L2BridgeRollingHashMessageNumber uint64    `json:"endL1L2BridgeRollingHashMessageNumber"`
	DynamicChainConfigHash                specHex   `json:"dynamicChainConfigHash"`
	ParentFtxRollingHash                  specHex   `json:"parentFtxRollingHash"`
	ParentFtxNumber                       uint64    `json:"parentFtxNumber"`
	EndFtxRollingHash                     specHex   `json:"endFtxRollingHash"`
	EndProcessedFtxNumber                 uint64    `json:"endProcessedFtxNumber"`
	ParentDataRollingHash                 specHex   `json:"parentDataRollingHash"`
	EndDataRollingHash                    specHex   `json:"endDataRollingHash"`
	ParentBlockHash                       specHex   `json:"parentBlockHash"`
	EndBlockHash                          specHex   `json:"endBlockHash"`
	ParentDataTailTakeBytes               uint64    `json:"parentDataTailTakeBytes"`
	FinalDataTailDiscardBytes             uint64    `json:"finalDataTailDiscardBytes"`
	FilteredAddresses                     []specHex `json:"filteredAddresses"`
	ProgramVks                            []specHex `json:"programVks"`
	BlockCount                            uint64    `json:"blockCount"`
	L2MessagingBlocksOffsets              []uint16  `json:"l2MessagingBlocksOffsets"`
	L2L1Messages                          []specHex `json:"l2L1Messages"`
}

// referenceRollupPublicInputs reads the public inputs of the spec's reference
// rollup response. An unknown field fails, so a field added to the spec shows
// up here.
func referenceRollupPublicInputs(t *testing.T) RollupPublicInputs {
	t.Helper()
	data, err := os.ReadFile(referenceRollupResponse)
	require.NoError(t, err)
	var response struct {
		PublicInputs json.RawMessage `json:"publicInputs"`
	}
	require.NoError(t, json.Unmarshal(data, &response))
	dec := json.NewDecoder(bytes.NewReader(response.PublicInputs))
	dec.DisallowUnknownFields()
	var spec specRollupPublicInputs
	require.NoError(t, dec.Decode(&spec), "the spec's public inputs should parse")

	hash := func(h specHex) [32]byte {
		require.Len(t, h, 32, "a hash should be 32 bytes")
		return [32]byte(h)
	}
	hashes := func(hs []specHex) [][32]byte {
		out := make([][32]byte, len(hs))
		for i, h := range hs {
			out[i] = hash(h)
		}
		return out
	}
	pi := RollupPublicInputs{
		EndBlockNumber:                        spec.EndBlockNumber,
		EndBlockTimestamp:                     spec.EndBlockTimestamp,
		EndL1L2BridgeRollingHash:              hash(spec.EndL1L2BridgeRollingHash),
		EndL1L2BridgeRollingHashMessageNumber: spec.EndL1L2BridgeRollingHashMessageNumber,
		DynamicChainConfigHash:                hash(spec.DynamicChainConfigHash),
		ParentFtxRollingHash:                  hash(spec.ParentFtxRollingHash),
		ParentFtxNumber:                       spec.ParentFtxNumber,
		EndFtxRollingHash:                     hash(spec.EndFtxRollingHash),
		EndProcessedFtxNumber:                 spec.EndProcessedFtxNumber,
		ParentDataRollingHash:                 hash(spec.ParentDataRollingHash),
		EndDataRollingHash:                    hash(spec.EndDataRollingHash),
		ParentBlockHash:                       hash(spec.ParentBlockHash),
		EndBlockHash:                          hash(spec.EndBlockHash),
		ParentDataTailTakeBytes:               spec.ParentDataTailTakeBytes,
		FinalDataTailDiscardBytes:             spec.FinalDataTailDiscardBytes,
		L2L1Messages:                          hashes(spec.L2L1Messages),
		FilteredAddresses:                     make([][20]byte, len(spec.FilteredAddresses)),
		ProgramVks:                            hashes(spec.ProgramVks),
		BlockCount:                            spec.BlockCount,
		L2MessagingBlocksOffsets:              append([]uint16{}, spec.L2MessagingBlocksOffsets...),
	}
	for i, a := range spec.FilteredAddresses {
		require.Len(t, a, 20, "an address should be 20 bytes")
		pi.FilteredAddresses[i] = [20]byte(a)
	}
	return pi
}

// referenceRollupFrame returns the hash and the frame (schema id || SSZ public
// inputs) of the reference rollup guest output.
func referenceRollupFrame(t *testing.T) (hash, framed []byte) {
	t.Helper()
	out, err := os.ReadFile(referenceRollupOutput)
	require.NoError(t, err)
	require.Greater(t, len(out), guestOutputHashSize, "the guest output should hold a hash and a frame")
	return out[:guestOutputHashSize], out[guestOutputHashSize:]
}

// Positions of the list offsets in the SSZ fixed head of the rollup public
// inputs.
const (
	rollupMessagesOffsetPos   = 312
	rollupFilteredOffsetPos   = 316
	rollupProgramVksOffsetPos = 320
	rollupBlocksOffsetPos     = 332
)

func TestDecodeRollupPublicInputs(t *testing.T) {
	hash, framed := referenceRollupFrame(t)
	require.NoError(t, checkGuestOutputFrame(hash, framed, rollupOutputSchemaID),
		"the reference output should pass the frame check")

	got, err := decodeRollupPublicInputs(framed)
	require.NoError(t, err)
	assert.Equal(t, referenceRollupPublicInputs(t), got, "every field should match the spec's public inputs")
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
