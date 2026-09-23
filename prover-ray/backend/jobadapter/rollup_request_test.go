package jobadapter

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rollupRequestFixture = "10-14-getZkRollupProofV1.request.json"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	require.NoError(t, err)
	return b
}

// TestDecodeRollupRequest_Golden decodes the reference fixture and checks the
// envelope, chunk objects, boundary hash, and embedded proofs.
func TestDecodeRollupRequest_Golden(t *testing.T) {
	req, err := DecodeRollupRequest(readFixture(t, rollupRequestFixture))
	require.NoError(t, err)

	assert.Equal(t,
		mustHex(t, "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d"),
		req.ProgramVk)
	assert.Equal(t, uint64(59144), req.ChainID)
	assert.Equal(t, uint64(4), req.StartOffset)
	assert.Equal(t, mustHex(t, "0xabababab"), req.OpaquePrefixBytes)
	assert.Equal(t, filledHash(0x47), req.ParentDataRollingHash)

	require.NotNil(t, req.BoundaryPrevDataRollingHash)
	assert.Equal(t, filledHash(0x39), *req.BoundaryPrevDataRollingHash)

	require.Len(t, req.Conflations, 2)
	require.Len(t, req.Conflations[0].BlockRlps, 2)
	assert.Equal(t, mustHex(t, "0xf90215a0"), req.Conflations[0].BlockRlps[0])

	require.Len(t, req.Chunks, 1)
	assert.Equal(t, filledHash(0x1a), req.Chunks[0].ChunkHash)
	assert.False(t, req.Chunks[0].IsCalldata)
	assert.Equal(t, uint64(0), req.Chunks[0].CalldataLength)

	require.Len(t, req.L2ExecutionProofs, 2)
	p0 := req.L2ExecutionProofs[0]
	assert.Equal(t, mustHex(t, "0xabcdef"), p0.Proof)
	assert.Equal(t, uint64(10), p0.StartBlockNumber)
	assert.Equal(t, mustHex(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), p0.ProgramVk)
	assert.Equal(t, uint64(11), p0.PublicInputs.EndBlockNumber)
	assert.Equal(t, uint64(1763000200), p0.PublicInputs.EndBlockTimestamp)
	assert.Equal(t, filledHash(0xc0), p0.PublicInputs.DynamicChainConfigHash)
	assert.Len(t, p0.L2L1Messages, 1)
	assert.Len(t, p0.TxFroms, 2)
	assert.Len(t, p0.FilteredAddresses, 2)

	assert.Equal(t, uint64(12), req.L2ExecutionProofs[1].StartBlockNumber)
	assert.Equal(t, uint64(14), req.L2ExecutionProofs[1].PublicInputs.EndBlockNumber)
}

// TestDecodeRollupRequest_BlobChunkStart verifies the fresh-chunk-start form:
// startOffset 0 needs no boundary hash and no opaque prefix.
func TestDecodeRollupRequest_BlobChunkStart(t *testing.T) {
	obj := unmarshalObject(t, readFixture(t, rollupRequestFixture))
	pr := obj[proofRequestKey].(map[string]any)
	pr[startOffsetKey] = 0
	delete(pr, opaquePrefixBytesKey)
	delete(pr, boundaryPrevDataRollingHashKey)

	req, err := DecodeRollupRequest(marshal(t, obj))
	require.NoError(t, err)
	assert.Equal(t, uint64(0), req.StartOffset)
	assert.Nil(t, req.BoundaryPrevDataRollingHash)
	assert.Empty(t, req.OpaquePrefixBytes)
}

func TestDecodeRollupRequest_InvalidJSON(t *testing.T) {
	_, err := DecodeRollupRequest([]byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing JSON")
}

// TestDecodeRollupRequest_InvalidShape sweeps the rollup envelope error paths.
func TestDecodeRollupRequest_InvalidShape(t *testing.T) {
	pr := func(o map[string]any) map[string]any { return o[proofRequestKey].(map[string]any) }
	chunk0 := func(o map[string]any) map[string]any { return pr(o)[chunksKey].([]any)[0].(map[string]any) }
	proof0 := func(o map[string]any) map[string]any {
		return pr(o)[l2ExecutionProofsKey].([]any)[0].(map[string]any)
	}
	pi0 := func(o map[string]any) map[string]any { return proof0(o)[publicInputsKey].(map[string]any) }

	cases := []struct {
		name    string
		mutate  func(o map[string]any)
		wantErr string
	}{
		{"MissingProgramVk", func(o map[string]any) { delete(o, programVkKey) }, programVkKey},
		{"ProgramVkWrongLength", func(o map[string]any) { o[programVkKey] = "0x1234" }, programVkKey},
		{"MissingProofRequest", func(o map[string]any) { delete(o, proofRequestKey) }, proofRequestKey},
		{"MissingChainID", func(o map[string]any) { delete(pr(o), chainIDKey) }, chainIDKey},
		{"MissingConflations", func(o map[string]any) { delete(pr(o), conflationsKey) }, conflationsKey},
		{"EmptyConflations", func(o map[string]any) { pr(o)[conflationsKey] = []any{} }, conflationsKey},
		{"MissingChunks", func(o map[string]any) { delete(pr(o), chunksKey) }, chunksKey},
		{"EmptyChunks", func(o map[string]any) { pr(o)[chunksKey] = []any{} }, chunksKey},
		{"MissingProofs", func(o map[string]any) { delete(pr(o), l2ExecutionProofsKey) }, l2ExecutionProofsKey},
		{"EmptyProofs", func(o map[string]any) { pr(o)[l2ExecutionProofsKey] = []any{} }, l2ExecutionProofsKey},
		{"LengthMismatch",
			func(o map[string]any) {
				proofs := pr(o)[l2ExecutionProofsKey].([]any)
				pr(o)[l2ExecutionProofsKey] = proofs[:1]
			},
			"same length"},
		{"MissingParentDataRollingHash",
			func(o map[string]any) { delete(pr(o), parentDataRollingHashKey) }, parentDataRollingHashKey},
		{"MissingStartOffset", func(o map[string]any) { delete(pr(o), startOffsetKey) }, startOffsetKey},
		{"BoundaryRequiredWhenMidChunk",
			func(o map[string]any) { delete(pr(o), boundaryPrevDataRollingHashKey) },
			boundaryPrevDataRollingHashKey},
		{"OpaquePrefixLengthMismatch",
			func(o map[string]any) { pr(o)[opaquePrefixBytesKey] = "0xabab" },
			opaquePrefixBytesKey},
		{"ConflationBlockRlpsEmpty",
			func(o map[string]any) {
				pr(o)[conflationsKey].([]any)[0].(map[string]any)[blockRlpsKey] = []any{}
			},
			blockRlpsKey},
		{"MissingChunkHash", func(o map[string]any) { delete(chunk0(o), chunkHashKey) }, chunkHashKey},
		{"BlobChunkNonZeroCalldata",
			func(o map[string]any) { chunk0(o)[calldataLengthKey] = 5 },
			calldataLengthKey},
		{"CalldataChunkZeroLength",
			func(o map[string]any) {
				chunk0(o)[isCalldataKey] = true
				chunk0(o)[calldataLengthKey] = 0
			},
			calldataLengthKey},
		{"ChunkIsCalldataNotBool",
			func(o map[string]any) { chunk0(o)[isCalldataKey] = "yes" },
			isCalldataKey},
		{"ProofMissingProgramVk", func(o map[string]any) { delete(proof0(o), programVkKey) }, programVkKey},
		{"ProofMissingProof", func(o map[string]any) { delete(proof0(o), proofKey) }, proofKey},
		{"ProofPublicInputsMissingField", func(o map[string]any) { delete(pi0(o), "endBlockHash") }, "endBlockHash"},
		{"ProofMissingL2L1Messages", func(o map[string]any) { delete(proof0(o), l2L1MessagesKey) }, l2L1MessagesKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := unmarshalObject(t, readFixture(t, rollupRequestFixture))
			tc.mutate(obj)
			_, err := DecodeRollupRequest(marshal(t, obj))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func unmarshalObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var obj map[string]any
	require.NoError(t, json.Unmarshal(data, &obj))
	return obj
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
