//! Scenario tests for the l2-execution guest's conflation logic, built on the plan DSL.
//!
//! One rich happy-path scenario exercises every public-input field at once over a realistic
//! 2-block range: real signed transactions, L2->L1 bridge messages, L1<->L2 bridge storage, and
//! forced transactions spanning both blocks — checked against independently-derived expected
//! values (Python-minted where the guest's own formula would otherwise only be checked against
//! itself). Twelve one-mutation scenarios each drift a single field or hook away from a realistic
//! default range, one per rejection the guest's conflation logic enforces.

const std = @import("std");
const testing = std.testing;

const executor = @import("zesu_executor");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const conflation_plan = @import("conflation_plan.zig");

const types = executor.executor_types;
const api = l2_execution.test_api;

const ZERO_HASH: [32]u8 = @splat(0);

/// The plan DSL's own default base fee. The chain-config hash formula takes it as a separate
/// argument (it is not itself a `ChainConfig` field), so computing the expected
/// `dynamic_chain_config_hash` needs this value directly.
const RANGE_BASE_FEE: u64 = 1_000_000_000;

// ─── A realistic 2-block range exercising every public-input field at once ─────────────────────
//
// Four real secp256k1-signed legacy (type-0) transactions for chain_id 59144, minted with
// rollup_spec/.venv/bin/python (coincurve): nonce 0-3, gasPrice=1e9, gas=21000, to=0xbb*20,
// value=1000/2000/3000/4000, data=b"", one distinct private key per tx
// (keccak256(b"l2exec-range-fixture/T<n>")). T1 and T2 ride in block 0, T3 in block 1; T4 never
// appears in a block, only as the second forced transaction's witness. Generating snippet (an
// EIP-155 legacy tx: r/s/recid from `PrivateKey(priv).sign_recoverable(keccak(unsigned_rlp),
// hasher=None)`, v = chainId*2+35+recid, sender = keccak(pubkey_uncompressed[1:])[-20:]):
//
//   from eth_utils import keccak
//   from coincurve import PrivateKey
//   priv = keccak(b"l2exec-range-fixture/T1")                   # one label per tx
//   fields = [nonce, 1_000_000_000, 21000, TO, value, b""]       # each RLP-encoded individually
//   unsigned = rlp_list(fields + [59144, b"", b""])              # EIP-155 signing preimage
//   sig = PrivateKey(priv).sign_recoverable(keccak(unsigned), hasher=None)  # r(32) s(32) recid(1)
//   signed = rlp_list(fields + [v, r, s])                        # v = 59144*2 + 35 + recid
const T1_RLP = [_]u8{
    0xf8, 0x68, 0x80, 0x84, 0x3b, 0x9a, 0xca, 0x00, 0x82, 0x52, 0x08, 0x94, 0xbb, 0xbb, 0xbb, 0xbb,
    0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
    0x82, 0x03, 0xe8, 0x80, 0x83, 0x01, 0xce, 0x33, 0xa0, 0x68, 0x1e, 0x7a, 0x01, 0xf9, 0x5d, 0x52,
    0x59, 0xdb, 0xb4, 0x5e, 0x5f, 0xdb, 0xa5, 0xe6, 0xa3, 0xc3, 0xc2, 0xe1, 0x0a, 0x55, 0xb7, 0x2f,
    0xb3, 0x09, 0x71, 0x5c, 0xaf, 0xca, 0x80, 0x81, 0x82, 0xa0, 0x05, 0x30, 0xf5, 0xe7, 0x87, 0x58,
    0xbd, 0x76, 0x4c, 0x9b, 0x88, 0x5b, 0x1a, 0x0e, 0x7c, 0x11, 0xeb, 0x99, 0x09, 0xf5, 0x4d, 0x4b,
    0xa5, 0x3a, 0x62, 0x83, 0xf4, 0xe9, 0xc0, 0xb2, 0x40, 0x06,
};
const T1_SENDER = [_]u8{
    0x84, 0x30, 0x35, 0xbd, 0xa9, 0x0a, 0x1b, 0xa3, 0x7b, 0x23, 0xa1, 0xfd, 0xbe, 0x62, 0xda, 0x52,
    0x4e, 0xf3, 0xe2, 0xa3,
};
const T2_RLP = [_]u8{
    0xf8, 0x68, 0x01, 0x84, 0x3b, 0x9a, 0xca, 0x00, 0x82, 0x52, 0x08, 0x94, 0xbb, 0xbb, 0xbb, 0xbb,
    0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
    0x82, 0x07, 0xd0, 0x80, 0x83, 0x01, 0xce, 0x34, 0xa0, 0x84, 0xb1, 0x4a, 0xe2, 0x25, 0xe0, 0x3c,
    0xbc, 0xb7, 0x5a, 0x9a, 0x69, 0x09, 0x65, 0xc1, 0x32, 0xc8, 0xd6, 0xf5, 0xfa, 0x19, 0xd7, 0xb5,
    0x2e, 0x97, 0x5b, 0xa4, 0x3d, 0xde, 0xda, 0x4f, 0x5f, 0xa0, 0x63, 0x81, 0xf7, 0xf3, 0x3c, 0xcc,
    0x44, 0x93, 0x56, 0x64, 0x44, 0x8b, 0x8a, 0x7d, 0xa0, 0xe4, 0x89, 0x04, 0x52, 0x78, 0x74, 0xb5,
    0xa3, 0xff, 0xe8, 0xaa, 0x36, 0x39, 0xab, 0xdb, 0xe9, 0xf7,
};
const T2_SENDER = [_]u8{
    0x58, 0x30, 0x5d, 0x39, 0xef, 0xe2, 0xb0, 0xb9, 0xde, 0x41, 0x26, 0x54, 0x9e, 0x6f, 0xd3, 0x73,
    0x2e, 0xe7, 0xce, 0xff,
};
const T3_RLP = [_]u8{
    0xf8, 0x68, 0x02, 0x84, 0x3b, 0x9a, 0xca, 0x00, 0x82, 0x52, 0x08, 0x94, 0xbb, 0xbb, 0xbb, 0xbb,
    0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
    0x82, 0x0b, 0xb8, 0x80, 0x83, 0x01, 0xce, 0x34, 0xa0, 0x78, 0x49, 0x97, 0x26, 0x8f, 0x3f, 0x1d,
    0x29, 0xbd, 0x75, 0x1b, 0x21, 0x4a, 0x96, 0x43, 0x96, 0x66, 0x20, 0x33, 0x9f, 0x69, 0x05, 0xd6,
    0x15, 0xb2, 0x3a, 0xbd, 0x3e, 0xfd, 0x10, 0xfd, 0x58, 0xa0, 0x61, 0xd2, 0xce, 0x00, 0x0a, 0x5f,
    0x22, 0xed, 0xcf, 0x1b, 0x22, 0x92, 0xf2, 0xb9, 0xd5, 0x92, 0x44, 0xa1, 0x84, 0x61, 0xe4, 0xdf,
    0x9f, 0xf7, 0xcc, 0x78, 0xf9, 0x1b, 0xd1, 0x77, 0x87, 0xbe,
};
const T3_SENDER = [_]u8{
    0x26, 0xcb, 0x46, 0x99, 0x5a, 0x42, 0x7f, 0xb9, 0x76, 0xa9, 0x6c, 0x58, 0x8b, 0x09, 0xc5, 0x87,
    0x01, 0xf1, 0x47, 0xff,
};
const T4_RLP = [_]u8{
    0xf8, 0x68, 0x03, 0x84, 0x3b, 0x9a, 0xca, 0x00, 0x82, 0x52, 0x08, 0x94, 0xbb, 0xbb, 0xbb, 0xbb,
    0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
    0x82, 0x0f, 0xa0, 0x80, 0x83, 0x01, 0xce, 0x34, 0xa0, 0x0a, 0x0e, 0xef, 0x62, 0x1d, 0xd9, 0xca,
    0x64, 0xbe, 0x53, 0x3f, 0x02, 0x04, 0xea, 0xca, 0xb6, 0x93, 0xb3, 0x85, 0x34, 0xb6, 0x2a, 0xa2,
    0xd2, 0xb9, 0x36, 0x70, 0xd1, 0x8d, 0xda, 0x6a, 0x84, 0xa0, 0x44, 0xd3, 0xaf, 0x85, 0x4d, 0xc0,
    0x79, 0xda, 0x86, 0x95, 0xa3, 0x03, 0x2a, 0x0b, 0xc9, 0x79, 0xab, 0x5a, 0xcf, 0xdf, 0x70, 0x40,
    0x07, 0x54, 0xbd, 0xc7, 0x85, 0xa9, 0x41, 0x58, 0xdf, 0x22,
};
const T4_SENDER = [_]u8{
    0x90, 0x9c, 0x96, 0x32, 0x2d, 0xbe, 0x7c, 0x94, 0xb7, 0xcf, 0x16, 0x80, 0x21, 0xb3, 0x2a, 0x9c,
    0x2b, 0x53, 0x63, 0x46,
};

/// L2MessageService's `MessageSent` event topic0, copied from the guest's own constant of the
/// same value (the log-matching logic it feeds is otherwise private to the guest).
const BRIDGE_MESSAGE_SENT_TOPIC0: [32]u8 = .{
    0xe8, 0x56, 0xc2, 0xb8, 0xbd, 0x4e, 0xb0, 0x02,
    0x7c, 0xe3, 0x2e, 0xea, 0xf5, 0x95, 0xc2, 0x1b,
    0x0b, 0x6b, 0x46, 0x44, 0xb3, 0x26, 0xe5, 0xb7,
    0xbd, 0x80, 0xa1, 0xcf, 0x8d, 0xb7, 0x2e, 0x6c,
};
const NON_BRIDGE_TOPIC0: [32]u8 = @splat(0x01);
const MESSAGE_HASH_1: [32]u8 = @splat(0x51);
const MESSAGE_HASH_2: [32]u8 = @splat(0x52);

/// The bridge address `bridgeStorage` switches on automatically, copied from the plan DSL's own
/// default of the same value — every MessageSent log below is emitted at this address.
const L2_MESSAGE_SERVICE_ADDRESS: [20]u8 = @splat(0xee);

const PARENT_BRIDGE_HASH: [32]u8 = @splat(0x11);
const END_BRIDGE_HASH: [32]u8 = @splat(0x22);

/// Comfortably above the plan DSL's own default range (block numbers ~1_000_000/1_000_001), so
/// both forced transactions below clear their deadline check regardless of which block handles
/// them.
const FTX_DEADLINE: u64 = 2_000_000;

/// end_ftx_rolling_hash, independently computed over the same two steps with the same python
/// venv — mirrors the guest's own keccak(prev || txHash || deadline_be32 || from) chain, starting
/// from zero32, chained through FTX1 (T1) then FTX2 (T4):
//
//   def step(prev, tx_hash, deadline, sender):
//       return keccak(prev + tx_hash + deadline.to_bytes(32, "big") + sender)
//   end = step(step(b"\x00" * 32, T1_hash, FTX_DEADLINE, T1_sender), T4_hash, FTX_DEADLINE, T4_sender)
const EXPECTED_END_FTX_ROLLING_HASH = [_]u8{
    0x08, 0x0d, 0x38, 0xc0, 0x6e, 0x38, 0xf3, 0xeb, 0xbf, 0x36, 0xdf, 0x77, 0xc4, 0x27, 0xdc, 0x83,
    0x94, 0x31, 0x51, 0x73, 0xe4, 0xbf, 0x79, 0x5a, 0x73, 0x10, 0x6a, 0x9e, 0xc0, 0x5b, 0x6d, 0xdf,
};

/// Mirrors the plan DSL's own base timestamp (1_700_000_000) plus one 12-second block time.
const EXPECTED_BLOCK1_TIMESTAMP: u64 = 1_700_000_012;

/// A log at a fixed placeholder position (this guest's message extraction only ever inspects
/// `address`/`topics`), duplicating `topics` onto the allocator like a real per-block execution's
/// own log construction would.
fn testLog(alloc: std.mem.Allocator, address: [20]u8, topics: []const [32]u8) !types.Log {
    return .{
        .address = address,
        .topics = try alloc.dupe([32]u8, topics),
        .data = &.{},
        .block_number = 0,
        .tx_hash = ZERO_HASH,
        .tx_index = 0,
        .block_hash = ZERO_HASH,
        .log_index = 0,
    };
}

test "a realistic 2-block range produces every public-input field exactly" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const msg_log_1 = try testLog(alloc, L2_MESSAGE_SERVICE_ADDRESS, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_1 });
    const non_matching_log = try testLog(alloc, L2_MESSAGE_SERVICE_ADDRESS, &.{ NON_BRIDGE_TOPIC0, ZERO_HASH, ZERO_HASH, ZERO_HASH });
    const msg_log_2 = try testLog(alloc, L2_MESSAGE_SERVICE_ADDRESS, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_2 });

    const ftx1 = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &T1_RLP,
        .acceptance = api.Acceptance.INCLUDED,
        .deadline = FTX_DEADLINE,
    };
    const ftx2 = l2_execution_ssz.ForcedTransactionWitness{
        .number = 2,
        .signed_tx_rlp = &T4_RLP,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };

    const block0_logs = [_][]const types.Log{ &.{msg_log_1}, &.{non_matching_log} };
    const block1_logs = [_][]const types.Log{&.{msg_log_2}};
    const blocks = [_]conflation_plan.BlockPlan{
        .{
            .signed_tx_rlps = &.{ &T1_RLP, &T2_RLP },
            .tx_logs = &block0_logs,
            .forced_transactions = &.{ftx1},
        },
        .{
            .signed_tx_rlps = &.{&T3_RLP},
            .tx_logs = &block1_logs,
            .forced_transactions = &.{ftx2},
        },
    };
    var plan = conflation_plan.ConflationPlan{ .blocks = &blocks, .parent_last_processed_ftx_number = 0 };
    plan.bridgeStorage(.parent, .{ .number = 5, .hash = PARENT_BRIDGE_HASH });
    plan.bridgeStorage(.end, .{ .number = 7, .hash = END_BRIDGE_HASH });

    const built = try plan.build(alloc);
    const output = try plan.run(alloc);

    // Header chain: the guest only ever parrots these back, so they're checked against the DSL's
    // own independently-derived header hashes rather than the guest's own pass-through logic.
    try testing.expectEqualSlices(u8, &built.parent_block_hash, &output.public_inputs.parent_block_hash);
    try testing.expectEqualSlices(u8, &built.end_block_hash, &output.public_inputs.end_block_hash);
    try testing.expectEqual(plan.start_block_number, output.start_block_number);
    try testing.expectEqual(plan.start_block_number + 1, output.public_inputs.end_block_number);
    try testing.expectEqual(EXPECTED_BLOCK1_TIMESTAMP, output.public_inputs.end_block_timestamp);

    // L2->L1 messages, in block order; T2's non-matching-topic0 log is collected by neither.
    const expected_messages = [_][32]u8{ MESSAGE_HASH_1, MESSAGE_HASH_2 };
    const expected_messages_hash = try api.hashDigestListFn(alloc, &expected_messages);
    try testing.expectEqualSlices(u8, &expected_messages_hash, &output.public_inputs.l2_l1_messages_hash);
    try testing.expectEqual(@as(usize, 2), output.l2_l1_messages.len);
    try testing.expectEqualSlices(u8, &MESSAGE_HASH_1, &output.l2_l1_messages[0]);
    try testing.expectEqualSlices(u8, &MESSAGE_HASH_2, &output.l2_l1_messages[1]);

    // L1<->L2 bridge: parent/end numbers and hashes echo exactly what bridgeStorage declared.
    try testing.expectEqual(@as(u64, 5), output.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &PARENT_BRIDGE_HASH, &output.public_inputs.parent_l1_l2_bridge_rolling_hash);
    try testing.expectEqual(@as(u64, 7), output.public_inputs.end_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &END_BRIDGE_HASH, &output.public_inputs.end_l1_l2_bridge_rolling_hash);

    // Chain config hash over the range's real address/coinbase/chainId, at the range's base fee.
    const chain_config = l2_execution_ssz.ChainConfig{
        .l2_message_service_address = plan.l2_message_service_address,
        .coinbase = plan.coinbase,
        .chain_id = plan.chain_id,
    };
    const expected_chain_config_hash = api.chainConfigHashFn(chain_config, RANGE_BASE_FEE);
    try testing.expectEqualSlices(u8, &expected_chain_config_hash, &output.public_inputs.dynamic_chain_config_hash);

    // Forced transactions: FTX1 (INCLUDED, tx=T1) and FTX2 (FILTERED_ADDRESS_FROM, tx=T4) both
    // update the rolling hash across the range; only FTX2 bubbles up a filtered address.
    try testing.expectEqualSlices(u8, &plan.parent_ftx_rolling_hash, &output.public_inputs.parent_ftx_rolling_hash);
    try testing.expectEqual(plan.parent_last_processed_ftx_number, output.public_inputs.parent_processed_ftx_number);
    try testing.expectEqual(@as(u64, 2), output.public_inputs.end_processed_ftx_number);
    try testing.expectEqualSlices(u8, &EXPECTED_END_FTX_ROLLING_HASH, &output.public_inputs.end_ftx_rolling_hash);

    const expected_filtered_hash = try api.hashAddressListFn(alloc, &.{T4_SENDER});
    try testing.expectEqualSlices(u8, &expected_filtered_hash, &output.public_inputs.filtered_addresses_hash);
    try testing.expectEqual(@as(usize, 1), output.filtered_addresses.len);
    try testing.expectEqualSlices(u8, &T4_SENDER, &output.filtered_addresses[0]);

    // tx_froms, in block-then-transaction order: T1's sender, T2's sender, then T3's sender.
    const expected_tx_froms = [_][20]u8{ T1_SENDER, T2_SENDER, T3_SENDER };
    const expected_tx_froms_hash = try api.hashAddressListFn(alloc, &expected_tx_froms);
    try testing.expectEqualSlices(u8, &expected_tx_froms_hash, &output.public_inputs.tx_froms_hash);
    try testing.expectEqual(@as(usize, 3), output.tx_froms.len);
    try testing.expectEqualSlices(u8, &T1_SENDER, &output.tx_froms[0]);
    try testing.expectEqualSlices(u8, &T2_SENDER, &output.tx_froms[1]);
    try testing.expectEqualSlices(u8, &T3_SENDER, &output.tx_froms[2]);
}

// ─── One mutation away from a realistic default range ──────────────────────────────────────────
//
// Each scenario takes the plan DSL's own 2-block default range and drifts exactly one field or
// hook away from it, catching the guest's conflation-level checks the same way a real range
// would: a violation introduced mid-range, not just at the first block.

const JUNK_HASH: [32]u8 = @splat(0xfe);
const NON_COINBASE_ADDRESS: [20]u8 = @splat(0xfa);

test "an empty payload list is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const plan = conflation_plan.ConflationPlan{ .blocks = &.{} };
    try plan.expectReject(arena.allocator(), error.EmptyPayloads);
}

test "a corrupted stateless input is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const plan = conflation_plan.ConflationPlan{ .corrupt_stateless_input_at = 1 };
    try plan.expectReject(arena.allocator(), error.InvalidStatelessInput);
}

test "a mismatched inner chain id is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{ .{}, .{ .chain_id = 1 } };
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };
    try plan.expectReject(arena.allocator(), error.ChainIdMismatch);
}

test "a broken parent-hash chain is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const plan = conflation_plan.ConflationPlan{
        .override_parent_hash_at = .{ .index = 1, .parent_hash = JUNK_HASH },
    };
    try plan.expectReject(arena.allocator(), error.ParentHashChainMismatch);
}

test "a non-constant base fee is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{ .{}, .{ .base_fee = RANGE_BASE_FEE + 1 } };
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };
    try plan.expectReject(arena.allocator(), error.BaseFeeNotConstant);
}

test "a fee recipient other than the range's coinbase is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{ .{}, .{ .fee_recipient = NON_COINBASE_ADDRESS } };
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };
    try plan.expectReject(arena.allocator(), error.FeeRecipientMismatch);
}

test "a missing parent header witness is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const plan = conflation_plan.ConflationPlan{ .drop_witness_headers_at = 1 };
    try plan.expectReject(arena.allocator(), error.MissingParentHeaderWitness);
}

test "a genesis range with a zero parent hash is accepted" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{.{}};
    const plan = conflation_plan.ConflationPlan{ .start_block_number = 0, .blocks = &blocks };
    const output = try plan.run(arena.allocator());
    try testing.expectEqual(@as(u64, 0), output.public_inputs.end_block_number);
}

test "a genesis range with a nonzero parent hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{.{}};
    const plan = conflation_plan.ConflationPlan{
        .start_block_number = 0,
        .blocks = &blocks,
        .override_genesis_parent_hash = JUNK_HASH,
    };
    try plan.expectReject(arena.allocator(), error.InvalidGenesisParentHash);
}

test "non-empty execution requests are rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{ .{}, .{ .non_empty_execution_requests = true } };
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };
    try plan.expectReject(arena.allocator(), error.ExecutionRequestsNotSupported);
}

test "non-empty withdrawals are rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{ .{}, .{ .non_empty_withdrawals = true } };
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };
    try plan.expectReject(arena.allocator(), error.WithdrawalsNotSupported);
}

test "an unsupported fork is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const blocks = [_]conflation_plan.BlockPlan{ .{}, .{ .active_fork_idx = 0x11 } };
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };
    try plan.expectReject(arena.allocator(), error.UnsupportedFork);
}
