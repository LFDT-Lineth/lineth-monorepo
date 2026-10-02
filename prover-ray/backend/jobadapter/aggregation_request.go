package jobadapter

import (
	"encoding/json"
	"fmt"
)

const (
	decodeAggregationOp = "DecodeAggregationRequest"

	rollupProofsKey = "rollupProofs"
	l2L1RootsKey    = "l2L1Roots"
	programVksKey   = "programVks"
)

// EmbeddedRollupProof is one rollupProof an aggregation request carries for the
// guest to verify. ProgramVk is the rollup proof's own verifying-key hash.
type EmbeddedRollupProof struct {
	Proof             []byte
	StartBlockNumber  uint64
	PublicInputs      RollupProofPublicInputs
	L2L1Roots         [][32]byte
	FilteredAddresses [][20]byte
	ProgramVk         []byte
}

// RollupProofPublicInputs is the 20-field PI of an embedded rollupProof.
// ProgramVks is the sorted, distinct set of VKs verified beneath that proof (a
// guest output, not the envelope VK).
type RollupProofPublicInputs struct {
	EndBlockNumber                           uint64
	EndBlockTimestamp                        uint64
	L2L1BridgeTransactionTree                [32]byte
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
	ParentDataRollingHash                    [32]byte
	EndDataRollingHash                       [32]byte
	ParentBlockHash                          [32]byte
	EndBlockHash                             [32]byte
	StartOffset                              uint64
	EndOffset                                uint64
	ProgramVks                               [][32]byte
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

	programVk, err := getFixedHex(env, programVkKey, op, "", programVkByteSize)
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

	return &AggregationRequest{ProgramVk: programVk, RollupProofs: proofs}, nil
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
	l2L1RootsRaw, err := getArray(obj, l2L1RootsKey, op, ctx)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	l2L1Roots, err := hash32List(l2L1RootsRaw, op, ctx+l2L1RootsKey)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	filteredRaw, err := getArray(obj, filteredAddressesKey, op, ctx)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	filtered, err := addressList(filteredRaw, op, ctx+filteredAddressesKey)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	programVk, err := getFixedHex(obj, programVkKey, op, ctx, programVkByteSize)
	if err != nil {
		return EmbeddedRollupProof{}, err
	}
	return EmbeddedRollupProof{
		Proof:             proof,
		StartBlockNumber:  startBlockNumber,
		PublicInputs:      pi,
		L2L1Roots:         l2L1Roots,
		FilteredAddresses: filtered,
		ProgramVk:         programVk,
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
	pi.L2L1BridgeTransactionTree = h("l2L1BridgeTransactionTree")
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
	pi.ParentDataRollingHash = h(parentDataRollingHashKey)
	pi.EndDataRollingHash = h("endDataRollingHash")
	pi.ParentBlockHash = h("parentBlockHash")
	pi.EndBlockHash = h("endBlockHash")
	pi.StartOffset = n(startOffsetKey)
	pi.EndOffset = n("endOffset")
	if err != nil {
		return pi, err
	}
	programVksRaw, err := getArray(m, programVksKey, op, ctx)
	if err != nil {
		return pi, err
	}
	programVks, err := hash32List(programVksRaw, op, ctx+programVksKey)
	if err != nil {
		return pi, err
	}
	pi.ProgramVks = programVks
	return pi, nil
}
