//! SSZ codec for the rollup guest wire format: `SszRollupProofPrivateInput`/`SszRollupOutput`,
//! schema ids 0x1001/0x1801.
//!
//! Frame: 2-byte big-endian schema id || SSZ container bytes (SSZ itself little-endian). This
//! guest's own tests round-trip both the input and output containers byte-for-byte using this
//! module's own `encodeInput`/`decodeInput` and `encodeOutput`/`decodeOutput` — there is no
//! external fixture to match; `rollup_spec/src/rollup_spec/rollup_ssz.py` is an illustrative
//! reference implementation of the same schema, not an authority this codec is checked against.
//!
//! Every `List` field's wire encoding follows plain SSZ: a list of VARIABLE-size elements
//! (`conflations`, `l2_execution_proofs`) is an offset table over its region — the generic
//! `guest_common.ssz.decodeVariableList`/`encodeVariableList` every riscv-guests SSZ codec shares.
//! A list of FIXED-size elements (`chunks`, `program_vks`, `l2_l1_roots`, `filtered_addresses`, the
//! optional-as-list `boundary_prev_data_rolling_hash`) is just the concatenation of its elements
//! with no per-element offset table; element count is the region's byte length divided by the
//! element size (the `decodeBytes32List`/`decodeAddressList` pair below, local to this codec since
//! guest_common only carries the variable-size-element list convention).

const std = @import("std");
const guest_common = @import("guest_common");

pub const INPUT_SCHEMA_ID: u16 = 0x1001;
pub const OUTPUT_SCHEMA_ID: u16 = 0x1801;
const SCHEMA_ID_SIZE: usize = 2;

// ── SSZ list/vector bounds (mirrors rollup_ssz.py's MAX_* constants) ─────────────────────────────
pub const MAX_CONFLATIONS_PER_ROLLUP: usize = 1 << 10;
pub const MAX_L2_EXECUTION_PROOFS_PER_ROLLUP: usize = 1 << 10;
pub const MAX_CHUNKS_PER_ROLLUP: usize = 1 << 12;
pub const MAX_BLOCK_RLPS_PER_CONFLATION: usize = 1 << 12;
pub const MAX_BYTES_PER_BLOCK_RLP: usize = 1 << 24;
pub const MAX_PROOF_BYTES: usize = 1 << 24;
pub const MAX_PROGRAM_VKS: usize = 1 << 10;
pub const MAX_L2_L1_ROOTS: usize = 1 << 16;
pub const MAX_FILTERED_ADDRESSES: usize = 1 << 16;
pub const MAX_L2_L1_MESSAGES_PER_EXEC_PROOF: usize = 1 << 16;
pub const MAX_TX_FROMS_PER_EXEC_PROOF: usize = 1 << 16;
pub const MAX_FILTERED_ADDRESSES_PER_EXEC_PROOF: usize = 1 << 16;
/// `opaque_prefix_bytes`/`opaque_suffix_bytes` are each strictly shorter than one chunk, so the
/// chunk byte size (4096 32-byte words) is the wire format's bound for both.
pub const BLOB_BYTES_LENGTH: usize = 4096 * 32;

// ── Logical values ────────────────────────────────────────────────────────────────────────────

pub const ConflationWitness = struct {
    /// Zero-copy slices into the decoded input buffer.
    block_rlps: []const []const u8,
};

/// The 16-field l2-execution public-input tuple, in wire order.
pub const L2ExecutionProofPublicInput = struct {
    parent_block_hash: [32]u8,
    end_block_hash: [32]u8,
    end_block_number: u64,
    end_block_timestamp: u64,
    l2_l1_messages_hash: [32]u8,
    parent_l1_l2_bridge_rolling_hash: [32]u8,
    parent_l1_l2_bridge_rolling_hash_message_number: u64,
    end_l1_l2_bridge_rolling_hash: [32]u8,
    end_l1_l2_bridge_rolling_hash_message_number: u64,
    dynamic_chain_config_hash: [32]u8,
    parent_ftx_rolling_hash: [32]u8,
    parent_ftx_number: u64,
    end_ftx_rolling_hash: [32]u8,
    end_processed_ftx_number: u64,
    filtered_addresses_hash: [32]u8,
    tx_froms_hash: [32]u8,
};

pub const L2ExecutionProof = struct {
    public_inputs: L2ExecutionProofPublicInput,
    start_block_number: u64,
    /// Zero-copy slice into the decoded input buffer.
    proof: []const u8,
    l2_l1_messages: []const [32]u8,
    tx_froms: []const [20]u8,
    filtered_addresses: []const [20]u8,
};

pub const VerifiableL2ExecutionProof = struct {
    proof: L2ExecutionProof,
    program_vk: [32]u8,
};

/// The rollup guest's own input: an already-proven contiguous run of l2-execution proofs plus the
/// conflation/chunk witnesses the real rollup guest would recursively verify and fold (this stub
/// decodes but never verifies or folds them).
pub const RollupProofPrivateInput = struct {
    parent_data_rolling_hash: [32]u8,
    start_offset: u64,
    chain_id: u64,
    conflations: []const ConflationWitness,
    chunks: []const [32]u8,
    l2_execution_proofs: []const VerifiableL2ExecutionProof,
    /// Zero-copy slices into the decoded input buffer.
    opaque_prefix_bytes: []const u8,
    opaque_suffix_bytes: []const u8,
    /// `null` when the wire `boundary_prev_data_rolling_hash` list is empty — the wire format's
    /// `Optional[Hash32]` modelling (absent is the empty list, present is a single-element list).
    boundary_prev_data_rolling_hash: ?[32]u8,
};

/// The 20-field rollup/rollup-aggregation public-input tuple, in wire order. Every field but
/// `program_vks` is fixed-size.
pub const RollupPublicInput = struct {
    end_block_number: u64,
    end_block_timestamp: u64,
    l2_l1_bridge_transaction_tree: [32]u8,
    parent_l1_l2_bridge_rolling_hash: [32]u8,
    parent_l1_l2_bridge_rolling_hash_message_number: u64,
    end_l1_l2_bridge_rolling_hash: [32]u8,
    end_l1_l2_bridge_rolling_hash_message_number: u64,
    dynamic_chain_config_hash: [32]u8,
    parent_ftx_rolling_hash: [32]u8,
    parent_ftx_number: u64,
    end_ftx_rolling_hash: [32]u8,
    end_processed_ftx_number: u64,
    filtered_addresses_hash: [32]u8,
    parent_data_rolling_hash: [32]u8,
    end_data_rolling_hash: [32]u8,
    parent_block_hash: [32]u8,
    end_block_hash: [32]u8,
    start_offset: u64,
    end_offset: u64,
    program_vks: []const [32]u8,
};

/// The rollup guest's own output: `RollupProof` with `proof` omitted (the prover layer attaches it
/// separately — a guest cannot attest its own proof).
pub const RollupOutput = struct {
    public_inputs: RollupPublicInput,
    start_block_number: u64,
    l2_l1_roots: []const [32]u8,
    filtered_addresses: []const [20]u8,
};

// ── Primitive reads/writes and the generic "List[VariableSizeType, N]" codec ─────────────────────
// Little-endian integer reads/writes and the offset-table list codec live in guest_common.ssz:
// identical machinery every riscv-guests SSZ codec builds on.
const readU32 = guest_common.ssz.readU32;
const readU64 = guest_common.ssz.readU64;
const writeU32 = guest_common.ssz.writeU32;
const writeU64 = guest_common.ssz.writeU64;
const decodeVariableList = guest_common.ssz.decodeVariableList;

inline fn getHash(bytes: []const u8, pos: *usize) [32]u8 {
    var out: [32]u8 = undefined;
    @memcpy(&out, bytes[pos.*..][0..32]);
    pos.* += 32;
    return out;
}

inline fn getU64(bytes: []const u8, pos: *usize) u64 {
    const v = readU64(bytes, pos.*);
    pos.* += 8;
    return v;
}

inline fn putHash(out: []u8, pos: *usize, value: [32]u8) void {
    @memcpy(out[pos.*..][0..32], &value);
    pos.* += 32;
}

inline fn putU64(out: []u8, pos: *usize, value: u64) void {
    writeU64(out, pos.*, value);
    pos.* += 8;
}

// ── Fixed-size-element list codec ─────────────────────────────────────────────────────────────
// SSZ encodes `List[FixedSizeType, N]` as the plain concatenation of its elements — no offset
// table — so element count is the region's byte length divided by the element size.

fn decodeBytes32List(alloc: std.mem.Allocator, data: []const u8, max_len: usize) ![]const [32]u8 {
    if (data.len % 32 != 0) return error.InvalidSsz;
    const n = data.len / 32;
    if (n > max_len) return error.BoundsViolation;
    const out = try alloc.alloc([32]u8, n);
    for (0..n) |i| @memcpy(&out[i], data[i * 32 ..][0..32]);
    return out;
}

fn encodeBytes32List(alloc: std.mem.Allocator, items: []const [32]u8) ![]u8 {
    const out = try alloc.alloc(u8, items.len * 32);
    for (items, 0..) |item, i| @memcpy(out[i * 32 ..][0..32], &item);
    return out;
}

fn decodeAddressList(alloc: std.mem.Allocator, data: []const u8, max_len: usize) ![]const [20]u8 {
    if (data.len % 20 != 0) return error.InvalidSsz;
    const n = data.len / 20;
    if (n > max_len) return error.BoundsViolation;
    const out = try alloc.alloc([20]u8, n);
    for (0..n) |i| @memcpy(&out[i], data[i * 20 ..][0..20]);
    return out;
}

fn encodeAddressList(alloc: std.mem.Allocator, items: []const [20]u8) ![]u8 {
    const out = try alloc.alloc(u8, items.len * 20);
    for (items, 0..) |item, i| @memcpy(out[i * 20 ..][0..20], &item);
    return out;
}

// ── ConflationWitness ─────────────────────────────────────────────────────────────────────────
// Fixed head: block_rlps offset(4) = 4 — the only field, so this container is entirely variable.
const CONFLATION_WITNESS_FIXED_SIZE: usize = 4;

fn decodeConflationWitness(alloc: std.mem.Allocator, bytes: []const u8) !ConflationWitness {
    if (bytes.len < CONFLATION_WITNESS_FIXED_SIZE) return error.InvalidSsz;
    const off = readU32(bytes, 0);
    if (off != CONFLATION_WITNESS_FIXED_SIZE or off > bytes.len) return error.InvalidSsz;
    const block_rlps = try decodeVariableList(alloc, bytes[off..], MAX_BLOCK_RLPS_PER_CONFLATION);
    for (block_rlps) |rlp| {
        if (rlp.len > MAX_BYTES_PER_BLOCK_RLP) return error.BoundsViolation;
    }
    return .{ .block_rlps = block_rlps };
}

// ── L2ExecutionProofPublicInput (16 fixed fields, 368 bytes, no offsets) ─────────────────────────
const EXEC_PI_FIXED_SIZE: usize = 368;

fn decodeExecPublicInput(bytes: []const u8) !L2ExecutionProofPublicInput {
    if (bytes.len < EXEC_PI_FIXED_SIZE) return error.InvalidSsz;
    var pos: usize = 0;
    var v: L2ExecutionProofPublicInput = undefined;
    v.parent_block_hash = getHash(bytes, &pos);
    v.end_block_hash = getHash(bytes, &pos);
    v.end_block_number = getU64(bytes, &pos);
    v.end_block_timestamp = getU64(bytes, &pos);
    v.l2_l1_messages_hash = getHash(bytes, &pos);
    v.parent_l1_l2_bridge_rolling_hash = getHash(bytes, &pos);
    v.parent_l1_l2_bridge_rolling_hash_message_number = getU64(bytes, &pos);
    v.end_l1_l2_bridge_rolling_hash = getHash(bytes, &pos);
    v.end_l1_l2_bridge_rolling_hash_message_number = getU64(bytes, &pos);
    v.dynamic_chain_config_hash = getHash(bytes, &pos);
    v.parent_ftx_rolling_hash = getHash(bytes, &pos);
    v.parent_ftx_number = getU64(bytes, &pos);
    v.end_ftx_rolling_hash = getHash(bytes, &pos);
    v.end_processed_ftx_number = getU64(bytes, &pos);
    v.filtered_addresses_hash = getHash(bytes, &pos);
    v.tx_froms_hash = getHash(bytes, &pos);
    std.debug.assert(pos == EXEC_PI_FIXED_SIZE);
    return v;
}

// ── L2ExecutionProof ──────────────────────────────────────────────────────────────────────────
// Fixed head: public_inputs(368, inline — itself fully fixed) + start_block_number(8) +
// proof offset(4) + l2_l1_messages offset(4) + tx_froms offset(4) + filtered_addresses offset(4).
const EXEC_PROOF_FIXED_SIZE: usize = EXEC_PI_FIXED_SIZE + 8 + 4 + 4 + 4 + 4;

fn decodeL2ExecutionProof(alloc: std.mem.Allocator, bytes: []const u8) !L2ExecutionProof {
    if (bytes.len < EXEC_PROOF_FIXED_SIZE) return error.InvalidSsz;
    const public_inputs = try decodeExecPublicInput(bytes[0..EXEC_PI_FIXED_SIZE]);
    const start_block_number = readU64(bytes, EXEC_PI_FIXED_SIZE);
    const off_proof = readU32(bytes, EXEC_PI_FIXED_SIZE + 8);
    const off_messages = readU32(bytes, EXEC_PI_FIXED_SIZE + 12);
    const off_tx_froms = readU32(bytes, EXEC_PI_FIXED_SIZE + 16);
    const off_filtered = readU32(bytes, EXEC_PI_FIXED_SIZE + 20);
    if (off_proof != EXEC_PROOF_FIXED_SIZE) return error.InvalidSsz;
    if (off_messages < off_proof or off_tx_froms < off_messages or off_filtered < off_tx_froms) {
        return error.InvalidSsz;
    }
    if (off_filtered > bytes.len) return error.InvalidSsz;

    const proof = bytes[off_proof..off_messages];
    if (proof.len > MAX_PROOF_BYTES) return error.BoundsViolation;
    const l2_l1_messages = try decodeBytes32List(alloc, bytes[off_messages..off_tx_froms], MAX_L2_L1_MESSAGES_PER_EXEC_PROOF);
    const tx_froms = try decodeAddressList(alloc, bytes[off_tx_froms..off_filtered], MAX_TX_FROMS_PER_EXEC_PROOF);
    const filtered_addresses = try decodeAddressList(alloc, bytes[off_filtered..], MAX_FILTERED_ADDRESSES_PER_EXEC_PROOF);

    return .{
        .public_inputs = public_inputs,
        .start_block_number = start_block_number,
        .proof = proof,
        .l2_l1_messages = l2_l1_messages,
        .tx_froms = tx_froms,
        .filtered_addresses = filtered_addresses,
    };
}

// ── VerifiableL2ExecutionProof ────────────────────────────────────────────────────────────────
// Fixed head: proof offset(4) + program_vk(32, inline) = 36.
const VERIFIABLE_EXEC_PROOF_FIXED_SIZE: usize = 36;

fn decodeVerifiableL2ExecutionProof(alloc: std.mem.Allocator, bytes: []const u8) !VerifiableL2ExecutionProof {
    if (bytes.len < VERIFIABLE_EXEC_PROOF_FIXED_SIZE) return error.InvalidSsz;
    const off_proof = readU32(bytes, 0);
    if (off_proof != VERIFIABLE_EXEC_PROOF_FIXED_SIZE or off_proof > bytes.len) return error.InvalidSsz;
    var program_vk: [32]u8 = undefined;
    @memcpy(&program_vk, bytes[4..36]);
    const proof = try decodeL2ExecutionProof(alloc, bytes[off_proof..]);
    return .{ .proof = proof, .program_vk = program_vk };
}

// ── RollupProofPrivateInput (the rollup guest INPUT) ─────────────────────────────────────────────
// Fixed head: parent_data_rolling_hash(32) + start_offset(8) + chain_id(8) + 6 offsets(4 each).
const INPUT_FIXED_SIZE: usize = 32 + 8 + 8 + 4 * 6;

/// Decode the rollup guest input: the 0x1001 schema id followed by the SSZ
/// `SszRollupProofPrivateInput`. Strict: rejects a wrong schema id, a too-short frame, a
/// misaligned/out-of-order/out-of-bounds offset, or a list exceeding its wire-format bound.
pub fn decodeInput(alloc: std.mem.Allocator, data: []const u8) !RollupProofPrivateInput {
    if (data.len < SCHEMA_ID_SIZE) return error.MalformedFrame;
    if (std.mem.readInt(u16, data[0..2], .big) != INPUT_SCHEMA_ID) return error.MalformedFrame;

    const body = data[SCHEMA_ID_SIZE..];
    if (body.len < INPUT_FIXED_SIZE) return error.InvalidSsz;

    var parent_data_rolling_hash: [32]u8 = undefined;
    @memcpy(&parent_data_rolling_hash, body[0..32]);
    const start_offset = readU64(body, 32);
    const chain_id = readU64(body, 40);

    const off_conflations = readU32(body, 48);
    const off_chunks = readU32(body, 52);
    const off_proofs = readU32(body, 56);
    const off_prefix = readU32(body, 60);
    const off_suffix = readU32(body, 64);
    const off_boundary = readU32(body, 68);

    if (off_conflations != INPUT_FIXED_SIZE) return error.InvalidSsz;
    if (off_chunks < off_conflations or off_proofs < off_chunks or off_prefix < off_proofs or
        off_suffix < off_prefix or off_boundary < off_suffix)
    {
        return error.InvalidSsz;
    }
    if (off_boundary > body.len) return error.InvalidSsz;

    const conflation_slices = try decodeVariableList(alloc, body[off_conflations..off_chunks], MAX_CONFLATIONS_PER_ROLLUP);
    const conflations = try alloc.alloc(ConflationWitness, conflation_slices.len);
    for (conflation_slices, 0..) |slice, i| conflations[i] = try decodeConflationWitness(alloc, slice);

    const chunks = try decodeBytes32List(alloc, body[off_chunks..off_proofs], MAX_CHUNKS_PER_ROLLUP);

    const proof_slices = try decodeVariableList(alloc, body[off_proofs..off_prefix], MAX_L2_EXECUTION_PROOFS_PER_ROLLUP);
    const l2_execution_proofs = try alloc.alloc(VerifiableL2ExecutionProof, proof_slices.len);
    for (proof_slices, 0..) |slice, i| l2_execution_proofs[i] = try decodeVerifiableL2ExecutionProof(alloc, slice);

    const opaque_prefix_bytes = body[off_prefix..off_suffix];
    if (opaque_prefix_bytes.len > BLOB_BYTES_LENGTH) return error.BoundsViolation;
    const opaque_suffix_bytes = body[off_suffix..off_boundary];
    if (opaque_suffix_bytes.len > BLOB_BYTES_LENGTH) return error.BoundsViolation;

    const boundary_list = try decodeBytes32List(alloc, body[off_boundary..], 1);
    const boundary_prev_data_rolling_hash: ?[32]u8 = if (boundary_list.len == 1) boundary_list[0] else null;

    return .{
        .parent_data_rolling_hash = parent_data_rolling_hash,
        .start_offset = start_offset,
        .chain_id = chain_id,
        .conflations = conflations,
        .chunks = chunks,
        .l2_execution_proofs = l2_execution_proofs,
        .opaque_prefix_bytes = opaque_prefix_bytes,
        .opaque_suffix_bytes = opaque_suffix_bytes,
        .boundary_prev_data_rolling_hash = boundary_prev_data_rolling_hash,
    };
}

fn encodeExecPublicInput(alloc: std.mem.Allocator, v: L2ExecutionProofPublicInput) ![]u8 {
    const out = try alloc.alloc(u8, EXEC_PI_FIXED_SIZE);
    var pos: usize = 0;
    putHash(out, &pos, v.parent_block_hash);
    putHash(out, &pos, v.end_block_hash);
    putU64(out, &pos, v.end_block_number);
    putU64(out, &pos, v.end_block_timestamp);
    putHash(out, &pos, v.l2_l1_messages_hash);
    putHash(out, &pos, v.parent_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, v.parent_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, v.end_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, v.end_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, v.dynamic_chain_config_hash);
    putHash(out, &pos, v.parent_ftx_rolling_hash);
    putU64(out, &pos, v.parent_ftx_number);
    putHash(out, &pos, v.end_ftx_rolling_hash);
    putU64(out, &pos, v.end_processed_ftx_number);
    putHash(out, &pos, v.filtered_addresses_hash);
    putHash(out, &pos, v.tx_froms_hash);
    std.debug.assert(pos == EXEC_PI_FIXED_SIZE);
    return out;
}

fn encodeL2ExecutionProof(alloc: std.mem.Allocator, v: L2ExecutionProof) ![]u8 {
    const pi_bytes = try encodeExecPublicInput(alloc, v.public_inputs);
    const messages_bytes = try encodeBytes32List(alloc, v.l2_l1_messages);
    const tx_froms_bytes = try encodeAddressList(alloc, v.tx_froms);
    const filtered_bytes = try encodeAddressList(alloc, v.filtered_addresses);

    const off_proof = EXEC_PROOF_FIXED_SIZE;
    const off_messages = off_proof + v.proof.len;
    const off_tx_froms = off_messages + messages_bytes.len;
    const off_filtered = off_tx_froms + tx_froms_bytes.len;

    const out = try alloc.alloc(u8, off_filtered + filtered_bytes.len);
    @memcpy(out[0..EXEC_PI_FIXED_SIZE], pi_bytes);
    var pos: usize = EXEC_PI_FIXED_SIZE;
    putU64(out, &pos, v.start_block_number);
    writeU32(out, pos, @intCast(off_proof));
    pos += 4;
    writeU32(out, pos, @intCast(off_messages));
    pos += 4;
    writeU32(out, pos, @intCast(off_tx_froms));
    pos += 4;
    writeU32(out, pos, @intCast(off_filtered));
    pos += 4;
    std.debug.assert(pos == EXEC_PROOF_FIXED_SIZE);
    @memcpy(out[off_proof..][0..v.proof.len], v.proof);
    @memcpy(out[off_messages..][0..messages_bytes.len], messages_bytes);
    @memcpy(out[off_tx_froms..][0..tx_froms_bytes.len], tx_froms_bytes);
    @memcpy(out[off_filtered..], filtered_bytes);
    return out;
}

fn encodeVerifiableL2ExecutionProof(alloc: std.mem.Allocator, v: VerifiableL2ExecutionProof) ![]u8 {
    const proof_bytes = try encodeL2ExecutionProof(alloc, v.proof);
    const out = try alloc.alloc(u8, VERIFIABLE_EXEC_PROOF_FIXED_SIZE + proof_bytes.len);
    writeU32(out, 0, @intCast(VERIFIABLE_EXEC_PROOF_FIXED_SIZE));
    @memcpy(out[4..36], &v.program_vk);
    @memcpy(out[36..], proof_bytes);
    return out;
}

fn encodeConflationWitness(alloc: std.mem.Allocator, v: ConflationWitness) ![]u8 {
    const list_bytes = try guest_common.ssz.encodeVariableList(alloc, v.block_rlps);
    const out = try alloc.alloc(u8, CONFLATION_WITNESS_FIXED_SIZE + list_bytes.len);
    writeU32(out, 0, @intCast(CONFLATION_WITNESS_FIXED_SIZE));
    @memcpy(out[CONFLATION_WITNESS_FIXED_SIZE..], list_bytes);
    return out;
}

/// Encode the rollup guest input: the 0x1001 schema id followed by the SSZ
/// `SszRollupProofPrivateInput`. Not used by the guest itself at runtime (it only ever decodes) —
/// kept so the input codec's byte-exact round-trip can be asserted against `decodeInput` in this
/// guest's own tests, from literal readable Zig values rather than an externally-produced fixture.
pub fn encodeInput(alloc: std.mem.Allocator, v: RollupProofPrivateInput) ![]u8 {
    const conflation_blobs = try alloc.alloc([]const u8, v.conflations.len);
    for (v.conflations, 0..) |c, i| conflation_blobs[i] = try encodeConflationWitness(alloc, c);
    const conflations_bytes = try guest_common.ssz.encodeVariableList(alloc, conflation_blobs);

    const chunks_bytes = try encodeBytes32List(alloc, v.chunks);

    const proof_blobs = try alloc.alloc([]const u8, v.l2_execution_proofs.len);
    for (v.l2_execution_proofs, 0..) |p, i| proof_blobs[i] = try encodeVerifiableL2ExecutionProof(alloc, p);
    const proofs_bytes = try guest_common.ssz.encodeVariableList(alloc, proof_blobs);

    var boundary_buf: [1][32]u8 = undefined;
    const boundary_slice: []const [32]u8 = if (v.boundary_prev_data_rolling_hash) |h| blk: {
        boundary_buf[0] = h;
        break :blk boundary_buf[0..1];
    } else &.{};
    const boundary_bytes = try encodeBytes32List(alloc, boundary_slice);

    const off_conflations = INPUT_FIXED_SIZE;
    const off_chunks = off_conflations + conflations_bytes.len;
    const off_proofs = off_chunks + chunks_bytes.len;
    const off_prefix = off_proofs + proofs_bytes.len;
    const off_suffix = off_prefix + v.opaque_prefix_bytes.len;
    const off_boundary = off_suffix + v.opaque_suffix_bytes.len;
    const body_len = off_boundary + boundary_bytes.len;

    const out = try alloc.alloc(u8, SCHEMA_ID_SIZE + body_len);
    std.mem.writeInt(u16, out[0..2], INPUT_SCHEMA_ID, .big);
    const body = out[SCHEMA_ID_SIZE..];

    var pos: usize = 0;
    putHash(body, &pos, v.parent_data_rolling_hash);
    putU64(body, &pos, v.start_offset);
    putU64(body, &pos, v.chain_id);
    writeU32(body, pos, @intCast(off_conflations));
    pos += 4;
    writeU32(body, pos, @intCast(off_chunks));
    pos += 4;
    writeU32(body, pos, @intCast(off_proofs));
    pos += 4;
    writeU32(body, pos, @intCast(off_prefix));
    pos += 4;
    writeU32(body, pos, @intCast(off_suffix));
    pos += 4;
    writeU32(body, pos, @intCast(off_boundary));
    pos += 4;
    std.debug.assert(pos == INPUT_FIXED_SIZE);

    @memcpy(body[off_conflations..][0..conflations_bytes.len], conflations_bytes);
    @memcpy(body[off_chunks..][0..chunks_bytes.len], chunks_bytes);
    @memcpy(body[off_proofs..][0..proofs_bytes.len], proofs_bytes);
    @memcpy(body[off_prefix..][0..v.opaque_prefix_bytes.len], v.opaque_prefix_bytes);
    @memcpy(body[off_suffix..][0..v.opaque_suffix_bytes.len], v.opaque_suffix_bytes);
    @memcpy(body[off_boundary..], boundary_bytes);

    return out;
}

// ── RollupPublicInput (20 fields; only `program_vks` is variable) ────────────────────────────────
// Fixed head: 19 fixed fields (11 hashes * 32 + 8 u64s * 8 = 416) + program_vks offset(4) = 420.
const ROLLUP_PI_FIXED_SIZE: usize = 420;

fn decodeRollupPublicInput(alloc: std.mem.Allocator, bytes: []const u8) !RollupPublicInput {
    if (bytes.len < ROLLUP_PI_FIXED_SIZE) return error.InvalidSsz;
    var pos: usize = 0;
    var v: RollupPublicInput = undefined;
    v.end_block_number = getU64(bytes, &pos);
    v.end_block_timestamp = getU64(bytes, &pos);
    v.l2_l1_bridge_transaction_tree = getHash(bytes, &pos);
    v.parent_l1_l2_bridge_rolling_hash = getHash(bytes, &pos);
    v.parent_l1_l2_bridge_rolling_hash_message_number = getU64(bytes, &pos);
    v.end_l1_l2_bridge_rolling_hash = getHash(bytes, &pos);
    v.end_l1_l2_bridge_rolling_hash_message_number = getU64(bytes, &pos);
    v.dynamic_chain_config_hash = getHash(bytes, &pos);
    v.parent_ftx_rolling_hash = getHash(bytes, &pos);
    v.parent_ftx_number = getU64(bytes, &pos);
    v.end_ftx_rolling_hash = getHash(bytes, &pos);
    v.end_processed_ftx_number = getU64(bytes, &pos);
    v.filtered_addresses_hash = getHash(bytes, &pos);
    v.parent_data_rolling_hash = getHash(bytes, &pos);
    v.end_data_rolling_hash = getHash(bytes, &pos);
    v.parent_block_hash = getHash(bytes, &pos);
    v.end_block_hash = getHash(bytes, &pos);
    v.start_offset = getU64(bytes, &pos);
    v.end_offset = getU64(bytes, &pos);
    std.debug.assert(pos == ROLLUP_PI_FIXED_SIZE - 4);

    const off_vks = readU32(bytes, pos);
    if (off_vks != ROLLUP_PI_FIXED_SIZE or off_vks > bytes.len) return error.InvalidSsz;
    v.program_vks = try decodeBytes32List(alloc, bytes[off_vks..], MAX_PROGRAM_VKS);
    return v;
}

fn encodeRollupPublicInput(alloc: std.mem.Allocator, v: RollupPublicInput) ![]u8 {
    const vks_bytes = try encodeBytes32List(alloc, v.program_vks);
    const out = try alloc.alloc(u8, ROLLUP_PI_FIXED_SIZE + vks_bytes.len);

    var pos: usize = 0;
    putU64(out, &pos, v.end_block_number);
    putU64(out, &pos, v.end_block_timestamp);
    putHash(out, &pos, v.l2_l1_bridge_transaction_tree);
    putHash(out, &pos, v.parent_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, v.parent_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, v.end_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, v.end_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, v.dynamic_chain_config_hash);
    putHash(out, &pos, v.parent_ftx_rolling_hash);
    putU64(out, &pos, v.parent_ftx_number);
    putHash(out, &pos, v.end_ftx_rolling_hash);
    putU64(out, &pos, v.end_processed_ftx_number);
    putHash(out, &pos, v.filtered_addresses_hash);
    putHash(out, &pos, v.parent_data_rolling_hash);
    putHash(out, &pos, v.end_data_rolling_hash);
    putHash(out, &pos, v.parent_block_hash);
    putHash(out, &pos, v.end_block_hash);
    putU64(out, &pos, v.start_offset);
    putU64(out, &pos, v.end_offset);
    writeU32(out, pos, @intCast(ROLLUP_PI_FIXED_SIZE));
    pos += 4;
    std.debug.assert(pos == ROLLUP_PI_FIXED_SIZE);
    @memcpy(out[ROLLUP_PI_FIXED_SIZE..], vks_bytes);
    return out;
}

// ── RollupOutput (the rollup guest OUTPUT) ───────────────────────────────────────────────────────
// Fixed head: public_inputs offset(4) + start_block_number(8) + l2_l1_roots offset(4) +
// filtered_addresses offset(4) = 20.
const OUTPUT_FIXED_SIZE: usize = 4 + 8 + 4 + 4;

/// Encode the rollup guest's actual wire output: the 0x1801 schema id followed by the SSZ
/// `SszRollupOutput`.
pub fn encodeOutput(alloc: std.mem.Allocator, v: RollupOutput) ![]u8 {
    const pi_bytes = try encodeRollupPublicInput(alloc, v.public_inputs);
    const roots_bytes = try encodeBytes32List(alloc, v.l2_l1_roots);
    const filtered_bytes = try encodeAddressList(alloc, v.filtered_addresses);

    const body_len = OUTPUT_FIXED_SIZE + pi_bytes.len + roots_bytes.len + filtered_bytes.len;
    const out = try alloc.alloc(u8, SCHEMA_ID_SIZE + body_len);
    std.mem.writeInt(u16, out[0..2], OUTPUT_SCHEMA_ID, .big);
    const body = out[SCHEMA_ID_SIZE..];

    const off_roots = OUTPUT_FIXED_SIZE + pi_bytes.len;
    const off_filtered = off_roots + roots_bytes.len;
    writeU32(body, 0, @intCast(OUTPUT_FIXED_SIZE));
    writeU64(body, 4, v.start_block_number);
    writeU32(body, 12, @intCast(off_roots));
    writeU32(body, 16, @intCast(off_filtered));

    @memcpy(body[OUTPUT_FIXED_SIZE..][0..pi_bytes.len], pi_bytes);
    @memcpy(body[off_roots..][0..roots_bytes.len], roots_bytes);
    @memcpy(body[off_filtered..], filtered_bytes);
    return out;
}

/// Decode a rollup guest output frame. Not used by the guest itself at runtime (it only ever
/// encodes) — kept so the output codec's byte-exact round-trip can be asserted against
/// `encodeOutput` in this guest's own tests.
pub fn decodeOutput(alloc: std.mem.Allocator, data: []const u8) !RollupOutput {
    if (data.len < SCHEMA_ID_SIZE) return error.MalformedFrame;
    if (std.mem.readInt(u16, data[0..2], .big) != OUTPUT_SCHEMA_ID) return error.MalformedFrame;
    const body = data[SCHEMA_ID_SIZE..];
    if (body.len < OUTPUT_FIXED_SIZE) return error.InvalidSsz;

    const off_pi = readU32(body, 0);
    const start_block_number = readU64(body, 4);
    const off_roots = readU32(body, 12);
    const off_filtered = readU32(body, 16);
    if (off_pi != OUTPUT_FIXED_SIZE) return error.InvalidSsz;
    if (off_roots < off_pi or off_filtered < off_roots or off_filtered > body.len) return error.InvalidSsz;

    const public_inputs = try decodeRollupPublicInput(alloc, body[off_pi..off_roots]);
    const l2_l1_roots = try decodeBytes32List(alloc, body[off_roots..off_filtered], MAX_L2_L1_ROOTS);
    const filtered_addresses = try decodeAddressList(alloc, body[off_filtered..], MAX_FILTERED_ADDRESSES);

    return .{
        .public_inputs = public_inputs,
        .start_block_number = start_block_number,
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = filtered_addresses,
    };
}
