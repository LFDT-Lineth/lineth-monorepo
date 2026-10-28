//! Manual SSZ codec for the extended l2-execution guest input wire format.
//!
//! Mirrors the Python reference codec's wire format byte-for-byte: a 2-byte
//! big-endian schema id (0x0002 for the input) followed by the SSZ encoding of
//! the containers below. This codec's Input `encode`/`decode` byte layout
//! matches the Python reference's `encode_bytes` exactly (verified by the
//! golden-vector test).
//!
//! Each payload's `stateless_input_ssz` is carried opaquely — a zero-copy
//! slice into the input buffer — and never decoded here; that stays the
//! vanilla stateless-input SSZ decoder's job (e.g. zesu's `ssz_decode.decode`),
//! invoked one level up once this codec has split the extended envelope apart.
//!
//! Container layouts (fixed-head byte sizes — see each *_FIXED_SIZE constant below for the field
//! breakdown):
//!   SszL2ExecutionProofPrivateInput: 232 bytes
//!   SszLineaPayloadInput:              8 bytes
//!   SszForcedTransactionWitness:      21 bytes

const std = @import("std");
const guest_common = @import("guest_common");
const l2_execution_proof = guest_common.l2_execution_proof;

pub const INPUT_SCHEMA_ID: u16 = 0x0002;
const SCHEMA_ID_SIZE: usize = 2;

// ── SSZ list/vector bounds ───────────────────────────────────────────────────
// These bound only merkleization, never `encode`/`decode` — the wire bytes
// this codec produces/consumes do not depend on them. Values are chosen
// generously and MUST match the Python reference codec's bounds exactly (kept
// here only so a decoder can reject a maliciously huge list length early).
pub const MAX_PAYLOADS: usize = 1 << 16;
pub const MAX_FTX_PER_PAYLOAD: usize = 1 << 16;
pub const MAX_TX_FROMS: usize = 1 << 16;
pub const MAX_STATELESS_INPUT_BYTES: usize = 1 << 30;
pub const MAX_TX_BYTES: usize = 1 << 30; // matches the consensus-layer Transaction ByteList limit

// ── Logical values ────────────────────────────────────────────────────────────

/// System-contract addresses handed to the execution engine (field order = wire order, and matches
/// zesu's `input.SystemContracts`). Supplied by the coordinator from the node's genesis config. Not
/// part of the `dynamicChainConfigHash` preimage, which is anchored in the L1 verifier.
pub const SystemContracts = struct {
    beacon_roots: [20]u8,
    history_storage: [20]u8,
    deposit: [20]u8,
    withdrawal_request: [20]u8,
    consolidation_request: [20]u8,
    builder_deposit_request: [20]u8,
    builder_exit_request: [20]u8,

    pub const ENCODED_SIZE: usize = 7 * 20;

    pub fn encode(self: SystemContracts) [ENCODED_SIZE]u8 {
        var out: [ENCODED_SIZE]u8 = undefined;
        inline for (std.meta.fields(SystemContracts), 0..) |field, i| {
            @memcpy(out[i * 20 ..][0..20], &@field(self, field.name));
        }
        return out;
    }

    pub fn decode(bytes: *const [ENCODED_SIZE]u8) SystemContracts {
        var out: SystemContracts = undefined;
        inline for (std.meta.fields(SystemContracts), 0..) |field, i| {
            @memcpy(&@field(out, field.name), bytes[i * 20 ..][0..20]);
        }
        return out;
    }

    /// Copy from any struct with the same field names (e.g. zesu's `input.SystemContracts`),
    /// without this codec depending on it.
    pub fn from(src: anytype) SystemContracts {
        var out: SystemContracts = undefined;
        inline for (std.meta.fields(SystemContracts)) |field| {
            @field(out, field.name) = @field(src, field.name);
        }
        return out;
    }
};

pub const ChainConfig = struct {
    l2_message_service_address: [20]u8,
    coinbase: [20]u8,
    chain_id: u64,
    system_contracts: SystemContracts,
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

/// The public-input tuple plus the range metadata that travels outside the guest's wire output
/// (`proof` is attached by the prover layer above the guest).
pub const L2ExecutionProofOutput = struct {
    public_inputs: l2_execution_proof.L2ExecutionProofPublicInput,
    start_block_number: u64,
    tx_froms: []const [20]u8,
};

// ── Primitive reads/writes and the generic "List[VariableSizeType, N]" codec ─
//
// The little-endian integer reads/writes and the generic SSZ offset-table list codec live in
// guest_common.ssz: identical machinery every riscv-guests SSZ codec builds on, with nothing
// specific to this container set. Local aliases keep call sites short.

const readU32 = guest_common.ssz.readU32;
const readU64 = guest_common.ssz.readU64;
const writeU32 = guest_common.ssz.writeU32;
const writeU64 = guest_common.ssz.writeU64;
const decodeVariableList = guest_common.ssz.decodeVariableList;
const encodeVariableList = guest_common.ssz.encodeVariableList;

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
// Fixed head: hash(32) + u64(8) + chain_config(20+20+8+140=188) + payloads offset(4) = 232.
const SYSTEM_CONTRACTS_OFFSET: usize = 88;
const PAYLOADS_OFFSET_POS: usize = SYSTEM_CONTRACTS_OFFSET + SystemContracts.ENCODED_SIZE;
const INPUT_FIXED_SIZE: usize = PAYLOADS_OFFSET_POS + 4;

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
    const system_contracts = SystemContracts.decode(body[SYSTEM_CONTRACTS_OFFSET..][0..SystemContracts.ENCODED_SIZE]);

    const off_payloads = readU32(body, PAYLOADS_OFFSET_POS);
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
            .system_contracts = system_contracts,
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
    @memcpy(body[SYSTEM_CONTRACTS_OFFSET..][0..SystemContracts.ENCODED_SIZE], &v.chain_config.system_contracts.encode());
    writeU32(body, PAYLOADS_OFFSET_POS, @intCast(INPUT_FIXED_SIZE));
    @memcpy(body[INPUT_FIXED_SIZE..], payloads_bytes);

    return out;
}
