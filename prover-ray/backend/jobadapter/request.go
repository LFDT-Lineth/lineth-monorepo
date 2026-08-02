package jobadapter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/ssz"
)

const (
	guestProgramIDKey = "guestProgramId"
	chainIDKey        = "chainId"
	forkNameKey       = "forkName"
	payloadsKey       = "payloads"

	proofRequestKey                 = "proofRequest"
	chainConfigKey                  = "chainConfig"
	l2MessageServiceAddressKey      = "l2MessageServiceAddress"
	coinbaseKey                     = "coinbase"
	parentFtxRollingHashKey         = "parentFtxRollingHash"
	parentLastProcessedFtxNumberKey = "parentLastProcessedFtxNumber"
	statelessInputKey               = "statelessInput"
	newPayloadRequestKey            = "newPayloadRequest"
	executionPayloadKey             = "executionPayload"
	executionRequestsKey            = "executionRequests"
	rollupExtensionKey              = "rollupExtension"
	forcedTransactionsKey           = "forcedTransactions"
	blockNumberKey                  = "blockNumber"
	numberKey                       = "number"
	deadlineKey                     = "deadline"
	signedTxRlpKey                  = "signedTxRlp"
	acceptanceKey                   = "acceptance"
	guestProgramIDByteSize          = 32
	hashByteSize                    = 32
	addressByteSize                 = 20

	forcedTxIncluded            = "INCLUDED"
	forcedTxBadNonce            = "BAD_NONCE"
	forcedTxBadBalance          = "BAD_BALANCE"
	forcedTxFilteredAddressFrom = "FILTERED_ADDRESS_FROM"
	forcedTxFilteredAddressTo   = "FILTERED_ADDRESS_TO"
)

var validForcedTransactionAcceptances = map[string]struct{}{
	forcedTxIncluded:            {},
	forcedTxBadNonce:            {},
	forcedTxBadBalance:          {},
	forcedTxFilteredAddressFrom: {},
	forcedTxFilteredAddressTo:   {},
}

// L2ExecutionPayload is one block's worth of a decoded L2 execution request:
// the framed SSZ the guest reads (the output of [ssz.EncodeStatelessInput]) and
// the block number that payload proves.
type L2ExecutionPayload struct {
	BlockNumber        uint64
	FramedSSZ          []byte
	ForcedTransactions []json.RawMessage
}

// L2ExecutionRequest is a decoded getZkL2ExecutionProofV1 request: routing
// metadata, the range-level chain identity, and one [L2ExecutionPayload] per
// block. The block range is implied by the payloads (their
// executionPayload.blockNumber), as in the reference decoder.
type L2ExecutionRequest struct {
	// GuestProgramID is routing metadata; this decoder validates its shape but
	// does not verify it against the configured guest ELF (open question #6 in
	// wiki backend-overview.md).
	GuestProgramID []byte
	ChainID        uint64
	ForkName       string
	Payloads       []L2ExecutionPayload
}

// DecodeL2ExecutionRequest parses a getZkL2ExecutionProofV1 request body and
// SSZ-encodes each payload's statelessInput, porting
// proof_io_v1.py::decode_request and _decode_payload: it injects {chainId,
// forkName} into each payload's statelessInput and converts the
// already-validated empty executionRequests list into the empty container shape
// expected by [ssz.EncodeStatelessInput]. It also validates and preserves
// rollupExtension.forcedTransactions for the Runner capability check.
func DecodeL2ExecutionRequest(data []byte) (*L2ExecutionRequest, error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: parsing JSON: %w", err)
	}

	gpidRaw, err := requireField(env, guestProgramIDKey, "")
	if err != nil {
		return nil, err
	}
	guestProgramID, err := hexString(gpidRaw, guestProgramIDKey)
	if err != nil {
		return nil, err
	}
	if len(guestProgramID) != guestProgramIDByteSize {
		return nil, fmt.Errorf(
			"DecodeL2ExecutionRequest: %s must be %d bytes, got %d",
			guestProgramIDKey,
			guestProgramIDByteSize,
			len(guestProgramID),
		)
	}

	prRaw, err := requireField(env, proofRequestKey, "")
	if err != nil {
		return nil, err
	}
	proofRequest, err := object(prRaw, proofRequestKey)
	if err != nil {
		return nil, err
	}

	ccRaw, err := requireField(proofRequest, chainConfigKey, "proofRequest.")
	if err != nil {
		return nil, err
	}
	chainConfig, err := decodeChainConfig(ccRaw)
	if err != nil {
		return nil, err
	}

	if err := validateFixedHexField(proofRequest, parentFtxRollingHashKey, "proofRequest.", hashByteSize); err != nil {
		return nil, err
	}
	parentLastProcessedFtxNumberRaw, err := requireField(
		proofRequest,
		parentLastProcessedFtxNumberKey,
		"proofRequest.",
	)
	if err != nil {
		return nil, err
	}
	if _, err := u64(parentLastProcessedFtxNumberRaw, "proofRequest."+parentLastProcessedFtxNumberKey); err != nil {
		return nil, err
	}

	payloadsRaw, err := requireField(proofRequest, payloadsKey, "proofRequest.")
	if err != nil {
		return nil, err
	}
	var payloadObjs []json.RawMessage
	if err := json.Unmarshal(payloadsRaw, &payloadObjs); err != nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: proofRequest.payloads must be an array: %w", err)
	}
	if payloadObjs == nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: proofRequest.payloads must be an array")
	}
	if len(payloadObjs) == 0 {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: proofRequest.payloads must be non-empty")
	}

	payloads := make([]L2ExecutionPayload, len(payloadObjs))
	for i, raw := range payloadObjs {
		p, err := decodeL2ExecutionPayload(raw, i, chainConfig.chainID, chainConfig.forkName)
		if err != nil {
			return nil, err
		}
		payloads[i] = p
	}

	return &L2ExecutionRequest{
		GuestProgramID: guestProgramID,
		ChainID:        chainConfig.chainID,
		ForkName:       chainConfig.forkName,
		Payloads:       payloads,
	}, nil
}

type decodedChainConfig struct {
	chainID  uint64
	forkName string
}

func decodeChainConfig(raw json.RawMessage) (decodedChainConfig, error) {
	chainConfig, err := object(raw, "proofRequest.chainConfig")
	if err != nil {
		return decodedChainConfig{}, err
	}
	if err := validateFixedHexField(
		chainConfig,
		l2MessageServiceAddressKey,
		"proofRequest.chainConfig.",
		addressByteSize,
	); err != nil {
		return decodedChainConfig{}, err
	}
	if err := validateFixedHexField(
		chainConfig,
		coinbaseKey,
		"proofRequest.chainConfig.",
		addressByteSize,
	); err != nil {
		return decodedChainConfig{}, err
	}

	chainIDRaw, err := requireField(chainConfig, chainIDKey, "proofRequest.chainConfig.")
	if err != nil {
		return decodedChainConfig{}, err
	}
	chainID, err := u64(chainIDRaw, "proofRequest.chainConfig.chainId")
	if err != nil {
		return decodedChainConfig{}, err
	}

	forkNameRaw, err := requireField(chainConfig, forkNameKey, "proofRequest.chainConfig.")
	if err != nil {
		return decodedChainConfig{}, err
	}
	var forkName string
	if err := json.Unmarshal(forkNameRaw, &forkName); err != nil {
		return decodedChainConfig{}, fmt.Errorf("DecodeL2ExecutionRequest: proofRequest.chainConfig.forkName: %w", err)
	}

	return decodedChainConfig{chainID: chainID, forkName: forkName}, nil
}

// decodeL2ExecutionPayload builds the encoder object for one payload: it injects
// chainConfig into statelessInput and converts the already-validated empty
// executionRequests list into the empty container shape expected by the SSZ
// encoder. Mirrors proof_io_v1.py::_decode_payload.
func decodeL2ExecutionPayload(
	raw json.RawMessage,
	index int,
	chainID uint64,
	forkName string,
) (L2ExecutionPayload, error) {
	ctx := fmt.Sprintf("proofRequest.payloads[%d].", index)

	payload, err := object(raw, strings.TrimSuffix(ctx, "."))
	if err != nil {
		return L2ExecutionPayload{}, err
	}

	siRaw, err := requireField(payload, statelessInputKey, ctx)
	if err != nil {
		return L2ExecutionPayload{}, err
	}
	statelessInput, err := object(siRaw, ctx+"statelessInput")
	if err != nil {
		return L2ExecutionPayload{}, err
	}

	nprRaw, err := requireField(statelessInput, newPayloadRequestKey, ctx+"statelessInput.")
	if err != nil {
		return L2ExecutionPayload{}, err
	}
	newPayloadRequest, err := object(nprRaw, ctx+"statelessInput.newPayloadRequest")
	if err != nil {
		return L2ExecutionPayload{}, err
	}

	if err := rejectNonEmptyExecutionRequests(newPayloadRequest, ctx); err != nil {
		return L2ExecutionPayload{}, err
	}

	blockNumber, err := payloadBlockNumber(newPayloadRequest, ctx)
	if err != nil {
		return L2ExecutionPayload{}, err
	}

	// Build the encoder_obj: executionRequests -> {}, chainConfig injected.
	newPayloadRequest[executionRequestsKey] = json.RawMessage("{}")
	nprEncoded, err := json.Marshal(newPayloadRequest)
	if err != nil {
		return L2ExecutionPayload{}, fmt.Errorf("DecodeL2ExecutionRequest: %sstatelessInput.newPayloadRequest: %w", ctx, err)
	}
	statelessInput[newPayloadRequestKey] = nprEncoded
	chainConfig, err := json.Marshal(map[string]any{chainIDKey: chainID, forkNameKey: forkName})
	if err != nil {
		return L2ExecutionPayload{}, fmt.Errorf("DecodeL2ExecutionRequest: %schainConfig: %w", ctx, err)
	}
	statelessInput[chainConfigKey] = chainConfig

	encoderObj, err := json.Marshal(statelessInput)
	if err != nil {
		return L2ExecutionPayload{}, fmt.Errorf("DecodeL2ExecutionRequest: %sstatelessInput: %w", ctx, err)
	}
	framedSSZ, err := ssz.EncodeStatelessInput(encoderObj)
	if err != nil {
		return L2ExecutionPayload{}, fmt.Errorf("DecodeL2ExecutionRequest: %sstatelessInput: %w", ctx, err)
	}
	forcedTransactions, err := payloadForcedTransactions(payload, ctx)
	if err != nil {
		return L2ExecutionPayload{}, err
	}

	return L2ExecutionPayload{
		BlockNumber:        blockNumber,
		FramedSSZ:          framedSSZ,
		ForcedTransactions: forcedTransactions,
	}, nil
}

// rejectNonEmptyExecutionRequests enforces that executionRequests is present as
// an array and empty (the rollup rejects EIP-7685 requests, rollup_spec §2.1).
func rejectNonEmptyExecutionRequests(newPayloadRequest map[string]json.RawMessage, ctx string) error {
	erRaw, err := requireField(newPayloadRequest, executionRequestsKey, ctx+"statelessInput.newPayloadRequest.")
	if err != nil {
		return err
	}
	var executionRequests []json.RawMessage
	if err := json.Unmarshal(erRaw, &executionRequests); err != nil {
		return fmt.Errorf("DecodeL2ExecutionRequest: %sexecutionRequests must be an array: %w", ctx, err)
	}
	if executionRequests == nil {
		return fmt.Errorf("DecodeL2ExecutionRequest: %sexecutionRequests must be an array", ctx)
	}
	if len(executionRequests) != 0 {
		return fmt.Errorf("DecodeL2ExecutionRequest: %sexecutionRequests must be empty", ctx)
	}
	return nil
}

func payloadForcedTransactions(payload map[string]json.RawMessage, ctx string) ([]json.RawMessage, error) {
	reRaw, err := requireField(payload, rollupExtensionKey, ctx)
	if err != nil {
		return nil, err
	}
	rollupExtension, err := object(reRaw, ctx+"rollupExtension")
	if err != nil {
		return nil, err
	}
	ftRaw, err := requireField(rollupExtension, forcedTransactionsKey, ctx+"rollupExtension.")
	if err != nil {
		return nil, err
	}
	var forcedTransactions []json.RawMessage
	if err := json.Unmarshal(ftRaw, &forcedTransactions); err != nil {
		return nil, fmt.Errorf(
			"DecodeL2ExecutionRequest: %srollupExtension.forcedTransactions must be an array: %w",
			ctx,
			err,
		)
	}
	if forcedTransactions == nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: %srollupExtension.forcedTransactions must be an array", ctx)
	}
	for i, raw := range forcedTransactions {
		itemCtx := fmt.Sprintf("%srollupExtension.forcedTransactions[%d].", ctx, i)
		if err := validateForcedTransaction(raw, itemCtx); err != nil {
			return nil, err
		}
	}
	return forcedTransactions, nil
}

func validateForcedTransaction(raw json.RawMessage, ctx string) error {
	forcedTransaction, err := object(raw, strings.TrimSuffix(ctx, "."))
	if err != nil {
		return err
	}

	numberRaw, err := requireField(forcedTransaction, numberKey, ctx)
	if err != nil {
		return err
	}
	if _, err := u64(numberRaw, ctx+numberKey); err != nil {
		return err
	}

	deadlineRaw, err := requireField(forcedTransaction, deadlineKey, ctx)
	if err != nil {
		return err
	}
	if _, err := u64(deadlineRaw, ctx+deadlineKey); err != nil {
		return err
	}

	signedTxRaw, err := requireField(forcedTransaction, signedTxRlpKey, ctx)
	if err != nil {
		return err
	}
	if _, err := hexString(signedTxRaw, ctx+signedTxRlpKey); err != nil {
		return err
	}

	acceptanceRaw, err := requireField(forcedTransaction, acceptanceKey, ctx)
	if err != nil {
		return err
	}
	var acceptance string
	if err := json.Unmarshal(acceptanceRaw, &acceptance); err != nil {
		return fmt.Errorf("DecodeL2ExecutionRequest: %s%s must be a string: %w", ctx, acceptanceKey, err)
	}
	if _, ok := validForcedTransactionAcceptances[acceptance]; !ok {
		return fmt.Errorf("DecodeL2ExecutionRequest: %s%s has unsupported value %q", ctx, acceptanceKey, acceptance)
	}
	return nil
}

func payloadBlockNumber(newPayloadRequest map[string]json.RawMessage, ctx string) (uint64, error) {
	epRaw, err := requireField(newPayloadRequest, executionPayloadKey, ctx+"statelessInput.newPayloadRequest.")
	if err != nil {
		return 0, err
	}
	executionPayload, err := object(epRaw, ctx+"statelessInput.newPayloadRequest.executionPayload")
	if err != nil {
		return 0, err
	}
	bnRaw, err := requireField(executionPayload, blockNumberKey, ctx+"statelessInput.newPayloadRequest.executionPayload.")
	if err != nil {
		return 0, err
	}
	return u64(bnRaw, ctx+"statelessInput.newPayloadRequest.executionPayload.blockNumber")
}

// small JSON helpers

func requireField(m map[string]json.RawMessage, key, ctx string) (json.RawMessage, error) {
	v, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: missing %s%s", ctx, key)
	}
	return v, nil
}

func object(raw json.RawMessage, ctx string) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: %s must be an object: %w", ctx, err)
	}
	if obj == nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: %s must be an object", ctx)
	}
	return obj, nil
}

// u64 parses an Ethereum quantity that may appear as a JSON number or a
// 0x-prefixed hex string (both show up across tooling; matches the reference).
func u64(raw json.RawMessage, ctx string) (uint64, error) {
	var n uint64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && strings.HasPrefix(s, "0x") {
		if v, err := strconv.ParseUint(s[2:], 16, 64); err == nil {
			return v, nil
		}
	}
	return 0, fmt.Errorf("DecodeL2ExecutionRequest: %s must be a uint64 (number or 0x-hex), got %s", ctx, raw)
}

func hexString(raw json.RawMessage, ctx string) ([]byte, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: %s must be a hex string: %w", ctx, err)
	}
	if !strings.HasPrefix(s, "0x") {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: %s must be a 0x-prefixed hex string", ctx)
	}
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, fmt.Errorf("DecodeL2ExecutionRequest: %s: invalid hex: %w", ctx, err)
	}
	return b, nil
}

func validateFixedHexField(m map[string]json.RawMessage, key, ctx string, wantBytes int) error {
	raw, err := requireField(m, key, ctx)
	if err != nil {
		return err
	}
	b, err := hexString(raw, ctx+key)
	if err != nil {
		return err
	}
	if len(b) != wantBytes {
		return fmt.Errorf("DecodeL2ExecutionRequest: %s%s must be %d bytes, got %d", ctx, key, wantBytes, len(b))
	}
	return nil
}
