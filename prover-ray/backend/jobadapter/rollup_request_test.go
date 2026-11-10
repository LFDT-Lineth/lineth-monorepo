package jobadapter

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const referenceRollupRequest = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.request.json"
const referenceRollupResponse = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/10-14-getZkRollupProofV1.response.json"

func readSpecFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return b
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	require.NoError(t, err)
	return b
}

// TestDecodeRollupRequest_Golden decodes the reference fixture and checks the
// envelope, chunk objects, boundary hash, and embedded proofs.
func TestDecodeRollupRequest_Golden(t *testing.T) {
	req, err := DecodeRollupRequest(readSpecFixture(t, referenceRollupRequest))
	require.NoError(t, err)

	vk := filledHash(0x22)
	assert.Equal(t, vk[:], req.ProgramVk)
	assert.Equal(t, uint64(59144), req.ChainID)
	assert.Equal(t, filledHash(0x47), req.ParentDataRollingHash)
	assert.Equal(t, uint64(60), req.ParentDataTailTakeBytes)

	require.NotNil(t, req.BoundaryPrevDataRollingHash)
	assert.Equal(t, filledHash(0x39), *req.BoundaryPrevDataRollingHash)

	require.Len(t, req.Conflations, 2)
	require.Len(t, req.Conflations[0].BlockRlps, 2)
	assert.Equal(t, mustHex(t, "0xf90215a0"), req.Conflations[0].BlockRlps[0])

	require.Len(t, req.Chunks, 1)
	assert.Equal(t, mustHex(t, "0x01787e92ee92078d97d6eb761e9224d362199ed44889a0543ac63ec6eec40062"),
		req.Chunks[0].ChunkHash[:])
	assert.False(t, req.Chunks[0].IsCalldata)
	assert.NotEmpty(t, req.Chunks[0].BlobBytes)
	assert.Empty(t, req.Chunks[0].CalldataBytes)
	assert.Equal(t, mustHex(t, "0xb7a1e1fb958a4b99a2caf2a9b820288febfecf07e4f2a8ce7e1e6cd440f5e7b7"+
		"67e7643954195a1264dcd090945cbeeb"), req.Chunks[0].KZGCommitment)
	assert.Equal(t, mustHex(t, "0x8ce30126356d26665f7b0b2f04244c59c1dff059e3c3ceeac0260a615fa92784"+
		"80261456261a383301bee674cf491b59"), req.Chunks[0].KZGProof)

	require.Len(t, req.L2ExecutionProofs, 2)
	p0 := req.L2ExecutionProofs[0]
	assert.Equal(t, mustHex(t, "0xabcdef"), p0.Proof)
	assert.Equal(t, uint64(10), p0.StartBlockNumber)
	assert.Equal(t, mustHex(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), p0.ProgramVk)
	assert.Equal(t, uint64(11), p0.PublicInputs.EndBlockNumber)
	assert.Equal(t, uint64(1763000200), p0.PublicInputs.EndBlockTimestamp)
	assert.Equal(t, filledHash(0xc0), p0.PublicInputs.DynamicChainConfigHash)
	assert.Equal(t, uint64(2), p0.PublicInputs.BlockCount)
	assert.Len(t, p0.PublicInputs.L2L1Messages, 1)
	assert.Equal(t, filledHash(0x08), p0.PublicInputs.L2L1Messages[0])
	assert.Empty(t, p0.PublicInputs.L2MessagingBlocksOffsets)
	assert.Len(t, p0.PublicInputs.FilteredAddresses, 2)

	assert.Equal(t, uint64(12), req.L2ExecutionProofs[1].StartBlockNumber)
	assert.Equal(t, uint64(14), req.L2ExecutionProofs[1].PublicInputs.EndBlockNumber)
}

// TestDecodeRollupRequest_OptionalBoundaryHash verifies boundaryPrevDataRollingHash
// is optional: absent, it decodes to nil with no cross-check.
func TestDecodeRollupRequest_OptionalBoundaryHash(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
	pr := obj[proofRequestKey].(map[string]any)
	delete(pr, boundaryPrevDataRollingHashKey)

	req, err := DecodeRollupRequest(marshal(t, obj))
	require.NoError(t, err)
	assert.Nil(t, req.BoundaryPrevDataRollingHash)
}

// TestDecodeRollupRequest_CalldataChunk verifies a calldata chunk decodes with
// empty blob and KZG fields.
func TestDecodeRollupRequest_CalldataChunk(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
	chunk := obj[proofRequestKey].(map[string]any)[chunksKey].([]any)[0].(map[string]any)
	chunk[isCalldataKey] = true
	chunk[blobBytesKey] = "0x"
	chunk[calldataBytesKey] = "0xdead"
	chunk[kzgCommitmentKey] = "0x"
	chunk[kzgProofKey] = "0x"

	req, err := DecodeRollupRequest(marshal(t, obj))
	require.NoError(t, err)
	require.Len(t, req.Chunks, 1)
	assert.True(t, req.Chunks[0].IsCalldata)
	assert.Equal(t, mustHex(t, "0xdead"), req.Chunks[0].CalldataBytes)
	assert.Empty(t, req.Chunks[0].BlobBytes)
	assert.Empty(t, req.Chunks[0].KZGCommitment)
	assert.Empty(t, req.Chunks[0].KZGProof)
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

	//nolint:goconst
	cases := []struct {
		name    string
		mutate  func(o map[string]any)
		wantErr string
	}{
		{"MissingGuestProgramId", func(o map[string]any) { delete(o, guestProgramIDKey) }, guestProgramIDKey},
		{"GuestProgramIdWrongLength", func(o map[string]any) { o[guestProgramIDKey] = "0x1234" }, guestProgramIDKey},
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
		{"MissingParentDataTailTakeBytes",
			func(o map[string]any) { delete(pr(o), parentDataTailTakeBytesKey) }, parentDataTailTakeBytesKey},
		{"ConflationBlockRlpsEmpty",
			func(o map[string]any) {
				pr(o)[conflationsKey].([]any)[0].(map[string]any)[blockRlpsKey] = []any{}
			},
			blockRlpsKey},
		{"MissingChunkHash", func(o map[string]any) { delete(chunk0(o), chunkHashKey) }, chunkHashKey},
		{"ChunkIsCalldataNotBool",
			func(o map[string]any) { chunk0(o)[isCalldataKey] = "yes" },
			isCalldataKey},
		{"BlobChunkWithCalldataBytes",
			func(o map[string]any) { chunk0(o)[calldataBytesKey] = "0xdead" },
			calldataBytesKey},
		{"BlobChunkEmptyBlob",
			func(o map[string]any) { chunk0(o)[blobBytesKey] = "0x" },
			blobBytesKey},
		{"CalldataChunkEmptyCalldata",
			func(o map[string]any) { chunk0(o)[isCalldataKey] = true },
			calldataBytesKey},
		{"MissingKZGCommitment", func(o map[string]any) { delete(chunk0(o), kzgCommitmentKey) }, kzgCommitmentKey},
		{"MissingKZGProof", func(o map[string]any) { delete(chunk0(o), kzgProofKey) }, kzgProofKey},
		{"BlobChunkEmptyKZGCommitment", func(o map[string]any) { chunk0(o)[kzgCommitmentKey] = "0x" }, kzgCommitmentKey},
		{"BlobChunkEmptyKZGProof", func(o map[string]any) { chunk0(o)[kzgProofKey] = "0x" }, kzgProofKey},
		{"BlobChunkKZGCommitmentWrongLength",
			func(o map[string]any) { chunk0(o)[kzgCommitmentKey] = "0x" + strings.Repeat("ab", 47) },
			kzgCommitmentKey},
		{"BlobChunkKZGProofWrongLength",
			func(o map[string]any) { chunk0(o)[kzgProofKey] = "0x" + strings.Repeat("ab", 49) },
			kzgProofKey},
		{"CalldataChunkWithKZGCommitment",
			func(o map[string]any) {
				c := chunk0(o)
				c[isCalldataKey] = true
				c[blobBytesKey] = "0x"
				c[calldataBytesKey] = "0xdead"
			},
			kzgCommitmentKey},
		{"CalldataChunkWithKZGProof",
			func(o map[string]any) {
				c := chunk0(o)
				c[isCalldataKey] = true
				c[blobBytesKey] = "0x"
				c[calldataBytesKey] = "0xdead"
				c[kzgCommitmentKey] = "0x"
			},
			kzgProofKey},
		{"ProofMissingProgramVk", func(o map[string]any) { delete(proof0(o), programVkKey) }, programVkKey},
		{"ProofMissingProof", func(o map[string]any) { delete(proof0(o), proofKey) }, proofKey},
		{"ProofPublicInputsMissingField", func(o map[string]any) { delete(pi0(o), "endBlockHash") }, "endBlockHash"},
		{"ProofPublicInputsMissingL2L1Messages",
			func(o map[string]any) { delete(pi0(o), l2L1MessagesKey) }, l2L1MessagesKey},
		{"ProofPublicInputsMissingBlockCount",
			func(o map[string]any) { delete(pi0(o), blockCountKey) }, blockCountKey},
		{"ProofPublicInputsMissingOffsets",
			func(o map[string]any) { delete(pi0(o), l2MessagingBlocksOffsetsKey) }, l2MessagingBlocksOffsetsKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
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
