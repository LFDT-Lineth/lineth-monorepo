//! Manual SSZ codec for the extended l2-execution guest wire format.
//!
//! Mirrors the Python reference codec's wire format byte-for-byte: a 2-byte
//! big-endian schema id (0x0002 for the input, 0x0003 for the output)
//! followed by the SSZ encoding of the containers below. The Python side is
//! the oracle: this codec's `encode`/`decode` byte layout must match its
//! `encode_bytes` exactly (verified by the golden-vector test).
//!
//! Each payload's `stateless_input_ssz` is carried opaquely — a zero-copy
//! slice into the input buffer — and never decoded here; that stays the
//! vanilla stateless-input SSZ decoder's job (e.g. zesu's `ssz_decode.decode`),
//! invoked one level up once this codec has split the extended envelope apart.
//!
//! Container layouts (fixed-head byte sizes):
//!   SszL2ExecutionProofPrivateInput:  92 bytes  [32+8+20+20+8+4]
//!   SszLineaPayloadInput:              8 bytes  [4+4]
//!   SszForcedTransactionWitness:      21 bytes  [8+4+1+8]
//!   SszL2ExecutionProofOutput:        32 bytes  (ONLY `keccak256(public_inputs)` — see
//!     `hashPublicInputs`/`encodeOutput`; `L2ExecutionProofOutput`'s other fields —
//!     `start_block_number`, `l2_l1_messages`, `tx_froms`, `filtered_addresses` — are off-chain/
//!     native-tooling data (see `l2_execution_json.zig`), never part of this wire format)
//!   SszL2ExecutionProofPublicInput:  368 bytes  (16 fields, all fixed-size) — never written to the
//!     wire itself; only its hash is (`encodePublicInputsBytes` exists purely for logging/off-chain
//!     visibility, e.g. the guest's `zkvm_log` call).

const std = @import("std");

pub const INPUT_SCHEMA_ID: u16 = 0x0002;
pub const OUTPUT_SCHEMA_ID: u16 = 0x0003;
const SCHEMA_ID_SIZE: usize = 2;

// ── SSZ list/vector bounds ───────────────────────────────────────────────────
// These bound only merkleization, never `encode`/`decode` — the wire bytes
// this codec produces/consumes do not depend on them. Values are chosen
// generously and MUST match the Python reference codec's bounds exactly (kept
// here only so a decoder can reject a maliciously huge list length early).
pub const MAX_PAYLOADS: usize = 1 << 16;
pub const MAX_FTX_PER_PAYLOAD: usize = 1 << 16;
pub const MAX_MESSAGES: usize = 1 << 16;
pub const MAX_TX_FROMS: usize = 1 << 16;
pub const MAX_FILTERED: usize = 1 << 16;
pub const MAX_STATELESS_INPUT_BYTES: usize = 1 << 30;
pub const MAX_TX_BYTES: usize = 1 << 30; // matches the consensus-layer Transaction ByteList limit

// ── Logical values ────────────────────────────────────────────────────────────

pub const ChainConfig = struct {
    l2_message_service_address: [20]u8,
    coinbase: [20]u8,
    chain_id: u64,
};

pub const ForcedTransactionWitness = struct {
    number: u64,
    /// Zero-copy slice into the decoded buffer.
    signed_tx_rlp: []const u8,
    /// The `ForcedTransactionAcceptance` enum value (0..4).
    acceptance: u8,
    deadline: u64,
};

pub const LineaPayloadInput = struct {
    /// Zero-copy slice into the decoded buffer: the opaque, already
    /// 0x0001-framed vanilla stateless-input SSZ bytes.
    stateless_input_ssz: []const u8,
    forced_transactions: []const ForcedTransactionWitness,
};

pub const L2ExecutionProofPrivateInput = struct {
    parent_ftx_rolling_hash: [32]u8,
    parent_last_processed_ftx_number: u64,
    chain_config: ChainConfig,
    payloads: []const LineaPayloadInput,
};

/// The 16-field l2-execution public input tuple, in wire order.
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
    parent_processed_ftx_number: u64,
    end_ftx_rolling_hash: [32]u8,
    end_processed_ftx_number: u64,
    filtered_addresses_hash: [32]u8,
    tx_froms_hash: [32]u8,
};

/// The guest's output: the public-input tuple plus the revealed hash
/// preimages the rollup guest needs (`proof` is attached by the prover layer
/// above the guest, so it has no place in this wire format).
pub const L2ExecutionProofOutput = struct {
    public_inputs: L2ExecutionProofPublicInput,
    start_block_number: u64,
    l2_l1_messages: []const [32]u8,
    tx_froms: []const [20]u8,
    filtered_addresses: []const [20]u8,
};

// ── Primitive reads/writes (little-endian, matching SSZ) ────────────────────

inline fn readU32(data: []const u8, off: usize) u32 {
    return std.mem.readInt(u32, data[off..][0..4], .little);
}

inline fn readU64(data: []const u8, off: usize) u64 {
    return std.mem.readInt(u64, data[off..][0..8], .little);
}

inline fn writeU32(out: []u8, off: usize, value: u32) void {
    std.mem.writeInt(u32, out[off..][0..4], value, .little);
}

inline fn writeU64(out: []u8, off: usize, value: u64) void {
    std.mem.writeInt(u64, out[off..][0..8], value, .little);
}

// ── Generic "List[VariableSizeType, N]" codec ────────────────────────────────
//
// SSZ encodes a list of variable-size elements exactly like a container's
// variable-field region: an offset table (4 bytes per element, each an
// absolute offset from the start of this region) followed by the
// concatenated element bytes, in order.

fn decodeVariableList(alloc: std.mem.Allocator, data: []const u8, max_len: usize) ![]const []const u8 {
    if (data.len == 0) return &.{};
    if (data.len < 4) return error.InvalidSsz;

    const first_off = readU32(data, 0);
    if (first_off == 0 or first_off % 4 != 0) return error.InvalidSsz;
    if (first_off > data.len) return error.InvalidSsz;
    const n = first_off / 4;
    if (n > max_len) return error.InvalidSsz;

    const result = try alloc.alloc([]const u8, n);
    for (0..n) |i| {
        const off_i = readU32(data, i * 4);
        const end_i: u32 = if (i + 1 < n) readU32(data, (i + 1) * 4) else blk: {
            if (data.len > std.math.maxInt(u32)) return error.InvalidSsz;
            break :blk @intCast(data.len);
        };
        if (off_i > data.len or end_i > data.len or off_i > end_i) return error.InvalidSsz;
        result[i] = data[off_i..end_i];
    }
    return result;
}

fn encodeVariableList(alloc: std.mem.Allocator, items: []const []const u8) ![]u8 {
    const n = items.len;
    var total: usize = n * 4;
    for (items) |item| total += item.len;

    const out = try alloc.alloc(u8, total);
    var offset: u32 = @intCast(n * 4);
    for (items, 0..) |item, i| {
        writeU32(out, i * 4, offset);
        offset += @intCast(item.len);
    }
    var pos: usize = n * 4;
    for (items) |item| {
        @memcpy(out[pos..][0..item.len], item);
        pos += item.len;
    }
    return out;
}

// ── ForcedTransactionWitness ──────────────────────────────────────────────────
// Fixed head: number(8) + signed_tx_rlp offset(4) + acceptance(1) + deadline(8) = 21.
const FTW_FIXED_SIZE: usize = 21;

fn decodeForcedTransactionWitness(bytes: []const u8) !ForcedTransactionWitness {
    if (bytes.len < FTW_FIXED_SIZE) return error.InvalidSsz;
    const number = readU64(bytes, 0);
    const off_tx = readU32(bytes, 8);
    // The only variable field is last, so its offset must exactly equal the
    // fixed-head size — anything else is not the canonical encoding.
    if (off_tx != FTW_FIXED_SIZE or off_tx > bytes.len) return error.InvalidSsz;
    const acceptance = bytes[12];
    const deadline = readU64(bytes, 13);
    return .{
        .number = number,
        .signed_tx_rlp = bytes[off_tx..],
        .acceptance = acceptance,
        .deadline = deadline,
    };
}

fn encodeForcedTransactionWitness(alloc: std.mem.Allocator, v: ForcedTransactionWitness) ![]u8 {
    const out = try alloc.alloc(u8, FTW_FIXED_SIZE + v.signed_tx_rlp.len);
    writeU64(out, 0, v.number);
    writeU32(out, 8, @intCast(FTW_FIXED_SIZE));
    out[12] = v.acceptance;
    writeU64(out, 13, v.deadline);
    @memcpy(out[FTW_FIXED_SIZE..], v.signed_tx_rlp);
    return out;
}

// ── LineaPayloadInput ─────────────────────────────────────────────────────────
// Fixed head: stateless_input_ssz offset(4) + forced_transactions offset(4) = 8.
const LPI_FIXED_SIZE: usize = 8;

fn decodeLineaPayloadInput(alloc: std.mem.Allocator, bytes: []const u8) !LineaPayloadInput {
    if (bytes.len < LPI_FIXED_SIZE) return error.InvalidSsz;
    const off_ssz = readU32(bytes, 0);
    const off_ftx = readU32(bytes, 4);
    if (off_ssz != LPI_FIXED_SIZE or off_ssz > off_ftx or off_ftx > bytes.len) return error.InvalidSsz;

    const stateless_input_ssz = bytes[off_ssz..off_ftx];
    if (stateless_input_ssz.len > MAX_STATELESS_INPUT_BYTES) return error.InvalidSsz;

    const ftx_slices = try decodeVariableList(alloc, bytes[off_ftx..], MAX_FTX_PER_PAYLOAD);
    const forced_transactions = try alloc.alloc(ForcedTransactionWitness, ftx_slices.len);
    for (ftx_slices, 0..) |slice, i| {
        forced_transactions[i] = try decodeForcedTransactionWitness(slice);
        if (forced_transactions[i].signed_tx_rlp.len > MAX_TX_BYTES) return error.InvalidSsz;
    }

    return .{
        .stateless_input_ssz = stateless_input_ssz,
        .forced_transactions = forced_transactions,
    };
}

fn encodeLineaPayloadInput(alloc: std.mem.Allocator, v: LineaPayloadInput) ![]u8 {
    const ftx_bufs = try alloc.alloc([]const u8, v.forced_transactions.len);
    for (v.forced_transactions, 0..) |ftx, i| {
        ftx_bufs[i] = try encodeForcedTransactionWitness(alloc, ftx);
    }
    const ftx_list_bytes = try encodeVariableList(alloc, ftx_bufs);

    const out = try alloc.alloc(u8, LPI_FIXED_SIZE + v.stateless_input_ssz.len + ftx_list_bytes.len);
    writeU32(out, 0, @intCast(LPI_FIXED_SIZE));
    writeU32(out, 4, @intCast(LPI_FIXED_SIZE + v.stateless_input_ssz.len));
    @memcpy(out[LPI_FIXED_SIZE..][0..v.stateless_input_ssz.len], v.stateless_input_ssz);
    @memcpy(out[LPI_FIXED_SIZE + v.stateless_input_ssz.len ..], ftx_list_bytes);
    return out;
}

// ── L2ExecutionProofPrivateInput (the extended guest INPUT) ──────────────────
// Fixed head: hash(32) + u64(8) + chain_config(20+20+8=48) + payloads offset(4) = 92.
const INPUT_FIXED_SIZE: usize = 92;

/// Decode the extended l2-execution guest input: the 0x0002 schema id
/// followed by the SSZ `SszL2ExecutionProofPrivateInput`.
pub fn decodeInput(alloc: std.mem.Allocator, data: []const u8) !L2ExecutionProofPrivateInput {
    if (data.len < SCHEMA_ID_SIZE) return error.InvalidSsz;
    if (std.mem.readInt(u16, data[0..2], .big) != INPUT_SCHEMA_ID) return error.InvalidSsz;

    const body = data[SCHEMA_ID_SIZE..];
    if (body.len < INPUT_FIXED_SIZE) return error.InvalidSsz;

    var parent_ftx_rolling_hash: [32]u8 = undefined;
    @memcpy(&parent_ftx_rolling_hash, body[0..32]);
    const parent_last_processed_ftx_number = readU64(body, 32);

    var l2_message_service_address: [20]u8 = undefined;
    @memcpy(&l2_message_service_address, body[40..60]);
    var coinbase: [20]u8 = undefined;
    @memcpy(&coinbase, body[60..80]);
    const chain_id = readU64(body, 80);

    const off_payloads = readU32(body, 88);
    if (off_payloads != INPUT_FIXED_SIZE or off_payloads > body.len) return error.InvalidSsz;

    const payload_slices = try decodeVariableList(alloc, body[off_payloads..], MAX_PAYLOADS);
    const payloads = try alloc.alloc(LineaPayloadInput, payload_slices.len);
    for (payload_slices, 0..) |slice, i| {
        payloads[i] = try decodeLineaPayloadInput(alloc, slice);
    }

    return .{
        .parent_ftx_rolling_hash = parent_ftx_rolling_hash,
        .parent_last_processed_ftx_number = parent_last_processed_ftx_number,
        .chain_config = .{
            .l2_message_service_address = l2_message_service_address,
            .coinbase = coinbase,
            .chain_id = chain_id,
        },
        .payloads = payloads,
    };
}

/// Encode the extended l2-execution guest input. Inverse of `decodeInput`.
/// Not used by the guest at runtime (the guest only ever decodes its input) —
/// kept so the codec's byte-exact round-trip can be asserted against the
/// golden vector, the same gate the Python reference codec is held to.
pub fn encodeInput(alloc: std.mem.Allocator, v: L2ExecutionProofPrivateInput) ![]u8 {
    const payload_bufs = try alloc.alloc([]const u8, v.payloads.len);
    for (v.payloads, 0..) |p, i| payload_bufs[i] = try encodeLineaPayloadInput(alloc, p);
    const payloads_bytes = try encodeVariableList(alloc, payload_bufs);

    const out = try alloc.alloc(u8, SCHEMA_ID_SIZE + INPUT_FIXED_SIZE + payloads_bytes.len);
    std.mem.writeInt(u16, out[0..2], INPUT_SCHEMA_ID, .big);
    const body = out[SCHEMA_ID_SIZE..];

    @memcpy(body[0..32], &v.parent_ftx_rolling_hash);
    writeU64(body, 32, v.parent_last_processed_ftx_number);
    @memcpy(body[40..60], &v.chain_config.l2_message_service_address);
    @memcpy(body[60..80], &v.chain_config.coinbase);
    writeU64(body, 80, v.chain_config.chain_id);
    writeU32(body, 88, @intCast(INPUT_FIXED_SIZE));
    @memcpy(body[INPUT_FIXED_SIZE..], payloads_bytes);

    return out;
}

// ── L2ExecutionProofOutput (the extended guest OUTPUT) ────────────────────────
// The plain public-input tuple, SSZ-encoded, has no variable fields (368 bytes).
const PI_FIXED_SIZE: usize = 368;
// The wire output is ONLY keccak256(public_inputs) — nothing else.
const OUTPUT_BODY_SIZE: usize = 32;
/// Total wire-output size: the 0x0003 schema id (2 bytes) + keccak256(public_inputs) (32 bytes).
pub const OUTPUT_SIZE: usize = SCHEMA_ID_SIZE + OUTPUT_BODY_SIZE;

/// Write a 32-byte hash at the cursor and advance it.
inline fn putHash(out: []u8, pos: *usize, value: [32]u8) void {
    @memcpy(out[pos.*..][0..32], &value);
    pos.* += 32;
}

/// Write a little-endian u64 at the cursor and advance it.
inline fn putU64(out: []u8, pos: *usize, value: u64) void {
    writeU64(out, pos.*, value);
    pos.* += 8;
}

fn encodePublicInputs(out: []u8, pi: L2ExecutionProofPublicInput) void {
    var pos: usize = 0;
    putHash(out, &pos, pi.parent_block_hash);
    putHash(out, &pos, pi.end_block_hash);
    putU64(out, &pos, pi.end_block_number);
    putU64(out, &pos, pi.end_block_timestamp);
    putHash(out, &pos, pi.l2_l1_messages_hash);
    putHash(out, &pos, pi.parent_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, pi.parent_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, pi.end_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, pi.end_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, pi.dynamic_chain_config_hash);
    putHash(out, &pos, pi.parent_ftx_rolling_hash);
    putU64(out, &pos, pi.parent_processed_ftx_number);
    putHash(out, &pos, pi.end_ftx_rolling_hash);
    putU64(out, &pos, pi.end_processed_ftx_number);
    putHash(out, &pos, pi.filtered_addresses_hash);
    putHash(out, &pos, pi.tx_froms_hash);
    std.debug.assert(pos == PI_FIXED_SIZE);
}

/// SSZ-encode the plain public-input tuple to its fixed 368-byte wire representation. Exposed for
/// callers that need the plain tuple outside `encodeOutput`'s hash-only wire output — namely
/// `hashPublicInputs` below and the guest's pre-hash debug log (`zkvm_log`, see
/// `evm_execution_guest.zig`).
pub fn encodePublicInputsBytes(pi: L2ExecutionProofPublicInput) [PI_FIXED_SIZE]u8 {
    var out: [PI_FIXED_SIZE]u8 = undefined;
    encodePublicInputs(&out, pi);
    return out;
}

/// keccak256 of the SSZ-encoded plain public-input tuple — the single field `encodeOutput` commits
/// in place of the 16-field tuple itself.
pub fn hashPublicInputs(pi: L2ExecutionProofPublicInput) [32]u8 {
    const encoded = encodePublicInputsBytes(pi);
    var out: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(&encoded, &out, .{});
    return out;
}

/// Encode the extended l2-execution guest's ACTUAL wire output: the 0x0003 schema id followed by
/// ONLY `keccak256(public_inputs)` (see `hashPublicInputs`) — 32 bytes, nothing else. Returns a
/// fixed-size stack array (no allocator, no error union) since this is the only output shape.
///
/// Deliberately NOT zesu's vanilla `Result{out, len, success}` convention (`run.zig`): zesu commits
/// on failure too, but what it commits is the SSZ hash_tree_root of the WHOLE (untrusted, always
/// available pre-execution) `NewPayloadRequest` paired with `success=0x00` — a binding commitment
/// to which specific input was rejected, not to anything execution produced. There is no
/// input-derived equivalent here that's worth committing on failure: any invalidity is a hard
/// Zig-error guest rejection (`exit(1)`, nothing written to `write_output`), so `encodeOutput` is
/// only ever reached after a full, successful `L2ExecutionProofPublicInput` already exists — a
/// `success` field on this type would be permanently `true` and couldn't mean anything.
/// `start_block_number` and the `l2_l1_messages`/`tx_froms`/`filtered_addresses` preimages on
/// `L2ExecutionProofOutput` are NOT part of this wire format; they exist for off-chain/native
/// tooling only (see `l2_execution_json.zig`'s `encodeOutputJson`). The plain 16-field
/// public-input tuple is never written to the wire either; it is only available via
/// `encodePublicInputsBytes`/`hashPublicInputs`, for logging or off-chain inspection.
pub fn encodeOutput(pi: L2ExecutionProofPublicInput) [OUTPUT_SIZE]u8 {
    var out: [OUTPUT_SIZE]u8 = undefined;
    std.mem.writeInt(u16, out[0..2], OUTPUT_SCHEMA_ID, .big);
    @memcpy(out[SCHEMA_ID_SIZE..], &hashPublicInputs(pi));
    return out;
}
