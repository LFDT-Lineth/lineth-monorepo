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

// guestProgramID is the routing id in the request fixtures.
const guestProgramID = "0x17d2e0660946012c80c5fe6bbecc2076a6f6f5aa58606efe66a14426d2ffe46f"

const referenceL2ExecutionRequest = "../../../rollup_spec/src/rollup_spec/prover_io/testdata/getZkL2ExecutionProofV1.request.json"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return b
}

// TestDecodeRequest_SingleBlock is the end-to-end golden: the single-block
// request fixture must decode to one payload whose framed SSZ is byte-for-byte
// the reference encoder's output (single_block_expected.ssz, a copy of
// utils/ssz/testdata/stateless_input_payload0.ssz). This pins the whole envelope
// path: chainConfig injection, executionRequests reduction, and SSZ encoding.
func TestDecodeRequest_SingleBlock(t *testing.T) {
	req, err := DecodeRequest(readFixture(t, "request_single_block.json"))
	require.NoError(t, err)

	assert.Equal(t, uint64(59144), req.ChainID)
	assert.Equal(t, "Amsterdam", req.ForkName)

	wantID, err := hex.DecodeString(strings.TrimPrefix(guestProgramID, "0x"))
	require.NoError(t, err)
	assert.Equal(t, wantID, req.GuestProgramID)

	require.Len(t, req.Payloads, 1)
	assert.Equal(t, uint64(1000501), req.Payloads[0].BlockNumber)
	assert.Empty(t, req.Payloads[0].ForcedTransactions)

	wantSSZ := readFixture(t, "single_block_expected.ssz")
	assert.Equal(t, wantSSZ, req.Payloads[0].FramedSSZ,
		"framed SSZ must equal the reference encoder output")
}

// TestDecodeRequest_MultiBlock verifies that a request with M > 1 payloads
// decodes to M payloads with the block numbers in range order. The decoder
// does not reject multi-block; the adapter does (single-block only for now).
func TestDecodeRequest_MultiBlock(t *testing.T) {
	req, err := DecodeRequest(readFixture(t, "request_multi_block.json"))
	require.NoError(t, err)

	require.Len(t, req.Payloads, 2)
	assert.Equal(t, uint64(1000501), req.Payloads[0].BlockNumber)
	assert.Equal(t, uint64(1000502), req.Payloads[1].BlockNumber)
	assert.Len(t, req.Payloads[0].ForcedTransactions, 1)
	assert.Len(t, req.Payloads[1].ForcedTransactions, 2)
}

// TestDecodeRequest_ReferenceFixture verifies the Go decoder accepts the
// language-neutral V1 request fixture from rollup_spec.
func TestDecodeRequest_ReferenceFixture(t *testing.T) {
	data, err := os.ReadFile(referenceL2ExecutionRequest)
	require.NoError(t, err)

	req, err := DecodeRequest(data)
	require.NoError(t, err)

	assert.Equal(t, uint64(59144), req.ChainID)
	assert.Equal(t, "Amsterdam", req.ForkName)
	require.Len(t, req.Payloads, 2)
	assert.Equal(t, uint64(1000501), req.Payloads[0].BlockNumber)
	assert.Equal(t, uint64(1000502), req.Payloads[1].BlockNumber)
	assert.Len(t, req.Payloads[0].ForcedTransactions, 1)
	assert.Len(t, req.Payloads[1].ForcedTransactions, 2)
}

// TestDecodeRequest_Malformed sweeps the envelope error paths: each case breaks
// one field of the valid single-block request and asserts an error naming it.
func TestDecodeRequest_Malformed(t *testing.T) {
	base := readFixture(t, "request_single_block.json")

	pr := func(o map[string]any) map[string]any {
		return o[proofRequestKey].(map[string]any)
	}
	payload0 := func(o map[string]any) map[string]any {
		return pr(o)["payloads"].([]any)[0].(map[string]any)
	}
	npr := func(o map[string]any) map[string]any {
		return payload0(o)[statelessInputKey].(map[string]any)[newPayloadRequestKey].(map[string]any)
	}
	chainConfig := func(o map[string]any) map[string]any {
		return pr(o)[chainConfigKey].(map[string]any)
	}
	execPayload := func(o map[string]any) map[string]any {
		return npr(o)[executionPayloadKey].(map[string]any)
	}
	rollupExtension := func(o map[string]any) map[string]any {
		return payload0(o)[rollupExtensionKey].(map[string]any)
	}

	cases := []struct {
		name    string
		mutate  func(o map[string]any)
		wantErr string
	}{
		{"MissingGuestProgramID",
			func(o map[string]any) { delete(o, guestProgramIDKey) },
			guestProgramIDKey},
		{"MissingProofRequest",
			func(o map[string]any) { delete(o, proofRequestKey) },
			proofRequestKey},
		{"MissingChainConfig",
			func(o map[string]any) { delete(pr(o), chainConfigKey) },
			chainConfigKey},
		{"MissingL2MessageServiceAddress",
			func(o map[string]any) { delete(chainConfig(o), l2MessageServiceAddressKey) },
			l2MessageServiceAddressKey},
		{"BadL2MessageServiceAddress",
			func(o map[string]any) { chainConfig(o)[l2MessageServiceAddressKey] = "0x1234" },
			l2MessageServiceAddressKey},
		{"MissingCoinbase",
			func(o map[string]any) { delete(chainConfig(o), coinbaseKey) },
			coinbaseKey},
		{"BadCoinbase",
			func(o map[string]any) { chainConfig(o)[coinbaseKey] = "0x1234" },
			coinbaseKey},
		{"MissingForkName",
			func(o map[string]any) { delete(pr(o)["chainConfig"].(map[string]any), forkNameKey) },
			forkNameKey},
		{"MissingChainID",
			func(o map[string]any) { delete(pr(o)["chainConfig"].(map[string]any), chainIDKey) },
			chainIDKey},
		{"MissingParentFtxRollingHash",
			func(o map[string]any) { delete(pr(o), parentFtxRollingHashKey) },
			parentFtxRollingHashKey},
		{"BadParentFtxRollingHash",
			func(o map[string]any) { pr(o)[parentFtxRollingHashKey] = "0x1234" },
			parentFtxRollingHashKey},
		{"MissingParentLastProcessedFtxNumber",
			func(o map[string]any) { delete(pr(o), parentLastProcessedFtxNumberKey) },
			parentLastProcessedFtxNumberKey},
		{"BadParentLastProcessedFtxNumber",
			func(o map[string]any) { pr(o)[parentLastProcessedFtxNumberKey] = "not-a-number" },
			parentLastProcessedFtxNumberKey},
		{"MissingPayloads",
			func(o map[string]any) { delete(pr(o), payloadsKey) },
			payloadsKey},
		{"EmptyPayloads",
			func(o map[string]any) { pr(o)[payloadsKey] = []any{} },
			payloadsKey},
		{"PayloadsNotArray",
			func(o map[string]any) { pr(o)[payloadsKey] = map[string]any{} },
			payloadsKey},
		{"PayloadsNull",
			func(o map[string]any) { pr(o)[payloadsKey] = nil },
			payloadsKey},
		{"MissingStatelessInput",
			func(o map[string]any) { delete(payload0(o), statelessInputKey) },
			statelessInputKey},
		{"MissingRollupExtension",
			func(o map[string]any) { delete(payload0(o), rollupExtensionKey) },
			rollupExtensionKey},
		{"MissingForcedTransactions",
			func(o map[string]any) { delete(rollupExtension(o), forcedTransactionsKey) },
			forcedTransactionsKey},
		{"ForcedTransactionsNotArray",
			func(o map[string]any) { rollupExtension(o)[forcedTransactionsKey] = map[string]any{"x": 1} },
			forcedTransactionsKey},
		{"ForcedTransactionsNull",
			func(o map[string]any) { rollupExtension(o)[forcedTransactionsKey] = nil },
			forcedTransactionsKey},
		{"ForcedTransactionBadSignedTx",
			func(o map[string]any) {
				rollupExtension(o)[forcedTransactionsKey] = []any{map[string]any{
					numberKey:      16,
					deadlineKey:    1000599,
					signedTxRlpKey: "02f86b",
					acceptanceKey:  forcedTxIncluded,
				}}
			},
			signedTxRlpKey},
		{"ForcedTransactionBadAcceptance",
			func(o map[string]any) {
				rollupExtension(o)[forcedTransactionsKey] = []any{map[string]any{
					numberKey:      16,
					deadlineKey:    1000599,
					signedTxRlpKey: "0x02f86b",
					acceptanceKey:  "NOPE",
				}}
			},
			acceptanceKey},
		{"ExecutionRequestsNonEmpty",
			func(o map[string]any) { npr(o)[executionRequestsKey] = []any{map[string]any{}} },
			executionRequestsKey},
		{"ExecutionRequestsNotArray",
			func(o map[string]any) { npr(o)[executionRequestsKey] = map[string]any{"x": 1} },
			executionRequestsKey},
		{"ExecutionRequestsNull",
			func(o map[string]any) { npr(o)[executionRequestsKey] = nil },
			executionRequestsKey},
		{"GuestProgramIDNotString",
			func(o map[string]any) { o[guestProgramIDKey] = 123 },
			guestProgramIDKey},
		{"GuestProgramIDNoPrefix",
			func(o map[string]any) { o[guestProgramIDKey] = strings.TrimPrefix(guestProgramID, "0x") },
			guestProgramIDKey},
		{"GuestProgramIDBadHex",
			func(o map[string]any) { o[guestProgramIDKey] = "0xzz" },
			guestProgramIDKey},
		{"GuestProgramIDWrongLength",
			func(o map[string]any) { o[guestProgramIDKey] = "0x1234" },
			guestProgramIDKey},
		{"ChainIDNotParseable",
			func(o map[string]any) { chainConfig(o)[chainIDKey] = "not-a-number" },
			chainIDKey},
		{"MissingExecutionPayload",
			func(o map[string]any) { delete(npr(o), executionPayloadKey) },
			executionPayloadKey},
		{"MissingBlockNumber",
			func(o map[string]any) { delete(execPayload(o), blockNumberKey) },
			blockNumberKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var obj map[string]any
			require.NoError(t, json.Unmarshal(base, &obj))
			tc.mutate(obj)
			raw, err := json.Marshal(obj)
			require.NoError(t, err)

			_, err = DecodeRequest(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestDecodeRequest_InvalidJSON asserts a clear error for garbage input.
func TestDecodeRequest_InvalidJSON(t *testing.T) {
	_, err := DecodeRequest([]byte("not json"))
	require.Error(t, err)
}

// TestDecodeRequest_ChainIDHexString verifies a chainId given as a 0x-hex
// quantity (not a JSON number) is accepted, matching the reference's _u64.
func TestDecodeRequest_ChainIDHexString(t *testing.T) {
	var obj map[string]any
	require.NoError(t, json.Unmarshal(readFixture(t, "request_single_block.json"), &obj))
	obj[proofRequestKey].(map[string]any)[chainConfigKey].(map[string]any)[chainIDKey] = "0xe708"
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	req, err := DecodeRequest(raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(59144), req.ChainID)
}
