package backend

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
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

// JSON input model (readable encoder_obj form)

// statelessInputJSON's sections are pointers so an absent section is reported
// as a clear error rather than silently encoded at zero values, matching the
// reference encoder's strictness.
type statelessInputJSON struct {
	NewPayloadRequest *newPayloadRequestJSON `json:"newPayloadRequest"`
	ExecutionWitness  *executionWitnessJSON  `json:"executionWitness"`
	ChainConfig       *chainConfigJSON       `json:"chainConfig"`
}

type newPayloadRequestJSON struct {
	ExecutionPayload      executionPayloadJSON   `json:"executionPayload"`
	VersionedHashes       []string               `json:"versionedHashes"`
	ParentBeaconBlockRoot string                 `json:"parentBeaconBlockRoot"`
	ExecutionRequests     *executionRequestsJSON `json:"executionRequests"`
}

type executionPayloadJSON struct {
	ParentHash      string           `json:"parentHash"`
	FeeRecipient    string           `json:"feeRecipient"`
	StateRoot       string           `json:"stateRoot"`
	ReceiptsRoot    string           `json:"receiptsRoot"`
	LogsBloom       string           `json:"logsBloom"`
	PrevRandao      string           `json:"prevRandao"`
	BlockNumber     uint64           `json:"blockNumber"`
	GasLimit        uint64           `json:"gasLimit"`
	GasUsed         uint64           `json:"gasUsed"`
	Timestamp       uint64           `json:"timestamp"`
	ExtraData       string           `json:"extraData"`
	BaseFeePerGas   string           `json:"baseFeePerGas"`
	BlockHash       string           `json:"blockHash"`
	Transactions    []string         `json:"transactions"`
	Withdrawals     []withdrawalJSON `json:"withdrawals"`
	BlobGasUsed     uint64           `json:"blobGasUsed"`
	ExcessBlobGas   uint64           `json:"excessBlobGas"`
	BlockAccessList string           `json:"blockAccessList"`
	// SlotNumber is absent from the readable payload; canonical default 0.
	SlotNumber uint64 `json:"slotNumber"`
}

type withdrawalJSON struct {
	Index          uint64 `json:"index"`
	ValidatorIndex uint64 `json:"validatorIndex"`
	Address        string `json:"address"`
	Amount         uint64 `json:"amount"`
}

// executionRequestsJSON models the object form the encoder expects. The rollup
// rejects EIP-7685 requests (rollup_spec §2.1): all three lists must be empty.
type executionRequestsJSON struct {
	Deposits       []json.RawMessage `json:"deposits"`
	Withdrawals    []json.RawMessage `json:"withdrawals"`
	Consolidations []json.RawMessage `json:"consolidations"`
}

type executionWitnessJSON struct {
	State   []string `json:"state"`
	Codes   []string `json:"codes"`
	Headers []string `json:"headers"`
}

type chainConfigJSON struct {
	ChainID  uint64 `json:"chainId"`
	ForkName string `json:"forkName"`
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

// container encoders

func encodeWithdrawal(w withdrawalJSON) ([]byte, error) {
	addr, err := hexToFixed(w.Address, 20)
	if err != nil {
		return nil, fmt.Errorf("withdrawal address: %w", err)
	}
	// All fields fixed-size: index | validator_index | address | amount.
	return sszContainer(
		fixed(sszUint64(w.Index)),
		fixed(sszUint64(w.ValidatorIndex)),
		fixed(addr),
		fixed(sszUint64(w.Amount)),
	), nil
}

func encodeExecutionPayload(ep executionPayloadJSON) ([]byte, error) {
	parentHash, err := hexToFixed(ep.ParentHash, 32)
	if err != nil {
		return nil, fmt.Errorf("parentHash: %w", err)
	}
	feeRecipient, err := hexToFixed(ep.FeeRecipient, 20)
	if err != nil {
		return nil, fmt.Errorf("feeRecipient: %w", err)
	}
	stateRoot, err := hexToFixed(ep.StateRoot, 32)
	if err != nil {
		return nil, fmt.Errorf("stateRoot: %w", err)
	}
	receiptsRoot, err := hexToFixed(ep.ReceiptsRoot, 32)
	if err != nil {
		return nil, fmt.Errorf("receiptsRoot: %w", err)
	}
	logsBloom, err := hexToFixed(ep.LogsBloom, 256)
	if err != nil {
		return nil, fmt.Errorf("logsBloom: %w", err)
	}
	prevRandao, err := hexToFixed(ep.PrevRandao, 32)
	if err != nil {
		return nil, fmt.Errorf("prevRandao: %w", err)
	}
	extraData, err := hexToBytes(ep.ExtraData)
	if err != nil {
		return nil, fmt.Errorf("extraData: %w", err)
	}
	if len(extraData) > maxExtraDataBytes {
		return nil, fmt.Errorf("extraData: expected <= %d bytes, got %d", maxExtraDataBytes, len(extraData))
	}
	baseFee, err := parseUint256Hex(ep.BaseFeePerGas)
	if err != nil {
		return nil, fmt.Errorf("baseFeePerGas: %w", err)
	}
	baseFeeBytes, err := sszUint256(baseFee)
	if err != nil {
		return nil, fmt.Errorf("baseFeePerGas: %w", err)
	}
	blockHash, err := hexToFixed(ep.BlockHash, 32)
	if err != nil {
		return nil, fmt.Errorf("blockHash: %w", err)
	}

	txs := make([][]byte, len(ep.Transactions))
	for i, tx := range ep.Transactions {
		b, err := hexToBytes(tx)
		if err != nil {
			return nil, fmt.Errorf("transactions[%d]: %w", i, err)
		}
		txs[i] = b
	}

	withdrawals := make([][]byte, len(ep.Withdrawals))
	for i, w := range ep.Withdrawals {
		b, err := encodeWithdrawal(w)
		if err != nil {
			return nil, fmt.Errorf("withdrawals[%d]: %w", i, err)
		}
		withdrawals[i] = b
	}

	blockAccessList, err := hexToBytes(ep.BlockAccessList)
	if err != nil {
		return nil, fmt.Errorf("blockAccessList: %w", err)
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
		fixed(sszUint64(ep.BlockNumber)),
		fixed(sszUint64(ep.GasLimit)),
		fixed(sszUint64(ep.GasUsed)),
		fixed(sszUint64(ep.Timestamp)),
		variable(extraData),
		fixed(baseFeeBytes),
		fixed(blockHash),
		variable(sszListVariable(txs)),
		variable(sszListFixed(withdrawals)),
		fixed(sszUint64(ep.BlobGasUsed)),
		fixed(sszUint64(ep.ExcessBlobGas)),
		variable(blockAccessList),
		fixed(sszUint64(ep.SlotNumber)),
	), nil
}

// encodeExecutionRequests encodes an ExecutionRequests container. The rollup
// requires all three request lists empty; a non-empty list is rejected, and so
// is an absent executionRequests object (the reference encoder requires it).
func encodeExecutionRequests(er *executionRequestsJSON) ([]byte, error) {
	if er == nil {
		return nil, fmt.Errorf("missing executionRequests")
	}
	if len(er.Deposits) != 0 {
		return nil, fmt.Errorf("executionRequests.deposits must be empty")
	}
	if len(er.Withdrawals) != 0 {
		return nil, fmt.Errorf("executionRequests.withdrawals must be empty")
	}
	if len(er.Consolidations) != 0 {
		return nil, fmt.Errorf("executionRequests.consolidations must be empty")
	}
	// Three empty variable-size lists: deposits | withdrawals | consolidations.
	return sszContainer(
		variable(nil),
		variable(nil),
		variable(nil),
	), nil
}

func encodeNewPayloadRequest(npr newPayloadRequestJSON) ([]byte, error) {
	ep, err := encodeExecutionPayload(npr.ExecutionPayload)
	if err != nil {
		return nil, fmt.Errorf("executionPayload: %w", err)
	}

	versionedHashes := make([][]byte, len(npr.VersionedHashes))
	for i, h := range npr.VersionedHashes {
		b, err := hexToFixed(h, 32)
		if err != nil {
			return nil, fmt.Errorf("versionedHashes[%d]: %w", i, err)
		}
		versionedHashes[i] = b
	}

	parentBeacon, err := hexToFixed(npr.ParentBeaconBlockRoot, 32)
	if err != nil {
		return nil, fmt.Errorf("parentBeaconBlockRoot: %w", err)
	}

	execRequests, err := encodeExecutionRequests(npr.ExecutionRequests)
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

func encodeExecutionWitness(w executionWitnessJSON) ([]byte, error) {
	encodeList := func(name string, items []string) ([]byte, error) {
		elems := make([][]byte, len(items))
		for i, s := range items {
			b, err := hexToBytes(s)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
			}
			elems[i] = b
		}
		return sszListVariable(elems), nil
	}

	state, err := encodeList("state", w.State)
	if err != nil {
		return nil, err
	}
	codes, err := encodeList("codes", w.Codes)
	if err != nil {
		return nil, err
	}
	headers, err := encodeList("headers", w.Headers)
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

func encodeChainConfig(cc chainConfigJSON) ([]byte, error) {
	idx, err := forkIndex(cc.ForkName)
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
		fixed(sszUint64(cc.ChainID)),
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

func encodeStatelessInput(in *statelessInputJSON) ([]byte, error) {
	npr, err := encodeNewPayloadRequest(*in.NewPayloadRequest)
	if err != nil {
		return nil, fmt.Errorf("newPayloadRequest: %w", err)
	}

	witness, err := encodeExecutionWitness(*in.ExecutionWitness)
	if err != nil {
		return nil, fmt.Errorf("executionWitness: %w", err)
	}

	chainConfig, err := encodeChainConfig(*in.ChainConfig)
	if err != nil {
		return nil, fmt.Errorf("chainConfig: %w", err)
	}

	// public_keys are recovered from the payload transactions (the readable
	// request does not carry them).
	chainID := new(big.Int).SetUint64(in.ChainConfig.ChainID)
	txs := in.NewPayloadRequest.ExecutionPayload.Transactions
	keys := make([][]byte, len(txs))
	for i, tx := range txs {
		raw, err := hexToBytes(tx)
		if err != nil {
			return nil, fmt.Errorf("public_keys: transactions[%d]: %w", i, err)
		}
		key, err := recoverPublicKey(raw, chainID)
		if err != nil {
			return nil, fmt.Errorf("public_keys: transactions[%d]: %w", i, err)
		}
		keys[i] = key
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
	var in statelessInputJSON
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, fmt.Errorf("EncodeStatelessInput: parsing JSON: %w", err)
	}
	if in.NewPayloadRequest == nil {
		return nil, fmt.Errorf("EncodeStatelessInput: missing newPayloadRequest")
	}
	if in.ExecutionWitness == nil {
		return nil, fmt.Errorf("EncodeStatelessInput: missing executionWitness")
	}
	if in.ChainConfig == nil {
		return nil, fmt.Errorf("EncodeStatelessInput: missing chainConfig")
	}

	raw, err := encodeStatelessInput(&in)
	if err != nil {
		return nil, fmt.Errorf("EncodeStatelessInput: %w", err)
	}

	framed := make([]byte, 0, len(statelessInputSchemaID)+len(raw))
	framed = append(framed, statelessInputSchemaID...)
	framed = append(framed, raw...)
	return framed, nil
}
