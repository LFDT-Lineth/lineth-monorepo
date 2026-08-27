//! Host-only adapter: turn a vanilla BPO2 (or other pre-Amsterdam) stateless-input SSZ into an
//! extended L2 guest input (schema 0x0002) that the Amsterdam-only l2-execution guest will accept.
//!
//! The wrap tool (`vanilla_wrap`) only dummy-fills the outer rollup envelope; it copies the inner
//! vanilla bytes verbatim. Mainnet BPO2 blocks still fail after wrapping because:
//!   1. `runL2Execution` rejects non-empty beacon-chain withdrawals (Linea has none);
//!   2. the guest fork check requires `fork_name == "Amsterdam"` (schema byte 0x15, not 0x14);
//!   3. Amsterdam gas accounting (EIP-7778) and stripping withdrawals change the post-state, so
//!      the BPO2 header's `state_root` / `receipts_root` no longer match;
//!   4. Amsterdam requires an EIP-7928 block access list matching execution.
//!
//! Mainnet files from `zesu-convert` are zkevm v0.8.0 SSZ (inline `chain_id`, 20-byte head). This
//! guest's zesu pin still decodes v0.4.1 (nested `SszChainConfig`, 16-byte head), so the adapter
//! transcodes that layout first.
//!
//! Then it re-executes the block under Amsterdam via `executeStatelessInputTrace` (the same
//! `computeStateRootDelta` path the guest uses, without the post-execution commitment check),
//! writes the computed commitments back into the payload, re-encodes as Amsterdam vanilla SSZ,
//! then dummy-wraps that as extended input.

const std = @import("std");

const ssz_decode = @import("zesu_ssz_decode");
const mpt = @import("zesu_mpt");
const executor = @import("zesu_executor");
const l2_execution = @import("l2_execution");
const vanilla_wrap = @import("vanilla_wrap");
const stateless_input_encode = @import("stateless_input_encode");

const KECCAK_EMPTY: [32]u8 = .{
    0xc5, 0xd2, 0x46, 0x01, 0x86, 0xf7, 0x23, 0x3c, 0x92, 0x7e, 0x7d, 0xb2, 0xdc, 0xc7, 0x03, 0xc0,
    0xe5, 0x00, 0xb6, 0x53, 0xca, 0x82, 0x27, 0x3b, 0x7b, 0xfa, 0xd8, 0x04, 0x5d, 0x85, 0xa4, 0x70,
};

const types = executor.executor_types;
const rlp = executor.executor_rlp_encode;

pub const AMSTERDAM_FORK_NAME: []const u8 = "Amsterdam";
pub const AMSTERDAM_FORK_IDX: u64 = 0x15;

/// Decode `vanilla_ssz`, strip L1-only fields, re-execute as Amsterdam, patch header commitments,
/// re-encode as Amsterdam vanilla SSZ, and wrap as extended `L2ExecutionProofPrivateInput`.
pub fn adaptVanillaToAmsterdamExtended(alloc: std.mem.Allocator, vanilla_ssz: []const u8) ![]u8 {
    var si = ssz_decode.decode(alloc, vanilla_ssz) catch |err| switch (err) {
        error.InvalidSsz => try ssz_decode.decode(alloc, try transcodeV08ToV041(alloc, vanilla_ssz)),
        else => return err,
    };

    const requests = si.new_payload_request.execution_requests;
    if (requests.deposits.len != 0 or requests.withdrawals.len != 0 or requests.consolidations.len != 0) {
        return error.ExecutionRequestsNotSupported;
    }

    si.new_payload_request.execution_payload.withdrawals = &.{};
    si.chain_config.fork_name = AMSTERDAM_FORK_NAME;
    si.chain_config.active_fork_idx = AMSTERDAM_FORK_IDX;
    // Present and already-elapsed so wrap's activation-schedule skip does not fire, and the
    // guest's single-fork check sees Amsterdam active from genesis.
    si.chain_config.activation_block = null;
    si.chain_config.activation_timestamp = 0;

    var node_index = try mpt.buildNodeIndex(alloc, si.witness.nodes);
    defer node_index.deinit();

    const trace = try l2_execution.executeStatelessInputTrace(alloc, si, AMSTERDAM_FORK_NAME, &node_index, false);

    si.new_payload_request.execution_payload.state_root = trace.proof.post_state_root;
    si.new_payload_request.execution_payload.receipts_root = trace.proof.receipts_root;
    si.new_payload_request.execution_payload.gas_used = trace.cumulative_gas;
    si.new_payload_request.execution_payload.blob_gas_used = trace.blob_gas_used;
    si.new_payload_request.execution_payload.logs_bloom = bloomFromReceipts(trace.proof.receipts);
    si.new_payload_request.execution_payload.block_access_list = try encodeAccessedAsBal(alloc, trace.accessed, trace.post_alloc);

    const amsterdam_vanilla = try stateless_input_encode.encode(alloc, si);
    return vanilla_wrap.wrapVanillaAsExtended(alloc, amsterdam_vanilla);
}

/// zesu's decoder (this guest's pin) speaks zkevm v0.4.1: 16-byte all-variable head with a nested
/// `SszChainConfig`. Mainnet fixtures from `zesu-convert` / stateless-executor are zkevm v0.8.0:
/// 20-byte mixed head with inline `chain_id` and no nested fork config (fork is the schema byte).
fn transcodeV08ToV041(alloc: std.mem.Allocator, data: []const u8) ![]u8 {
    const payload = if (data.len >= 4 and std.mem.readInt(u32, data[0..4], .little) == data.len - 4)
        data[4..]
    else
        data;
    if (payload.len < 22 or payload[1] != 0x01) return error.InvalidSsz;

    const body = payload[2..];
    const off_npr = std.mem.readInt(u32, body[0..4], .little);
    // v0.8.0 fixed head is 20 bytes (off_npr, off_witness, chain_id u64, off_pubkeys).
    if (off_npr != 20) return error.InvalidSsz;
    const off_wit = std.mem.readInt(u32, body[4..8], .little);
    const chain_id = std.mem.readInt(u64, body[8..16], .little);
    const off_pk = std.mem.readInt(u32, body[16..20], .little);
    if (off_wit < off_npr or off_pk < off_wit or off_pk > body.len) return error.InvalidSsz;

    const npr = body[off_npr..off_wit];
    const wit = body[off_wit..off_pk];
    const pubkeys = body[off_pk..];

    // Nested SszChainConfig the v0.4.1 decoder expects, with activation_timestamp = 0 so the
    // single-fork guest's schedule check sees Amsterdam already active.
    var chain_config: [32]u8 = undefined;
    std.mem.writeInt(u64, chain_config[0..8], chain_id, .little);
    std.mem.writeInt(u32, chain_config[8..12], 12, .little); // offset → SszForkConfig
    std.mem.writeInt(u32, chain_config[12..16], 4, .little); // offset → SszForkActivation
    std.mem.writeInt(u32, chain_config[16..20], 8, .little); // activation_block list (empty)
    std.mem.writeInt(u32, chain_config[20..24], 8, .little); // activation_timestamp list
    std.mem.writeInt(u64, chain_config[24..32], 0, .little); // timestamp 0

    const npr_off: u32 = 16;
    const wit_off: u32 = npr_off + @as(u32, @intCast(npr.len));
    const cc_off: u32 = wit_off + @as(u32, @intCast(wit.len));
    const pk_off: u32 = cc_off + @as(u32, @intCast(chain_config.len));

    var out: std.ArrayList(u8) = .empty;
    try out.append(alloc, payload[0]);
    try out.append(alloc, payload[1]);
    var tmp: [4]u8 = undefined;
    std.mem.writeInt(u32, &tmp, npr_off, .little);
    try out.appendSlice(alloc, &tmp);
    std.mem.writeInt(u32, &tmp, wit_off, .little);
    try out.appendSlice(alloc, &tmp);
    std.mem.writeInt(u32, &tmp, cc_off, .little);
    try out.appendSlice(alloc, &tmp);
    std.mem.writeInt(u32, &tmp, pk_off, .little);
    try out.appendSlice(alloc, &tmp);
    try out.appendSlice(alloc, npr);
    try out.appendSlice(alloc, wit);
    try out.appendSlice(alloc, &chain_config);
    try out.appendSlice(alloc, pubkeys);
    return out.toOwnedSlice(alloc);
}

fn bloomFromReceipts(receipts: []const types.Receipt) [256]u8 {
    var bloom: [256]u8 = @splat(0);
    for (receipts) |r| {
        orBloom(&bloom, r.logs_bloom);
    }
    return bloom;
}

fn orBloom(dst: *[256]u8, src: [256]u8) void {
    for (dst, src) |*d, s| d.* |= s;
}

fn hashToU256(h: [32]u8) u256 {
    return std.mem.readInt(u256, &h, .big);
}

fn encodeAccessedAsBal(
    alloc: std.mem.Allocator,
    accessed: []const types.AccessedEntry,
    post_alloc: std.AutoHashMapUnmanaged(types.Address, types.AllocAccount),
) ![]u8 {
    if (accessed.len == 0) return &.{};

    // EIP-7928 / zesu validatePostExecution require strictly ascending address order.
    const sorted = try alloc.dupe(types.AccessedEntry, accessed);
    std.mem.sort(types.AccessedEntry, sorted, {}, struct {
        fn lessThan(_: void, a: types.AccessedEntry, b: types.AccessedEntry) bool {
            return std.mem.order(u8, &a.address, &b.address) == .lt;
        }
    }.lessThan);

    var items = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, sorted.len);
    for (sorted) |entry| {
        try items.append(alloc, try encodeBalEntry(alloc, entry, post_alloc));
    }
    return rlp.encodeList(alloc, items.items);
}

fn encodeBalEntry(
    alloc: std.mem.Allocator,
    entry: types.AccessedEntry,
    post_alloc: std.AutoHashMapUnmanaged(types.Address, types.AllocAccount),
) ![]u8 {
    var parts = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, 6);
    try parts.append(alloc, try rlp.encodeBytes(alloc, &entry.address));

    {
        var sc_items = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, entry.storage_changes.len);
        for (entry.storage_changes) |sc| {
            const slot_rlp = try rlp.encodeU256(alloc, hashToU256(sc.slot));
            const bai_rlp = try rlp.encodeU64(alloc, 0);
            const val_rlp = try rlp.encodeU256(alloc, sc.post_value);
            const changes_list = try rlp.encodeList(alloc, &.{try rlp.encodeList(alloc, &.{ bai_rlp, val_rlp })});
            try sc_items.append(alloc, try rlp.encodeList(alloc, &.{ slot_rlp, changes_list }));
        }
        try parts.append(alloc, try rlp.encodeList(alloc, sc_items.items));
    }

    {
        var sr_items = try std.ArrayListUnmanaged([]const u8).initCapacity(alloc, entry.storage_reads.len);
        for (entry.storage_reads) |slot| try sr_items.append(alloc, try rlp.encodeU256(alloc, hashToU256(slot)));
        try parts.append(alloc, try rlp.encodeList(alloc, sr_items.items));
    }

    {
        var bal_items = std.ArrayListUnmanaged([]const u8).empty;
        if (entry.pre_balance != entry.post_balance) {
            const bai_rlp = try rlp.encodeU64(alloc, 0);
            const val_rlp = try rlp.encodeU256(alloc, entry.post_balance);
            try bal_items.append(alloc, try rlp.encodeList(alloc, &.{ bai_rlp, val_rlp }));
        }
        try parts.append(alloc, try rlp.encodeList(alloc, bal_items.items));
    }

    {
        var nc_items = std.ArrayListUnmanaged([]const u8).empty;
        if (entry.pre_nonce != entry.post_nonce) {
            const bai_rlp = try rlp.encodeU64(alloc, 0);
            const val_rlp = try rlp.encodeU64(alloc, entry.post_nonce);
            try nc_items.append(alloc, try rlp.encodeList(alloc, &.{ bai_rlp, val_rlp }));
        }
        try parts.append(alloc, try rlp.encodeList(alloc, nc_items.items));
    }

    {
        var cc_items = std.ArrayListUnmanaged([]const u8).empty;
        if (!std.mem.eql(u8, &entry.pre_code_hash, &entry.post_code_hash)) {
            const code: []const u8 = blk: {
                if (std.mem.eql(u8, &entry.post_code_hash, &KECCAK_EMPTY)) break :blk &.{};
                if (post_alloc.get(entry.address)) |acct| break :blk acct.code;
                break :blk &.{};
            };
            const bai_rlp = try rlp.encodeU64(alloc, 0);
            const code_rlp = try rlp.encodeBytes(alloc, code);
            try cc_items.append(alloc, try rlp.encodeList(alloc, &.{ bai_rlp, code_rlp }));
        }
        try parts.append(alloc, try rlp.encodeList(alloc, cc_items.items));
    }

    return rlp.encodeList(alloc, parts.items);
}
