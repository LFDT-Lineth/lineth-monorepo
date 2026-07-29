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
)

// DecodedPayload is one block's worth of a decoded request: the framed SSZ the
// guest reads (the output of [ssz.EncodeStatelessInput]) and the block
// number that payload proves.
type DecodedPayload struct {
	BlockNumber uint64
	FramedSSZ   []byte
}

// Request is a decoded getZkL2ExecutionProofV1 request: routing metadata, the
// range-level chain identity, and one [DecodedPayload] per block. The block
// range is implied by the payloads (their executionPayload.blockNumber), as in
// the reference decoder.
type Request struct {
	// GuestProgramID is routing metadata; it is not verified here
	// (open question #6 in wiki backend-overview.md).
	GuestProgramID []byte
	ChainID        uint64
	ForkName       string
	Payloads       []DecodedPayload
}

// DecodeRequest parses a getZkL2ExecutionProofV1 request body and SSZ-encodes
// each payload's statelessInput, porting proof_io_v1.py::decode_request and
// _decode_payload: it injects {chainId, forkName} into each payload's
// statelessInput and reduces executionRequests to {} (rejecting any non-empty
// list) before calling [ssz.EncodeStatelessInput].
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

	prRaw, err := requireField(env, "proofRequest", "")
	if err != nil {
		return nil, err
	}
	var proofRequest map[string]json.RawMessage
	if err := json.Unmarshal(prRaw, &proofRequest); err != nil {
		return nil, fmt.Errorf("DecodeRequest: proofRequest: %w", err)
	}

	ccRaw, err := requireField(proofRequest, "chainConfig", "proofRequest.")
	if err != nil {
		return nil, err
	}
	var chainConfig map[string]json.RawMessage
	if err := json.Unmarshal(ccRaw, &chainConfig); err != nil {
		return nil, fmt.Errorf("DecodeRequest: proofRequest.chainConfig: %w", err)
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

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput: %w", ctx, err)
	}
	siRaw, err := requireField(payload, "statelessInput", ctx)
	if err != nil {
		return DecodedPayload{}, err
	}
	var statelessInput map[string]json.RawMessage
	if err := json.Unmarshal(siRaw, &statelessInput); err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput: %w", ctx, err)
	}

	nprRaw, err := requireField(statelessInput, "newPayloadRequest", ctx+"statelessInput.")
	if err != nil {
		return DecodedPayload{}, err
	}
	var newPayloadRequest map[string]json.RawMessage
	if err := json.Unmarshal(nprRaw, &newPayloadRequest); err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput.newPayloadRequest: %w", ctx, err)
	}

	if err := rejectNonEmptyExecutionRequests(newPayloadRequest, ctx); err != nil {
		return DecodedPayload{}, err
	}

	blockNumber, err := payloadBlockNumber(newPayloadRequest, ctx)
	if err != nil {
		return DecodedPayload{}, err
	}

	// Build the encoder_obj: executionRequests -> {}, chainConfig injected.
	newPayloadRequest["executionRequests"] = json.RawMessage("{}")
	nprEncoded, err := json.Marshal(newPayloadRequest)
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput.newPayloadRequest: %w", ctx, err)
	}
	statelessInput["newPayloadRequest"] = nprEncoded
	chainConfig, err := json.Marshal(map[string]any{chainIDKey: chainID, forkNameKey: forkName})
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %schainConfig: %w", ctx, err)
	}
	statelessInput["chainConfig"] = chainConfig

	encoderObj, err := json.Marshal(statelessInput)
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput: %w", ctx, err)
	}
	framedSSZ, err := ssz.EncodeStatelessInput(encoderObj)
	if err != nil {
		return DecodedPayload{}, fmt.Errorf("DecodeRequest: %sstatelessInput: %w", ctx, err)
	}

	return DecodedPayload{BlockNumber: blockNumber, FramedSSZ: framedSSZ}, nil
}

// rejectNonEmptyExecutionRequests enforces that executionRequests is present as
// an array and empty (the rollup rejects EIP-7685 requests, rollup_spec §2.1).
func rejectNonEmptyExecutionRequests(newPayloadRequest map[string]json.RawMessage, ctx string) error {
	erRaw, err := requireField(newPayloadRequest, "executionRequests", ctx+"statelessInput.newPayloadRequest.")
	if err != nil {
		return err
	}
	var executionRequests []json.RawMessage
	if err := json.Unmarshal(erRaw, &executionRequests); err != nil {
		return fmt.Errorf("DecodeRequest: %sexecutionRequests must be an array: %w", ctx, err)
	}
	if len(executionRequests) != 0 {
		return fmt.Errorf("DecodeRequest: %sexecutionRequests must be empty", ctx)
	}
	return nil
}

func payloadBlockNumber(newPayloadRequest map[string]json.RawMessage, ctx string) (uint64, error) {
	epRaw, err := requireField(newPayloadRequest, "executionPayload", ctx+"statelessInput.newPayloadRequest.")
	if err != nil {
		return 0, err
	}
	var executionPayload map[string]json.RawMessage
	if err := json.Unmarshal(epRaw, &executionPayload); err != nil {
		return 0, fmt.Errorf("DecodeRequest: %sstatelessInput.newPayloadRequest.executionPayload: %w", ctx, err)
	}
	bnRaw, err := requireField(executionPayload, "blockNumber", ctx+"statelessInput.newPayloadRequest.executionPayload.")
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
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, fmt.Errorf("DecodeRequest: %s: invalid hex: %w", ctx, err)
	}
	return b, nil
}
