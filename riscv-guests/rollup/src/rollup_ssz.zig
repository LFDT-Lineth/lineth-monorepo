//! Wire contract of the rollup guest's input: the `RollupProofPrivateInput` container behind the
//! 0x0004 schema id. The decoder accepts only the canonical encoding, and the byte lists alias the
//! decoded buffer.
//!
//! The already-proven l2-execution proofs the input embeds are core_serde's contract, and so is the
//! guest's output (`core_serde.rollup_proof`).

const core_serde = @import("core_serde");
const lineth_da = @import("lineth_da");
const ssz = @import("guest_common").ssz;

const Allocator = @import("std").mem.Allocator;
const l2_execution_proof = core_serde.l2_execution_proof;
const rollup_proof = core_serde.rollup_proof;

pub const INPUT_SCHEMA_ID: u16 = 0x0004;

// List bounds of the wire schema: encoders and decoders reject longer lists.
/// Conflations one rollup proof covers.
pub const MAX_CONFLATIONS_PER_ROLLUP: usize = 1 << 10;
/// Paired one to one with the conflations.
pub const MAX_L2_EXECUTION_PROOFS_PER_ROLLUP: usize = rollup_proof.MAX_L2_EXECUTION_PROOFS_PER_ROLLUP;
/// Chunks one rollup proof touches.
pub const MAX_CHUNKS_PER_ROLLUP: usize = 1 << 12;
/// Full block RLPs (one per block) of one conflation.
pub const MAX_BLOCK_RLPS_PER_CONFLATION: usize = 1 << 12;
/// 16 MiB: a full canonical block RLP including all transaction bodies.
pub const MAX_BYTES_PER_BLOCK_RLP: usize = 1 << 24;
/// 16 MiB: a single calldata submission.
pub const MAX_CALLDATA_BYTES_PER_CHUNK: usize = lineth_da.decompress.max_calldata_bytes;
/// An EIP-4844 blob: 4096 field elements of 32 bytes.
pub const BLOB_BYTES_LENGTH = lineth_da.encoding.blob_kzg.blob_len;
pub const BYTES_PER_COMMITMENT = lineth_da.encoding.blob_kzg.commitment_len;
pub const BYTES_PER_PROOF = lineth_da.encoding.blob_kzg.proof_len;

pub const ConflationWitness = struct {
    /// The canonical full block RLPs of the conflation, one per block.
    block_rlps: ssz.VariableList(ssz.ByteList(MAX_BYTES_PER_BLOCK_RLP), MAX_BLOCK_RLPS_PER_CONFLATION),
};

/// One touched chunk: its anchored hash, its declared kind and the bytes it is checked against. A
/// blob carries its physical bytes, commitment and proof; calldata its exact submission.
pub const ChunkWitness = lineth_da.decompress.Chunk;

pub const RollupProofPrivateInput = struct {
    parent_data_rolling_hash: [32]u8,
    parent_data_tail_take_bytes: u64,
    chain_id: u64,
    conflations: ssz.VariableList(ConflationWitness, MAX_CONFLATIONS_PER_ROLLUP),
    chunks: ssz.VariableList(ChunkWitness, MAX_CHUNKS_PER_ROLLUP),
    l2_execution_proofs: ssz.VariableList(l2_execution_proof.VerifiableL2ExecutionProof, MAX_L2_EXECUTION_PROOFS_PER_ROLLUP),
    /// At most one hash: absent is the empty list, present the single hash.
    boundary_prev_data_rolling_hash: ssz.FixedList([32]u8, 1),
};

/// The input in `data`, the 0x0004 schema id followed by the SSZ container.
pub fn decodeInput(alloc: Allocator, data: []const u8) ssz.FramedDecodeError!RollupProofPrivateInput {
    return ssz.decodeFramed(RollupProofPrivateInput, alloc, data, INPUT_SCHEMA_ID);
}

/// The frame `decodeInput` reads.
pub fn encodeInput(alloc: Allocator, input: RollupProofPrivateInput) ssz.EncodeError![]u8 {
    return ssz.encodeFramed(RollupProofPrivateInput, alloc, INPUT_SCHEMA_ID, input);
}
