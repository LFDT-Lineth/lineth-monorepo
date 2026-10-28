//! Engine-API block-hash check (execution_engine/new_payload.py `is_valid_block_hash`): rebuild the
//! header the payload implies and require its hash to equal `block_hash`. This binds the payload's
//! transactions, withdrawals, requests and block access list to the header the proof commits to.
//!
//! Adapted from zesu's `stateless/executor/block_hash.zig`, which zesu's own `executeStatelessInput`
//! runs but does not expose (same reason `execution.zig` copies `buildEnv`). Keep in sync with it on
//! a zesu bump.

const std = @import("std");

const executor = @import("zesu_executor");
const primitives = @import("zesu_primitives");
const input = @import("zesu_input");
const mpt = @import("zesu_mpt");

const rlp = executor.executor_rlp_encode;
const mpt_builder = mpt.builder;

/// keccak256(rlp([])): the ommers hash of every post-merge header.
const EMPTY_OMMER_HASH: [32]u8 = .{
    0x1d, 0xcc, 0x4d, 0xe8, 0xde, 0xc7, 0x5d, 0x7a, 0xab, 0x85, 0xb5, 0x67, 0xb6, 0xcc, 0xd4, 0x1a,
    0xd3, 0x12, 0x45, 0x1b, 0x94, 0x8a, 0x74, 0x13, 0xf0, 0xa1, 0x42, 0xfd, 0x40, 0xd4, 0x93, 0x47,
};

/// RLP-encoded header implied by `req` under `spec`.
pub fn encodeHeader(
    alloc: std.mem.Allocator,
    req: input.NewPayloadRequest,
    spec: primitives.SpecId,
    requests_hash: [32]u8,
    block_access_list_hash: [32]u8,
) ![]u8 {
    const ep = &req.execution_payload;
    var fields = std.ArrayListUnmanaged([]const u8).empty;

    try fields.append(alloc, try rlp.encodeBytes(alloc, &ep.parent_hash)); // [0]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &EMPTY_OMMER_HASH)); // [1]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &ep.fee_recipient)); // [2]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &ep.state_root)); // [3]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &try executor.computeRawTxRoot(alloc, ep.raw_transactions))); // [4]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &ep.receipts_root)); // [5]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &ep.logs_bloom)); // [6]
    try fields.append(alloc, try rlp.encodeU64(alloc, 0)); // [7] difficulty
    try fields.append(alloc, try rlp.encodeU64(alloc, ep.block_number)); // [8]
    try fields.append(alloc, try rlp.encodeU64(alloc, ep.gas_limit)); // [9]
    try fields.append(alloc, try rlp.encodeU64(alloc, ep.gas_used)); // [10]
    try fields.append(alloc, try rlp.encodeU64(alloc, ep.timestamp)); // [11]
    try fields.append(alloc, try rlp.encodeBytes(alloc, ep.extra_data)); // [12]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &ep.prev_randao)); // [13]
    try fields.append(alloc, try rlp.encodeBytes(alloc, &[_]u8{0} ** 8)); // [14] nonce
    if (primitives.isEnabledIn(spec, .london)) {
        try fields.append(alloc, try rlp.encodeU64(alloc, ep.base_fee_per_gas)); // [15]
    }
    if (primitives.isEnabledIn(spec, .shanghai)) {
        try fields.append(alloc, try rlp.encodeBytes(alloc, &try withdrawalsRoot(alloc, ep.withdrawals))); // [16]
    }
    if (primitives.isEnabledIn(spec, .cancun)) {
        try fields.append(alloc, try rlp.encodeU64(alloc, ep.blob_gas_used)); // [17]
        try fields.append(alloc, try rlp.encodeU64(alloc, ep.excess_blob_gas)); // [18]
        try fields.append(alloc, try rlp.encodeBytes(alloc, &req.parent_beacon_block_root)); // [19]
    }
    if (primitives.isEnabledIn(spec, .prague)) {
        try fields.append(alloc, try rlp.encodeBytes(alloc, &requests_hash)); // [20]
    }
    if (primitives.isEnabledIn(spec, .amsterdam)) {
        try fields.append(alloc, try rlp.encodeBytes(alloc, &block_access_list_hash)); // [21]
        try fields.append(alloc, try rlp.encodeU64(alloc, ep.slot_number orelse 0)); // [22]
    }

    return rlp.encodeList(alloc, fields.items);
}

/// Fails with `error.InvalidBlockHash` unless the header `req` implies hashes to its `block_hash`.
pub fn validate(
    alloc: std.mem.Allocator,
    req: input.NewPayloadRequest,
    spec: primitives.SpecId,
    requests_hash: [32]u8,
    block_access_list_hash: [32]u8,
) !void {
    const header = try encodeHeader(alloc, req, spec, requests_hash, block_access_list_hash);
    if (!std.mem.eql(u8, &rlp.keccak256(header), &req.execution_payload.block_hash)) return error.InvalidBlockHash;
}

/// Withdrawals trie: key = RLP(index), value = RLP([index, validator_index, address, amount]).
fn withdrawalsRoot(alloc: std.mem.Allocator, withdrawals: []const input.Withdrawal) ![32]u8 {
    if (withdrawals.len == 0) return mpt_builder.EMPTY_TRIE_HASH;
    const items = try alloc.alloc(mpt_builder.KV, withdrawals.len);
    for (withdrawals, 0..) |wd, i| {
        items[i].key = try rlp.encodeU64(alloc, i);
        items[i].value = try rlp.encodeList(alloc, &.{
            try rlp.encodeU64(alloc, wd.index),
            try rlp.encodeU64(alloc, wd.validator_index),
            try rlp.encodeBytes(alloc, &wd.address),
            try rlp.encodeU64(alloc, wd.amount),
        });
    }
    return mpt_builder.trieRoot(alloc, items);
}
