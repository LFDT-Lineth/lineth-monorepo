// Package jobadapter drives getZkL2ExecutionProofV1 proof requests from a
// filesystem queue: it polls a requests directory for request files, drives
// each through a [backend.Core], and writes responses. It is the boundary
// where the coordinator's readable JSON is SSZ-encoded for the guest,
// mirroring rollup_spec/src/rollup_spec/proof_io_v1.py.
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

	proofRequestKey        = "proofRequest"
	chainConfigKey         = "chainConfig"
	statelessInputKey      = "statelessInput"
	newPayloadRequestKey   = "newPayloadRequest"
	executionPayloadKey    = "executionPayload"
	executionRequestsKey   = "executionRequests"
	rollupExtensionKey     = "rollupExtension"
	forcedTransactionsKey  = "forcedTransactions"
	blockNumberKey         = "blockNumber"
	numberKey              = "number"
	deadlineKey            = "deadline"
	signedTxRlpKey         = "signedTxRlp"
	acceptanceKey          = "acceptance"
	guestProgramIDByteSize = 32

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

// DecodedPayload is one block's worth of a decoded request: the framed SSZ the
// guest reads (the output of [ssz.EncodeStatelessInput]) and the block
// number that payload proves.
type DecodedPayload struct {
	BlockNumber        uint64
	FramedSSZ          []byte
	ForcedTransactions []json.RawMessage
}

// Request is a decoded getZkL2ExecutionProofV1 request: routing metadata, the
// range-level chain identity, and one [DecodedPayload] per block. The block
// range is implied by the payloads (their executionPayload.blockNumber), as in
// the reference decoder.
type Request struct {
	// GuestProgramID is routing metadata; this decoder validates its shape but
	// does not verify it against the configured guest ELF (open question #6 in
	// wiki backend-overview.md).
	GuestProgramID []byte
	ChainID        uint64
	ForkName       string
	Payloads       []DecodedPayload
}

// DecodeRequest parses a getZkL2ExecutionProofV1 request body and SSZ-encodes
// each payload's statelessInput, porting proof_io_v1.py::decode_request and
// _decode_payload: it injects {chainId, forkName} into each payload's
// statelessInput and reduces executionRequests to {} (rejecting any non-empty
// list) before calling [ssz.EncodeStatelessInput]. It also validates and
// preserves rollupExtension.forcedTransactions for the adapter capability
// check.
func DecodeRequest(data []byte) (*Request, error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("DecodeRequest: parsing JSON: %w", err)
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
			"DecodeRequest: %s must be %d bytes, got %d",
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
	chainConfig, err := object(ccRaw, "proofRequest.chainConfig")
	if err != nil {
		return nil, err
	}
	chainIDRaw, err := requireField(chainConfig, chainIDKey, "proofRequest.chainConfig.")
	if err != nil {
		return nil, err
	}
	chainID, err := u64(chainIDRaw, "proofRequest.chainConfig.chainId")
	if err != nil {
		return nil, err
	}
	forkNameRaw, err := requireField(chainConfig, forkNameKey, "proofRequest.chainConfig.")
	if err != nil {
		return nil, err
	}
	var forkName string
	if err := json.Unmarshal(forkNameRaw, &forkName); err != nil {
		return nil, fmt.Errorf("DecodeRequest: proofRequest.chainConfig.forkName: %w", err)
	}

	payloadsRaw, err := requireField(proofRequest, payloadsKey, "proofRequest.")
	if err != nil {
		return nil, err
	}
	var payloadObjs []json.RawMessage
	if err := json.Unmarshal(payloadsRaw, &payloadObjs); err != nil {
		return nil, fmt.Errorf("DecodeRequest: proofRequest.payloads must be an array: %w", err)
	}
	if payloadObjs == nil {
		return nil, fmt.Errorf("DecodeRequest: proofRequest.payloads must be an array")
	}
	if len(payloadObjs) == 0 {
		return nil, fmt.Errorf("DecodeRequest: proofRequest.payloads must be non-empty")
	}

	payloads := make([]DecodedPayload, len(payloadObjs))
	for i, raw := range payloadObjs {
		p, err := decodePayload(raw, i, chainID, forkName)
		if err != nil {
			return nil, err
		}
		payloads[i] = p
	}

	return &Request{
		GuestProgramID: guestProgramID,
		ChainID:        chainID,
		ForkName:       forkName,
		Payloads:       payloads,
	}, nil
}

// decodePayload builds the encoder_obj for one payload (statelessInput with
// chainConfig injected and executionRequests reduced to {}), SSZ-encodes it,
// and reads its block number. Mirrors proof_io_v1.py::_decode_payload.
func decodePayload(raw json.RawMessage, index int, chainID uint64, forkName string) (DecodedPayload, error) {
	ctx := fmt.Sprintf("proofRequest.payloads[%d].", index)

	payload, err := object(raw, strings.TrimSuffix(ctx, "."))
	if err != nil {
		return DecodedPayload{}, err
	}

	siRaw, err := requireField(payload, statelessInputKey, ctx)
	if err != nil {
		return DecodedPayload{}, err
	}
	statelessInput, err := object(siRaw, ctx+"statelessInput")
	if err != nil {
		return DecodedPayload{}, err
	}

	nprRaw, err := requireField(statelessInput, newPayloadRequestKey, ctx+"statelessInput.")
	if err != nil {
		return DecodedPayload{}, err
	}
	newPayloadRequest, err := object(nprRaw, ctx+"statelessInput.newPayloadRequest")
	if err != nil {
		return DecodedPayload{}, err
	}

	if err := rejectNonEmptyExecutionRequests(newPayloadRequest, ctx); err != nil {
		return DecodedPayload{}, err
	}

	blockNumber, err := payloadBlockNumber(newPayloadRequest, ctx)
	if err != nil {
		return DecodedPayload{}, err
	}

	// Build the encoder_obj: executionRequests -> {}, chainConfig injected.
	newPayloadRequest[executionRequestsKey] = json.RawMessage("{}")
	nprEncoded, err := json.Marshal(newPayloadRequest)
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput.newPayloadRequest: %w", ctx, err)
	}
	statelessInput[newPayloadRequestKey] = nprEncoded
	chainConfig, err := json.Marshal(map[string]any{chainIDKey: chainID, forkNameKey: forkName})
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %schainConfig: %w", ctx, err)
	}
	statelessInput[chainConfigKey] = chainConfig

	encoderObj, err := json.Marshal(statelessInput)
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput: %w", ctx, err)
	}
	framedSSZ, err := ssz.EncodeStatelessInput(encoderObj)
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput: %w", ctx, err)
	}
	forcedTransactions, err := payloadForcedTransactions(payload, ctx)
	if err != nil {
		return DecodedPayload{}, err
	}

	return DecodedPayload{
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
		return fmt.Errorf("DecodeRequest: %sexecutionRequests must be an array: %w", ctx, err)
	}
	if executionRequests == nil {
		return fmt.Errorf("DecodeRequest: %sexecutionRequests must be an array", ctx)
	}
	if len(executionRequests) != 0 {
		return fmt.Errorf("DecodeRequest: %sexecutionRequests must be empty", ctx)
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
		return nil, fmt.Errorf("DecodeRequest: %srollupExtension.forcedTransactions must be an array: %w", ctx, err)
	}
	if forcedTransactions == nil {
		return nil, fmt.Errorf("DecodeRequest: %srollupExtension.forcedTransactions must be an array", ctx)
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
		return fmt.Errorf("DecodeRequest: %s%s must be a string: %w", ctx, acceptanceKey, err)
	}
	if _, ok := validForcedTransactionAcceptances[acceptance]; !ok {
		return fmt.Errorf("DecodeRequest: %s%s has unsupported value %q", ctx, acceptanceKey, acceptance)
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
		return nil, fmt.Errorf("DecodeRequest: missing %s%s", ctx, key)
	}
	return v, nil
}

func object(raw json.RawMessage, ctx string) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("DecodeRequest: %s must be an object: %w", ctx, err)
	}
	if obj == nil {
		return nil, fmt.Errorf("DecodeRequest: %s must be an object", ctx)
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
	return 0, fmt.Errorf("DecodeRequest: %s must be a uint64 (number or 0x-hex), got %s", ctx, raw)
}

func hexString(raw json.RawMessage, ctx string) ([]byte, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("DecodeRequest: %s must be a hex string: %w", ctx, err)
	}
	if !strings.HasPrefix(s, "0x") {
		return nil, fmt.Errorf("DecodeRequest: %s must be a 0x-prefixed hex string", ctx)
	}
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, fmt.Errorf("DecodeRequest: %s: invalid hex: %w", ctx, err)
	}
	return b, nil
}
