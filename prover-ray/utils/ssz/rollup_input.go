package ssz

import (
	"encoding/binary"
	"fmt"
	"slices"
)

// This file encodes the rollup guest input (schema 0x0004), as
// rollup_spec/rollup_ssz.py::encode_rollup_input.

// rollupInputSchemaID is the big-endian schema id of the rollup input.
var rollupInputSchemaID = []byte{0x00, 0x04}

// ConflationWitness is one conflation: the RLP blocks its paired proof attests.
type ConflationWitness struct {
	BlockRlps [][]byte
}

// ChunkWitness is one touched chunk: its L1-anchored binding hash, kind, and
// serialized bytes. For a calldata chunk CalldataBytes is set and BlobBytes,
// KZGCommitment and KZGProof are empty; for a blob chunk BlobBytes is set,
// CalldataBytes is empty, and KZGCommitment and KZGProof are the 48-byte
// commitment of BlobBytes and its EIP-4844 blob proof.
type ChunkWitness struct {
	ChunkHash     [32]byte
	IsCalldata    bool
	BlobBytes     []byte
	CalldataBytes []byte
	KZGCommitment []byte
	KZGProof      []byte
}

// L2ExecutionPublicInputs has the fields of backend.PublicInputs, in the same
// order, so that one converts to the other.
type L2ExecutionPublicInputs struct {
	ParentBlockHash                       [32]byte
	EndBlockHash                          [32]byte
	EndBlockNumber                        uint64
	EndBlockTimestamp                     uint64
	EndL1L2BridgeRollingHash              [32]byte
	EndL1L2BridgeRollingHashMessageNumber uint64
	DynamicChainConfigHash                [32]byte
	ParentFtxRollingHash                  [32]byte
	ParentFtxNumber                       uint64
	EndFtxRollingHash                     [32]byte
	EndProcessedFtxNumber                 uint64
	FilteredAddresses                     [][20]byte
	TxFromsHash                           [32]byte
	BlockCount                            uint64
	L2MessagingBlocksOffsets              []uint16
	L2L1Messages                          [][32]byte
}

// L2ExecutionProof is an l2-execution proof the rollup guest verifies under
// ProgramVk. PublicInputsSSZ, when set, is the SSZ of its public inputs and is
// encoded as is, in place of PublicInputs.
type L2ExecutionProof struct {
	PublicInputs     L2ExecutionPublicInputs
	PublicInputsSSZ  []byte
	StartBlockNumber uint64
	Proof            []byte
	ProgramVk        [32]byte
}

// RollupInput is the rollup guest input. ParentBlobDataRollingHash is optional.
type RollupInput struct {
	ParentDataRollingHash              [32]byte
	ParentSharedBlobCarryOverTailBytes uint64
	ChainID                            uint64
	Conflations                        []ConflationWitness
	Chunks                             []ChunkWitness
	L2ExecutionProofs                  []L2ExecutionProof
	ParentBlobDataRollingHash          *[32]byte
}

// EncodeRollupInput encodes in as 0x0004 || SSZ(RollupProofPrivateInput).
func EncodeRollupInput(in RollupInput) ([]byte, error) {
	conflations, err := encodeList(in.Conflations, encodeConflationWitness)
	if err != nil {
		return nil, fmt.Errorf("rollup input conflations: %w", err)
	}
	chunks, err := encodeList(in.Chunks, encodeChunkWitness)
	if err != nil {
		return nil, fmt.Errorf("rollup input chunks: %w", err)
	}
	proofs, err := encodeList(in.L2ExecutionProofs, encodeL2ExecutionProof)
	if err != nil {
		return nil, fmt.Errorf("rollup input l2-execution proofs: %w", err)
	}
	// An optional hash is a list of at most one hash.
	var parentBlob []byte
	if in.ParentBlobDataRollingHash != nil {
		parentBlob = in.ParentBlobDataRollingHash[:]
	}
	body, err := sszContainer(
		fixed(in.ParentDataRollingHash[:]),
		fixed(sszUint64(in.ParentSharedBlobCarryOverTailBytes)),
		fixed(sszUint64(in.ChainID)),
		variable(conflations),
		variable(chunks),
		variable(proofs),
		variable(parentBlob),
	)
	if err != nil {
		return nil, fmt.Errorf("rollup input: %w", err)
	}
	return append(slices.Clone(rollupInputSchemaID), body...), nil
}

// encodeList encodes an SSZ list of variable-size elements.
func encodeList[T any](elems []T, encode func(T) ([]byte, error)) ([]byte, error) {
	encoded := make([][]byte, len(elems))
	for i, e := range elems {
		var err error
		if encoded[i], err = encode(e); err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
	}
	return sszListVariable(encoded)
}

func encodeConflationWitness(c ConflationWitness) ([]byte, error) {
	blockRlps, err := sszListVariable(c.BlockRlps)
	if err != nil {
		return nil, err
	}
	return sszContainer(variable(blockRlps))
}

func encodeChunkWitness(c ChunkWitness) ([]byte, error) {
	isCalldata := []byte{0}
	if c.IsCalldata {
		isCalldata[0] = 1
	}
	return sszContainer(
		fixed(c.ChunkHash[:]),
		fixed(isCalldata),
		variable(c.BlobBytes),
		variable(c.CalldataBytes),
		variable(c.KZGCommitment),
		variable(c.KZGProof),
	)
}

func encodeL2ExecutionProof(p L2ExecutionProof) ([]byte, error) {
	publicInputs := p.PublicInputsSSZ
	if publicInputs == nil {
		var err error
		if publicInputs, err = encodeL2ExecutionPublicInputs(p.PublicInputs); err != nil {
			return nil, err
		}
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

// encodeL2ExecutionPublicInputs uses the field order of the 0x0003 guest output.
func encodeL2ExecutionPublicInputs(pi L2ExecutionPublicInputs) ([]byte, error) {
	messages := make([][]byte, len(pi.L2L1Messages))
	for i := range pi.L2L1Messages {
		messages[i] = pi.L2L1Messages[i][:]
	}
	addresses := make([][]byte, len(pi.FilteredAddresses))
	for i := range pi.FilteredAddresses {
		addresses[i] = pi.FilteredAddresses[i][:]
	}
	offsets := make([][]byte, len(pi.L2MessagingBlocksOffsets))
	for i, o := range pi.L2MessagingBlocksOffsets {
		offsets[i] = binary.LittleEndian.AppendUint16(nil, o)
	}
	return sszContainer(
		fixed(pi.ParentBlockHash[:]),
		fixed(pi.EndBlockHash[:]),
		fixed(sszUint64(pi.EndBlockNumber)),
		fixed(sszUint64(pi.EndBlockTimestamp)),
		variable(sszListFixed(messages)),
		fixed(pi.EndL1L2BridgeRollingHash[:]),
		fixed(sszUint64(pi.EndL1L2BridgeRollingHashMessageNumber)),
		fixed(pi.DynamicChainConfigHash[:]),
		fixed(pi.ParentFtxRollingHash[:]),
		fixed(sszUint64(pi.ParentFtxNumber)),
		fixed(pi.EndFtxRollingHash[:]),
		fixed(sszUint64(pi.EndProcessedFtxNumber)),
		variable(sszListFixed(addresses)),
		fixed(pi.TxFromsHash[:]),
		fixed(sszUint64(pi.BlockCount)),
		variable(sszListFixed(offsets)),
	)
}
