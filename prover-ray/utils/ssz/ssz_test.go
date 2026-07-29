package ssz

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// payload0JSONNoChainConfig omits chainConfig from an otherwise valid payload.
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

// TestEncodeStatelessInput_GoldenVectors asserts byte-exact agreement with the
// reference SSZ outputs for the L2 execution request fixture's payloads.
// See testdata/README.md for provenance and regeneration instructions.
func TestEncodeStatelessInput_GoldenVectors(t *testing.T) {
	for _, name := range []string{"payload0", "payload1"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile("testdata/stateless_input_" + name + ".json")
			require.NoError(t, err)

			got, err := EncodeStatelessInput(input)
			require.NoError(t, err)

			want, err := os.ReadFile("testdata/stateless_input_" + name + ".ssz")
			require.NoError(t, err, "missing testdata/stateless_input_"+name+".ssz; see testdata/README.md")

			assert.Equal(t, want, got,
				"SSZ output differs from reference; see testdata/README.md for regeneration instructions")
		})
	}
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

func TestParseJSONObject_RejectsTrailingJSON(t *testing.T) {
	_, err := parseJSONObject([]byte(`{"a":1} {}`), "payload")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected trailing JSON value")

	_, err = parseJSONObject([]byte(`{"a":1} trailing`), "payload")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected trailing data")
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

// TestEncodeStatelessInput_UnsupportedFork rejects known but inactive forks.
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

// TestEncodeStatelessInput_MalformedInputs covers malformed payload fields.
func TestEncodeStatelessInput_MalformedInputs(t *testing.T) {
	base, err := os.ReadFile("testdata/stateless_input_full.json")
	require.NoError(t, err)

	npr := func(o map[string]any) map[string]any {
		return o["newPayloadRequest"].(map[string]any)
	}
	ep := func(o map[string]any) map[string]any {
		return npr(o)["executionPayload"].(map[string]any)
	}
	witness := func(o map[string]any) map[string]any {
		return o["executionWitness"].(map[string]any)
	}

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
		{"MissingExecutionPayload",
			func(o map[string]any) { delete(npr(o), "executionPayload") },
			"missing executionPayload"},
		{"MissingBlockNumber",
			func(o map[string]any) { delete(ep(o), "blockNumber") },
			"missing blockNumber"},
		{"MissingTransactions",
			func(o map[string]any) { delete(ep(o), "transactions") },
			"missing transactions"},
		{"NullTransactions",
			func(o map[string]any) { ep(o)["transactions"] = nil },
			"transactions"},
		{"MissingWithdrawals",
			func(o map[string]any) { delete(ep(o), "withdrawals") },
			"missing withdrawals"},
		{"NullWithdrawals",
			func(o map[string]any) { ep(o)["withdrawals"] = nil },
			"withdrawals"},
		{"MissingWithdrawalAmount",
			func(o map[string]any) { delete(ep(o)["withdrawals"].([]any)[0].(map[string]any), "amount") },
			"missing amount"},
		{"MissingVersionedHashes",
			func(o map[string]any) { delete(npr(o), "versionedHashes") },
			"missing versionedHashes"},
		{"NullVersionedHashes",
			func(o map[string]any) { npr(o)["versionedHashes"] = nil },
			"versionedHashes"},
		{"MissingWitnessState",
			func(o map[string]any) { delete(witness(o), "state") },
			"missing state"},
		{"NullWitnessState",
			func(o map[string]any) { witness(o)["state"] = nil },
			"state"},
		{"MissingWitnessCodes",
			func(o map[string]any) { delete(witness(o), "codes") },
			"missing codes"},
		{"NullWitnessCodes",
			func(o map[string]any) { witness(o)["codes"] = nil },
			"codes"},
		{"MissingWitnessHeaders",
			func(o map[string]any) { delete(witness(o), "headers") },
			"missing headers"},
		{"NullWitnessHeaders",
			func(o map[string]any) { witness(o)["headers"] = nil },
			"headers"},
		{"MissingChainID",
			func(o map[string]any) { delete(o["chainConfig"].(map[string]any), "chainId") },
			"missing chainId"},
		{"MissingForkName",
			func(o map[string]any) { delete(o["chainConfig"].(map[string]any), "forkName") },
			"missing forkName"},
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
		{"NullExecutionRequestsList",
			func(o map[string]any) {
				npr(o)["executionRequests"] = map[string]any{"deposits": nil}
			},
			"executionRequests.deposits"},
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
		{"TooManyWithdrawals",
			func(o map[string]any) {
				firstWithdrawal := ep(o)["withdrawals"].([]any)[0]
				withdrawals := make([]any, maxWithdrawalsPerPayload+1)
				for i := range withdrawals {
					withdrawals[i] = firstWithdrawal
				}
				ep(o)["withdrawals"] = withdrawals
			},
			"withdrawals"},
		{"BlockAccessListNotHex",
			func(o map[string]any) { ep(o)["blockAccessList"] = notHex },
			"blockAccessList"},
		{"VersionedHashWrongLength",
			func(o map[string]any) { npr(o)["versionedHashes"] = []any{"0x77"} },
			"versionedHashes[0]"},
		{"TooManyVersionedHashes",
			func(o map[string]any) {
				hashes := make([]any, maxBlobCommitmentsPerBlock+1)
				for i := range hashes {
					hashes[i] = "0x" + strings.Repeat("77", 32)
				}
				npr(o)["versionedHashes"] = hashes
			},
			"versionedHashes"},
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
		{"WitnessHeaderTooLarge",
			func(o map[string]any) {
				witness(o)["headers"] = []any{"0x" + strings.Repeat("aa", maxBytesPerHeader+1)}
			},
			"headers[0]"},
		{"TooManyWitnessHeaders",
			func(o map[string]any) {
				headers := make([]any, maxWitnessHeaders+1)
				for i := range headers {
					headers[i] = "0x01"
				}
				witness(o)["headers"] = headers
			},
			"headers"},
		{"TooManyPublicKeys",
			func(o map[string]any) {
				transactions := make([]any, maxPublicKeys+1)
				for i := range transactions {
					transactions[i] = "0xdead"
				}
				ep(o)["transactions"] = transactions
			},
			"public_keys"},
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

func TestAddSSZLength_RejectsUint32Overflow(t *testing.T) {
	_, err := addSSZLength("container", maxSSZSerializedBytes, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uint32 offset limit")
}

func TestCheckedAllocHelpers_RejectIntOverflow(t *testing.T) {
	_, err := checkedAllocCap("container", uint64(maxInt)+1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Go allocation limit")

	_, err = checkedSliceLen("frame", maxInt, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Go allocation limit")
}
