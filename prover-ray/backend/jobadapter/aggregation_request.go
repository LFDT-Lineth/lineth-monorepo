package jobadapter

import (
	"encoding/json"
	"fmt"
)

const (
	decodeAggregationOp = "DecodeAggregationRequest"

	rollupProofsKey              = "rollupProofs"
	programVksKey                = "programVks"
	finalDataTailDiscardBytesKey = "finalDataTailDiscardBytes"
)

// EmbeddedRollupProof is one rollupProof an aggregation request carries for the
// guest to verify. ProgramVk is the rollup proof's own verifying-key hash.
type EmbeddedRollupProof struct {
	Proof            []byte
	StartBlockNumber uint64
	PublicInputs     RollupProofPublicInputs
	ProgramVk        []byte
}

// RollupProofPublicInputs is the 22-field rollup PI of an embedded rollupProof.
// ProgramVks is the sorted, distinct set of VKs verified beneath that proof (a
// guest output, not the envelope VK).
type RollupProofPublicInputs struct {
	EndBlockNumber                           uint64
	EndBlockTimestamp                        uint64
	ParentL1L2BridgeRollingHash              [32]byte
	ParentL1L2BridgeRollingHashMessageNumber uint64
	EndL1L2BridgeRollingHash                 [32]byte
	EndL1L2BridgeRollingHashMessageNumber    uint64
	DynamicChainConfigHash                   [32]byte
	ParentFtxRollingHash                     [32]byte
	ParentFtxNumber                          uint64
	EndFtxRollingHash                        [32]byte
	EndProcessedFtxNumber                    uint64
	ParentDataRollingHash                    [32]byte
	EndDataRollingHash                       [32]byte
	ParentBlockHash                          [32]byte
	EndBlockHash                             [32]byte
	ParentDataTailTakeBytes                  uint64
	FinalDataTailDiscardBytes                uint64
	L2L1Messages                             [][32]byte
	FilteredAddresses                        [][20]byte
	ProgramVks                               [][32]byte
	BlockCount                               uint64
	L2MessagingBlocksOffsets                 []uint16
}

// AggregationRequest is a decoded getZkRollupAggregationProofV1 request: the
// flat list of rollup proofs the guest verifies. There is no chainId.
type AggregationRequest struct {
	ProgramVk    []byte
	RollupProofs []EmbeddedRollupProof
}

// DecodeAggregationRequest parses a getZkRollupAggregationProofV1 request and
// validates its shape. Like the rollup decoder, it builds no guest input bytes.
func DecodeAggregationRequest(data []byte) (*AggregationRequest, error) {
	const op = decodeAggregationOp

	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, decErrf(op, "parsing JSON: %w", err)
	}

	guestProgramID, err := getFixedHex(env, guestProgramIDKey, op, "", programVkByteSize)
	if err != nil {
		return nil, err
	}

	proofRequest, err := getObject(env, proofRequestKey, op, "")
	if err != nil {
		return nil, err
	}

	rollupProofsRaw, err := getArray(proofRequest, rollupProofsKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}
	if len(rollupProofsRaw) == 0 {
		return nil, decErrf(op, "proofRequest.%s must be a non-empty array", rollupProofsKey)
	}

	proofs := make([]EmbeddedRollupProof, len(rollupProofsRaw))
	for i, raw := range rollupProofsRaw {
		p, err := decodeEmbeddedRollupProof(raw, op, fmt.Sprintf("proofRequest.rollupProofs[%d].", i))
		if err != nil {
			return nil, err
		}
		proofs[i] = p
	}

	return &AggregationRequest{ProgramVk: guestProgramID, RollupProofs: proofs}, nil
}

func decodeEmbeddedRollupProof(raw json.RawMessage, op, ctx string) (EmbeddedRollupProof, error) {
	obj, err := objectOf(raw, op, trimDot(ctx))
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	proof, err := getHexBytes(obj, proofKey, op, ctx)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	startBlockNumber, err := getU64(obj, startBlockNumberKey, op, ctx)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	piObj, err := getObject(obj, publicInputsKey, op, ctx)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	pi, err := decodeRollupProofPublicInputs(piObj, op, ctx+publicInputsKey+".")
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	programVk, err := getFixedHex(obj, programVkKey, op, ctx, programVkByteSize)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	return EmbeddedRollupProof{
		Proof:            proof,
		StartBlockNumber: startBlockNumber,
		PublicInputs:     pi,
		ProgramVk:        programVk,
	}, nil
}

func decodeRollupProofPublicInputs(
	m map[string]json.RawMessage, op, ctx string,
) (RollupProofPublicInputs, error) {
	var pi RollupProofPublicInputs
	var err error
	h := func(key string) [32]byte {
		if err != nil {
			return [32]byte{}
		}
		var v [32]byte
		v, err = getHash32(m, key, op, ctx)
		return v
	}
	n := func(key string) uint64 {
		if err != nil {
			return 0
		}
		var v uint64
		v, err = getU64(m, key, op, ctx)
		return v
	}
	pi.EndBlockNumber = n(endBlockNumberKey)
	pi.EndBlockTimestamp = n("endBlockTimestamp")
	pi.ParentL1L2BridgeRollingHash = h("parentL1L2BridgeRollingHash")
	pi.ParentL1L2BridgeRollingHashMessageNumber = n("parentL1L2BridgeRollingHashMessageNumber")
	pi.EndL1L2BridgeRollingHash = h("endL1L2BridgeRollingHash")
	pi.EndL1L2BridgeRollingHashMessageNumber = n("endL1L2BridgeRollingHashMessageNumber")
	pi.DynamicChainConfigHash = h("dynamicChainConfigHash")
	pi.ParentFtxRollingHash = h("parentFtxRollingHash")
	pi.ParentFtxNumber = n("parentFtxNumber")
	pi.EndFtxRollingHash = h("endFtxRollingHash")
	pi.EndProcessedFtxNumber = n("endProcessedFtxNumber")
	pi.ParentDataRollingHash = h(parentDataRollingHashKey)
	pi.EndDataRollingHash = h("endDataRollingHash")
	pi.ParentBlockHash = h("parentBlockHash")
	pi.EndBlockHash = h("endBlockHash")
	pi.ParentDataTailTakeBytes = n(parentDataTailTakeBytesKey)
	pi.FinalDataTailDiscardBytes = n(finalDataTailDiscardBytesKey)
	pi.BlockCount = n(blockCountKey)
	if err != nil {
		return pi, err
	}

	l2L1MessagesRaw, err := getArray(m, l2L1MessagesKey, op, ctx)
	if err != nil {
		return pi, err
	}
	l2L1Messages, err := hash32List(l2L1MessagesRaw, op, ctx+l2L1MessagesKey)
	if err != nil {
		return pi, err
	}
	pi.L2L1Messages = l2L1Messages

	filteredRaw, err := getArray(m, filteredAddressesKey, op, ctx)
	if err != nil {
		return pi, err
	}
	filtered, err := addressList(filteredRaw, op, ctx+filteredAddressesKey)
	if err != nil {
		return pi, err
	}
	pi.FilteredAddresses = filtered

	programVksRaw, err := getArray(m, programVksKey, op, ctx)
	if err != nil {
		return pi, err
	}
	programVks, err := hash32List(programVksRaw, op, ctx+programVksKey)
	if err != nil {
		return pi, err
	}
	pi.ProgramVks = programVks

	offsetsRaw, err := getArray(m, l2MessagingBlocksOffsetsKey, op, ctx)
	if err != nil {
		return pi, err
	}
	offsets, err := uint16List(offsetsRaw, op, ctx+l2MessagingBlocksOffsetsKey)
	if err != nil {
		return pi, err
	}
	pi.L2MessagingBlocksOffsets = offsets

	return pi, nil
}
