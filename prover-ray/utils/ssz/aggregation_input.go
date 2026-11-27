package ssz

import (
	"encoding/binary"
	"fmt"
	"slices"
)

// This file encodes the rollup-aggregation guest input (schema 0x0006), as
// rollup_spec/rollup_aggregation_ssz.py::encode_aggregation_input.

// aggregationInputSchemaID is the big-endian schema id of the aggregation input.
var aggregationInputSchemaID = []byte{0x00, 0x06}

// RollupPublicInputs has the fields of the rollup guest output, in the same
// order.
type RollupPublicInputs struct {
	EndBlockNumber                        uint64
	EndBlockTimestamp                     uint64
	EndL1L2BridgeRollingHash              [32]byte
	EndL1L2BridgeRollingHashMessageNumber uint64
	DynamicChainConfigHash                [32]byte
	ParentFtxRollingHash                  [32]byte
	ParentFtxNumber                       uint64
	EndFtxRollingHash                     [32]byte
	EndProcessedFtxNumber                 uint64
	ParentDataRollingHash                 [32]byte
	EndDataRollingHash                    [32]byte
	ParentBlockHash                       [32]byte
	EndBlockHash                          [32]byte
	ParentSharedBlobCarryOverTailBytes    uint64
	FinalSharedBlobCarryOverTailBytes     uint64
	L2L1Messages                          [][32]byte
	FilteredAddresses                     [][20]byte
	ProgramVks                            [][32]byte
	BlockCount                            uint64
	L2MessagingBlocksOffsets              []uint16
}

// RollupProof is a rollup proof the aggregation guest verifies under
// ProgramVk.
type RollupProof struct {
	PublicInputs     RollupPublicInputs
	StartBlockNumber uint64
	Proof            []byte
	ProgramVk        [32]byte
}

// AggregationInput is the rollup-aggregation guest input.
type AggregationInput struct {
	RollupProofs []RollupProof
}

// EncodeAggregationInput encodes in as
// 0x0006 || SSZ(RollupAggregationProofPrivateInput).
func EncodeAggregationInput(in AggregationInput) ([]byte, error) {
	proofs, err := encodeList(in.RollupProofs, encodeRollupProof)
	if err != nil {
		return nil, fmt.Errorf("aggregation input rollup proofs: %w", err)
	}
	body, err := sszContainer(variable(proofs))
	if err != nil {
		return nil, fmt.Errorf("aggregation input: %w", err)
	}
	return append(slices.Clone(aggregationInputSchemaID), body...), nil
}

func encodeRollupProof(p RollupProof) ([]byte, error) {
	publicInputs, err := encodeRollupPublicInputs(p.PublicInputs)
	if err != nil {
		return nil, err
	}
	proof, err := sszContainer(
		variable(publicInputs),
		fixed(sszUint64(p.StartBlockNumber)),
		variable(p.Proof),
	)
	if err != nil {
		return nil, err
	}
	return sszContainer(variable(proof), fixed(p.ProgramVk[:]))
}

// encodeRollupPublicInputs uses the field order of the 0x0005 guest output.
func encodeRollupPublicInputs(pi RollupPublicInputs) ([]byte, error) {
	messages := make([][]byte, len(pi.L2L1Messages))
	for i := range pi.L2L1Messages {
		messages[i] = pi.L2L1Messages[i][:]
	}
	addresses := make([][]byte, len(pi.FilteredAddresses))
	for i := range pi.FilteredAddresses {
		addresses[i] = pi.FilteredAddresses[i][:]
	}
	programVks := make([][]byte, len(pi.ProgramVks))
	for i := range pi.ProgramVks {
		programVks[i] = pi.ProgramVks[i][:]
	}
	offsets := make([][]byte, len(pi.L2MessagingBlocksOffsets))
	for i, o := range pi.L2MessagingBlocksOffsets {
		offsets[i] = binary.LittleEndian.AppendUint16(nil, o)
	}
	return sszContainer(
		fixed(sszUint64(pi.EndBlockNumber)),
		fixed(sszUint64(pi.EndBlockTimestamp)),
		fixed(pi.EndL1L2BridgeRollingHash[:]),
		fixed(sszUint64(pi.EndL1L2BridgeRollingHashMessageNumber)),
		fixed(pi.DynamicChainConfigHash[:]),
		fixed(pi.ParentFtxRollingHash[:]),
		fixed(sszUint64(pi.ParentFtxNumber)),
		fixed(pi.EndFtxRollingHash[:]),
		fixed(sszUint64(pi.EndProcessedFtxNumber)),
		fixed(pi.ParentDataRollingHash[:]),
		fixed(pi.EndDataRollingHash[:]),
		fixed(pi.ParentBlockHash[:]),
		fixed(pi.EndBlockHash[:]),
		fixed(sszUint64(pi.ParentSharedBlobCarryOverTailBytes)),
		fixed(sszUint64(pi.FinalSharedBlobCarryOverTailBytes)),
		variable(sszListFixed(messages)),
		variable(sszListFixed(addresses)),
		variable(sszListFixed(programVks)),
		fixed(sszUint64(pi.BlockCount)),
		variable(sszListFixed(offsets)),
	)
}
