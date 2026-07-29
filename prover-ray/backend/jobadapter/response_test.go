package jobadapter

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceL2ExecutionResponse = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/getZkL2ExecutionProofV1.response.json"

func TestNewExecutionResponse_MapsV1Fields(t *testing.T) {
	result := backend.Result{
		ProofBytes: []byte{0xca, 0xfe},
		PublicInputs: backend.PublicInputs{
			ParentBlockHash:                          filledHash(0x01),
			EndBlockHash:                             filledHash(0x02),
			EndBlockNumber:                           1000503,
			EndBlockTimestamp:                        1763000101,
			L2L1MessagesHash:                         filledHash(0x03),
			ParentL1L2BridgeRollingHash:              filledHash(0x04),
			ParentL1L2BridgeRollingHashMessageNumber: 11,
			EndL1L2BridgeRollingHash:                 filledHash(0x05),
			EndL1L2BridgeRollingHashMessageNumber:    12,
			DynamicChainConfigHash:                   filledHash(0x06),
			ParentFtxRollingHash:                     filledHash(0x07),
			ParentProcessedFtxNumber:                 13,
			EndFtxRollingHash:                        filledHash(0x08),
			EndProcessedFtxNumber:                    14,
			FilteredAddressesHash:                    filledHash(0x09),
			TxFromsHash:                              filledHash(0x0a),
		},
	}

	resp := newExecutionResponse(result, 1000501, "test-version")

	assert.Equal(t, "test-version", resp.ProverVersion)
	assert.Equal(t, "0xcafe", resp.ProofHex)
	assert.Equal(t, uint64(1000501), resp.StartBlockNumber)
	assert.Empty(t, resp.L2L1Messages)
	assert.Empty(t, resp.TxFroms)
	assert.Empty(t, resp.FilteredAddresses)

	pi := resp.PublicInputs
	assert.Equal(t, repeatHex(0x01), pi.ParentBlockHash)
	assert.Equal(t, repeatHex(0x02), pi.EndBlockHash)
	assert.Equal(t, uint64(1000503), pi.EndBlockNumber)
	assert.Equal(t, uint64(1763000101), pi.EndBlockTimestamp)
	assert.Equal(t, repeatHex(0x03), pi.L2L1MessagesHash)
	assert.Equal(t, repeatHex(0x04), pi.ParentL1L2BridgeRollingHash)
	assert.Equal(t, uint64(11), pi.ParentL1L2BridgeRollingHashMessageNumber)
	assert.Equal(t, repeatHex(0x05), pi.EndL1L2BridgeRollingHash)
	assert.Equal(t, uint64(12), pi.EndL1L2BridgeRollingHashMessageNumber)
	assert.Equal(t, repeatHex(0x06), pi.DynamicChainConfigHash)
	assert.Equal(t, repeatHex(0x07), pi.ParentFtxRollingHash)
	assert.Equal(t, uint64(13), pi.ParentProcessedFtxNumber)
	assert.Equal(t, repeatHex(0x08), pi.EndFtxRollingHash)
	assert.Equal(t, uint64(14), pi.EndProcessedFtxNumber)
	assert.Equal(t, repeatHex(0x09), pi.FilteredAddressesHash)
	assert.Equal(t, repeatHex(0x0a), pi.TxFromsHash)
}

func TestNewExecutionResponse_MatchesReferenceResponseShape(t *testing.T) {
	raw, err := jsonMarshalObject(newExecutionResponse(backend.Result{}, 42, "test-version"))
	require.NoError(t, err)

	assertResponseShapeMatchesReference(t, raw)
	assert.NotContains(t, raw, "status")
	assert.NotContains(t, raw, "error")
	assert.NotContains(t, raw, "jobId")
}

func TestNewExecutionResponse_MatchesReferenceResponseValues(t *testing.T) {
	// This is the target test for the fully wired response path. It is expected
	// to fail until proof serialization, public-input extraction, and the
	// l2L1Messages/txFroms/filteredAddresses response arrays are provided by the
	// backend result.
	t.Skip("enable after proof serialization, public-input extraction, and l2L1Messages/txFroms/filteredAddresses response arrays are wired")

	got, err := jsonMarshalObject(newExecutionResponse(backend.Result{}, 1000501, ""))
	require.NoError(t, err)

	assert.Equal(t, readReferenceResponse(t), got)
}

func assertResponseShapeMatchesReference(t *testing.T, got map[string]any) {
	t.Helper()
	want := readReferenceResponse(t)
	assertJSONShape(t, want, got, "response")
}

func filledHash(b byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = b
	}
	return out
}

func repeatHex(b byte) string {
	return hexHash(filledHash(b))
}

func jsonMarshalObject(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func readReferenceResponse(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(referenceL2ExecutionResponse)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	return raw
}

func assertJSONShape(t *testing.T, want, got map[string]any, path string) {
	t.Helper()
	require.Len(t, got, len(want), "%s field count", path)
	for key, wantValue := range want {
		gotValue, ok := got[key]
		require.True(t, ok, "%s.%s missing", path, key)
		assertJSONValueKind(t, wantValue, gotValue, path+"."+key)
	}
}

func assertJSONValueKind(t *testing.T, want, got any, path string) {
	t.Helper()
	switch wantValue := want.(type) {
	case map[string]any:
		gotValue, ok := got.(map[string]any)
		require.True(t, ok, "%s type: expected object, got %T", path, got)
		assertJSONShape(t, wantValue, gotValue, path)
	case []any:
		_, ok := got.([]any)
		require.True(t, ok, "%s type: expected array, got %T", path, got)
	default:
		assert.Equal(t, reflect.TypeOf(want), reflect.TypeOf(got), "%s JSON type", path)
	}
}
