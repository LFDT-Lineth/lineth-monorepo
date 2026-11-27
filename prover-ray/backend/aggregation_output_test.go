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
	// referenceAggregationResponse is the spec's reference aggregation response.
	referenceAggregationResponse = "../../rollup_spec/src/rollup_spec/prover_io/testdata/" +
		"10-18-getZkRollupAggregationProofV1.response.json"
	// guestAggregationOutput is the aggregation guest's output for its committed
	// sample request.
	guestAggregationOutput = "../../riscv-guests/rollup_aggregation/test/testdata/sample.output.bin"
)

// specAggregationPublicInputs is the publicInputs object of the spec's
// aggregation response.
type specAggregationPublicInputs struct {
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
	ParentSharedBlobCarryOverTailBytes    uint64    `json:"parentSharedBlobCarryOverTailBytes"`
	FinalSharedBlobCarryOverTailBytes     uint64    `json:"finalSharedBlobCarryOverTailBytes"`
	L2L1TreeDepth                         uint64    `json:"l2L1TreeDepth"`
	L2L1Roots                             []specHex `json:"l2L1Roots"`
	FilteredAddresses                     []specHex `json:"filteredAddresses"`
	ProgramIDs                            []specHex `json:"programIds"`
	L2MessagingBlocksOffsets              []uint16  `json:"l2MessagingBlocksOffsets"`
}

// referenceAggregationPublicInputs reads them. An unknown field fails, so a
// new spec field shows up here.
func referenceAggregationPublicInputs(t *testing.T) AggregationPublicInputs {
	t.Helper()
	data, err := os.ReadFile(referenceAggregationResponse)
	require.NoError(t, err)
	var response struct {
		PublicInputs json.RawMessage `json:"publicInputs"`
	}
	require.NoError(t, json.Unmarshal(data, &response))
	dec := json.NewDecoder(bytes.NewReader(response.PublicInputs))
	dec.DisallowUnknownFields()
	var spec specAggregationPublicInputs
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
	pi := AggregationPublicInputs{
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
		ParentSharedBlobCarryOverTailBytes:    spec.ParentSharedBlobCarryOverTailBytes,
		FinalSharedBlobCarryOverTailBytes:     spec.FinalSharedBlobCarryOverTailBytes,
		L2L1TreeDepth:                         spec.L2L1TreeDepth,
		L2L1Roots:                             hashes(spec.L2L1Roots),
		FilteredAddresses:                     make([][20]byte, len(spec.FilteredAddresses)),
		ProgramIDs:                            hashes(spec.ProgramIDs),
		L2MessagingBlocksOffsets:              append([]uint16{}, spec.L2MessagingBlocksOffsets...),
	}
	for i, a := range spec.FilteredAddresses {
		require.Len(t, a, 20, "an address should be 20 bytes")
		pi.FilteredAddresses[i] = [20]byte(a)
	}
	return pi
}

// aggregationFrame returns the hash and the frame of the aggregation output in
// file.
func aggregationFrame(t *testing.T, file string) (hash, framed []byte) {
	t.Helper()
	out, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Greater(t, len(out), guestOutputHashSize, "the guest output should hold a hash and a frame")
	return out[:guestOutputHashSize], out[guestOutputHashSize:]
}

// Positions of the list offsets in the fixed head of the aggregation public
// inputs.
const (
	aggregationRootsOffsetPos      = 320
	aggregationFilteredOffsetPos   = 324
	aggregationProgramIDsOffsetPos = 328
	aggregationBlocksOffsetPos     = 332
)

// The outputs come from rollup_spec, see utils/ssz/testdata/generate.py.
func TestDecodeAggregationPublicInputs(t *testing.T) {
	withOffsets := referenceAggregationPublicInputs(t)
	withOffsets.L2MessagingBlocksOffsets = []uint16{1, 258}

	for name, tc := range map[string]struct {
		output string
		want   AggregationPublicInputs
	}{
		"reference": {"../utils/ssz/testdata/aggregation_output.ssz", referenceAggregationPublicInputs(t)},
		"offsets":   {"../utils/ssz/testdata/aggregation_output_offsets.ssz", withOffsets},
	} {
		t.Run(name, func(t *testing.T) {
			hash, framed := aggregationFrame(t, tc.output)
			require.NoError(t, checkGuestOutputFrame(hash, framed, aggregationOutputSchemaID),
				"the output should pass the frame check")
			got, err := decodeAggregationPublicInputs(framed)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got, "every field should match the spec's public inputs")
		})
	}
}

func TestDecodeAggregationPublicInputs_GuestOutput(t *testing.T) {
	hash, framed := aggregationFrame(t, guestAggregationOutput)
	require.NoError(t, checkGuestOutputFrame(hash, framed, aggregationOutputSchemaID),
		"the guest output should pass the frame check")
	got, err := decodeAggregationPublicInputs(framed)
	require.NoError(t, err, "the guest output should decode")
	assert.NotEmpty(t, got.L2MessagingBlocksOffsets, "the guest output should carry messaging-block offsets")
}

func TestDecodeAggregationPublicInputs_Rejects(t *testing.T) {
	_, framed := aggregationFrame(t, "../utils/ssz/testdata/aggregation_output.ssz")
	filteredStart := offsetAt(framed, aggregationFilteredOffsetPos)
	programIDsStart := offsetAt(framed, aggregationProgramIDsOffsetPos)

	cases := map[string]struct {
		framed  []byte
		wantErr string
	}{
		"rollup schema id": {append([]byte{0x00, 0x05}, framed[2:]...),
			"schema id is 0x0005, want 0x0007"},
		"truncated fixed head": {framed[:guestOutputSchemaIDSize+aggregationPublicInputsFixedSize-1],
			"shorter than the 336-byte fixed head"},
		"l2L1Roots not behind the head": {withOffset(framed, aggregationRootsOffsetPos, aggregationPublicInputsFixedSize+1),
			"l2L1Roots offset is 337, want 336"},
		"filteredAddresses before l2L1Roots": {withOffset(framed, aggregationFilteredOffsetPos, aggregationPublicInputsFixedSize-1),
			errOffsetsOutOfOrder},
		"partial program ID": {withOffset(framed, aggregationBlocksOffsetPos, programIDsStart+31),
			"programIds is 31 bytes, not a multiple of 32"},
		"partial filtered address": {withOffset(framed, aggregationProgramIDsOffsetPos, filteredStart+19),
			"filteredAddresses is 19 bytes, not a multiple of 20"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeAggregationPublicInputs(tc.framed)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
