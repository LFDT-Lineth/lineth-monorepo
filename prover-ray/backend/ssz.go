package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// SSZ encoder for the Amsterdam stateless block input (EIP-8025).
//
// This is a hand-written port of the reference encoder
// rollup_spec/src/rollup_spec/stateless_input.py::encode_stateless_input_ssz.
// The wire schema mirrors execution-specs `stateless_ssz.py` at the pinned
// commit (a456712e); the container field orders below are byte-for-byte
// significant and must not be reordered. The golden vector in
// testdata/stateless_input_payload0.ssz pins the exact output.
//
// Input is the readable "encoder_obj" form produced by
// proof_io_v1.py::_decode_payload: the coordinator's statelessInput with
// chainConfig injected and executionRequests reduced to {}.

// statelessInputSchemaID is the two-byte big-endian schema id every framed
// stateless input is prefixed with (execution-specs `stateless_ssz.py::SCHEMA_ID`).
var statelessInputSchemaID = []byte{0x00, 0x01}

// protocolForks mirrors rollup_spec/fork.py::ProtocolFork; the SSZ active_fork
// value is the index into this ordered list. Amsterdam is index 20 at the
// pinned execution-specs commit. Re-sync if the pin moves.
var protocolForks = []string{
	"Frontier", "Homestead", "DAOFork", "TangerineWhistle", "SpuriousDragon",
	"Byzantium", "StPetersburg", "Istanbul", "MuirGlacier", "Berlin",
	"London", "ArrowGlacier", "GrayGlacier", "Paris", "Shanghai",
	"Cancun", "Prague", "Osaka", "BPO1", "BPO2",
	"Amsterdam",
}

// activeFork is the single fork this backend supports, matching
// rollup_spec/fork.py::ACTIVE_FORK.
const activeFork = "Amsterdam"

// maxExtraDataBytes bounds extra_data, mirroring
// rollup_spec/canonical_ssz.py::MAX_EXTRA_DATA_BYTES (ByteList[2**5]). The
// reference SSZ encoder enforces this at encode time via the ByteList type;
// we check it explicitly.
const maxExtraDataBytes = 32

// SSZ list/vector bounds mirrored from rollup_spec/stateless_input.py and
// rollup_spec/canonical_ssz.py. The hand-written encoder must reject inputs
// outside these bounds just like the reference remerkleable encoder does.
const (
	maxBytesPerTransaction     = 1 << 30
	maxTransactionsPerPayload  = 1 << 20
	maxWithdrawalsPerPayload   = 1 << 4
	maxBlobCommitmentsPerBlock = 4096
	maxWitnessNodes            = 1 << 20
	maxWitnessCodes            = 1 << 16
	maxWitnessHeaders          = 256
	maxBytesPerWitnessNode     = 1 << 20
	maxBytesPerCode            = 1 << 24
	maxBytesPerHeader          = 1 << 10
	maxPublicKeys              = 1 << 15
)

// JSON input model (readable encoder_obj form). The Python reference consumes a
// dict and indexes required fields directly. Keep the Go port map-based too, so
// missing required fields error before any SSZ bytes are built.

type jsonObj map[string]json.RawMessage

func parseJSONObject(data []byte, ctx string) (jsonObj, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var obj jsonObj
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("%s: parsing JSON object: %w", ctx, err)
	}
	if obj == nil {
		return nil, fmt.Errorf("%s: expected object", ctx)
	}
	return obj, nil
}

func requireRaw(obj jsonObj, key string) (json.RawMessage, error) {
	raw, ok := obj[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	return raw, nil
}

func requireObject(obj jsonObj, key string) (jsonObj, error) {
	raw, err := requireRaw(obj, key)
	if err != nil {
		return nil, err
	}
	child, err := parseJSONObject(raw, key)
	if err != nil {
		return nil, err
	}
	return child, nil
}

func requireString(obj jsonObj, key string) (string, error) {
	raw, err := requireRaw(obj, key)
	if err != nil {
		return "", err
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s: expected string: %w", key, err)
	}
	return value, nil
}

func requireStringList(obj jsonObj, key string) ([]string, error) {
	raw, err := requireRaw(obj, key)
	if err != nil {
		return nil, err
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%s: expected string array: %w", key, err)
	}
	if values == nil {
		return nil, fmt.Errorf("%s: expected string array", key)
	}
	return values, nil
}

func requireObjectList(obj jsonObj, key string) ([]jsonObj, error) {
	raw, err := requireRaw(obj, key)
	if err != nil {
		return nil, err
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil, fmt.Errorf("%s: expected object array: %w", key, err)
	}
	if rawItems == nil {
		return nil, fmt.Errorf("%s: expected object array", key)
	}
	items := make([]jsonObj, len(rawItems))
	for i, rawItem := range rawItems {
		item, err := parseJSONObject(rawItem, fmt.Sprintf("%s[%d]", key, i))
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func requireUint64(obj jsonObj, key string) (uint64, error) {
	raw, err := requireRaw(obj, key)
	if err != nil {
		return 0, err
	}
	return parseUint64JSON(raw, key)
}

func optionalUint64(obj jsonObj, key string, defaultValue uint64) (uint64, error) {
	raw, ok := obj[key]
	if !ok {
		return defaultValue, nil
	}
	return parseUint64JSON(raw, key)
}

func parseUint64JSON(raw json.RawMessage, key string) (uint64, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var value any
	if err := dec.Decode(&value); err != nil {
		return 0, fmt.Errorf("%s: expected uint64: %w", key, err)
	}

	switch v := value.(type) {
	case json.Number:
		n, err := strconv.ParseUint(v.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s: expected uint64: %w", key, err)
		}
		return n, nil
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s: expected decimal uint64 string: %w", key, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%s: expected uint64, got %T", key, value)
	}
}

// SSZ serialization primitives

// sszField is one field of an SSZ container: either fixed-size (serialized
// inline) or variable-size (serialized as a 4-byte offset in the fixed section,
// with its data appended to the heap).
type sszField struct {
	data     []byte
	variable bool
}

func fixed(data []byte) sszField    { return sszField{data: data, variable: false} }
func variable(data []byte) sszField { return sszField{data: data, variable: true} }

// sszContainer serializes an ordered list of fields per the SSZ spec: fixed
// fields inline, variable fields as uint32 little-endian offsets (relative to
// the start of the container) followed by their data in the heap section.
func sszContainer(fields ...sszField) []byte {
	fixedLen := 0
	for _, f := range fields {
		if f.variable {
			fixedLen += 4
		} else {
			fixedLen += len(f.data)
		}
	}

	head := make([]byte, 0, fixedLen)
	var heap []byte
	offset := fixedLen
	for _, f := range fields {
		if f.variable {
			var off [4]byte
			binary.LittleEndian.PutUint32(off[:], uint32(offset)) //nolint:gosec // offsets are bounded by payload size
			head = append(head, off[:]...)
			heap = append(heap, f.data...)
			offset += len(f.data)
		} else {
			head = append(head, f.data...)
		}
	}
	return append(head, heap...)
}

// sszListFixed encodes a list whose elements are all fixed-size: their
// encodings concatenated. An empty list encodes to no bytes.
func sszListFixed(elems [][]byte) []byte {
	var out []byte
	for _, e := range elems {
		out = append(out, e...)
	}
	return out
}

// sszListVariable encodes a list whose elements are variable-size: a section of
// uint32 little-endian offsets (relative to the start of the list) followed by
// the element data. An empty list encodes to no bytes.
func sszListVariable(elems [][]byte) []byte {
	head := make([]byte, 0, 4*len(elems))
	var heap []byte
	offset := 4 * len(elems)
	for _, e := range elems {
		var off [4]byte
		binary.LittleEndian.PutUint32(off[:], uint32(offset)) //nolint:gosec // offsets are bounded by payload size
		head = append(head, off[:]...)
		heap = append(heap, e...)
		offset += len(e)
	}
	return append(head, heap...)
}

func sszUint64(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

// sszUint256 encodes a non-negative big.Int as 32-byte little-endian.
func sszUint256(v *big.Int) ([]byte, error) {
	if v.Sign() < 0 {
		return nil, fmt.Errorf("uint256 must be non-negative")
	}
	if v.BitLen() > 256 {
		return nil, fmt.Errorf("uint256 overflow (%d bits)", v.BitLen())
	}
	var be [32]byte
	v.FillBytes(be[:]) // big-endian, left-padded
	le := make([]byte, 32)
	for i := range 32 {
		le[i] = be[31-i]
	}
	return le, nil
}

// hex helpers

func hexToBytes(s string) ([]byte, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	b, err := hex.DecodeString(t)
	if err != nil {
		return nil, fmt.Errorf("invalid hex %q: %w", s, err)
	}
	return b, nil
}

// hexToFixed decodes a hex string and requires it to be exactly n bytes.
func hexToFixed(s string, n int) ([]byte, error) {
	b, err := hexToBytes(s)
	if err != nil {
		return nil, err
	}
	if len(b) != n {
		return nil, fmt.Errorf("expected %d bytes, got %d (%q)", n, len(b), s)
	}
	return b, nil
}

func checkListLen(name string, got, max int) error {
	if got > max {
		return fmt.Errorf("%s: expected <= %d items, got %d", name, max, got)
	}
	return nil
}

func checkByteLen(name string, got, max int) error {
	if got > max {
		return fmt.Errorf("%s: expected <= %d bytes, got %d", name, max, got)
	}
	return nil
}

// parseUint256Hex parses a 0x-prefixed hex quantity (matching the Python
// reference's int(value, 16) for base_fee_per_gas).
func parseUint256Hex(s string) (*big.Int, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if t == "" {
		return nil, fmt.Errorf("empty hex quantity")
	}
	v, ok := new(big.Int).SetString(t, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex quantity %q", s)
	}
	return v, nil
}

// JSON object -> SSZ bytes. These functions mirror
// rollup_spec/stateless_input.py's _ssz_*_from_obj helpers.

func sszWithdrawalFromObj(obj jsonObj) ([]byte, error) {
	index, err := requireUint64(obj, "index")
	if err != nil {
		return nil, err
	}
	validatorIndex, err := requireUint64(obj, "validatorIndex")
	if err != nil {
		return nil, err
	}
	address, err := requireString(obj, "address")
	if err != nil {
		return nil, err
	}
	amount, err := requireUint64(obj, "amount")
	if err != nil {
		return nil, err
	}

	addr, err := hexToFixed(address, 20)
	if err != nil {
		return nil, fmt.Errorf("withdrawal address: %w", err)
	}
	// All fields fixed-size: index | validator_index | address | amount.
	return sszContainer(
		fixed(sszUint64(index)),
		fixed(sszUint64(validatorIndex)),
		fixed(addr),
		fixed(sszUint64(amount)),
	), nil
}

func sszExecutionPayloadFromObj(obj jsonObj) ([]byte, error) {
	parentHashValue, err := requireString(obj, "parentHash")
	if err != nil {
		return nil, err
	}
	parentHash, err := hexToFixed(parentHashValue, 32)
	if err != nil {
		return nil, fmt.Errorf("parentHash: %w", err)
	}
	feeRecipientValue, err := requireString(obj, "feeRecipient")
	if err != nil {
		return nil, err
	}
	feeRecipient, err := hexToFixed(feeRecipientValue, 20)
	if err != nil {
		return nil, fmt.Errorf("feeRecipient: %w", err)
	}
	stateRootValue, err := requireString(obj, "stateRoot")
	if err != nil {
		return nil, err
	}
	stateRoot, err := hexToFixed(stateRootValue, 32)
	if err != nil {
		return nil, fmt.Errorf("stateRoot: %w", err)
	}
	receiptsRootValue, err := requireString(obj, "receiptsRoot")
	if err != nil {
		return nil, err
	}
	receiptsRoot, err := hexToFixed(receiptsRootValue, 32)
	if err != nil {
		return nil, fmt.Errorf("receiptsRoot: %w", err)
	}
	logsBloomValue, err := requireString(obj, "logsBloom")
	if err != nil {
		return nil, err
	}
	logsBloom, err := hexToFixed(logsBloomValue, 256)
	if err != nil {
		return nil, fmt.Errorf("logsBloom: %w", err)
	}
	prevRandaoValue, err := requireString(obj, "prevRandao")
	if err != nil {
		return nil, err
	}
	prevRandao, err := hexToFixed(prevRandaoValue, 32)
	if err != nil {
		return nil, fmt.Errorf("prevRandao: %w", err)
	}
	blockNumber, err := requireUint64(obj, "blockNumber")
	if err != nil {
		return nil, err
	}
	gasLimit, err := requireUint64(obj, "gasLimit")
	if err != nil {
		return nil, err
	}
	gasUsed, err := requireUint64(obj, "gasUsed")
	if err != nil {
		return nil, err
	}
	timestamp, err := requireUint64(obj, "timestamp")
	if err != nil {
		return nil, err
	}
	extraDataValue, err := requireString(obj, "extraData")
	if err != nil {
		return nil, err
	}
	extraData, err := hexToBytes(extraDataValue)
	if err != nil {
		return nil, fmt.Errorf("extraData: %w", err)
	}
	if len(extraData) > maxExtraDataBytes {
		return nil, fmt.Errorf("extraData: expected <= %d bytes, got %d", maxExtraDataBytes, len(extraData))
	}
	baseFeePerGas, err := requireString(obj, "baseFeePerGas")
	if err != nil {
		return nil, err
	}
	baseFee, err := parseUint256Hex(baseFeePerGas)
	if err != nil {
		return nil, fmt.Errorf("baseFeePerGas: %w", err)
	}
	baseFeeBytes, err := sszUint256(baseFee)
	if err != nil {
		return nil, fmt.Errorf("baseFeePerGas: %w", err)
	}
	blockHashValue, err := requireString(obj, "blockHash")
	if err != nil {
		return nil, err
	}
	blockHash, err := hexToFixed(blockHashValue, 32)
	if err != nil {
		return nil, fmt.Errorf("blockHash: %w", err)
	}

	transactionValues, err := requireStringList(obj, "transactions")
	if err != nil {
		return nil, err
	}
	if err := checkListLen("transactions", len(transactionValues), maxTransactionsPerPayload); err != nil {
		return nil, err
	}
	txs := make([][]byte, len(transactionValues))
	for i, tx := range transactionValues {
		b, err := hexToBytes(tx)
		if err != nil {
			return nil, fmt.Errorf("transactions[%d]: %w", i, err)
		}
		if err := checkByteLen(fmt.Sprintf("transactions[%d]", i), len(b), maxBytesPerTransaction); err != nil {
			return nil, err
		}
		txs[i] = b
	}

	withdrawalObjs, err := requireObjectList(obj, "withdrawals")
	if err != nil {
		return nil, err
	}
	if err := checkListLen("withdrawals", len(withdrawalObjs), maxWithdrawalsPerPayload); err != nil {
		return nil, err
	}
	withdrawals := make([][]byte, len(withdrawalObjs))
	for i, withdrawalObj := range withdrawalObjs {
		b, err := sszWithdrawalFromObj(withdrawalObj)
		if err != nil {
			return nil, fmt.Errorf("withdrawals[%d]: %w", i, err)
		}
		withdrawals[i] = b
	}

	blobGasUsed, err := requireUint64(obj, "blobGasUsed")
	if err != nil {
		return nil, err
	}
	excessBlobGas, err := requireUint64(obj, "excessBlobGas")
	if err != nil {
		return nil, err
	}
	blockAccessListValue, err := requireString(obj, "blockAccessList")
	if err != nil {
		return nil, err
	}
	blockAccessList, err := hexToBytes(blockAccessListValue)
	if err != nil {
		return nil, fmt.Errorf("blockAccessList: %w", err)
	}
	if err := checkByteLen("blockAccessList", len(blockAccessList), maxBytesPerTransaction); err != nil {
		return nil, err
	}
	slotNumber, err := optionalUint64(obj, "slotNumber", 0)
	if err != nil {
		return nil, err
	}

	// Field order mirrors canonical_ssz.ExecutionPayload plus the two Amsterdam
	// fields (block_access_list, slot_number) appended by SszExecutionPayload.
	return sszContainer(
		fixed(parentHash),
		fixed(feeRecipient),
		fixed(stateRoot),
		fixed(receiptsRoot),
		fixed(logsBloom),
		fixed(prevRandao),
		fixed(sszUint64(blockNumber)),
		fixed(sszUint64(gasLimit)),
		fixed(sszUint64(gasUsed)),
		fixed(sszUint64(timestamp)),
		variable(extraData),
		fixed(baseFeeBytes),
		fixed(blockHash),
		variable(sszListVariable(txs)),
		variable(sszListFixed(withdrawals)),
		fixed(sszUint64(blobGasUsed)),
		fixed(sszUint64(excessBlobGas)),
		variable(blockAccessList),
		fixed(sszUint64(slotNumber)),
	), nil
}

func sszExecutionRequestsFromObj(obj jsonObj) ([]byte, error) {
	for _, key := range []string{"deposits", "withdrawals", "consolidations"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("executionRequests.%s: expected array: %w", key, err)
		}
		if len(items) != 0 {
			return nil, fmt.Errorf("executionRequests.%s must be empty", key)
		}
	}
	// Three empty variable-size lists: deposits | withdrawals | consolidations.
	return sszContainer(
		variable(nil),
		variable(nil),
		variable(nil),
	), nil
}

func sszNewPayloadRequestFromObj(obj jsonObj) ([]byte, error) {
	executionPayloadObj, err := requireObject(obj, "executionPayload")
	if err != nil {
		return nil, err
	}
	ep, err := sszExecutionPayloadFromObj(executionPayloadObj)
	if err != nil {
		return nil, fmt.Errorf("executionPayload: %w", err)
	}

	versionedHashesValues, err := requireStringList(obj, "versionedHashes")
	if err != nil {
		return nil, err
	}
	if err := checkListLen("versionedHashes", len(versionedHashesValues), maxBlobCommitmentsPerBlock); err != nil {
		return nil, err
	}
	versionedHashes := make([][]byte, len(versionedHashesValues))
	for i, h := range versionedHashesValues {
		b, err := hexToFixed(h, 32)
		if err != nil {
			return nil, fmt.Errorf("versionedHashes[%d]: %w", i, err)
		}
		versionedHashes[i] = b
	}

	parentBeaconBlockRoot, err := requireString(obj, "parentBeaconBlockRoot")
	if err != nil {
		return nil, err
	}
	parentBeacon, err := hexToFixed(parentBeaconBlockRoot, 32)
	if err != nil {
		return nil, fmt.Errorf("parentBeaconBlockRoot: %w", err)
	}

	executionRequestsObj, err := requireObject(obj, "executionRequests")
	if err != nil {
		return nil, err
	}
	execRequests, err := sszExecutionRequestsFromObj(executionRequestsObj)
	if err != nil {
		return nil, err
	}

	return sszContainer(
		variable(ep),
		variable(sszListFixed(versionedHashes)),
		fixed(parentBeacon),
		variable(execRequests),
	), nil
}

func sszExecutionWitnessFromObj(obj jsonObj) ([]byte, error) {
	encodeList := func(name string, maxItems, maxBytes int) ([]byte, error) {
		items, err := requireStringList(obj, name)
		if err != nil {
			return nil, err
		}
		if err := checkListLen(name, len(items), maxItems); err != nil {
			return nil, err
		}
		elems := make([][]byte, len(items))
		for i, s := range items {
			b, err := hexToBytes(s)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
			}
			if err := checkByteLen(fmt.Sprintf("%s[%d]", name, i), len(b), maxBytes); err != nil {
				return nil, err
			}
			elems[i] = b
		}
		return sszListVariable(elems), nil
	}

	state, err := encodeList("state", maxWitnessNodes, maxBytesPerWitnessNode)
	if err != nil {
		return nil, err
	}
	codes, err := encodeList("codes", maxWitnessCodes, maxBytesPerCode)
	if err != nil {
		return nil, err
	}
	headers, err := encodeList("headers", maxWitnessHeaders, maxBytesPerHeader)
	if err != nil {
		return nil, err
	}

	return sszContainer(
		variable(state),
		variable(codes),
		variable(headers),
	), nil
}

// forkIndex resolves a fork name to its SSZ active_fork index, mirroring
// rollup_spec/fork.py: the name must be a known ProtocolFork and must be the
// single fork this backend supports (Amsterdam).
func forkIndex(name string) (uint64, error) {
	idx := -1
	for i, f := range protocolForks {
		if f == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, fmt.Errorf("unknown fork name %q", name)
	}
	if name != activeFork {
		return 0, fmt.Errorf("unsupported fork %q: this backend supports only %s", name, activeFork)
	}
	return uint64(idx), nil //nolint:gosec // idx is a small non-negative index
}

func sszChainConfigFromObj(obj jsonObj) ([]byte, error) {
	chainID, err := requireUint64(obj, "chainId")
	if err != nil {
		return nil, err
	}
	forkName, err := requireString(obj, "forkName")
	if err != nil {
		return nil, err
	}
	idx, err := forkIndex(forkName)
	if err != nil {
		return nil, err
	}

	// SszForkActivation: two empty optional (max-length-1) uint64 lists.
	activation := sszContainer(
		variable(nil), // block_number
		variable(nil), // timestamp
	)

	// SszForkConfig: fork index | activation | blob_schedule (empty list).
	forkConfig := sszContainer(
		fixed(sszUint64(idx)),
		variable(activation),
		variable(nil), // blob_schedule: empty List[SszBlobSchedule, 1]
	)

	// SszChainConfig: chain_id | active_fork.
	return sszContainer(
		fixed(sszUint64(chainID)),
		variable(forkConfig),
	), nil
}

// recoverPublicKey recovers the 65-byte uncompressed SEC1 public key
// (0x04 || x || y) for a signed transaction, mirroring
// rollup_spec/fork.py::recover_transaction_public_key. It is the SSZ
// public_keys field, derived from the transactions already in the payload.
func recoverPublicKey(txBytes []byte, chainID *big.Int) ([]byte, error) {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return nil, fmt.Errorf("decoding transaction: %w", err)
	}

	// Signing-hash selection mirrors the reference
	// (_signature_recovery_parameters): a pre-EIP-155 legacy transaction signs
	// over the Homestead hash; an EIP-155 legacy transaction signs over the
	// payload chain id (its v is validated against it in recoveryID); a typed
	// transaction signs over its OWN embedded chain id, which the reference
	// does not check against the payload chain id.
	var signer types.Signer
	switch {
	case tx.Type() == types.LegacyTxType && !tx.Protected():
		signer = types.HomesteadSigner{}
	case tx.Type() == types.LegacyTxType:
		if chainID.Sign() <= 0 {
			return nil, fmt.Errorf("invalid payload chain id %s", chainID)
		}
		signer = types.LatestSignerForChainID(chainID)
	default:
		txChainID := tx.ChainId()
		if txChainID.Sign() <= 0 {
			return nil, fmt.Errorf("invalid transaction chain id %s", txChainID)
		}
		signer = types.LatestSignerForChainID(txChainID)
	}

	v, r, s := tx.RawSignatureValues()
	recID, err := recoveryID(tx, v, chainID)
	if err != nil {
		return nil, err
	}
	// The reference rejects r outside [1, N) and s outside [1, N/2] for every
	// transaction type; Ecrecover alone does not enforce the s upper bound.
	if !crypto.ValidateSignatureValues(recID, r, s, true) {
		return nil, fmt.Errorf("invalid signature values (r or s out of range)")
	}

	sig := make([]byte, 65)
	r.FillBytes(sig[0:32])
	s.FillBytes(sig[32:64])
	sig[64] = recID

	hash := signer.Hash(tx)
	pub, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		return nil, fmt.Errorf("recovering public key: %w", err)
	}
	if len(pub) != 65 || pub[0] != 0x04 {
		return nil, fmt.Errorf("recovered key is not a 65-byte uncompressed SEC1 key")
	}
	return pub, nil
}

// recoveryID normalizes a transaction's v value to the 0/1 secp256k1 recovery
// id crypto.Ecrecover expects.
func recoveryID(tx *types.Transaction, v, chainID *big.Int) (byte, error) {
	if tx.Type() == types.LegacyTxType {
		if tx.Protected() {
			// EIP-155: v = 35 + 2*chainId + recid.
			rec := new(big.Int).Sub(v, big.NewInt(35))
			rec.Sub(rec, new(big.Int).Mul(chainID, big.NewInt(2)))
			return smallRecID(rec)
		}
		// Pre-EIP-155: v = 27 + recid.
		return smallRecID(new(big.Int).Sub(v, big.NewInt(27)))
	}
	// Typed transactions carry v directly as the y-parity (0 or 1).
	return smallRecID(v)
}

func smallRecID(rec *big.Int) (byte, error) {
	if rec.Sign() < 0 || rec.Cmp(big.NewInt(1)) > 0 {
		return 0, fmt.Errorf("invalid signature recovery id %s", rec)
	}
	return byte(rec.Uint64()), nil
}

func recoverPublicKeys(transactions []string, chainID uint64) ([][]byte, error) {
	if err := checkListLen("public_keys", len(transactions), maxPublicKeys); err != nil {
		return nil, err
	}

	keys := make([][]byte, len(transactions))
	payloadChainID := new(big.Int).SetUint64(chainID)
	for i, tx := range transactions {
		raw, err := hexToBytes(tx)
		if err != nil {
			return nil, fmt.Errorf("transactions[%d]: %w", i, err)
		}
		key, err := recoverPublicKey(raw, payloadChainID)
		if err != nil {
			return nil, fmt.Errorf("transactions[%d]: %w", i, err)
		}
		keys[i] = key
	}
	return keys, nil
}

func sszStatelessInputFromObj(obj jsonObj) ([]byte, error) {
	nprObj, err := requireObject(obj, "newPayloadRequest")
	if err != nil {
		return nil, err
	}
	npr, err := sszNewPayloadRequestFromObj(nprObj)
	if err != nil {
		return nil, fmt.Errorf("newPayloadRequest: %w", err)
	}

	witnessObj, err := requireObject(obj, "executionWitness")
	if err != nil {
		return nil, err
	}
	witness, err := sszExecutionWitnessFromObj(witnessObj)
	if err != nil {
		return nil, fmt.Errorf("executionWitness: %w", err)
	}

	chainConfigObj, err := requireObject(obj, "chainConfig")
	if err != nil {
		return nil, err
	}
	chainConfig, err := sszChainConfigFromObj(chainConfigObj)
	if err != nil {
		return nil, fmt.Errorf("chainConfig: %w", err)
	}

	// public_keys are recovered from the payload transactions (the readable
	// request does not carry them).
	executionPayloadObj, err := requireObject(nprObj, "executionPayload")
	if err != nil {
		return nil, fmt.Errorf("newPayloadRequest: %w", err)
	}
	txs, err := requireStringList(executionPayloadObj, "transactions")
	if err != nil {
		return nil, fmt.Errorf("public_keys: %w", err)
	}
	chainID, err := requireUint64(chainConfigObj, "chainId")
	if err != nil {
		return nil, fmt.Errorf("public_keys: %w", err)
	}
	keys, err := recoverPublicKeys(txs, chainID)
	if err != nil {
		return nil, fmt.Errorf("public_keys: %w", err)
	}

	return sszContainer(
		variable(npr),
		variable(witness),
		variable(chainConfig),
		variable(sszListFixed(keys)),
	), nil
}

// EncodeStatelessInput SSZ-encodes the coordinator's per-block payload into the
// byte slice the guest reads at _in_start: the two-byte 0x0001 schema id
// followed by the SSZ SszStatelessInput. The [u64 LE len] frame is added later
// by [buildZkcInputs]; this returns the framed SSZ only.
//
// The input is the readable encoder_obj form produced by
// proof_io_v1.py::_decode_payload. Byte-for-byte compatibility with the Python
// reference encoder is pinned by testdata/stateless_input_payload0.ssz.
func EncodeStatelessInput(payload []byte) ([]byte, error) {
	obj, err := parseJSONObject(payload, "statelessInput")
	if err != nil {
		return nil, fmt.Errorf("EncodeStatelessInput: %w", err)
	}

	raw, err := sszStatelessInputFromObj(obj)
	if err != nil {
		return nil, fmt.Errorf("EncodeStatelessInput: %w", err)
	}

	framed := make([]byte, 0, len(statelessInputSchemaID)+len(raw))
	framed = append(framed, statelessInputSchemaID...)
	framed = append(framed, raw...)
	return framed, nil
}
