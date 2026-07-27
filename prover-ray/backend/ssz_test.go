package backend

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// payload0JSON is the encoder_obj form of payload[0] from
// rollup_spec/src/rollup_spec/prover_io/testdata/getZkL2ExecutionProofV1.request.json,
// as defined by proof_io_v1.py::_decode_payload: statelessInput fields with
// executionRequests replaced by {} and chainConfig injected at the top level.
// May differ from the coordinator's wire format.
//
// The corresponding expected SSZ output is in testdata/stateless_input_payload0.ssz.
// See testdata/README.md for metadata and regeneration instructions.
const payload0JSON = `{
  "newPayloadRequest": {
    "executionPayload": {
      "parentHash":      "0x0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a",
      "feeRecipient":    "0x0000000000000000000000000000000000000000",
      "stateRoot":       "0x1111111111111111111111111111111111111111111111111111111111111111",
      "receiptsRoot":    "0x2222222222222222222222222222222222222222222222222222222222222222",
      "logsBloom":       "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
      "prevRandao":      "0x3333333333333333333333333333333333333333333333333333333333333333",
      "blockNumber":     1000501,
      "gasLimit":        30000000,
      "gasUsed":         12000000,
      "timestamp":       1763000101,
      "extraData":       "0x",
      "baseFeePerGas":   "0x01",
      "blockHash":       "0x0101010101010101010101010101010101010101010101010101010101010101",
      "transactions":    ["0x02f86882e7088001843b9aca008252089422222222222222222222222222222222222222228080c080a0b09c2773ac7819dcf8cea66d878d5a41ebc04079162d4f78b7e8add87caa81a8a002cb3f20eb686af3b082e97d96a9fd7f68fabb9e927885a29738824b1d2dcd29"],
      "withdrawals":     [],
      "blobGasUsed":     0,
      "excessBlobGas":   0,
      "blockAccessList": "0x"
    },
    "versionedHashes":       [],
    "parentBeaconBlockRoot": "0x4444444444444444444444444444444444444444444444444444444444444444",
    "executionRequests":     {}
  },
  "executionWitness": {
    "state":   ["0xf800000000"],
    "codes":   ["0x6000000000"],
    "headers": ["0xf90000000a"]
  },
  "chainConfig": {
    "chainId":  59144,
    "forkName": "Amsterdam"
  }
}`

// payload0JSONNoChainConfig is payload0JSON with chainConfig omitted; used to
// verify that a missing chainConfig produces a clear error.
const payload0JSONNoChainConfig = `{
  "newPayloadRequest": {
    "executionPayload": {
      "parentHash":      "0x0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a",
      "feeRecipient":    "0x0000000000000000000000000000000000000000",
      "stateRoot":       "0x1111111111111111111111111111111111111111111111111111111111111111",
      "receiptsRoot":    "0x2222222222222222222222222222222222222222222222222222222222222222",
      "logsBloom":       "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
      "prevRandao":      "0x3333333333333333333333333333333333333333333333333333333333333333",
      "blockNumber":     1000501,
      "gasLimit":        30000000,
      "gasUsed":         12000000,
      "timestamp":       1763000101,
      "extraData":       "0x",
      "baseFeePerGas":   "0x01",
      "blockHash":       "0x0101010101010101010101010101010101010101010101010101010101010101",
      "transactions":    [],
      "withdrawals":     [],
      "blobGasUsed":     0,
      "excessBlobGas":   0,
      "blockAccessList": "0x"
    },
    "versionedHashes":       [],
    "parentBeaconBlockRoot": "0x4444444444444444444444444444444444444444444444444444444444444444",
    "executionRequests":     {}
  },
  "executionWitness": {
    "state":   [],
    "codes":   [],
    "headers": []
  }
}`

// TestEncodeStatelessInput_GoldenVector asserts byte-exact agreement with the
// reference SSZ output stored in testdata/stateless_input_payload0.ssz.
// See testdata/README.md for provenance and regeneration instructions.
func TestEncodeStatelessInput_GoldenVector(t *testing.T) {
	got, err := EncodeStatelessInput([]byte(payload0JSON))
	require.NoError(t, err)

	want, err := os.ReadFile("testdata/stateless_input_payload0.ssz")
	require.NoError(t, err, "missing testdata/stateless_input_payload0.ssz; see testdata/README.md")

	assert.Equal(t, want, got,
		"SSZ output differs from reference; see testdata/README.md for regeneration instructions")
}

// TestEncodeStatelessInput_GoldenVector_Full asserts byte-exact agreement for
// a fixture that exercises every encoder path the payload0 fixture leaves
// empty: non-empty withdrawals, versionedHashes, extraData, blockAccessList,
// a non-zero slotNumber, a multi-byte baseFeePerGas, a legacy EIP-155
// transaction alongside the EIP-1559 one, and multi-entry witness lists.
// Input and expected output both live in testdata/ (see testdata/README.md).
func TestEncodeStatelessInput_GoldenVector_Full(t *testing.T) {
	input, err := os.ReadFile("testdata/stateless_input_full.json")
	require.NoError(t, err)

	got, err := EncodeStatelessInput(input)
	require.NoError(t, err)

	want, err := os.ReadFile("testdata/stateless_input_full.ssz")
	require.NoError(t, err, "missing testdata/stateless_input_full.ssz; see testdata/README.md")

	assert.Equal(t, want, got,
		"SSZ output differs from reference; see testdata/README.md for regeneration instructions")
}

// TestEncodeStatelessInput_InvalidJSON asserts a clear error for garbage input.
func TestEncodeStatelessInput_InvalidJSON(t *testing.T) {
	_, err := EncodeStatelessInput([]byte("not json"))
	require.Error(t, err)
}

// TestEncodeStatelessInput_MissingChainConfig asserts a clear error when
// chainConfig is absent from an otherwise valid payload.
func TestEncodeStatelessInput_MissingChainConfig(t *testing.T) {
	_, err := EncodeStatelessInput([]byte(payload0JSONNoChainConfig))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chainConfig")
}

// TestEncodeStatelessInput_NoTransactions_EmptyPublicKeys verifies that a
// payload with no transactions encodes successfully with an empty public_keys
// list (no public key recovery attempted, no panic).
func TestEncodeStatelessInput_NoTransactions_EmptyPublicKeys(t *testing.T) {
	noTx := `{
  "newPayloadRequest": {
    "executionPayload": {
      "parentHash":      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "feeRecipient":    "0x0000000000000000000000000000000000000000",
      "stateRoot":       "0x0000000000000000000000000000000000000000000000000000000000000000",
      "receiptsRoot":    "0x0000000000000000000000000000000000000000000000000000000000000000",
      "logsBloom":       "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
      "prevRandao":      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "blockNumber":     1,
      "gasLimit":        1000000,
      "gasUsed":         0,
      "timestamp":       1000,
      "extraData":       "0x",
      "baseFeePerGas":   "0x01",
      "blockHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
      "transactions":    [],
      "withdrawals":     [],
      "blobGasUsed":     0,
      "excessBlobGas":   0,
      "blockAccessList": "0x"
    },
    "versionedHashes":       [],
    "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "executionRequests":     {}
  },
  "executionWitness": { "state": [], "codes": [], "headers": [] },
  "chainConfig": { "chainId": 59144, "forkName": "Amsterdam" }
}`
	got, err := EncodeStatelessInput([]byte(noTx))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 2)
	assert.Equal(t, byte(0x00), got[0], "schema id high byte")
	assert.Equal(t, byte(0x01), got[1], "schema id low byte")
}

// TestEncodeStatelessInput_UnsupportedFork verifies that a forkName which is
// known to protocolForks but is not the single fork this backend supports
// (activeFork = Amsterdam) returns a clear error rather than silently encoding
// the wrong fork index. Truly unknown forkName values are covered by the
// UnknownForkName case in TestEncodeStatelessInput_NegativeCases.
func TestEncodeStatelessInput_UnsupportedFork(t *testing.T) {
	unsupportedFork := `{
  "newPayloadRequest": {
    "executionPayload": {
      "parentHash":      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "feeRecipient":    "0x0000000000000000000000000000000000000000",
      "stateRoot":       "0x0000000000000000000000000000000000000000000000000000000000000000",
      "receiptsRoot":    "0x0000000000000000000000000000000000000000000000000000000000000000",
      "logsBloom":       "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
      "prevRandao":      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "blockNumber":     1,
      "gasLimit":        1000000,
      "gasUsed":         0,
      "timestamp":       1000,
      "extraData":       "0x",
      "baseFeePerGas":   "0x01",
      "blockHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
      "transactions":    [],
      "withdrawals":     [],
      "blobGasUsed":     0,
      "excessBlobGas":   0,
      "blockAccessList": "0x"
    },
    "versionedHashes":       [],
    "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "executionRequests":     {}
  },
  "executionWitness": { "state": [], "codes": [], "headers": [] },
  "chainConfig": { "chainId": 59144, "forkName": "Prague" }
}`
	_, err := EncodeStatelessInput([]byte(unsupportedFork))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fork")
}

// TestEncodeStatelessInput_InvalidHexField verifies that a hex field with the
// wrong byte length (parentHash truncated to 31 bytes) returns a clear error.
func TestEncodeStatelessInput_InvalidHexField(t *testing.T) {
	badHex := `{
  "newPayloadRequest": {
    "executionPayload": {
      "parentHash":      "0x0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a",
      "feeRecipient":    "0x0000000000000000000000000000000000000000",
      "stateRoot":       "0x0000000000000000000000000000000000000000000000000000000000000000",
      "receiptsRoot":    "0x0000000000000000000000000000000000000000000000000000000000000000",
      "logsBloom":       "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
      "prevRandao":      "0x0000000000000000000000000000000000000000000000000000000000000000",
      "blockNumber":     1,
      "gasLimit":        1000000,
      "gasUsed":         0,
      "timestamp":       1000,
      "extraData":       "0x",
      "baseFeePerGas":   "0x01",
      "blockHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
      "transactions":    [],
      "withdrawals":     [],
      "blobGasUsed":     0,
      "excessBlobGas":   0,
      "blockAccessList": "0x"
    },
    "versionedHashes":       [],
    "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "executionRequests":     {}
  },
  "executionWitness": { "state": [], "codes": [], "headers": [] },
  "chainConfig": { "chainId": 59144, "forkName": "Amsterdam" }
}`
	_, err := EncodeStatelessInput([]byte(badHex))
	require.Error(t, err)
}

// TestEncodeStatelessInput_MalformedInputs sweeps the encoder's error paths:
// each case takes the valid full fixture, breaks exactly one field, and
// asserts a clear error naming that field. This pins the error messages as a
// contract and covers the negative branches the golden vectors cannot reach.
func TestEncodeStatelessInput_MalformedInputs(t *testing.T) {
	base, err := os.ReadFile("testdata/stateless_input_full.json")
	require.NoError(t, err)

	// Navigation helpers over the decoded JSON object.
	npr := func(o map[string]any) map[string]any {
		return o["newPayloadRequest"].(map[string]any)
	}
	ep := func(o map[string]any) map[string]any {
		return npr(o)["executionPayload"].(map[string]any)
	}
	witness := func(o map[string]any) map[string]any {
		return o["executionWitness"].(map[string]any)
	}

	// notHex is valid-length-looking but undecodable hex, reused across cases.
	const notHex = "0xzz"

	cases := []struct {
		name    string
		mutate  func(o map[string]any)
		wantErr string
	}{
		{"MissingNewPayloadRequest",
			func(o map[string]any) { delete(o, "newPayloadRequest") },
			"missing newPayloadRequest"},
		{"MissingExecutionWitness",
			func(o map[string]any) { delete(o, "executionWitness") },
			"missing executionWitness"},
		{"MissingExecutionRequests",
			func(o map[string]any) { delete(npr(o), "executionRequests") },
			"missing executionRequests"},
		{"NonEmptyDeposits",
			func(o map[string]any) {
				npr(o)["executionRequests"] = map[string]any{"deposits": []any{map[string]any{}}}
			},
			"deposits must be empty"},
		{"NonEmptyWithdrawalRequests",
			func(o map[string]any) {
				npr(o)["executionRequests"] = map[string]any{"withdrawals": []any{map[string]any{}}}
			},
			"withdrawals must be empty"},
		{"NonEmptyConsolidations",
			func(o map[string]any) {
				npr(o)["executionRequests"] = map[string]any{"consolidations": []any{map[string]any{}}}
			},
			"consolidations must be empty"},
		{"FeeRecipientWrongLength",
			func(o map[string]any) { ep(o)["feeRecipient"] = "0x1111" },
			"feeRecipient"},
		{"StateRootNotHex",
			func(o map[string]any) { ep(o)["stateRoot"] = notHex },
			"stateRoot"},
		{"ReceiptsRootWrongLength",
			func(o map[string]any) { ep(o)["receiptsRoot"] = "0x22" },
			"receiptsRoot"},
		{"LogsBloomWrongLength",
			func(o map[string]any) { ep(o)["logsBloom"] = "0x00" },
			"logsBloom"},
		{"PrevRandaoWrongLength",
			func(o map[string]any) { ep(o)["prevRandao"] = "0x33" },
			"prevRandao"},
		{"ExtraDataNotHex",
			func(o map[string]any) { ep(o)["extraData"] = notHex },
			"extraData"},
		{"ExtraDataTooLong",
			func(o map[string]any) { ep(o)["extraData"] = "0x" + strings.Repeat("aa", 33) },
			"extraData"},
		{"BaseFeeEmpty",
			func(o map[string]any) { ep(o)["baseFeePerGas"] = "0x" },
			"baseFeePerGas"},
		{"BaseFeeNotHex",
			func(o map[string]any) { ep(o)["baseFeePerGas"] = notHex },
			"baseFeePerGas"},
		{"BaseFeeOverflow",
			func(o map[string]any) { ep(o)["baseFeePerGas"] = "0x01" + strings.Repeat("00", 32) },
			"baseFeePerGas"},
		{"BlockHashWrongLength",
			func(o map[string]any) { ep(o)["blockHash"] = "0x02" },
			"blockHash"},
		{"TransactionNotHex",
			func(o map[string]any) { ep(o)["transactions"] = []any{notHex} },
			"transactions[0]"},
		{"TransactionUndecodable",
			func(o map[string]any) { ep(o)["transactions"] = []any{"0xdead"} },
			"transactions[0]"},
		{"WithdrawalAddressWrongLength",
			func(o map[string]any) {
				ep(o)["withdrawals"].([]any)[0].(map[string]any)["address"] = "0x55"
			},
			"withdrawals[0]"},
		{"BlockAccessListNotHex",
			func(o map[string]any) { ep(o)["blockAccessList"] = notHex },
			"blockAccessList"},
		{"VersionedHashWrongLength",
			func(o map[string]any) { npr(o)["versionedHashes"] = []any{"0x77"} },
			"versionedHashes[0]"},
		{"ParentBeaconBlockRootWrongLength",
			func(o map[string]any) { npr(o)["parentBeaconBlockRoot"] = "0x44" },
			"parentBeaconBlockRoot"},
		{"WitnessStateNotHex",
			func(o map[string]any) { witness(o)["state"] = []any{notHex} },
			"state[0]"},
		{"WitnessCodesNotHex",
			func(o map[string]any) { witness(o)["codes"] = []any{notHex} },
			"codes[0]"},
		{"WitnessHeadersNotHex",
			func(o map[string]any) { witness(o)["headers"] = []any{notHex} },
			"headers[0]"},
		{"UnknownForkName",
			func(o map[string]any) { o["chainConfig"].(map[string]any)["forkName"] = "Foo" },
			"unknown fork name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var obj map[string]any
			require.NoError(t, json.Unmarshal(base, &obj))
			tc.mutate(obj)
			raw, err := json.Marshal(obj)
			require.NoError(t, err)

			_, err = EncodeStatelessInput(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestBuildInputs_MultiBlock_NotImplemented asserts that Core.buildInputs
// returns ErrNotImplemented when a job spans more than one block.
// Multi-block SSZ conflation format is not yet decided
// (open question #1 in wiki backend-overview.md).
func TestBuildInputs_MultiBlock_NotImplemented(t *testing.T) {
	c := &Core{cfg: Config{}}
	_, err := c.buildInputs(Job{StartBlock: 1, EndBlock: 2, Type: ProofTypeL2Execution})
	require.ErrorIs(t, err, ErrNotImplemented)
}
