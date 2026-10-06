package jobadapter

import (
	"encoding/json"
	"fmt"
)

// Field keys for the rollup and aggregation requests. Keys in request.go
// (programVkKey, chainIDKey, proofRequestKey, the byte sizes) are reused.
const (
	decodeRollupOp = "DecodeRollupRequest"

	conflationsKey                 = "conflations"
	chunksKey                      = "chunks"
	l2ExecutionProofsKey           = "l2ExecutionProofs"
	parentDataRollingHashKey       = "parentDataRollingHash"
	startOffsetKey                 = "startOffset"
	opaquePrefixBytesKey           = "opaquePrefixBytes"
	opaqueSuffixBytesKey           = "opaqueSuffixBytes"
	boundaryPrevDataRollingHashKey = "boundaryPrevDataRollingHash"

	blockRlpsKey      = "blockRlps"
	chunkHashKey      = "chunkHash"
	isCalldataKey     = "isCalldata"
	calldataLengthKey = "calldataLength"

	proofKey             = "proof"
	startBlockNumberKey  = "startBlockNumber"
	publicInputsKey      = "publicInputs"
	l2L1MessagesKey      = "l2L1Messages"
	txFromsKey           = "txFroms"
	filteredAddressesKey = "filteredAddresses"
	endBlockNumberKey    = "endBlockNumber"
)

// ConflationWitness is one conflation: the RLP blocks its paired proof attests.
type ConflationWitness struct {
	BlockRlps [][]byte
}

// ChunkWitness is one touched chunk (rollup §3.1): its L1-anchored binding hash
// and kind. CalldataLength is 0 for a blob and the exact length for calldata.
type ChunkWitness struct {
	ChunkHash      [32]byte
	IsCalldata     bool
	CalldataLength uint64
}

// EmbeddedL2ExecutionProof is one l2ExecutionProof a rollup request carries for
// the guest to verify. ProgramVk is its verifying-key hash.
type EmbeddedL2ExecutionProof struct {
	Proof             []byte
	StartBlockNumber  uint64
	PublicInputs      L2ExecutionProofPublicInputs
	L2L1Messages      [][32]byte
	TxFroms           [][20]byte
	FilteredAddresses [][20]byte
	ProgramVk         []byte
}

// L2ExecutionProofPublicInputs is the 16-field PI of an embedded l2ExecutionProof.
type L2ExecutionProofPublicInputs struct {
	ParentBlockHash                          [32]byte
	EndBlockHash                             [32]byte
	EndBlockNumber                           uint64
	EndBlockTimestamp                        uint64
	L2L1MessagesHash                         [32]byte
	ParentL1L2BridgeRollingHash              [32]byte
	ParentL1L2BridgeRollingHashMessageNumber uint64
	EndL1L2BridgeRollingHash                 [32]byte
	EndL1L2BridgeRollingHashMessageNumber    uint64
	DynamicChainConfigHash                   [32]byte
	ParentFtxRollingHash                     [32]byte
	ParentFtxNumber                          uint64
	EndFtxRollingHash                        [32]byte
	EndProcessedFtxNumber                    uint64
	FilteredAddressesHash                    [32]byte
	TxFromsHash                              [32]byte
}

// RollupRequest is a decoded getZkRollupProofV1 request. Conflations pair 1:1
// with L2ExecutionProofs; BoundaryPrevDataRollingHash is set only when
// StartOffset > 0 (§3.4).
type RollupRequest struct {
	ProgramVk                   []byte
	ChainID                     uint64
	Conflations                 []ConflationWitness
	Chunks                      []ChunkWitness
	ParentDataRollingHash       [32]byte
	StartOffset                 uint64
	OpaquePrefixBytes           []byte
	OpaqueSuffixBytes           []byte
	BoundaryPrevDataRollingHash *[32]byte
	L2ExecutionProofs           []EmbeddedL2ExecutionProof
}

// DecodeRollupRequest parses a getZkRollupProofV1 request and validates its
// shape and the flexible-blobs invariants. It does not build guest input bytes:
// the recursion input format is undecided, so the runner passes a placeholder.
func DecodeRollupRequest(data []byte) (*RollupRequest, error) {
	const op = decodeRollupOp

	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, decErrf(op, "parsing JSON: %w", err)
	}

	programVk, err := getFixedHex(env, programVkKey, op, "", programVkByteSize)
	if err != nil {
		return nil, err
	}

	proofRequest, err := getObject(env, proofRequestKey, op, "")
	if err != nil {
		return nil, err
	}

	chainID, err := getU64(proofRequest, chainIDKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}

	conflationsRaw, err := getArray(proofRequest, conflationsKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}
	if len(conflationsRaw) == 0 {
		return nil, decErrf(op, "proofRequest.%s must be a non-empty array", conflationsKey)
	}

	chunksRaw, err := getArray(proofRequest, chunksKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}
	if len(chunksRaw) == 0 {
		return nil, decErrf(op, "proofRequest.%s must be a non-empty array", chunksKey)
	}

	proofsRaw, err := getArray(proofRequest, l2ExecutionProofsKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}
	if len(proofsRaw) == 0 {
		return nil, decErrf(op, "proofRequest.%s must be a non-empty array", l2ExecutionProofsKey)
	}
	if len(conflationsRaw) != len(proofsRaw) {
		return nil, decErrf(op,
			"proofRequest.%s and proofRequest.%s must have the same length (%d != %d)",
			conflationsKey, l2ExecutionProofsKey, len(conflationsRaw), len(proofsRaw))
	}

	parentDataRollingHash, err := getHash32(proofRequest, parentDataRollingHashKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}

	startOffset, err := getU64(proofRequest, startOffsetKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}

	// boundaryPrevDataRollingHash is optional, required only when startOffset > 0.
	var boundaryPrev *[32]byte
	if raw, ok := proofRequest[boundaryPrevDataRollingHashKey]; ok {
		h, err := hash32Of(raw, op, "proofRequest."+boundaryPrevDataRollingHashKey)
		if err != nil {
			return nil, err
		}
		boundaryPrev = &h
	}
	if startOffset > 0 && boundaryPrev == nil {
		return nil, decErrf(op, "proofRequest.%s is required when startOffset > 0", boundaryPrevDataRollingHashKey)
	}

	// opaquePrefixBytes/opaqueSuffixBytes default to empty (0x) when absent.
	opaquePrefix, err := optionalHex(proofRequest, opaquePrefixBytesKey, op)
	if err != nil {
		return nil, err
	}
	if uint64(len(opaquePrefix)) != startOffset {
		return nil, decErrf(op, "proofRequest.%s length (%d) must equal startOffset (%d)",
			opaquePrefixBytesKey, len(opaquePrefix), startOffset)
	}
	opaqueSuffix, err := optionalHex(proofRequest, opaqueSuffixBytesKey, op)
	if err != nil {
		return nil, err
	}

	conflations := make([]ConflationWitness, len(conflationsRaw))
	for i, raw := range conflationsRaw {
		c, err := decodeConflationWitness(raw, op, fmt.Sprintf("proofRequest.conflations[%d].", i))
		if err != nil {
			return nil, err
		}
		conflations[i] = c
	}

	chunks := make([]ChunkWitness, len(chunksRaw))
	for i, raw := range chunksRaw {
		c, err := decodeChunkWitness(raw, op, fmt.Sprintf("proofRequest.chunks[%d].", i))
		if err != nil {
			return nil, err
		}
		chunks[i] = c
	}

	proofs := make([]EmbeddedL2ExecutionProof, len(proofsRaw))
	for i, raw := range proofsRaw {
		p, err := decodeEmbeddedL2ExecutionProof(raw, op, fmt.Sprintf("proofRequest.l2ExecutionProofs[%d].", i))
		if err != nil {
			return nil, err
		}
		proofs[i] = p
	}

	return &RollupRequest{
		ProgramVk:                   programVk,
		ChainID:                     chainID,
		Conflations:                 conflations,
		Chunks:                      chunks,
		ParentDataRollingHash:       parentDataRollingHash,
		StartOffset:                 startOffset,
		OpaquePrefixBytes:           opaquePrefix,
		OpaqueSuffixBytes:           opaqueSuffix,
		BoundaryPrevDataRollingHash: boundaryPrev,
		L2ExecutionProofs:           proofs,
	}, nil
}

// optionalHex decodes a 0x-hex field that defaults to empty bytes when absent.
func optionalHex(m map[string]json.RawMessage, key, op string) ([]byte, error) {
	raw, ok := m[key]
	if !ok {
		return []byte{}, nil
	}
	return hexBytesOf(raw, op, "proofRequest."+key)
}

func decodeConflationWitness(raw json.RawMessage, op, ctx string) (ConflationWitness, error) {
	obj, err := objectOf(raw, op, trimDot(ctx))
	if err != nil {
		return ConflationWitness{}, err
	}
	blockRlpsRaw, err := getArray(obj, blockRlpsKey, op, ctx)
	if err != nil {
		return ConflationWitness{}, err
	}
	if len(blockRlpsRaw) == 0 {
		return ConflationWitness{}, decErrf(op, "%s%s must be a non-empty array", ctx, blockRlpsKey)
	}
	blockRlps := make([][]byte, len(blockRlpsRaw))
	for i, r := range blockRlpsRaw {
		b, err := hexBytesOf(r, op, fmt.Sprintf("%s%s[%d]", ctx, blockRlpsKey, i))
		if err != nil {
			return ConflationWitness{}, err
		}
		blockRlps[i] = b
	}
	return ConflationWitness{BlockRlps: blockRlps}, nil
}

func decodeChunkWitness(raw json.RawMessage, op, ctx string) (ChunkWitness, error) {
	obj, err := objectOf(raw, op, trimDot(ctx))
	if err != nil {
		return ChunkWitness{}, err
	}
	chunkHash, err := getHash32(obj, chunkHashKey, op, ctx)
	if err != nil {
		return ChunkWitness{}, err
	}
	isCalldata, err := getBool(obj, isCalldataKey, op, ctx)
	if err != nil {
		return ChunkWitness{}, err
	}
	calldataLength, err := getU64(obj, calldataLengthKey, op, ctx)
	if err != nil {
		return ChunkWitness{}, err
	}
	if isCalldata && calldataLength == 0 {
		return ChunkWitness{}, decErrf(op, "%s%s must be positive for a calldata chunk", ctx, calldataLengthKey)
	}
	if !isCalldata && calldataLength != 0 {
		return ChunkWitness{}, decErrf(op, "%s%s must be 0 for a blob chunk", ctx, calldataLengthKey)
	}
	return ChunkWitness{ChunkHash: chunkHash, IsCalldata: isCalldata, CalldataLength: calldataLength}, nil
}

func decodeEmbeddedL2ExecutionProof(raw json.RawMessage, op, ctx string) (EmbeddedL2ExecutionProof, error) {
	obj, err := objectOf(raw, op, trimDot(ctx))
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	proof, err := getHexBytes(obj, proofKey, op, ctx)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	startBlockNumber, err := getU64(obj, startBlockNumberKey, op, ctx)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	piObj, err := getObject(obj, publicInputsKey, op, ctx)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	pi, err := decodeL2ExecutionProofPublicInputs(piObj, op, ctx+publicInputsKey+".")
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	l2L1MessagesRaw, err := getArray(obj, l2L1MessagesKey, op, ctx)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	l2L1Messages, err := hash32List(l2L1MessagesRaw, op, ctx+l2L1MessagesKey)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	txFromsRaw, err := getArray(obj, txFromsKey, op, ctx)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	txFroms, err := addressList(txFromsRaw, op, ctx+txFromsKey)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	filteredRaw, err := getArray(obj, filteredAddressesKey, op, ctx)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	filtered, err := addressList(filteredRaw, op, ctx+filteredAddressesKey)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	programVk, err := getFixedHex(obj, programVkKey, op, ctx, programVkByteSize)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	return EmbeddedL2ExecutionProof{
		Proof:             proof,
		StartBlockNumber:  startBlockNumber,
		PublicInputs:      pi,
		L2L1Messages:      l2L1Messages,
		TxFroms:           txFroms,
		FilteredAddresses: filtered,
		ProgramVk:         programVk,
	}, nil
}

func decodeL2ExecutionProofPublicInputs(
	m map[string]json.RawMessage, op, ctx string,
) (L2ExecutionProofPublicInputs, error) {
	var pi L2ExecutionProofPublicInputs
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
	pi.ParentBlockHash = h("parentBlockHash")
	pi.EndBlockHash = h("endBlockHash")
	pi.EndBlockNumber = n(endBlockNumberKey)
	pi.EndBlockTimestamp = n("endBlockTimestamp")
	pi.L2L1MessagesHash = h("l2L1MessagesHash")
	pi.ParentL1L2BridgeRollingHash = h("parentL1L2BridgeRollingHash")
	pi.ParentL1L2BridgeRollingHashMessageNumber = n("parentL1L2BridgeRollingHashMessageNumber")
	pi.EndL1L2BridgeRollingHash = h("endL1L2BridgeRollingHash")
	pi.EndL1L2BridgeRollingHashMessageNumber = n("endL1L2BridgeRollingHashMessageNumber")
	pi.DynamicChainConfigHash = h("dynamicChainConfigHash")
	pi.ParentFtxRollingHash = h("parentFtxRollingHash")
	pi.ParentFtxNumber = n("parentFtxNumber")
	pi.EndFtxRollingHash = h("endFtxRollingHash")
	pi.EndProcessedFtxNumber = n("endProcessedFtxNumber")
	pi.FilteredAddressesHash = h("filteredAddressesHash")
	pi.TxFromsHash = h("txFromsHash")
	return pi, err
}

func trimDot(ctx string) string {
	if len(ctx) > 0 && ctx[len(ctx)-1] == '.' {
		return ctx[:len(ctx)-1]
	}
	return ctx
}
