//! Rollup guest business logic: `RollupProofPrivateInput -> RollupPublicInput`, by echo or sentinel.
//! Every output field is either derived from the l2-execution proofs and the request fields the
//! way the specification derives it, or set to a fixed sentinel constant: the data-availability
//! tail fields, the data rolling hash and the messaging offsets are sentinels.

const std = @import("std");
const core_serde = @import("core_serde");
const rollup_ssz = @import("rollup_ssz.zig");

const rollup_proof = core_serde.rollup_proof;

// ── Sentinels ─────────────────────────────────────────────────────────────────────────────────
// Each value is keccak256 of its own tag string, computed at comptime — `std.crypto.hash.sha3.
// Keccak256` is pure Zig (the legacy 0x01-delimiter variant, i.e. Ethereum's keccak256, not NIST
// SHA3's 0x06 delimiter), so there is nothing precomputed or pinned to fall out of sync with its
// source string. Integer sentinels take the hash's leading bytes, big-endian.

const Keccak256 = std.crypto.hash.sha3.Keccak256;

fn sentinelHash(comptime tag: []const u8) [32]u8 {
    @setEvalBranchQuota(10_000); // one Keccak-f[1600] permutation (24 rounds) exceeds the default 1000
    var out: [32]u8 = undefined;
    Keccak256.hash(tag, &out, .{});
    return out;
}

fn sentinelInt(comptime T: type, comptime tag: []const u8) T {
    return std.mem.readInt(T, sentinelHash(tag)[0..@sizeOf(T)], .big);
}

pub const END_DATA_ROLLING_HASH: [32]u8 = sentinelHash("lineth.stub.rollup.endDataRollingHash");
pub const FINAL_DATA_TAIL_DISCARD_BYTES: u64 = sentinelInt(u64, "lineth.stub.rollup.finalDataTailDiscardBytes");
pub const L2_MESSAGING_BLOCKS_OFFSET: u16 = sentinelInt(u16, "lineth.stub.rollup.l2MessagingBlocksOffset");

/// Runs the rollup guest's echo/sentinel mapping over a decoded input. Requires at least one
/// `l2_execution_proofs` element — "first"/"last" source every per-proof field.
pub fn run(alloc: std.mem.Allocator, input: rollup_ssz.RollupProofPrivateInput) !rollup_proof.RollupPublicInput {
    if (input.l2_execution_proofs.len == 0) return error.EmptyProofs;
    const first = input.l2_execution_proofs[0].proof.public_inputs;
    const last = input.l2_execution_proofs[input.l2_execution_proofs.len - 1].proof.public_inputs;

    const program_vks = try alloc.alloc([32]u8, input.l2_execution_proofs.len);
    var message_count: usize = 0;
    var address_count: usize = 0;
    var block_count: u64 = 0;
    var end_bridge_rolling_hash: [32]u8 = @splat(0);
    var end_bridge_message_number: u64 = 0;
    for (input.l2_execution_proofs, program_vks) |*p, *vk| {
        vk.* = p.program_vk;
        message_count += p.proof.public_inputs.l2_l1_messages.len;
        address_count += p.proof.public_inputs.filtered_addresses.len;
        block_count += p.proof.public_inputs.block_count;
        if (p.proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number != 0) {
            end_bridge_rolling_hash = p.proof.public_inputs.end_l1_l2_bridge_rolling_hash;
            end_bridge_message_number = p.proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number;
        }
    }

    const l2_l1_messages = try alloc.alloc([32]u8, message_count);
    const all_addresses = try alloc.alloc([20]u8, address_count);
    var message_pos: usize = 0;
    var address_pos: usize = 0;
    for (input.l2_execution_proofs) |p| {
        const pi = p.proof.public_inputs;
        @memcpy(l2_l1_messages[message_pos..][0..pi.l2_l1_messages.len], pi.l2_l1_messages);
        message_pos += pi.l2_l1_messages.len;
        @memcpy(all_addresses[address_pos..][0..pi.filtered_addresses.len], pi.filtered_addresses);
        address_pos += pi.filtered_addresses.len;
    }

    const filtered_addresses = sortedDistinct(20, all_addresses);
    if (filtered_addresses.len > rollup_proof.MAX_FILTERED_ADDRESSES) return error.BoundsViolation;

    return .{
        .end_block_number = last.end_block_number,
        .end_block_timestamp = last.end_block_timestamp,
        .end_l1_l2_bridge_rolling_hash = end_bridge_rolling_hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_bridge_message_number,
        .dynamic_chain_config_hash = first.dynamic_chain_config_hash,
        .parent_ftx_rolling_hash = first.parent_ftx_rolling_hash,
        .parent_ftx_number = first.parent_ftx_number,
        .end_ftx_rolling_hash = last.end_ftx_rolling_hash,
        .end_processed_ftx_number = last.end_processed_ftx_number,
        .parent_data_rolling_hash = input.parent_data_rolling_hash,
        .end_data_rolling_hash = END_DATA_ROLLING_HASH,
        .parent_block_hash = first.parent_block_hash,
        .end_block_hash = last.end_block_hash,
        .parent_data_tail_take_bytes = input.parent_data_tail_take_bytes,
        .final_data_tail_discard_bytes = FINAL_DATA_TAIL_DISCARD_BYTES,
        .l2_l1_messages = l2_l1_messages,
        .filtered_addresses = filtered_addresses,
        .program_vks = sortedDistinct(32, program_vks),
        .block_count = block_count,
        .l2_messaging_blocks_offsets = try alloc.dupe(u16, &.{L2_MESSAGING_BLOCKS_OFFSET}),
    };
}

/// The distinct elements of `items`, sorted ascending bytewise, in place at the head of `items`.
fn sortedDistinct(comptime n: usize, items: [][n]u8) []const [n]u8 {
    std.mem.sort([n]u8, items, {}, struct {
        fn lessThan(_: void, a: [n]u8, b: [n]u8) bool {
            return std.mem.order(u8, &a, &b) == .lt;
        }
    }.lessThan);

    var unique_count: usize = 0;
    for (items) |item| {
        if (unique_count == 0 or !std.mem.eql(u8, &item, &items[unique_count - 1])) {
            items[unique_count] = item;
            unique_count += 1;
        }
    }
    return items[0..unique_count];
}
