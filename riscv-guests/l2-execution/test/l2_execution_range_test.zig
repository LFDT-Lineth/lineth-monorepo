//! Scenario tests for the l2-execution guest's conflation logic, built on the plan DSL.
//!
//! One rich happy-path scenario exercises every public-input field at once over a realistic
//! 2-block range: real signed transactions, L2->L1 bridge messages, L1<->L2 bridge storage, and
//! forced transactions spanning both blocks — checked against independently-derived expected
//! values. Fifteen one-mutation scenarios each drift a single field or hook away from a realistic
//! default range, one per rejection (or, for a handful, one per accepted/observed edge case) the
//! guest's conflation logic enforces.
//!
//! A further set of scenarios exercises forced transactions that interact with the
//! L2MessageService bridge contract directly — the escape hatch's single most important use case,
//! a censored user forcing a bridge withdrawal through it — and the cross-range continuity of the
//! forced-transaction rolling hash carried by `parent_ftx_rolling_hash`/
//! `parent_last_processed_ftx_number`.

const std = @import("std");
const testing = std.testing;

const executor = @import("zesu_executor");
const mpt = @import("zesu_mpt");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const conflation_plan = @import("conflation_plan.zig");
const tx_fixtures = @import("tx_fixtures");

const types = executor.executor_types;
const api = l2_execution.test_api;

const ZERO_HASH: [32]u8 = @splat(0);

/// The plan DSL's own default base fee. The chain-config hash formula takes it as a separate
/// argument (it is not itself a `ChainConfig` field), so computing the expected
/// `dynamic_chain_config_hash` needs this value directly.
const RANGE_BASE_FEE: u64 = 1_000_000_000;

// ─── A realistic 2-block range exercising every public-input field at once ─────────────────────
//
// Four secp256k1-signed legacy (type-0) transactions for chain_id 59144, built and signed live in
// this file via zesu's own real secp256k1 backend (`buildSignedFixtureTx`, below) — the same
// signing primitive the guest's own sender-recovery (`recoverFixtureSender`) already trusts for
// verification. Shared fields: gasPrice=1e9, gas=21000, to=0xbb*20, data=b""; each tx has its own
// nonce (0-3), value (1000/2000/3000/4000), and private key
// (keccak256("l2exec-range-fixture/T<n>"), one label per tx). T1 and T2 ride in block 0, T3 in
// block 1; T4 never appears in a block, only as the second forced transaction's witness.
//
// Senders are NOT hand-frozen: `recoverFixtureSender` derives each one from the tx bytes below,
// so it can never drift out of sync with them. The scenario test's own
// `expected_end_ftx_rolling_hash` likewise derives from these transactions' own bytes rather than
// a pinned literal, so it too can never drift out of sync with them.

/// The plan DSL's own default chain_id (`ConflationPlan.chain_id`'s default), needed directly
/// here since sender recovery (EIP-155: recid comes from `v - chain_id*2 - 35`) happens before
/// the plan is built.
const RANGE_CHAIN_ID: u64 = 59144;

/// Fields shared by all four range fixtures (T1-T4); only `nonce`/`value` vary per tx (see each
/// `buildSignedFixtureTx` call site in the scenario test below).
const RANGE_TX_GAS_PRICE: u128 = 1_000_000_000;
const RANGE_TX_GAS: u64 = 21_000;
const RANGE_TX_TO: [20]u8 = @splat(0xbb);

/// Thin wrapper baking this file's own RANGE_* constants onto the shared signed-legacy-tx
/// builder — the same label/nonce/value triple derives the same private key and signs the same
/// transaction bytes as before, so T1-T4's senders and signatures are unchanged.
fn buildSignedFixtureTx(alloc: std.mem.Allocator, comptime label: []const u8, nonce: u64, value: u256) ![]const u8 {
    return tx_fixtures.buildSignedLegacyTx(alloc, label, .{
        .nonce = nonce,
        .gas_price = RANGE_TX_GAS_PRICE,
        .gas = RANGE_TX_GAS,
        .to = RANGE_TX_TO,
        .value = value,
        .chain_id = RANGE_CHAIN_ID,
    });
}

fn recoverFixtureSender(alloc: std.mem.Allocator, signed_tx_rlp: []const u8, chain_id: u64) ![20]u8 {
    const decoded = try executor.executor_tx_decode.decodeTxs(alloc, &.{signed_tx_rlp});
    var tx = decoded[0];
    return (try api.recoverSenderFn(alloc, &tx, chain_id)).?;
}

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

const PARENT_BRIDGE_HASH: [32]u8 = @splat(0x11);
const END_BRIDGE_HASH: [32]u8 = @splat(0x22);

/// Comfortably above the plan DSL's own default range (block numbers ~1_000_000/1_000_001), so
/// both forced transactions below clear their deadline check regardless of which block handles
/// them.
const FTX_DEADLINE: u64 = 2_000_000;

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

    // T1: nonce=0, value=1000, sender rides in block 0 and doubles as FTX1's INCLUDED witness.
    const t1_rlp = try buildSignedFixtureTx(alloc, "T1", 0, 1000);
    // T2: nonce=1, value=2000, sender rides in block 0 alongside T1.
    const t2_rlp = try buildSignedFixtureTx(alloc, "T2", 1, 2000);
    // T3: nonce=2, value=3000, sender rides in block 1.
    const t3_rlp = try buildSignedFixtureTx(alloc, "T3", 2, 3000);
    // T4: nonce=3, value=4000, sender never rides in a block — only bubbles up as FTX2's
    // FILTERED_ADDRESS_FROM witness.
    const t4_rlp = try buildSignedFixtureTx(alloc, "T4", 3, 4000);

    const msg_log_1 = try testLog(alloc, conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_1 });
    const non_matching_log = try testLog(alloc, conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS, &.{ NON_BRIDGE_TOPIC0, ZERO_HASH, ZERO_HASH, ZERO_HASH });
    const msg_log_2 = try testLog(alloc, conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_2 });

    // Senders are derived from the tx bytes themselves, not hand-copied literals — see
    // `recoverFixtureSender`'s doc comment above.
    const t1_sender = try recoverFixtureSender(alloc, t1_rlp, RANGE_CHAIN_ID);
    const t2_sender = try recoverFixtureSender(alloc, t2_rlp, RANGE_CHAIN_ID);
    const t3_sender = try recoverFixtureSender(alloc, t3_rlp, RANGE_CHAIN_ID);
    const t4_sender = try recoverFixtureSender(alloc, t4_rlp, RANGE_CHAIN_ID);

    // end_ftx_rolling_hash, computed independently from FTX1/FTX2's own derived (tx_hash, sender)
    // via the SAME already-trusted rolling-hash primitive the guest's own FTX loop uses
    // internally — this composition checks that the scenario's OWN FTX loop applies that
    // primitive to the right data in the right order, starting from zero32 and chained through
    // FTX1 (T1) then FTX2 (T4), exactly mirroring runL2Execution's FTX loop.
    const t1_tx_hash = mpt.keccak256(t1_rlp);
    const t4_tx_hash = mpt.keccak256(t4_rlp);
    const ftx_rolling_hash_after_ftx1 = api.addToForcedTxRollingHashFn(ZERO_HASH, t1_tx_hash, FTX_DEADLINE, t1_sender);
    const expected_end_ftx_rolling_hash = api.addToForcedTxRollingHashFn(ftx_rolling_hash_after_ftx1, t4_tx_hash, FTX_DEADLINE, t4_sender);

    const ftx1 = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = t1_rlp,
        .acceptance = api.Acceptance.INCLUDED,
        .deadline = FTX_DEADLINE,
    };
    const ftx2 = l2_execution_ssz.ForcedTransactionWitness{
        .number = 2,
        .signed_tx_rlp = t4_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };

    const block0_logs = [_][]const types.Log{ &.{msg_log_1}, &.{non_matching_log} };
    const block1_logs = [_][]const types.Log{&.{msg_log_2}};
    const blocks = [_]conflation_plan.BlockPlan{
        .{
            .signed_tx_rlps = &.{ t1_rlp, t2_rlp },
            .tx_logs = &block0_logs,
            .forced_transactions = &.{ftx1},
        },
        .{
            .signed_tx_rlps = &.{t3_rlp},
            .tx_logs = &block1_logs,
            .forced_transactions = &.{ftx2},
        },
    };
    var plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .parent_last_processed_ftx_number = 0,
        .l2_message_service_address = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
    };
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
    try testing.expectEqualSlices(u8, &expected_end_ftx_rolling_hash, &output.public_inputs.end_ftx_rolling_hash);

    const expected_filtered_hash = try api.hashAddressListFn(alloc, &.{t4_sender});
    try testing.expectEqualSlices(u8, &expected_filtered_hash, &output.public_inputs.filtered_addresses_hash);
    try testing.expectEqual(@as(usize, 1), output.filtered_addresses.len);
    try testing.expectEqualSlices(u8, &t4_sender, &output.filtered_addresses[0]);

    // tx_froms, in block-then-transaction order: T1's sender, T2's sender, then T3's sender.
    const expected_tx_froms = [_][20]u8{ t1_sender, t2_sender, t3_sender };
    const expected_tx_froms_hash = try api.hashAddressListFn(alloc, &expected_tx_froms);
    try testing.expectEqualSlices(u8, &expected_tx_froms_hash, &output.public_inputs.tx_froms_hash);
    try testing.expectEqual(@as(usize, 3), output.tx_froms.len);
    try testing.expectEqualSlices(u8, &t1_sender, &output.tx_froms[0]);
    try testing.expectEqualSlices(u8, &t2_sender, &output.tx_froms[1]);
    try testing.expectEqualSlices(u8, &t3_sender, &output.tx_froms[2]);
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

// Proves `bridgeStorage`'s decoupling: declaring bridge storage no longer forces the address on,
// so a plan can leave it at its suppressed zero default and the declared values are simply never
// realized.
test "bridge storage declared while the address stays at its suppressed zero default is never observed" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    var plan = conflation_plan.ConflationPlan{};
    plan.bridgeStorage(.parent, .{ .number = 5, .hash = PARENT_BRIDGE_HASH });
    plan.bridgeStorage(.end, .{ .number = 7, .hash = END_BRIDGE_HASH });

    const output = try plan.run(arena.allocator());

    try testing.expectEqual(@as(u64, 0), output.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &ZERO_HASH, &output.public_inputs.parent_l1_l2_bridge_rolling_hash);
    try testing.expectEqual(@as(u64, 0), output.public_inputs.end_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &ZERO_HASH, &output.public_inputs.end_l1_l2_bridge_rolling_hash);
}

test "a decreasing bridge message number across the range is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    var plan = conflation_plan.ConflationPlan{ .l2_message_service_address = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS };
    plan.bridgeStorage(.parent, .{ .number = 7, .hash = PARENT_BRIDGE_HASH });
    plan.bridgeStorage(.end, .{ .number = 5, .hash = END_BRIDGE_HASH });

    try plan.expectReject(arena.allocator(), error.RollingHashNumberDecreased);
}

test "a matching bridge log is ignored while the address stays at its suppressed zero default" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A realistic non-zero contract address for the log's own emitting address; the field that
    // actually stays suppressed at its zero default below is l2_message_service_address.
    const placeholder_log_address: [20]u8 = @splat(0xdd);
    // StubEngine attaches declared logs per DECODED transaction, so a real tx must ride in the
    // block for this log to have anything to attach to.
    const t_rlp = try buildSignedFixtureTx(alloc, "T12", 0, 1000);
    const bridge_log = try testLog(alloc, placeholder_log_address, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_1 });

    const block0_logs = [_][]const types.Log{&.{bridge_log}};
    const blocks = [_]conflation_plan.BlockPlan{
        .{ .signed_tx_rlps = &.{t_rlp}, .tx_logs = &block0_logs },
        .{},
    };
    // l2_message_service_address is left at the DSL's own suppressed zero default.
    const plan = conflation_plan.ConflationPlan{ .blocks = &blocks };

    const output = try plan.run(alloc);

    try testing.expectEqual(@as(usize, 0), output.l2_l1_messages.len);
    const expected_empty_hash = try api.hashDigestListFn(alloc, &.{});
    try testing.expectEqualSlices(u8, &expected_empty_hash, &output.public_inputs.l2_l1_messages_hash);
    try testing.expectEqual(@as(u64, 0), output.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &ZERO_HASH, &output.public_inputs.parent_l1_l2_bridge_rolling_hash);
    try testing.expectEqual(@as(u64, 0), output.public_inputs.end_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &ZERO_HASH, &output.public_inputs.end_l1_l2_bridge_rolling_hash);
}

test "an included forced transaction's bridge message reaches the public input" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // An ordinary transaction that also bridges a message, riding in the same block as the forced
    // transaction — realistic, since a block carrying a forced transaction is otherwise a
    // completely ordinary block that can carry ordinary bridge traffic too.
    const ordinary_bridge_tx_rlp = try buildSignedFixtureTx(alloc, "OrdinaryBridgeTx", 0, 1000);
    // The forced transaction itself, addressed to the bridge contract like a real forced withdrawal
    // would be, riding in the block it is declared INCLUDED for.
    const forced_bridge_tx_rlp = try tx_fixtures.buildSignedLegacyTx(alloc, "IncludedForcedBridgeTx", .{
        .nonce = 0,
        .gas_price = RANGE_TX_GAS_PRICE,
        .gas = RANGE_TX_GAS,
        .to = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
        .value = 0,
        .chain_id = RANGE_CHAIN_ID,
    });

    const ordinary_bridge_tx_sender = try recoverFixtureSender(alloc, ordinary_bridge_tx_rlp, RANGE_CHAIN_ID);
    const forced_bridge_tx_sender = try recoverFixtureSender(alloc, forced_bridge_tx_rlp, RANGE_CHAIN_ID);

    const ordinary_bridge_tx_message_log = try testLog(alloc, conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_1 });
    const forced_bridge_tx_message_log = try testLog(alloc, conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS, &.{ BRIDGE_MESSAGE_SENT_TOPIC0, ZERO_HASH, ZERO_HASH, MESSAGE_HASH_2 });
    const block0_logs = [_][]const types.Log{ &.{ordinary_bridge_tx_message_log}, &.{forced_bridge_tx_message_log} };

    const included_forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = forced_bridge_tx_rlp,
        .acceptance = api.Acceptance.INCLUDED,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{
            .signed_tx_rlps = &.{ ordinary_bridge_tx_rlp, forced_bridge_tx_rlp },
            .tx_logs = &block0_logs,
            .forced_transactions = &.{included_forced_transaction},
        },
        .{},
    };
    const plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .l2_message_service_address = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
    };

    const output = try plan.run(alloc);

    const expected_messages = [_][32]u8{ MESSAGE_HASH_1, MESSAGE_HASH_2 };
    const expected_messages_hash = try api.hashDigestListFn(alloc, &expected_messages);
    try testing.expectEqual(@as(usize, 2), output.l2_l1_messages.len);
    try testing.expectEqualSlices(u8, &MESSAGE_HASH_1, &output.l2_l1_messages[0]);
    try testing.expectEqualSlices(u8, &MESSAGE_HASH_2, &output.l2_l1_messages[1]);
    try testing.expectEqualSlices(u8, &expected_messages_hash, &output.public_inputs.l2_l1_messages_hash);

    try testing.expectEqual(@as(u64, 1), output.public_inputs.end_processed_ftx_number);
    const expected_end_ftx_rolling_hash = api.addToForcedTxRollingHashFn(ZERO_HASH, mpt.keccak256(forced_bridge_tx_rlp), FTX_DEADLINE, forced_bridge_tx_sender);
    try testing.expectEqualSlices(u8, &expected_end_ftx_rolling_hash, &output.public_inputs.end_ftx_rolling_hash);

    // Both senders reach tx_froms in block order: the forced transaction really executed in this
    // block, so it feeds L1 data-availability sender reconstruction exactly like the ordinary
    // transaction beside it does.
    try testing.expectEqual(@as(usize, 2), output.tx_froms.len);
    try testing.expectEqualSlices(u8, &ordinary_bridge_tx_sender, &output.tx_froms[0]);
    try testing.expectEqualSlices(u8, &forced_bridge_tx_sender, &output.tx_froms[1]);

    try testing.expectEqual(@as(usize, 0), output.filtered_addresses.len);
}

test "a forced transaction failing pre-validation produces no bridge message and no sender entry" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // An ordinary transaction riding in the block; its sender is the only one that should ever
    // reach tx_froms here.
    const ordinary_tx_rlp = try buildSignedFixtureTx(alloc, "OrdinaryTxBesideInvalidForcedTx", 0, 1000);
    const ordinary_tx_sender = try recoverFixtureSender(alloc, ordinary_tx_rlp, RANGE_CHAIN_ID);

    // A forced transaction addressed to the bridge contract, declared BAD_NONCE — the sequencer
    // declined to include it because the sender's on-chain nonce had already moved past it, exactly
    // the kind of stale forced withdrawal L1 must be told bridged no message.
    const invalid_forced_bridge_tx_rlp = try tx_fixtures.buildSignedLegacyTx(alloc, "BadNonceForcedBridgeTx", .{
        .nonce = 5,
        .gas_price = RANGE_TX_GAS_PRICE,
        .gas = RANGE_TX_GAS,
        .to = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
        .value = 0,
        .chain_id = RANGE_CHAIN_ID,
    });
    const invalid_forced_bridge_tx_sender = try recoverFixtureSender(alloc, invalid_forced_bridge_tx_rlp, RANGE_CHAIN_ID);

    const invalid_forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = invalid_forced_bridge_tx_rlp,
        .acceptance = api.Acceptance.BAD_NONCE,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{
            .signed_tx_rlps = &.{ordinary_tx_rlp},
            .forced_transactions = &.{invalid_forced_transaction},
        },
        .{},
    };
    // The refused sender needs a real account in the world state to read a nonce from at all; an
    // account nonce of 0 against the transaction's own nonce of 5 is what makes BAD_NONCE a truthful
    // declaration rather than an arbitrary one.
    const plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .l2_message_service_address = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
        .accounts = &.{.{ .address = invalid_forced_bridge_tx_sender, .nonce = 0, .balance = 0 }},
    };

    const output = try plan.run(alloc);

    try testing.expectEqual(@as(usize, 0), output.l2_l1_messages.len);
    const expected_empty_hash = try api.hashDigestListFn(alloc, &.{});
    try testing.expectEqualSlices(u8, &expected_empty_hash, &output.public_inputs.l2_l1_messages_hash);

    // tx_froms feeds L1's own data-availability sender reconstruction — a forced transaction that
    // failed pre-validation and never executed must never contribute a sender there, or L1 would
    // reconstruct a sender for a transaction that was never actually applied.
    try testing.expectEqual(@as(usize, 1), output.tx_froms.len);
    try testing.expectEqualSlices(u8, &ordinary_tx_sender, &output.tx_froms[0]);

    try testing.expectEqual(@as(u64, 1), output.public_inputs.end_processed_ftx_number);
    const expected_end_ftx_rolling_hash = api.addToForcedTxRollingHashFn(ZERO_HASH, mpt.keccak256(invalid_forced_bridge_tx_rlp), FTX_DEADLINE, invalid_forced_bridge_tx_sender);
    try testing.expectEqualSlices(u8, &expected_end_ftx_rolling_hash, &output.public_inputs.end_ftx_rolling_hash);
}

test "a range refusing one forced transaction by sender and another by recipient lists both kinds in order" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // Refused for its sender: the address L1 checks against its sanction list is the one recovered
    // from this transaction's signature.
    const refused_sender_tx_rlp = try buildSignedFixtureTx(alloc, "RefusedBySenderForcedTx", 0, 1000);
    const refused_sender = try recoverFixtureSender(alloc, refused_sender_tx_rlp, RANGE_CHAIN_ID);

    // Refused for its recipient, which here is this range's own bridge contract — a forced bridge
    // withdrawal the sequencer declines on compliance grounds. The address L1 checks for this one is
    // the recipient decoded from the signed bytes.
    const refused_recipient_tx_rlp = try tx_fixtures.buildSignedLegacyTx(alloc, "RefusedByRecipientForcedBridgeTx", .{
        .nonce = 0,
        .gas_price = RANGE_TX_GAS_PRICE,
        .gas = RANGE_TX_GAS,
        .to = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
        .value = 0,
        .chain_id = RANGE_CHAIN_ID,
    });
    const refused_recipient_tx_sender = try recoverFixtureSender(alloc, refused_recipient_tx_rlp, RANGE_CHAIN_ID);

    const refused_by_sender = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = refused_sender_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };
    const refused_by_recipient = l2_execution_ssz.ForcedTransactionWitness{
        .number = 2,
        .signed_tx_rlp = refused_recipient_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_TO,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{ .forced_transactions = &.{refused_by_sender} },
        .{ .forced_transactions = &.{refused_by_recipient} },
    };
    const plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .l2_message_service_address = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
    };

    const output = try plan.run(alloc);

    // One list, two different address kinds, in the block order their refusals happened. L1 hashes
    // this list and checks every entry against its sanction list, so picking the wrong field for
    // either refusal hands L1 an address it was never meant to check: a refusal aimed at a
    // sanctioned recipient would be validated against an unsanctioned sender, and pass.
    const expected_filtered_addresses = [_][20]u8{ refused_sender, conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS };
    const expected_filtered_addresses_hash = try api.hashAddressListFn(alloc, &expected_filtered_addresses);
    try testing.expectEqual(@as(usize, 2), output.filtered_addresses.len);
    try testing.expectEqualSlices(u8, &refused_sender, &output.filtered_addresses[0]);
    try testing.expectEqualSlices(u8, &conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS, &output.filtered_addresses[1]);
    try testing.expectEqualSlices(u8, &expected_filtered_addresses_hash, &output.public_inputs.filtered_addresses_hash);

    // The two kinds land on genuinely different fields: entry 0 differs from its transaction's
    // recipient, entry 1 differs from its transaction's sender. Swapping the two fields inside the
    // guest would therefore move both entries, which keeps the assertions above honest.
    try testing.expect(!std.mem.eql(u8, &output.filtered_addresses[0], &RANGE_TX_TO));
    try testing.expect(!std.mem.eql(u8, &output.filtered_addresses[1], &refused_recipient_tx_sender));
}

test "a forced transaction included in the last block advances the L1-to-L2 anchoring state" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A bridge-bound forced transaction riding in the range's LAST block, whose post-state IS the
    // range's end state — so the anchoring advance this range commits and the forced transaction's
    // own handling land at the same state boundary, the pairing a range carrying both has to get
    // right. Which of the two the anchoring advance is attributable to sits on the far side of the
    // execution seam a plan stubs out, so this scenario pins that pairing rather than a cause.
    const forced_bridge_tx_rlp = try tx_fixtures.buildSignedLegacyTx(alloc, "AnchoringRangeForcedBridgeTx", .{
        .nonce = 0,
        .gas_price = RANGE_TX_GAS_PRICE,
        .gas = RANGE_TX_GAS,
        .to = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
        .value = 0,
        .chain_id = RANGE_CHAIN_ID,
    });
    const forced_bridge_tx_sender = try recoverFixtureSender(alloc, forced_bridge_tx_rlp, RANGE_CHAIN_ID);
    const forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = forced_bridge_tx_rlp,
        .acceptance = api.Acceptance.INCLUDED,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{},
        .{ .signed_tx_rlps = &.{forced_bridge_tx_rlp}, .forced_transactions = &.{forced_transaction} },
    };
    var plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .l2_message_service_address = conflation_plan.DEFAULT_L2_MESSAGE_SERVICE_ADDRESS,
    };
    plan.bridgeStorage(.parent, .{ .number = 5, .hash = PARENT_BRIDGE_HASH });
    plan.bridgeStorage(.end, .{ .number = 7, .hash = END_BRIDGE_HASH });

    const output = try plan.run(alloc);

    try testing.expectEqual(@as(u64, 5), output.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &PARENT_BRIDGE_HASH, &output.public_inputs.parent_l1_l2_bridge_rolling_hash);
    try testing.expectEqual(@as(u64, 7), output.public_inputs.end_l1_l2_bridge_rolling_hash_message_number);
    try testing.expectEqualSlices(u8, &END_BRIDGE_HASH, &output.public_inputs.end_l1_l2_bridge_rolling_hash);

    try testing.expectEqual(@as(u64, 1), output.public_inputs.end_processed_ftx_number);
    const expected_end_ftx_rolling_hash = api.addToForcedTxRollingHashFn(ZERO_HASH, mpt.keccak256(forced_bridge_tx_rlp), FTX_DEADLINE, forced_bridge_tx_sender);
    try testing.expectEqualSlices(u8, &expected_end_ftx_rolling_hash, &output.public_inputs.end_ftx_rolling_hash);
}

// ─── Cross-range FTX continuity ─────────────────────────────────────────────────────────────────
//
// A range is rarely the very first one a chain ever proves: `parent_ftx_rolling_hash` and
// `parent_last_processed_ftx_number` carry the L1-checked FTX state a prior range already
// committed, so this range's own FTX loop must seed itself from them rather than from zero — the
// same way `parent_block_hash` seeds the header chain instead of assuming genesis. These scenarios
// pin the chaining itself, the out-of-order rejections at the exact parent boundary, and the
// pass-through case where a range with no forced transactions at all still carries that state
// forward unchanged.

/// A realistic non-zero parent FTX rolling hash, standing in for whatever a prior range actually
/// committed — so these scenarios assert real chaining from a non-trivial seed instead of one that
/// happens to start at zero regardless of whether seeding from the parent value works at all.
const PARENT_FTX_ROLLING_HASH: [32]u8 = @splat(0x7a);
/// The parent range's last processed FTX number these scenarios continue from.
const PARENT_FTX_NUMBER: u64 = 41;

test "a continuation range chains the FTX rolling hash from its parent value" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // FILTERED_ADDRESS_FROM keeps this scenario free of state reads (unlike BAD_NONCE/BAD_BALANCE)
    // while still exercising the same rolling-hash update every acceptance outcome shares — the
    // chaining this test targets happens before the guest ever looks at the acceptance value.
    const first_forced_tx_rlp = try buildSignedFixtureTx(alloc, "ContinuationRangeFirstForcedTx", 0, 1000);
    const second_forced_tx_rlp = try buildSignedFixtureTx(alloc, "ContinuationRangeSecondForcedTx", 0, 1000);
    const first_forced_tx_sender = try recoverFixtureSender(alloc, first_forced_tx_rlp, RANGE_CHAIN_ID);
    const second_forced_tx_sender = try recoverFixtureSender(alloc, second_forced_tx_rlp, RANGE_CHAIN_ID);

    // Numbers derived from the parent boundary rather than written as literals, so the sequence
    // stays contiguous with whatever `PARENT_FTX_NUMBER` declares.
    const first_forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = PARENT_FTX_NUMBER + 1,
        .signed_tx_rlp = first_forced_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };
    const second_forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = PARENT_FTX_NUMBER + 2,
        .signed_tx_rlp = second_forced_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{ .forced_transactions = &.{first_forced_transaction} },
        .{ .forced_transactions = &.{second_forced_transaction} },
    };
    const plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .parent_ftx_rolling_hash = PARENT_FTX_ROLLING_HASH,
        .parent_last_processed_ftx_number = PARENT_FTX_NUMBER,
    };

    const output = try plan.run(alloc);

    const rolling_hash_after_first = api.addToForcedTxRollingHashFn(PARENT_FTX_ROLLING_HASH, mpt.keccak256(first_forced_tx_rlp), FTX_DEADLINE, first_forced_tx_sender);
    const expected_end_ftx_rolling_hash = api.addToForcedTxRollingHashFn(rolling_hash_after_first, mpt.keccak256(second_forced_tx_rlp), FTX_DEADLINE, second_forced_tx_sender);
    try testing.expectEqualSlices(u8, &expected_end_ftx_rolling_hash, &output.public_inputs.end_ftx_rolling_hash);
    try testing.expectEqual(PARENT_FTX_NUMBER + 2, output.public_inputs.end_processed_ftx_number);

    try testing.expectEqualSlices(u8, &PARENT_FTX_ROLLING_HASH, &output.public_inputs.parent_ftx_rolling_hash);
    try testing.expectEqual(PARENT_FTX_NUMBER, output.public_inputs.parent_processed_ftx_number);
}

test "a range replaying the parent's last FTX number is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const replayed_forced_tx_rlp = try buildSignedFixtureTx(alloc, "ReplayedForcedTx", 0, 1000);
    const replayed_forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = PARENT_FTX_NUMBER,
        .signed_tx_rlp = replayed_forced_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{ .forced_transactions = &.{replayed_forced_transaction} },
        .{},
    };
    const plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .parent_ftx_rolling_hash = PARENT_FTX_ROLLING_HASH,
        .parent_last_processed_ftx_number = PARENT_FTX_NUMBER,
    };

    try plan.expectReject(alloc, error.ForcedTxOutOfOrder);
}

test "a range skipping an FTX number at the parent boundary is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // Declares the number one past the one this range owes, leaving the gap the parent boundary
    // requires to be filled first.
    const gapped_forced_tx_rlp = try buildSignedFixtureTx(alloc, "GappedForcedTx", 0, 1000);
    const gapped_forced_transaction = l2_execution_ssz.ForcedTransactionWitness{
        .number = PARENT_FTX_NUMBER + 2,
        .signed_tx_rlp = gapped_forced_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = FTX_DEADLINE,
    };
    const blocks = [_]conflation_plan.BlockPlan{
        .{ .forced_transactions = &.{gapped_forced_transaction} },
        .{},
    };
    const plan = conflation_plan.ConflationPlan{
        .blocks = &blocks,
        .parent_ftx_rolling_hash = PARENT_FTX_ROLLING_HASH,
        .parent_last_processed_ftx_number = PARENT_FTX_NUMBER,
    };

    try plan.expectReject(alloc, error.ForcedTxOutOfOrder);
}

test "an FTX-free continuation range carries the parent FTX state through unchanged" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const plan = conflation_plan.ConflationPlan{
        .parent_ftx_rolling_hash = PARENT_FTX_ROLLING_HASH,
        .parent_last_processed_ftx_number = PARENT_FTX_NUMBER,
    };

    const output = try plan.run(alloc);

    try testing.expectEqualSlices(u8, &PARENT_FTX_ROLLING_HASH, &output.public_inputs.end_ftx_rolling_hash);
    try testing.expectEqual(PARENT_FTX_NUMBER, output.public_inputs.end_processed_ftx_number);
    try testing.expectEqualSlices(u8, &PARENT_FTX_ROLLING_HASH, &output.public_inputs.parent_ftx_rolling_hash);
    try testing.expectEqual(PARENT_FTX_NUMBER, output.public_inputs.parent_processed_ftx_number);
}
