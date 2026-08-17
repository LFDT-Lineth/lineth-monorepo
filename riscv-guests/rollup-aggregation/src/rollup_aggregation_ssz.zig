//! Manual SSZ codec for the rollup-aggregation guest wire format (the Python reference codec's
//! `rollup_ssz.py`: `SszRollupAggregationProofPrivateInput`/`SszRollupAggregationOutput`, schema
//! ids 0x1002/0x1802).
//!
//! Frame: 2-byte big-endian schema id || SSZ container bytes (SSZ itself little-endian). Container
//! field orders, list bounds, and byte layouts mirror the Python reference codec exactly — this
//! guest's tests decode the checked-in golden vector and assert known field values, and round-trip
//! the output container byte-for-byte.
//!
//! `RollupPublicInput`'s container (the 20-field rollup/rollup-aggregation public-input tuple) is
//! decoded here independently of the `rollup` guest's own copy — both guests need it (the rollup
//! guest to EMIT it, this guest to decode it nested inside every nested `RollupProof` and to emit
//! it again), and this package takes no path dependency on `../rollup`, so the small codec is
//! duplicated rather than shared.
//!
//! Every `List` field's wire encoding follows plain SSZ: a list of VARIABLE-size elements
//! (`rollup_proofs`) is an offset table over its region — the generic
//! `guest_common.ssz.decodeVariableList`/`encodeVariableList` every riscv-guests SSZ codec shares.
//! A list of FIXED-size elements (`program_vks`, `l2_l1_roots`, `filtered_addresses`,
//! `l2_messaging_blocks_offsets`) is just the concatenation of its elements with no per-element
//! offset table; element count is the region's byte length divided by the element size (the
//! `decodeBytes32List`/`decodeAddressList`/`decodeU64List` helpers below).

const std = @import("std");
const guest_common = @import("guest_common");

pub const INPUT_SCHEMA_ID: u16 = 0x1002;
pub const OUTPUT_SCHEMA_ID: u16 = 0x1802;
const SCHEMA_ID_SIZE: usize = 2;

// ── SSZ list/vector bounds (mirrors rollup_ssz.py's MAX_* constants) ─────────────────────────────
pub const MAX_PROGRAM_VKS: usize = 1 << 10;
pub const MAX_L2_L1_ROOTS: usize = 1 << 16;
pub const MAX_FILTERED_ADDRESSES: usize = 1 << 16;
pub const MAX_L2_MESSAGING_BLOCKS_OFFSETS: usize = 1 << 16;
pub const MAX_ROLLUP_PROOFS_PER_AGGREGATION: usize = 1 << 10;
pub const MAX_PROOF_BYTES: usize = 1 << 24;

// ── Logical values ────────────────────────────────────────────────────────────────────────────

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

/// An already-proven rollup proof, as recursively verified by the rollup-aggregation guest: carries
/// its own `proof` bytes (unlike the guest OUTPUT container, which never carries `proof`).
pub const RollupProof = struct {
    public_inputs: RollupPublicInput,
    start_block_number: u64,
    /// Zero-copy slice into the decoded input buffer.
    proof: []const u8,
    l2_l1_roots: []const [32]u8,
    filtered_addresses: []const [20]u8,
};

pub const VerifiableRollupProof = struct {
    proof: RollupProof,
    program_vk: [32]u8,
};

/// The rollup-aggregation guest's own input: an already-proven contiguous run of rollup proofs
/// this stub decodes but never recursively verifies or aggregates.
pub const RollupAggregationProofPrivateInput = struct {
    rollup_proofs: []const VerifiableRollupProof,
};

/// The rollup-aggregation guest's own output: `FinalizationSubmission` with `proof` omitted (the
/// prover layer attaches it separately — a guest cannot attest its own proof).
pub const RollupAggregationOutput = struct {
    public_inputs: RollupPublicInput,
    l2_l1_roots: []const [32]u8,
    filtered_addresses: []const [20]u8,
    l2_messaging_blocks_offsets: []const u64,
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

fn decodeU64List(alloc: std.mem.Allocator, data: []const u8, max_len: usize) ![]const u64 {
    if (data.len % 8 != 0) return error.InvalidSsz;
    const n = data.len / 8;
    if (n > max_len) return error.BoundsViolation;
    const out = try alloc.alloc(u64, n);
    for (0..n) |i| out[i] = readU64(data, i * 8);
    return out;
}

fn encodeU64List(alloc: std.mem.Allocator, items: []const u64) ![]u8 {
    const out = try alloc.alloc(u8, items.len * 8);
    for (items, 0..) |item, i| writeU64(out, i * 8, item);
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

// ── RollupProof ───────────────────────────────────────────────────────────────────────────────
// Fixed head: public_inputs offset(4) + start_block_number(8) + proof offset(4) +
// l2_l1_roots offset(4) + filtered_addresses offset(4) = 24. `public_inputs` needs its own offset
// here (unlike the rollup guest's embedded `L2ExecutionProofPublicInput`, which is fully fixed):
// `RollupPublicInput` carries the variable `program_vks` list, so it is itself variable-size.
const ROLLUP_PROOF_FIXED_SIZE: usize = 4 + 8 + 4 + 4 + 4;

fn decodeRollupProof(alloc: std.mem.Allocator, bytes: []const u8) !RollupProof {
    if (bytes.len < ROLLUP_PROOF_FIXED_SIZE) return error.InvalidSsz;
    const off_pi = readU32(bytes, 0);
    const start_block_number = readU64(bytes, 4);
    const off_proof = readU32(bytes, 12);
    const off_roots = readU32(bytes, 16);
    const off_filtered = readU32(bytes, 20);
    if (off_pi != ROLLUP_PROOF_FIXED_SIZE) return error.InvalidSsz;
    if (off_proof < off_pi or off_roots < off_proof or off_filtered < off_roots) return error.InvalidSsz;
    if (off_filtered > bytes.len) return error.InvalidSsz;

    const public_inputs = try decodeRollupPublicInput(alloc, bytes[off_pi..off_proof]);
    const proof = bytes[off_proof..off_roots];
    if (proof.len > MAX_PROOF_BYTES) return error.BoundsViolation;
    const l2_l1_roots = try decodeBytes32List(alloc, bytes[off_roots..off_filtered], MAX_L2_L1_ROOTS);
    const filtered_addresses = try decodeAddressList(alloc, bytes[off_filtered..], MAX_FILTERED_ADDRESSES);

    return .{
        .public_inputs = public_inputs,
        .start_block_number = start_block_number,
        .proof = proof,
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = filtered_addresses,
    };
}

// ── VerifiableRollupProof ─────────────────────────────────────────────────────────────────────
// Fixed head: proof offset(4) + program_vk(32, inline) = 36.
const VERIFIABLE_ROLLUP_PROOF_FIXED_SIZE: usize = 36;

fn decodeVerifiableRollupProof(alloc: std.mem.Allocator, bytes: []const u8) !VerifiableRollupProof {
    if (bytes.len < VERIFIABLE_ROLLUP_PROOF_FIXED_SIZE) return error.InvalidSsz;
    const off_proof = readU32(bytes, 0);
    if (off_proof != VERIFIABLE_ROLLUP_PROOF_FIXED_SIZE or off_proof > bytes.len) return error.InvalidSsz;
    var program_vk: [32]u8 = undefined;
    @memcpy(&program_vk, bytes[4..36]);
    const proof = try decodeRollupProof(alloc, bytes[off_proof..]);
    return .{ .proof = proof, .program_vk = program_vk };
}

// ── RollupAggregationProofPrivateInput (the rollup-aggregation guest INPUT) ──────────────────────
// Fixed head: rollup_proofs offset(4) = 4 — the only field, so this container is entirely variable.
const INPUT_FIXED_SIZE: usize = 4;

/// Decode the rollup-aggregation guest input: the 0x1002 schema id followed by the SSZ
/// `SszRollupAggregationProofPrivateInput`. Strict: rejects a wrong schema id, a too-short frame, a
/// misaligned/out-of-order/out-of-bounds offset, or a list exceeding its wire-format bound.
pub fn decodeInput(alloc: std.mem.Allocator, data: []const u8) !RollupAggregationProofPrivateInput {
    if (data.len < SCHEMA_ID_SIZE) return error.MalformedFrame;
    if (std.mem.readInt(u16, data[0..2], .big) != INPUT_SCHEMA_ID) return error.MalformedFrame;

    const body = data[SCHEMA_ID_SIZE..];
    if (body.len < INPUT_FIXED_SIZE) return error.InvalidSsz;

    const off_proofs = readU32(body, 0);
    if (off_proofs != INPUT_FIXED_SIZE or off_proofs > body.len) return error.InvalidSsz;

    const proof_slices = try decodeVariableList(alloc, body[off_proofs..], MAX_ROLLUP_PROOFS_PER_AGGREGATION);
    const rollup_proofs = try alloc.alloc(VerifiableRollupProof, proof_slices.len);
    for (proof_slices, 0..) |slice, i| rollup_proofs[i] = try decodeVerifiableRollupProof(alloc, slice);

    return .{ .rollup_proofs = rollup_proofs };
}

// ── RollupAggregationOutput (the rollup-aggregation guest OUTPUT) ────────────────────────────────
// Fixed head: public_inputs offset(4) + l2_l1_roots offset(4) + filtered_addresses offset(4) +
// l2_messaging_blocks_offsets offset(4) = 16.
const OUTPUT_FIXED_SIZE: usize = 4 * 4;

/// Encode the rollup-aggregation guest's actual wire output: the 0x1802 schema id followed by the
/// SSZ `SszRollupAggregationOutput`.
pub fn encodeOutput(alloc: std.mem.Allocator, v: RollupAggregationOutput) ![]u8 {
    const pi_bytes = try encodeRollupPublicInput(alloc, v.public_inputs);
    const roots_bytes = try encodeBytes32List(alloc, v.l2_l1_roots);
    const filtered_bytes = try encodeAddressList(alloc, v.filtered_addresses);
    const offsets_bytes = try encodeU64List(alloc, v.l2_messaging_blocks_offsets);

    const body_len = OUTPUT_FIXED_SIZE + pi_bytes.len + roots_bytes.len + filtered_bytes.len + offsets_bytes.len;
    const out = try alloc.alloc(u8, SCHEMA_ID_SIZE + body_len);
    std.mem.writeInt(u16, out[0..2], OUTPUT_SCHEMA_ID, .big);
    const body = out[SCHEMA_ID_SIZE..];

    const off_roots = OUTPUT_FIXED_SIZE + pi_bytes.len;
    const off_filtered = off_roots + roots_bytes.len;
    const off_offsets = off_filtered + filtered_bytes.len;
    writeU32(body, 0, @intCast(OUTPUT_FIXED_SIZE));
    writeU32(body, 4, @intCast(off_roots));
    writeU32(body, 8, @intCast(off_filtered));
    writeU32(body, 12, @intCast(off_offsets));

    @memcpy(body[OUTPUT_FIXED_SIZE..][0..pi_bytes.len], pi_bytes);
    @memcpy(body[off_roots..][0..roots_bytes.len], roots_bytes);
    @memcpy(body[off_filtered..][0..filtered_bytes.len], filtered_bytes);
    @memcpy(body[off_offsets..], offsets_bytes);
    return out;
}

/// Decode a rollup-aggregation guest output frame. Not used by the guest itself at runtime (it
/// only ever encodes) — kept so the output codec's byte-exact round-trip can be asserted against
/// `encodeOutput` in this guest's own tests, the same gate the Python reference codec is held to.
pub fn decodeOutput(alloc: std.mem.Allocator, data: []const u8) !RollupAggregationOutput {
    if (data.len < SCHEMA_ID_SIZE) return error.MalformedFrame;
    if (std.mem.readInt(u16, data[0..2], .big) != OUTPUT_SCHEMA_ID) return error.MalformedFrame;
    const body = data[SCHEMA_ID_SIZE..];
    if (body.len < OUTPUT_FIXED_SIZE) return error.InvalidSsz;

    const off_pi = readU32(body, 0);
    const off_roots = readU32(body, 4);
    const off_filtered = readU32(body, 8);
    const off_offsets = readU32(body, 12);
    if (off_pi != OUTPUT_FIXED_SIZE) return error.InvalidSsz;
    if (off_roots < off_pi or off_filtered < off_roots or off_offsets < off_filtered) return error.InvalidSsz;
    if (off_offsets > body.len) return error.InvalidSsz;

    const public_inputs = try decodeRollupPublicInput(alloc, body[off_pi..off_roots]);
    const l2_l1_roots = try decodeBytes32List(alloc, body[off_roots..off_filtered], MAX_L2_L1_ROOTS);
    const filtered_addresses = try decodeAddressList(alloc, body[off_filtered..off_offsets], MAX_FILTERED_ADDRESSES);
    const l2_messaging_blocks_offsets = try decodeU64List(alloc, body[off_offsets..], MAX_L2_MESSAGING_BLOCKS_OFFSETS);

    return .{
        .public_inputs = public_inputs,
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = filtered_addresses,
        .l2_messaging_blocks_offsets = l2_messaging_blocks_offsets,
    };
}
