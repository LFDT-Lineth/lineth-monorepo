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
}

// TestDecodeRequest_Malformed sweeps the envelope error paths: each case breaks
// one field of the valid single-block request and asserts an error naming it.
func TestDecodeRequest_Malformed(t *testing.T) {
	base := readFixture(t, "request_single_block.json")

	pr := func(o map[string]any) map[string]any {
		return o["proofRequest"].(map[string]any)
	}
	payload0 := func(o map[string]any) map[string]any {
		return pr(o)["payloads"].([]any)[0].(map[string]any)
	}
	npr := func(o map[string]any) map[string]any {
		return payload0(o)["statelessInput"].(map[string]any)["newPayloadRequest"].(map[string]any)
	}
	chainConfig := func(o map[string]any) map[string]any {
		return pr(o)["chainConfig"].(map[string]any)
	}
	execPayload := func(o map[string]any) map[string]any {
		return npr(o)["executionPayload"].(map[string]any)
	}

	cases := []struct {
		name    string
		mutate  func(o map[string]any)
		wantErr string
	}{
		{"MissingGuestProgramID",
			func(o map[string]any) { delete(o, "guestProgramId") },
			"guestProgramId"},
		{"MissingProofRequest",
			func(o map[string]any) { delete(o, "proofRequest") },
			"proofRequest"},
		{"MissingChainConfig",
			func(o map[string]any) { delete(pr(o), "chainConfig") },
			"chainConfig"},
		{"MissingForkName",
			func(o map[string]any) { delete(pr(o)["chainConfig"].(map[string]any), "forkName") },
			"forkName"},
		{"MissingChainID",
			func(o map[string]any) { delete(pr(o)["chainConfig"].(map[string]any), "chainId") },
			"chainId"},
		{"MissingPayloads",
			func(o map[string]any) { delete(pr(o), "payloads") },
			"payloads"},
		{"EmptyPayloads",
			func(o map[string]any) { pr(o)["payloads"] = []any{} },
			"payloads"},
		{"PayloadsNotArray",
			func(o map[string]any) { pr(o)["payloads"] = map[string]any{} },
			"payloads"},
		{"MissingStatelessInput",
			func(o map[string]any) { delete(payload0(o), "statelessInput") },
			"statelessInput"},
		{"ExecutionRequestsNonEmpty",
			func(o map[string]any) { npr(o)["executionRequests"] = []any{map[string]any{}} },
			"executionRequests"},
		{"ExecutionRequestsNotArray",
			func(o map[string]any) { npr(o)["executionRequests"] = map[string]any{"x": 1} },
			"executionRequests"},
		{"GuestProgramIDNotString",
			func(o map[string]any) { o["guestProgramId"] = 123 },
			"guestProgramId"},
		{"GuestProgramIDBadHex",
			func(o map[string]any) { o["guestProgramId"] = "0xzz" },
			"guestProgramId"},
		{"ChainIDNotParseable",
			func(o map[string]any) { chainConfig(o)["chainId"] = "not-a-number" },
			"chainId"},
		{"MissingExecutionPayload",
			func(o map[string]any) { delete(npr(o), "executionPayload") },
			"executionPayload"},
		{"MissingBlockNumber",
			func(o map[string]any) { delete(execPayload(o), "blockNumber") },
			"blockNumber"},
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
	obj["proofRequest"].(map[string]any)["chainConfig"].(map[string]any)["chainId"] = "0xe708"
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	req, err := DecodeRequest(raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(59144), req.ChainID)
}
