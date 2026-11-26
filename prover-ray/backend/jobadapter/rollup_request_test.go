package jobadapter

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
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
// envelope, chunk objects, parent blob data rolling hash, and embedded proofs.
func TestDecodeRollupRequest_Golden(t *testing.T) {
	req, err := DecodeRollupRequest(readSpecFixture(t, referenceRollupRequest))
	require.NoError(t, err)

	vk := filledHash(0x22)
	assert.Equal(t, vk[:], req.ProgramVk)
	assert.Equal(t, uint64(59144), req.ChainID)
	assert.Equal(t, filledHash(0x47), req.ParentDataRollingHash)
	assert.Equal(t, uint64(68), req.ParentSharedBlobCarryOverTailBytes)

	require.NotNil(t, req.ParentBlobDataRollingHash)
	assert.Equal(t, filledHash(0x39), *req.ParentBlobDataRollingHash)

	require.Len(t, req.Conflations, 2)
	require.Len(t, req.Conflations[0].BlockRlps, 2)
	assert.Equal(t, mustHex(t, "0xf90215a0"), req.Conflations[0].BlockRlps[0])

	require.Len(t, req.Chunks, 1)
	assert.Equal(t, mustHex(t, "0x01b166d6411021bf24e7eb5911dd6f815ea30509afc19fc5b5bc781156c2dc78"),
		req.Chunks[0].ChunkHash[:])
	assert.False(t, req.Chunks[0].IsCalldata)
	assert.NotEmpty(t, req.Chunks[0].BlobBytes)
	assert.Empty(t, req.Chunks[0].CalldataBytes)
	assert.Equal(t, mustHex(t, "0x83f5402ceefaf0fec6856c8ba565834228193978ab06deadd71b07a18dd0461a"+
		"4047fe19be9033c1a91ba74498b74f03"), req.Chunks[0].KZGCommitment)
	assert.Equal(t, mustHex(t, "0x8fe9644760a26dbe51d26924fd57c29efb8752260116856d04f7643f3b5b6449"+
		"ee68adf7ba380d39e54bf8f726e8c34a"), req.Chunks[0].KZGProof)

	require.Len(t, req.L2ExecutionProofs, 2)
	p0 := req.L2ExecutionProofs[0]
	assert.Equal(t, mustHex(t, "0xabcdef"), p0.Proof)
	assert.Equal(t, uint64(10), p0.StartBlockNumber)
	assert.Equal(t, mustHex(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), p0.ProgramVk)
	assert.Equal(t, uint64(12), req.L2ExecutionProofs[1].StartBlockNumber)

	outputs := embeddedGuestOutputs(t, unmarshalObject(t, readSpecFixture(t, referenceRollupRequest)), l2ExecutionProofsKey)
	for i, p := range req.L2ExecutionProofs {
		assert.Equal(t, mustHex(t, outputs[i]), p.GuestOutput, "proof %d", i)
		assert.Zero(t, p.PublicInputs, "proof %d", i)
	}
}

// TestDecodeRollupRequest_PublicInputs checks that the embedded proofs of a
// request in the publicInputs format decode to their public inputs.
func TestDecodeRollupRequest_PublicInputs(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
	withPublicInputs(t, obj, l2ExecutionProofsKey, publicInputs(testPublicInputs))

	req, err := DecodeRollupRequest(marshal(t, obj))
	require.NoError(t, err)
	require.Len(t, req.L2ExecutionProofs, 2)
	for i, p := range req.L2ExecutionProofs {
		assert.Equal(t, testPublicInputs, p.PublicInputs, "proof %d", i)
		assert.Nil(t, p.GuestOutput, "proof %d", i)
	}
}

// TestDecodeRollupRequest_OptionalBoundaryHash verifies parentBlobDataRollingHash
// is optional with no carry-over bytes: absent, it decodes to nil.
func TestDecodeRollupRequest_OptionalBoundaryHash(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
	pr := obj[proofRequestKey].(map[string]any)
	pr[parentSharedBlobCarryOverTailBytesKey] = 0
	delete(pr, parentBlobDataRollingHashKey)

	req, err := DecodeRollupRequest(marshal(t, obj))
	require.NoError(t, err)
	assert.Nil(t, req.ParentBlobDataRollingHash)
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

// TestDecodeRollupRequest_GuestOutputOverPublicInputs checks that an embedded
// proof carrying both guestOutput and publicInputs is read from guestOutput.
func TestDecodeRollupRequest_GuestOutputOverPublicInputs(t *testing.T) {
	obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
	outputs := embeddedGuestOutputs(t, obj, l2ExecutionProofsKey)
	withPublicInputs(t, obj, l2ExecutionProofsKey, publicInputs(testPublicInputs))
	for i, p := range obj[proofRequestKey].(map[string]any)[l2ExecutionProofsKey].([]any) {
		p.(map[string]any)[guestOutputKey] = outputs[i]
	}

	req, err := DecodeRollupRequest(marshal(t, obj))
	require.NoError(t, err)
	for i, p := range req.L2ExecutionProofs {
		assert.Equal(t, mustHex(t, outputs[i]), p.GuestOutput, "proof %d", i)
		assert.Zero(t, p.PublicInputs, "proof %d", i)
	}
}

func TestDecodeRollupRequest_InvalidJSON(t *testing.T) {
	_, err := DecodeRollupRequest([]byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing JSON")
}

// TestDecodeRollupRequest_InvalidShape sweeps the rollup envelope error paths,
// on the request in the publicInputs format so the public-input object has its
// own cases.
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
		{"MissingParentSharedBlobCarryOverTailBytes",
			func(o map[string]any) { delete(pr(o), parentSharedBlobCarryOverTailBytesKey) }, parentSharedBlobCarryOverTailBytesKey},
		{"MissingParentBlobDataRollingHashWithCarryOver",
			func(o map[string]any) { delete(pr(o), parentBlobDataRollingHashKey) }, parentBlobDataRollingHashKey},
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
		{"ProofWithoutPublicInputsOrGuestOutput",
			func(o map[string]any) { delete(proof0(o), publicInputsKey) }, guestOutputKey},
		{"ProofGuestOutputNotHex",
			func(o map[string]any) {
				delete(proof0(o), publicInputsKey)
				proof0(o)[guestOutputKey] = "0xzz"
			},
			guestOutputKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := unmarshalObject(t, readSpecFixture(t, referenceRollupRequest))
			withPublicInputs(t, obj, l2ExecutionProofsKey, publicInputs(testPublicInputs))
			tc.mutate(obj)
			_, err := DecodeRollupRequest(marshal(t, obj))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// testPublicInputs are the public inputs that the publicInputs-format tests put
// in each embedded l2-execution proof.
var testPublicInputs = backend.PublicInputs{
	ParentBlockHash:                       filledHash(0x0a),
	EndBlockHash:                          filledHash(0x0b),
	EndBlockNumber:                        14,
	EndBlockTimestamp:                     1763000210,
	EndL1L2BridgeRollingHash:              filledHash(0x03),
	EndL1L2BridgeRollingHashMessageNumber: 5,
	DynamicChainConfigHash:                filledHash(0xc0),
	ParentFtxRollingHash:                  filledHash(0x04),
	ParentFtxNumber:                       15,
	EndFtxRollingHash:                     filledHash(0x05),
	EndProcessedFtxNumber:                 18,
	FilteredAddresses:                     [][20]byte{{0x09}},
	TxFromsHash:                           filledHash(0x07),
	BlockCount:                            2,
	L2MessagingBlocksOffsets:              []uint16{1},
	L2L1Messages:                          [][32]byte{filledHash(0x08)},
}

// withPublicInputs replaces the guestOutput of each proof in the proofsKey list
// of the request obj with the publicInputs object pi, as the prover's response
// writer makes it.
func withPublicInputs(t *testing.T, obj map[string]any, proofsKey string, pi any) {
	t.Helper()
	for _, p := range obj[proofRequestKey].(map[string]any)[proofsKey].([]any) {
		piObj, err := jsonMarshalObject(pi)
		require.NoError(t, err)
		proof := p.(map[string]any)
		delete(proof, guestOutputKey)
		proof[publicInputsKey] = piObj
	}
}

// embeddedGuestOutputs returns the guestOutput of each proof in the proofsKey
// list of the request obj.
func embeddedGuestOutputs(t *testing.T, obj map[string]any, proofsKey string) []string {
	t.Helper()
	proofs := obj[proofRequestKey].(map[string]any)[proofsKey].([]any)
	outputs := make([]string, 0, len(proofs))
	for _, p := range proofs {
		out, ok := p.(map[string]any)[guestOutputKey].(string)
		require.True(t, ok, "each embedded proof should carry guestOutput")
		outputs = append(outputs, out)
	}
	return outputs
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
