package jobadapter

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceAggregationResponse = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/10-18-getZkRollupAggregationProofV1.response.json"

// TestNewAggregationResponse_MatchesReferenceShape verifies that the response
// of a result with no public inputs has the exact V1 field shape, with empty
// lists (no top-level programVk; the revealed arrays live in publicInputs).
func TestNewAggregationResponse_MatchesReferenceShape(t *testing.T) {
	resp := newAggregationResponse(backend.Result{ProofBytes: []byte{0xde, 0xad}}, 1000501, "test-version")

	assert.Equal(t, "test-version", resp.ProverVersion)
	assert.Equal(t, "0xdead", resp.ProofHex)
	assert.Equal(t, uint64(1000501), resp.StartBlockNumber)
	assert.Empty(t, resp.PublicInputs.L2L1Roots)
	assert.Empty(t, resp.PublicInputs.FilteredAddresses)
	assert.Empty(t, resp.PublicInputs.ProgramIDs)
	assert.Empty(t, resp.PublicInputs.L2MessagingBlocksOffsets)

	raw, err := jsonMarshalObject(resp)
	require.NoError(t, err)
	want := unmarshalObject(t, readSpecFixture(t, referenceAggregationResponse))
	assertJSONShape(t, want, raw, "aggregationResponse")
	assert.NotContains(t, raw, "programVk")
	assert.NotContains(t, raw, "status")
}

// TestNewAggregationResponse_MatchesReferenceValues checks that a result
// holding the public inputs of the spec's reference response gives that
// response.
func TestNewAggregationResponse_MatchesReferenceValues(t *testing.T) {
	want := unmarshalObject(t, readSpecFixture(t, referenceAggregationResponse))
	pi := want[publicInputsKey].(map[string]any)
	hash := func(key string) [32]byte { return [32]byte(mustHex(t, pi[key].(string))) }
	number := func(key string) uint64 { return uint64(pi[key].(float64)) }
	hashes := func(key string) [][32]byte {
		list := pi[key].([]any)
		out := make([][32]byte, 0, len(list))
		for _, h := range list {
			out = append(out, [32]byte(mustHex(t, h.(string))))
		}
		return out
	}
	addressList := pi[filteredAddressesKey].([]any)
	addresses := make([][20]byte, 0, len(addressList))
	for _, a := range addressList {
		addresses = append(addresses, [20]byte(mustHex(t, a.(string))))
	}
	offsetList := pi[l2MessagingBlocksOffsetsKey].([]any)
	offsets := make([]uint16, 0, len(offsetList))
	for _, o := range offsetList {
		offsets = append(offsets, uint16(o.(float64)))
	}

	result := backend.Result{
		ProofBytes: mustHex(t, want[proofKey].(string)),
		AggregationPublicInputs: backend.AggregationPublicInputs{
			EndBlockNumber:                        number(endBlockNumberKey),
			EndBlockTimestamp:                     number("endBlockTimestamp"),
			EndL1L2BridgeRollingHash:              hash("endL1L2BridgeRollingHash"),
			EndL1L2BridgeRollingHashMessageNumber: number("endL1L2BridgeRollingHashMessageNumber"),
			DynamicChainConfigHash:                hash("dynamicChainConfigHash"),
			ParentFtxRollingHash:                  hash("parentFtxRollingHash"),
			ParentFtxNumber:                       number("parentFtxNumber"),
			EndFtxRollingHash:                     hash("endFtxRollingHash"),
			EndProcessedFtxNumber:                 number("endProcessedFtxNumber"),
			ParentDataRollingHash:                 hash(parentDataRollingHashKey),
			EndDataRollingHash:                    hash("endDataRollingHash"),
			ParentBlockHash:                       hash("parentBlockHash"),
			EndBlockHash:                          hash("endBlockHash"),
			ParentSharedBlobCarryOverTailBytes:    number(parentSharedBlobCarryOverTailBytesKey),
			FinalSharedBlobCarryOverTailBytes:     number(finalSharedBlobCarryOverTailBytesKey),
			L2L1TreeDepth:                         number("l2L1TreeDepth"),
			L2L1Roots:                             hashes("l2L1Roots"),
			FilteredAddresses:                     addresses,
			ProgramIDs:                            hashes("programIds"),
			L2MessagingBlocksOffsets:              offsets,
		},
	}
	resp := newAggregationResponse(result, uint64(want[startBlockNumberKey].(float64)), want["proverVersion"].(string))

	got, err := jsonMarshalObject(resp)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
