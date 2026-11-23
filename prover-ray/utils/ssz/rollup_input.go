package ssz

import "encoding/binary"

// This file encodes the rollup guest input (schema 0x0004), mirroring
// rollup_spec/rollup_ssz.py::encode_rollup_input and the guest's
// riscv-guests/rollup/src/rollup_ssz.zig. Golden-vector pinned against the
// Python reference in backend/jobadapter/rollup_input_test.go.

// rollupInputSchemaID is the two-byte big-endian schema id framing the 0x0004
// rollup input.
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

// L2ExecutionPublicInputs are the public inputs of an l2-execution proof. The
// fields are those of backend.PublicInputs, in the same order, so that one
// converts to the other; the wire order is in encodeL2ExecutionPublicInputs.
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

// L2ExecutionProof is one l2-execution proof the rollup guest verifies, with
// the program VK it is verified under.
type L2ExecutionProof struct {
	PublicInputs     L2ExecutionPublicInputs
	StartBlockNumber uint64
	Proof            []byte
	ProgramVk        [32]byte
}

// RollupInput is the decoded RollupProofPrivateInput ready to encode.
// BoundaryPrevDataRollingHash is optional.
type RollupInput struct {
	ParentDataRollingHash       [32]byte
	ParentDataTailTakeBytes     uint64
	ChainID                     uint64
	Conflations                 []ConflationWitness
	Chunks                      []ChunkWitness
	L2ExecutionProofs           []L2ExecutionProof
	BoundaryPrevDataRollingHash *[32]byte
}

// EncodeRollupInput encodes a RollupInput into framed SSZ bytes (0x0004 schema
// id) that the rollup guest reads.
func EncodeRollupInput(in RollupInput) []byte {
	var w container
	w.fixed(in.ParentDataRollingHash[:])
	w.uint64(in.ParentDataTailTakeBytes)
	w.uint64(in.ChainID)
	w.variable(variableList(in.Conflations, encodeConflationWitness))
	w.variable(variableList(in.Chunks, encodeChunkWitness))
	w.variable(variableList(in.L2ExecutionProofs, encodeL2ExecutionProof))
	// An optional hash is a list of at most one hash.
	var boundary []byte
	if in.BoundaryPrevDataRollingHash != nil {
		boundary = in.BoundaryPrevDataRollingHash[:]
	}
	w.variable(boundary)

	out := append([]byte(nil), rollupInputSchemaID...)
	return append(out, w.bytes()...)
}

func encodeConflationWitness(c ConflationWitness) []byte {
	var w container
	w.variable(listWithOffsets(c.BlockRlps))
	return w.bytes()
}

func encodeChunkWitness(c ChunkWitness) []byte {
	var w container
	w.fixed(c.ChunkHash[:])
	if c.IsCalldata {
		w.fixed([]byte{1})
	} else {
		w.fixed([]byte{0})
	}
	w.variable(c.BlobBytes)
	w.variable(c.CalldataBytes)
	w.variable(c.KZGCommitment)
	w.variable(c.KZGProof)
	return w.bytes()
}

// encodeL2ExecutionProof encodes the verifiable proof: the proof (public
// inputs, start block, proof bytes), then the program VK.
func encodeL2ExecutionProof(p L2ExecutionProof) []byte {
	var proof container
	proof.variable(encodeL2ExecutionPublicInputs(p.PublicInputs))
	proof.uint64(p.StartBlockNumber)
	proof.variable(p.Proof)

	var w container
	w.variable(proof.bytes())
	w.fixed(p.ProgramVk[:])
	return w.bytes()
}

// encodeL2ExecutionPublicInputs encodes the public inputs in the wire order of
// the l2-execution guest output (0x0003).
func encodeL2ExecutionPublicInputs(pi L2ExecutionPublicInputs) []byte {
	var messages, addresses, offsets []byte
	for _, m := range pi.L2L1Messages {
		messages = append(messages, m[:]...)
	}
	for _, a := range pi.FilteredAddresses {
		addresses = append(addresses, a[:]...)
	}
	for _, o := range pi.L2MessagingBlocksOffsets {
		offsets = binary.LittleEndian.AppendUint16(offsets, o)
	}

	var w container
	w.fixed(pi.ParentBlockHash[:])
	w.fixed(pi.EndBlockHash[:])
	w.uint64(pi.EndBlockNumber)
	w.uint64(pi.EndBlockTimestamp)
	w.variable(messages)
	w.fixed(pi.EndL1L2BridgeRollingHash[:])
	w.uint64(pi.EndL1L2BridgeRollingHashMessageNumber)
	w.fixed(pi.DynamicChainConfigHash[:])
	w.fixed(pi.ParentFtxRollingHash[:])
	w.uint64(pi.ParentFtxNumber)
	w.fixed(pi.EndFtxRollingHash[:])
	w.uint64(pi.EndProcessedFtxNumber)
	w.variable(addresses)
	w.fixed(pi.TxFromsHash[:])
	w.uint64(pi.BlockCount)
	w.variable(offsets)
	return w.bytes()
}

// variableList encodes an SSZ list of variable-size elements.
func variableList[T any](elems []T, encode func(T) []byte) []byte {
	encoded := make([][]byte, len(elems))
	for i, e := range elems {
		encoded[i] = encode(e)
	}
	return listWithOffsets(encoded)
}

// container builds an SSZ container in field order: a fixed-size field goes in
// the fixed part, a variable-size one leaves a 4-byte offset there and its
// bytes follow the fixed part.
type container struct {
	head  []byte
	slots []int
	tails [][]byte
}

func (c *container) fixed(b []byte) { c.head = append(c.head, b...) }

func (c *container) uint64(v uint64) { c.head = binary.LittleEndian.AppendUint64(c.head, v) }

func (c *container) variable(b []byte) {
	c.slots = append(c.slots, len(c.head))
	c.head = append(c.head, 0, 0, 0, 0)
	c.tails = append(c.tails, b)
}

func (c *container) bytes() []byte {
	out := append([]byte(nil), c.head...)
	for i, slot := range c.slots {
		binary.LittleEndian.PutUint32(out[slot:], uint32(len(out)))
		out = append(out, c.tails[i]...)
	}
	return out
}
