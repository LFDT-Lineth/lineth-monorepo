//! Extended SSZ decoder for SszStatelessInput — drop-in replacement for zesu's `ssz_decode` module.
//!
//! Adds v0.8.0 format support on top of zesu's existing v0.4.1 decoder.
//!
//! The two layouts differ only in the SszStatelessInput fixed region:
//!
//!   v0.4.1 (off_npr == 16): 4 × u32 variable offsets → [npr, witness, chain_config, pubkeys]
//!     The `chain_config` section is a nested SszChainConfig (chain_id + SszForkConfig).
//!
//!   v0.8.0 (off_npr == 20): 2 × u32 variable offsets + u64 chain_id inline + 1 × u32 offset
//!     → [off_npr, off_witness, chain_id_u64, off_pubkeys, then variable: npr, witness, pubkeys]
//!     No SszChainConfig variable section; fork identity comes from the schema_id byte alone.
//!
//! All other containers (SszNewPayloadRequest, SszExecutionPayload, SszExecutionWitness) are
//! identical across both format versions.
//!
//! The EF execution-spec-tests zkevm corpus (tests-zkevm@v0.6.2) uses v0.4.1 format.
//! Linea mainnet fixture archives encoded by stateless-executor ≥ v0.8.0 use v0.8.0 format.

const std = @import("std");
const input_mod = @import("input");
const rlp_decode = @import("rlp_decode");

// ── Primitive reads (little-endian) ──────────────────────────────────────────

inline fn readU32(data: []const u8, off: usize) u32 {
    return std.mem.readInt(u32, data[off..][0..4], .little);
}

inline fn readU64(data: []const u8, off: usize) u64 {
    return std.mem.readInt(u64, data[off..][0..8], .little);
}

// ── Fork enum ────────────────────────────────────────────────────────────────

fn forkNameFromSchemaByte(b: u8) []const u8 {
    return switch (b) {
        0x01 => "Frontier",
        0x02 => "Homestead",
        0x03 => "DAOFork",
        0x04 => "TangerineWhistle",
        0x05 => "SpuriousDragon",
        0x06 => "Byzantium",
        0x07 => "StPetersburg",
        0x08 => "Istanbul",
        0x09 => "MuirGlacier",
        0x0a => "Berlin",
        0x0b => "London",
        0x0c => "ArrowGlacier",
        0x0d => "GrayGlacier",
        0x0e => "Paris",
        0x0f => "Shanghai",
        0x10 => "Cancun",
        0x11 => "Prague",
        0x12 => "Osaka",
        0x13 => "BPO1",
        0x14 => "BPO2",
        0x15 => "Amsterdam",
        else => "",
    };
}

// ── List[ByteList] decoder ────────────────────────────────────────────────────

fn decodeByteListList(alloc: std.mem.Allocator, data: []const u8) ![]const []const u8 {
    if (data.len == 0) return &.{};
    if (data.len < 4) return error.InvalidSsz;

    const first_off = readU32(data, 0);
    if (first_off == 0 or first_off % 4 != 0) return error.InvalidSsz;
    if (first_off > data.len) return error.InvalidSsz;
    const n = first_off / 4;

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

// ── SszWithdrawal decoder ─────────────────────────────────────────────────────

const WITHDRAWAL_SIZE: usize = 44;

fn decodeWithdrawal(bytes: *const [WITHDRAWAL_SIZE]u8) input_mod.Withdrawal {
    const index = std.mem.readInt(u64, bytes[0..8], .little);
    const validator_index = std.mem.readInt(u64, bytes[8..16], .little);
    var address: [20]u8 = undefined;
    @memcpy(&address, bytes[16..36]);
    const amount = std.mem.readInt(u64, bytes[36..44], .little);
    return .{
        .index = index,
        .validator_index = validator_index,
        .address = address,
        .amount = amount,
    };
}

// ── Top-level decoder ─────────────────────────────────────────────────────────

const EP_FIXED_SIZE: usize = 540;

/// Decode SSZ-serialized SszStatelessInput, supporting both the v0.4.1 (16-byte fixed region)
/// and v0.8.0 (20-byte fixed region, inline chain_id) container layouts.
///
/// Detection: `off_npr == 16` → v0.4.1; `off_npr == 20` → v0.8.0; anything else → InvalidSsz.
pub fn decode(alloc: std.mem.Allocator, data: []const u8) !input_mod.StatelessInput {
    const payload = if (data.len >= 4 and
        std.mem.readInt(u32, data[0..4], .little) == data.len - 4)
        data[4..]
    else
        data;

    if (payload.len < 2 or payload[1] != 0x01) return error.InvalidSsz;
    const fork_name_bytes = forkNameFromSchemaByte(payload[0]);
    if (fork_name_bytes.len == 0) return error.InvalidSsz;

    const body = payload[2..];
    if (body.len < 16) return error.InvalidSsz;
    const off_npr: usize = readU32(body, 0);
    const off_witness: usize = readU32(body, 4);

    // Detect format by fixed-region size encoded as off_npr (first variable offset).
    const is_v8 = (off_npr == 20); // v0.8.0: inline chain_id, 3 variable fields
    const is_v4 = (off_npr == 16); // v0.4.1: nested SszChainConfig, 4 variable fields
    if (!is_v4 and !is_v8) return error.InvalidSsz;

    var chain_id: u64 = 1;
    var off_pubkeys: usize = undefined;
    var off_witness_end: usize = undefined; // off_chain_config (v0.4.1) or off_pubkeys (v0.8.0)
    var activation_block: ?u64 = null;
    var activation_timestamp: ?u64 = null;

    if (is_v8) {
        // v0.8.0: body layout = [4: off_npr=20][4: off_witness][8: chain_id][4: off_pubkeys][variable...]
        if (body.len < 20) return error.InvalidSsz;
        chain_id = readU64(body, 8);
        off_pubkeys = readU32(body, 16);
        off_witness_end = off_pubkeys; // witness ends where pubkeys begin
        if (off_witness > body.len or off_pubkeys > body.len) return error.InvalidSsz;
        if (off_npr > off_witness or off_witness > off_pubkeys) return error.InvalidSsz;
    } else {
        // v0.4.1: body layout = [4: off_npr=16][4: off_witness][4: off_chain_config][4: off_pubkeys][variable...]
        const off_chain_config: usize = readU32(body, 8);
        off_pubkeys = readU32(body, 12);
        off_witness_end = off_chain_config; // witness ends where chain_config begins
        if (off_witness > body.len or off_chain_config > body.len or off_pubkeys > body.len) return error.InvalidSsz;
        if (off_npr > off_witness or off_witness > off_chain_config or off_chain_config > off_pubkeys) return error.InvalidSsz;

        const chain_config_data = body[off_chain_config..off_pubkeys];
        if (chain_config_data.len < 12) return error.InvalidSsz;
        chain_id = readU64(chain_config_data, 0);
        const off_active_fork: usize = readU32(chain_config_data, 8);
        if (off_active_fork + 4 > chain_config_data.len) return error.InvalidSsz;
        {
            const af = chain_config_data[off_active_fork..];
            const off_activation: usize = readU32(af, 0);
            if (off_activation + 8 <= af.len) {
                const act = af[off_activation..];
                const off_bn: usize = readU32(act, 0);
                const off_ts: usize = readU32(act, 4);
                if (off_bn <= off_ts and off_ts <= act.len) {
                    if (off_ts - off_bn >= 8) activation_block = readU64(act, off_bn);
                    if (act.len - off_ts >= 8) activation_timestamp = readU64(act, off_ts);
                }
            }
        }
    }

    const npr_data = body[off_npr..off_witness];
    const witness_data = body[off_witness..off_witness_end];
    const pubkeys_data = body[off_pubkeys..];

    // ── SszNewPayloadRequest fixed region (44 bytes) ──────────────────────────
    if (npr_data.len < 44) return error.InvalidSsz;
    const off_ep: usize = readU32(npr_data, 0);
    const off_vh: usize = readU32(npr_data, 4);
    const off_er: usize = readU32(npr_data, 40);

    var parent_beacon_root: [32]u8 = undefined;
    @memcpy(&parent_beacon_root, npr_data[8..40]);

    if (off_ep < 44 or off_vh > npr_data.len or off_er > npr_data.len) return error.InvalidSsz;
    if (off_ep >= off_vh or off_vh > off_er) return error.InvalidSsz;

    const ep_data = npr_data[off_ep..off_vh];

    const vh_bytes = npr_data[off_vh..off_er];
    if (vh_bytes.len % 32 != 0) return error.InvalidSsz;
    const vh_count = vh_bytes.len / 32;
    const versioned_hashes = try alloc.alloc([32]u8, vh_count);
    for (0..vh_count) |i| @memcpy(&versioned_hashes[i], vh_bytes[i * 32 ..][0..32]);

    const er_data = npr_data[off_er..];
    if (er_data.len < 12) return error.InvalidSsz;
    const off_deposits: usize = readU32(er_data, 0);
    if (off_deposits < 12 or off_deposits % 4 != 0 or off_deposits > er_data.len) return error.InvalidSsz;
    const n_types = off_deposits / 4;
    var er_offsets: [8]usize = undefined;
    for (0..@min(n_types, 8)) |i| er_offsets[i] = readU32(er_data, i * 4);
    const getSlice = struct {
        fn f(er_bytes: []const u8, offs: []const usize, n: usize, idx: usize) ![]const u8 {
            if (idx >= n) return &.{};
            const start = offs[idx];
            const end = if (idx + 1 < n) offs[idx + 1] else er_bytes.len;
            if (start > end or end > er_bytes.len) return error.InvalidSsz;
            return er_bytes[start..end];
        }
    }.f;
    const execution_requests: input_mod.ExecutionRequests = .{
        .deposits = try getSlice(er_data, &er_offsets, n_types, 0),
        .withdrawals = try getSlice(er_data, &er_offsets, n_types, 1),
        .consolidations = try getSlice(er_data, &er_offsets, n_types, 2),
        .builder_deposits = try getSlice(er_data, &er_offsets, n_types, 3),
        .builder_exits = try getSlice(er_data, &er_offsets, n_types, 4),
    };

    // ── SszExecutionPayload fixed region ─────────────────────────────────────────
    const EP_V3_FIXED_SIZE: usize = 528;
    if (ep_data.len < EP_V3_FIXED_SIZE) return error.InvalidSsz;

    var parent_hash: [32]u8 = undefined;
    @memcpy(&parent_hash, ep_data[0..32]);

    var fee_recipient: [20]u8 = undefined;
    @memcpy(&fee_recipient, ep_data[32..52]);

    var state_root: [32]u8 = undefined;
    @memcpy(&state_root, ep_data[52..84]);

    var receipts_root: [32]u8 = undefined;
    @memcpy(&receipts_root, ep_data[84..116]);

    var logs_bloom: [256]u8 = undefined;
    @memcpy(&logs_bloom, ep_data[116..372]);

    var prev_randao: [32]u8 = undefined;
    @memcpy(&prev_randao, ep_data[372..404]);

    const block_number: u64 = readU64(ep_data, 404);
    const gas_limit: u64 = readU64(ep_data, 412);
    const gas_used: u64 = readU64(ep_data, 420);
    const timestamp: u64 = readU64(ep_data, 428);

    const off_extra_data: usize = readU32(ep_data, 436);
    const base_fee_per_gas: u64 = readU64(ep_data, 440);
    var block_hash: [32]u8 = undefined;
    @memcpy(&block_hash, ep_data[472..504]);
    const off_transactions: usize = readU32(ep_data, 504);
    const off_withdrawals: usize = readU32(ep_data, 508);
    const blob_gas_used: u64 = readU64(ep_data, 512);
    const excess_blob_gas: u64 = readU64(ep_data, 520);

    const ep_is_amsterdam = (off_extra_data == EP_FIXED_SIZE);
    const ep_is_v3 = (off_extra_data == EP_V3_FIXED_SIZE);
    if (!ep_is_amsterdam and !ep_is_v3) return error.InvalidSsz;
    if (ep_is_amsterdam and ep_data.len < EP_FIXED_SIZE) return error.InvalidSsz;
    const off_block_access_list: usize = if (ep_is_amsterdam) readU32(ep_data, 528) else ep_data.len;
    const slot_number: ?u64 = if (ep_is_amsterdam) readU64(ep_data, 532) else null;

    if (off_extra_data > off_transactions or off_transactions > off_withdrawals or
        off_withdrawals > off_block_access_list) return error.InvalidSsz;
    if (off_block_access_list > ep_data.len) return error.InvalidSsz;

    const extra_data = try alloc.dupe(u8, ep_data[off_extra_data..off_transactions]);

    const txs_raw = try decodeByteListList(alloc, ep_data[off_transactions..off_withdrawals]);
    const transactions = try alloc.alloc(input_mod.Transaction, txs_raw.len);
    for (txs_raw, 0..) |raw_tx, i| {
        transactions[i] = try rlp_decode.decodeSingleTx(alloc, raw_tx);
    }

    const block_access_list = try alloc.dupe(u8, ep_data[off_block_access_list..]);

    const wd_bytes = ep_data[off_withdrawals..off_block_access_list];
    if (wd_bytes.len % WITHDRAWAL_SIZE != 0) return error.InvalidSsz;
    const wcount = wd_bytes.len / WITHDRAWAL_SIZE;
    const withdrawals = try alloc.alloc(input_mod.Withdrawal, wcount);
    for (0..wcount) |i| {
        withdrawals[i] = decodeWithdrawal(wd_bytes[i * WITHDRAWAL_SIZE ..][0..WITHDRAWAL_SIZE]);
    }

    // ── SszExecutionWitness fixed region (12 bytes) ───────────────────────────
    if (witness_data.len < 12) return error.InvalidSsz;
    const off_state: usize = readU32(witness_data, 0);
    const off_codes: usize = readU32(witness_data, 4);
    const off_headers: usize = readU32(witness_data, 8);

    if (off_state < 12 or off_headers > witness_data.len) return error.InvalidSsz;
    if (off_state > off_codes or off_codes > off_headers) return error.InvalidSsz;

    const nodes = try decodeByteListList(alloc, witness_data[off_state..off_codes]);
    const codes = try decodeByteListList(alloc, witness_data[off_codes..off_headers]);
    const headers = try decodeByteListList(alloc, witness_data[off_headers..]);

    // ── Public keys ───────────────────────────────────────────────────────────
    const PUBKEY_SIZE: usize = 65;
    if (pubkeys_data.len % PUBKEY_SIZE != 0) return error.InvalidSsz;
    const pubkey_count = pubkeys_data.len / PUBKEY_SIZE;
    const public_keys = try alloc.alloc([]const u8, pubkey_count);
    for (0..pubkey_count) |i| {
        public_keys[i] = pubkeys_data[i * PUBKEY_SIZE ..][0..PUBKEY_SIZE];
    }

    return input_mod.StatelessInput{
        .new_payload_request = .{
            .execution_payload = .{
                .parent_hash = parent_hash,
                .fee_recipient = fee_recipient,
                .state_root = state_root,
                .receipts_root = receipts_root,
                .logs_bloom = logs_bloom,
                .prev_randao = prev_randao,
                .block_number = block_number,
                .gas_limit = gas_limit,
                .gas_used = gas_used,
                .timestamp = timestamp,
                .extra_data = extra_data,
                .base_fee_per_gas = base_fee_per_gas,
                .block_hash = block_hash,
                .transactions = transactions,
                .raw_transactions = txs_raw,
                .withdrawals = withdrawals,
                .blob_gas_used = blob_gas_used,
                .excess_blob_gas = excess_blob_gas,
                .slot_number = slot_number,
                .block_access_list = block_access_list,
            },
            .parent_beacon_block_root = parent_beacon_root,
            .versioned_hashes = versioned_hashes,
            .execution_requests = execution_requests,
        },
        .witness = .{
            .nodes = nodes,
            .codes = codes,
            .headers = headers,
        },
        .chain_config = .{
            .chain_id = if (chain_id != 0) chain_id else 1,
            .fork_name = if (fork_name_bytes.len > 0) fork_name_bytes else null,
            .active_fork_idx = payload[0],
            .activation_block = activation_block,
            .activation_timestamp = activation_timestamp,
        },
        .public_keys = public_keys,
    };
}
