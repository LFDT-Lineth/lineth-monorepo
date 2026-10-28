package jobadapter

import (
	"encoding/json"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// Field keys for the rollup request. Keys in request.go (guestProgramIDKey,
// chainIDKey, proofRequestKey, the byte sizes) are reused. parentDataRollingHashKey,
// parentDataTailTakeBytesKey, filteredAddressesKey, l2L1MessagesKey,
// endBlockNumberKey, blockCountKey, l2MessagingBlocksOffsetsKey, publicInputsKey,
// proofKey, and startBlockNumberKey are also used by aggregation_request.go.
const (
	decodeRollupOp = "DecodeRollupRequest"

	conflationsKey                 = "conflations"
	chunksKey                      = "chunks"
	l2ExecutionProofsKey           = "l2ExecutionProofs"
	parentDataRollingHashKey       = "parentDataRollingHash"
	parentDataTailTakeBytesKey     = "parentDataTailTakeBytes"
	boundaryPrevDataRollingHashKey = "boundaryPrevDataRollingHash"

	blockRlpsKey     = "blockRlps"
	chunkHashKey     = "chunkHash"
	isCalldataKey    = "isCalldata"
	blobBytesKey     = "blobBytes"
	calldataBytesKey = "calldataBytes"

	proofKey                    = "proof"
	startBlockNumberKey         = "startBlockNumber"
	publicInputsKey             = "publicInputs"
	l2L1MessagesKey             = "l2L1Messages"
	filteredAddressesKey        = "filteredAddresses"
	endBlockNumberKey           = "endBlockNumber"
	blockCountKey               = "blockCount"
	l2MessagingBlocksOffsetsKey = "l2MessagingBlocksOffsets"
)

// ConflationWitness is one conflation: the RLP blocks its paired proof attests.
type ConflationWitness struct {
	BlockRlps [][]byte
}

// ChunkWitness is one touched chunk: its L1-anchored binding hash, kind, and
// serialized bytes. For a calldata chunk CalldataBytes is set and BlobBytes is
// empty; for a blob chunk BlobBytes is set and CalldataBytes is empty.
type ChunkWitness struct {
	ChunkHash     [32]byte
	IsCalldata    bool
	BlobBytes     []byte
	CalldataBytes []byte
}

// EmbeddedL2ExecutionProof is one l2ExecutionProof a rollup request carries for
// the guest to verify. ProgramVk is its verifying-key hash.
type EmbeddedL2ExecutionProof struct {
	Proof            []byte
	StartBlockNumber uint64
	PublicInputs     backend.PublicInputs
	ProgramVk        []byte
}

// RollupRequest is a decoded getZkRollupProofV1 request. Conflations pair 1:1
// with L2ExecutionProofs. BoundaryPrevDataRollingHash is optional.
type RollupRequest struct {
	ProgramVk               []byte
	ChainID                 uint64
	Conflations             []ConflationWitness
	Chunks                  []ChunkWitness
	ParentDataRollingHash   [32]byte
	ParentDataTailTakeBytes uint64

	BoundaryPrevDataRollingHash *[32]byte
	L2ExecutionProofs           []EmbeddedL2ExecutionProof
}

// DecodeRollupRequest parses a getZkRollupProofV1 request and validates its
// shape and the chunk invariants. It does not build guest input bytes: the
// recursion input format is undecided, so the runner passes a placeholder.
func DecodeRollupRequest(data []byte) (*RollupRequest, error) {
	const op = decodeRollupOp

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

	parentDataTailTakeBytes, err := getU64(proofRequest, parentDataTailTakeBytesKey, op, "proofRequest.")
	if err != nil {
		return nil, err
	}

	// boundaryPrevDataRollingHash is optional.
	var boundaryPrev *[32]byte
	if raw, ok := proofRequest[boundaryPrevDataRollingHashKey]; ok {
		h, err := hash32Of(raw, op, "proofRequest."+boundaryPrevDataRollingHashKey)
		if err != nil {
			return nil, err
		}
		boundaryPrev = &h
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
		ProgramVk:                   guestProgramID,
		ChainID:                     chainID,
		Conflations:                 conflations,
		Chunks:                      chunks,
		ParentDataRollingHash:       parentDataRollingHash,
		ParentDataTailTakeBytes:     parentDataTailTakeBytes,
		BoundaryPrevDataRollingHash: boundaryPrev,
		L2ExecutionProofs:           proofs,
	}, nil
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
	blobBytes, err := getHexBytes(obj, blobBytesKey, op, ctx)
	if err != nil {
		return ChunkWitness{}, err
	}
	calldataBytes, err := getHexBytes(obj, calldataBytesKey, op, ctx)
	if err != nil {
		return ChunkWitness{}, err
	}
	if isCalldata {
		if len(calldataBytes) == 0 {
			return ChunkWitness{}, decErrf(op, "%s%s must be non-empty for a calldata chunk", ctx, calldataBytesKey)
		}
		if len(blobBytes) != 0 {
			return ChunkWitness{}, decErrf(op, "%s%s must be empty for a calldata chunk", ctx, blobBytesKey)
		}
	} else {
		if len(blobBytes) == 0 {
			return ChunkWitness{}, decErrf(op, "%s%s must be non-empty for a blob chunk", ctx, blobBytesKey)
		}
		if len(calldataBytes) != 0 {
			return ChunkWitness{}, decErrf(op, "%s%s must be empty for a blob chunk", ctx, calldataBytesKey)
		}
	}
	return ChunkWitness{
		ChunkHash:     chunkHash,
		IsCalldata:    isCalldata,
		BlobBytes:     blobBytes,
		CalldataBytes: calldataBytes,
	}, nil
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
	pi, err := decodeEmbeddedPublicInputs(piObj, op, ctx+publicInputsKey+".")
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	programVk, err := getFixedHex(obj, programVkKey, op, ctx, programVkByteSize)
	if err != nil {
		return EmbeddedL2ExecutionProof{}, err
	}
	return EmbeddedL2ExecutionProof{
		Proof:            proof,
		StartBlockNumber: startBlockNumber,
		PublicInputs:     pi,
		ProgramVk:        programVk,
	}, nil
}

// decodeEmbeddedPublicInputs decodes the 18-field backend.PublicInputs shape
// embedded in an l2ExecutionProof.
func decodeEmbeddedPublicInputs(
	m map[string]json.RawMessage, op, ctx string,
) (backend.PublicInputs, error) {
	var pi backend.PublicInputs
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
	pi.ParentL1L2BridgeRollingHash = h("parentL1L2BridgeRollingHash")
	pi.ParentL1L2BridgeRollingHashMessageNumber = n("parentL1L2BridgeRollingHashMessageNumber")
	pi.EndL1L2BridgeRollingHash = h("endL1L2BridgeRollingHash")
	pi.EndL1L2BridgeRollingHashMessageNumber = n("endL1L2BridgeRollingHashMessageNumber")
	pi.DynamicChainConfigHash = h("dynamicChainConfigHash")
	pi.ParentFtxRollingHash = h("parentFtxRollingHash")
	pi.ParentFtxNumber = n("parentFtxNumber")
	pi.EndFtxRollingHash = h("endFtxRollingHash")
	pi.EndProcessedFtxNumber = n("endProcessedFtxNumber")
	pi.TxFromsHash = h("txFromsHash")
	pi.BlockCount = n(blockCountKey)
	if err != nil {
		return pi, err
	}

	offsetsRaw, err := getArray(m, l2MessagingBlocksOffsetsKey, op, ctx)
	if err != nil {
		return pi, err
	}
	offsets, err := uint16List(offsetsRaw, op, ctx+l2MessagingBlocksOffsetsKey)
	if err != nil {
		return pi, err
	}
	pi.L2MessagingBlocksOffsets = offsets

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
	return pi, nil
}

// uint16List decodes a JSON array of integers into []uint16.
func uint16List(raw []json.RawMessage, op, prefix string) ([]uint16, error) {
	out := make([]uint16, len(raw))
	for i, r := range raw {
		v, err := uint64Of(r, op, fmt.Sprintf("%s[%d]", prefix, i))
		if err != nil {
			return nil, err
		}
		if v > 0xffff {
			return nil, decErrf(op, "%s[%d] must fit in uint16, got %d", prefix, i, v)
		}
		out[i] = uint16(v)
	}
	return out, nil
}

func trimDot(ctx string) string {
	if len(ctx) > 0 && ctx[len(ctx)-1] == '.' {
		return ctx[:len(ctx)-1]
	}
	return ctx
}
