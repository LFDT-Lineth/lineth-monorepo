//! Test-only SSZ encoder for the vanilla `SszStatelessInput` (Amsterdam stateless block execution) —
//! the exact byte-level inverse of the decoder this package's guest consumes. Every section below
//! mirrors that decoder's corresponding section, in the same order, so a schema change is a
//! side-by-side edit. This exists purely so tests can build a `StatelessInput` as readable, diffable
//! Zig and turn it into the bytes the guest's real decode path accepts — the vendored decoder ships
//! with no matching encoder of its own.
//!
//! Container layouts match the decoder exactly (fixed region sizes):
//!   SszStatelessInput:    16 bytes  [4+4+4+4] all-variable (v0.4.1)
//!   SszNewPayloadRequest: 44 bytes  [4+4+32+4]
//!   SszExecutionPayload: 540 bytes  (Amsterdam/V4 shape — see EP_FIXED_SIZE)
//!   SszExecutionWitness:  12 bytes  [4+4+4]
//!   SszWithdrawal:        44 bytes  fixed (8+8+20+8)
//!
//! Always produces the tightly-packed (canonical, zero-gap) form: every offset points immediately
//! past its own fixed head or the previous variable field, matching the real bytes this package's
//! guest is fed in practice. Always produces the Amsterdam (V4) execution-payload shape — this
//! package's guest fixes its fork to Amsterdam, and V4 is the wire shape that fork carries. Output
//! starts directly at the two schema bytes; the Ere length prefix belongs to the optional outer
//! transport framing the decoder strips before this format begins.

const std = @import("std");
const input = @import("zesu_input");

// ── Primitive writes (little-endian) ─────────────────────────────────────────

inline fn writeU32(out: []u8, off: usize, value: u32) void {
    std.mem.writeInt(u32, out[off..][0..4], value, .little);
}

inline fn writeU64(out: []u8, off: usize, value: u64) void {
    std.mem.writeInt(u64, out[off..][0..8], value, .little);
}

// ── List[ByteList] encoder ────────────────────────────────────────────────────

/// Encode SSZ `List[ByteList[...], N]` from a slice of opaque byte blobs. The exact inverse of the
/// decoder's byte-list-list reader: N×4-byte LE offsets (each the size of the offset table plus
/// every preceding element's length) followed by the concatenated element data, tightly packed.
fn encodeByteListList(alloc: std.mem.Allocator, items: []const []const u8) ![]u8 {
    const head = items.len * 4;
    var total: usize = head;
    for (items) |item| total += item.len;

    const out = try alloc.alloc(u8, total);
    var offset: u32 = @intCast(head);
    for (items, 0..) |item, i| {
        writeU32(out, i * 4, offset);
        offset += @intCast(item.len);
    }
    var pos: usize = head;
    for (items) |item| {
        @memcpy(out[pos..][0..item.len], item);
        pos += item.len;
    }
    return out;
}

// ── SszWithdrawal encoder ─────────────────────────────────────────────────────

/// SszWithdrawal fixed size: index(8) + validator_index(8) + address(20) + amount(uint64=8) = 44.
const WITHDRAWAL_SIZE: usize = 44;

fn encodeWithdrawal(out: *[WITHDRAWAL_SIZE]u8, w: input.Withdrawal) void {
    writeU64(out, 0, w.index);
    writeU64(out, 8, w.validator_index);
    @memcpy(out[16..36], &w.address);
    writeU64(out, 36, w.amount);
}

/// Withdrawals are a packed list of fixed-size items — no offset table, just concatenation.
fn encodeWithdrawals(alloc: std.mem.Allocator, withdrawals: []const input.Withdrawal) ![]u8 {
    const out = try alloc.alloc(u8, withdrawals.len * WITHDRAWAL_SIZE);
    for (withdrawals, 0..) |w, i| {
        encodeWithdrawal(out[i * WITHDRAWAL_SIZE ..][0..WITHDRAWAL_SIZE], w);
    }
    return out;
}

// ── SszExecutionRequests encoder ──────────────────────────────────────────────

/// Fixed head: one 4-byte offset per request type — deposits, withdrawals, consolidations,
/// builder_deposits, builder_exits (EIP-8282 / zkevm@v0.6.2) — in that order.
const ER_TYPE_COUNT: usize = 5;
const ER_FIXED_SIZE: usize = ER_TYPE_COUNT * 4;

fn encodeExecutionRequests(alloc: std.mem.Allocator, er: input.ExecutionRequests) ![]u8 {
    const parts = [_][]const u8{ er.deposits, er.withdrawals, er.consolidations, er.builder_deposits, er.builder_exits };
    comptime std.debug.assert(parts.len == ER_TYPE_COUNT);

    var total: usize = ER_FIXED_SIZE;
    for (parts) |p| total += p.len;

    const out = try alloc.alloc(u8, total);
    var offset: u32 = @intCast(ER_FIXED_SIZE);
    for (parts, 0..) |p, i| {
        writeU32(out, i * 4, offset);
        offset += @intCast(p.len);
    }
    var pos: usize = ER_FIXED_SIZE;
    for (parts) |p| {
        @memcpy(out[pos..][0..p.len], p);
        pos += p.len;
    }
    return out;
}

// ── SszExecutionPayload encoder (Amsterdam / V4) ──────────────────────────────

/// Fixed region byte offsets, matching the decoder's table exactly:
///   [0..32]    parent_hash
///   [32..52]   fee_recipient
///   [52..84]   state_root
///   [84..116]  receipts_root
///   [116..372] logs_bloom
///   [372..404] prev_randao
///   [404..412] block_number
///   [412..420] gas_limit
///   [420..428] gas_used
///   [428..436] timestamp
///   [436..440] → extra_data (variable offset)
///   [440..472] base_fee_per_gas (uint256 LE — only the low 8 bytes carry a value, since the decoded
///     struct field is a u64; the high 24 bytes are always written zero)
///   [472..504] block_hash
///   [504..508] → transactions (variable offset)
///   [508..512] → withdrawals (variable offset)
///   [512..520] blob_gas_used
///   [520..528] excess_blob_gas
///   [528..532] → block_access_list (variable offset)
///   [532..540] slot_number
const EP_FIXED_SIZE: usize = 540;

fn encodeExecutionPayload(alloc: std.mem.Allocator, ep: input.ExecutionPayload) ![]u8 {
    // The wire format's transaction list holds opaque RLP bytes, not the decoded `Transaction`
    // struct — `raw_transactions` is the field that round-trips through the wire, exactly like the
    // decoder populates it straight from this same list.
    const txs_bytes = try encodeByteListList(alloc, ep.raw_transactions);
    const wd_bytes = try encodeWithdrawals(alloc, ep.withdrawals);

    const off_extra_data: usize = EP_FIXED_SIZE;
    const off_transactions: usize = off_extra_data + ep.extra_data.len;
    const off_withdrawals: usize = off_transactions + txs_bytes.len;
    const off_bal: usize = off_withdrawals + wd_bytes.len;
    const total: usize = off_bal + ep.block_access_list.len;

    const out = try alloc.alloc(u8, total);
    @memcpy(out[0..32], &ep.parent_hash);
    @memcpy(out[32..52], &ep.fee_recipient);
    @memcpy(out[52..84], &ep.state_root);
    @memcpy(out[84..116], &ep.receipts_root);
    @memcpy(out[116..372], &ep.logs_bloom);
    @memcpy(out[372..404], &ep.prev_randao);
    writeU64(out, 404, ep.block_number);
    writeU64(out, 412, ep.gas_limit);
    writeU64(out, 420, ep.gas_used);
    writeU64(out, 428, ep.timestamp);
    writeU32(out, 436, @intCast(off_extra_data));
    @memset(out[440..472], 0);
    writeU64(out, 440, ep.base_fee_per_gas);
    @memcpy(out[472..504], &ep.block_hash);
    writeU32(out, 504, @intCast(off_transactions));
    writeU32(out, 508, @intCast(off_withdrawals));
    writeU64(out, 512, ep.blob_gas_used);
    writeU64(out, 520, ep.excess_blob_gas);
    writeU32(out, 528, @intCast(off_bal));
    writeU64(out, 532, ep.slot_number orelse 0);

    @memcpy(out[off_extra_data..off_transactions], ep.extra_data);
    @memcpy(out[off_transactions..off_withdrawals], txs_bytes);
    @memcpy(out[off_withdrawals..off_bal], wd_bytes);
    @memcpy(out[off_bal..], ep.block_access_list);

    return out;
}

// ── SszNewPayloadRequest encoder ──────────────────────────────────────────────

/// Fixed head: execution_payload offset(4) + versioned_hashes offset(4) +
/// parent_beacon_block_root(32) + execution_requests offset(4) = 44.
const NPR_FIXED_SIZE: usize = 44;

fn encodeNewPayloadRequest(alloc: std.mem.Allocator, npr: input.NewPayloadRequest) ![]u8 {
    const ep_bytes = try encodeExecutionPayload(alloc, npr.execution_payload);

    // versioned_hashes: List[Bytes32, 4096] — packed 32-byte elements, no offset table.
    const vh_bytes = try alloc.alloc(u8, npr.versioned_hashes.len * 32);
    for (npr.versioned_hashes, 0..) |h, i| @memcpy(vh_bytes[i * 32 ..][0..32], &h);

    const er_bytes = try encodeExecutionRequests(alloc, npr.execution_requests);

    const off_ep: usize = NPR_FIXED_SIZE;
    const off_vh: usize = off_ep + ep_bytes.len;
    const off_er: usize = off_vh + vh_bytes.len;
    const total: usize = off_er + er_bytes.len;

    const out = try alloc.alloc(u8, total);
    writeU32(out, 0, @intCast(off_ep));
    writeU32(out, 4, @intCast(off_vh));
    @memcpy(out[8..40], &npr.parent_beacon_block_root);
    writeU32(out, 40, @intCast(off_er));

    @memcpy(out[off_ep..off_vh], ep_bytes);
    @memcpy(out[off_vh..off_er], vh_bytes);
    @memcpy(out[off_er..], er_bytes);

    return out;
}

// ── SszExecutionWitness encoder ───────────────────────────────────────────────

/// Fixed head: state(nodes) offset(4) + codes offset(4) + headers offset(4) = 12.
const WITNESS_FIXED_SIZE: usize = 12;

fn encodeExecutionWitness(alloc: std.mem.Allocator, w: input.ExecutionWitness) ![]u8 {
    const nodes_bytes = try encodeByteListList(alloc, w.nodes);
    const codes_bytes = try encodeByteListList(alloc, w.codes);
    const headers_bytes = try encodeByteListList(alloc, w.headers);

    const off_state: usize = WITNESS_FIXED_SIZE;
    const off_codes: usize = off_state + nodes_bytes.len;
    const off_headers: usize = off_codes + codes_bytes.len;
    const total: usize = off_headers + headers_bytes.len;

    const out = try alloc.alloc(u8, total);
    writeU32(out, 0, @intCast(off_state));
    writeU32(out, 4, @intCast(off_codes));
    writeU32(out, 8, @intCast(off_headers));

    @memcpy(out[off_state..off_codes], nodes_bytes);
    @memcpy(out[off_codes..off_headers], codes_bytes);
    @memcpy(out[off_headers..], headers_bytes);

    return out;
}

// ── SszForkActivation encoder ─────────────────────────────────────────────────

/// Fixed head: block_number-list offset(4) + timestamp-list offset(4) = 8. Each list holds 0 or 1
/// uint64 — presence is entirely conveyed by the encoded length, exactly like the decoder reads it.
const ACTIVATION_FIXED_SIZE: usize = 8;

fn encodeForkActivation(alloc: std.mem.Allocator, activation_block: ?u64, activation_timestamp: ?u64) ![]u8 {
    const off_bn: usize = ACTIVATION_FIXED_SIZE;
    const off_ts: usize = off_bn + @as(usize, if (activation_block != null) 8 else 0);
    const total: usize = off_ts + @as(usize, if (activation_timestamp != null) 8 else 0);

    const out = try alloc.alloc(u8, total);
    writeU32(out, 0, @intCast(off_bn));
    writeU32(out, 4, @intCast(off_ts));
    if (activation_block) |b| writeU64(out, off_bn, b);
    if (activation_timestamp) |t| writeU64(out, off_ts, t);
    return out;
}

// ── SszForkConfig encoder ─────────────────────────────────────────────────────

/// Fixed head: activation_offset(4) = 4 (only field — fork identity travels in the schema prefix, not
/// here; zkevm@v0.6.2 dropped the fork/blob_schedule fields this container used to carry).
const FORK_CONFIG_FIXED_SIZE: usize = 4;

fn encodeForkConfig(alloc: std.mem.Allocator, activation_block: ?u64, activation_timestamp: ?u64) ![]u8 {
    const activation_bytes = try encodeForkActivation(alloc, activation_block, activation_timestamp);
    const out = try alloc.alloc(u8, FORK_CONFIG_FIXED_SIZE + activation_bytes.len);
    writeU32(out, 0, @intCast(FORK_CONFIG_FIXED_SIZE));
    @memcpy(out[FORK_CONFIG_FIXED_SIZE..], activation_bytes);
    return out;
}

// ── SszChainConfig encoder ────────────────────────────────────────────────────

/// Fixed head: chain_id(8) + active_fork offset(4) = 12.
const CHAIN_CONFIG_FIXED_SIZE: usize = 12;

fn encodeChainConfig(alloc: std.mem.Allocator, cc: input.ChainConfig) ![]u8 {
    const fork_config_bytes = try encodeForkConfig(alloc, cc.activation_block, cc.activation_timestamp);
    const out = try alloc.alloc(u8, CHAIN_CONFIG_FIXED_SIZE + fork_config_bytes.len);
    writeU64(out, 0, cc.chain_id);
    writeU32(out, 8, @intCast(CHAIN_CONFIG_FIXED_SIZE));
    @memcpy(out[CHAIN_CONFIG_FIXED_SIZE..], fork_config_bytes);
    return out;
}

// ── Top-level encoder ──────────────────────────────────────────────────────────

/// 2-byte schema id (fork byte from `chain_config.active_fork_idx` + revision byte 0x01), followed by
/// the 16-byte all-variable v0.4.1 fixed head: new_payload_request offset(4) + witness offset(4) +
/// chain_config offset(4) + public_keys offset(4).
const SCHEMA_SIZE: usize = 2;
const BODY_FIXED_SIZE: usize = 16;

/// Public keys are packed ByteVector[65] elements (uncompressed secp256k1, 0x04 prefix retained) — no
/// offset table, just concatenation.
const PUBKEY_SIZE: usize = 65;

/// Encode a `StatelessInput` into the SSZ `SszStatelessInput` bytes the decoder accepts. The exact
/// byte-level inverse of `decode`: `decode(alloc, encode(alloc, si))` reproduces `si`.
///
/// `chain_config.fork_name` carries no wire bytes of its own — it is a display string the decoder
/// derives from the schema's fork byte, so encoding reads `active_fork_idx` for that byte and leaves
/// `fork_name` unread.
pub fn encode(alloc: std.mem.Allocator, si: input.StatelessInput) ![]u8 {
    const npr_bytes = try encodeNewPayloadRequest(alloc, si.new_payload_request);
    const witness_bytes = try encodeExecutionWitness(alloc, si.witness);
    const chain_config_bytes = try encodeChainConfig(alloc, si.chain_config);

    const pubkeys_bytes = try alloc.alloc(u8, si.public_keys.len * PUBKEY_SIZE);
    for (si.public_keys, 0..) |key, i| {
        if (key.len != PUBKEY_SIZE) return error.InvalidPublicKeySize;
        @memcpy(pubkeys_bytes[i * PUBKEY_SIZE ..][0..PUBKEY_SIZE], key);
    }

    const off_npr: usize = BODY_FIXED_SIZE;
    const off_witness: usize = off_npr + npr_bytes.len;
    const off_chain_config: usize = off_witness + witness_bytes.len;
    const off_pubkeys: usize = off_chain_config + chain_config_bytes.len;
    const total: usize = SCHEMA_SIZE + off_pubkeys + pubkeys_bytes.len;

    const out = try alloc.alloc(u8, total);
    out[0] = @intCast(si.chain_config.active_fork_idx);
    out[1] = 0x01;

    const body = out[SCHEMA_SIZE..];
    writeU32(body, 0, @intCast(off_npr));
    writeU32(body, 4, @intCast(off_witness));
    writeU32(body, 8, @intCast(off_chain_config));
    writeU32(body, 12, @intCast(off_pubkeys));

    @memcpy(body[off_npr..off_witness], npr_bytes);
    @memcpy(body[off_witness..off_chain_config], witness_bytes);
    @memcpy(body[off_chain_config..off_pubkeys], chain_config_bytes);
    @memcpy(body[off_pubkeys..], pubkeys_bytes);

    return out;
}
