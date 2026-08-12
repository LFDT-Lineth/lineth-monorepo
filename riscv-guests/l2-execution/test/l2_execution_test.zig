//! Unit tests for src/l2_execution.zig.
//!
//! The golden `getZkL2ExecutionProofV1.{input,output}.ssz` fixtures (exercised by
//! test/l2_execution_ssz_test.zig) are hand-authored codec vectors with dummy witnesses
//! (`stateRoot=0x1111...`) — real execution cannot reproduce them, so `runL2Execution` is verified
//! here with focused unit tests instead: deterministic helpers against Python-computed expected
//! values, message extraction, forced-transaction dispatch, and witness-backed MPT reads against a
//! hand-built trie.

const std = @import("std");
const testing = std.testing;

const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const executor = @import("zesu_executor");
const mpt = @import("zesu_mpt");
const primitives = @import("zesu_primitives");
const tx_fixtures = @import("tx_fixtures");

const types = executor.executor_types;
const rlp = executor.executor_rlp_encode;
const api = l2_execution.test_api;

fn repeat(comptime n: usize, byte: u8) [n]u8 {
    var out: [n]u8 = undefined;
    for (&out) |*b| b.* = byte;
    return out;
}

// ─── Deterministic helpers vs Python-computed expected values ────────────────────────────────────
// Expected bytes computed with `rollup_spec/.venv/bin/python` against the same formulas in
// `rollup_spec/src/rollup_spec/l2_execution.py` / `block.py`.

test "chainConfigHash matches Readme.md's §2.1 dynamicChainConfigHash formula" {
    const chain_config = l2_execution_ssz.ChainConfig{
        .l2_message_service_address = repeat(20, 0x11),
        .coinbase = repeat(20, 0x00),
        .chain_id = 59144,
    };
    const got = api.chainConfigHashFn(chain_config, 1_000_000_000);
    const want = [_]u8{ 0xeb, 0x9a, 0xbc, 0xa2, 0x92, 0x7e, 0x7d, 0x36, 0x99, 0x9c, 0x8d, 0x0a, 0xe3, 0xf4, 0x94, 0xf7, 0xb0, 0x12, 0x0a, 0xde, 0xc4, 0x1f, 0x5c, 0xe1, 0x3a, 0x2b, 0x98, 0xdd, 0xa4, 0x38, 0x50, 0x06 };
    try testing.expectEqualSlices(u8, &want, &got);
}

test "addToForcedTxRollingHash matches Readme.md's §6.3 forced-tx rolling-hash formula" {
    const prev = repeat(32, 0x22);
    const tx_hash = repeat(32, 0x33);
    const from_address = repeat(20, 0x44);
    const got = api.addToForcedTxRollingHashFn(prev, tx_hash, 12345, from_address);
    const want = [_]u8{ 0x9e, 0xc6, 0xd4, 0x57, 0x32, 0x98, 0x0b, 0x86, 0xb3, 0x7d, 0xc1, 0xbd, 0x7e, 0xaa, 0xfd, 0xf6, 0x6b, 0xd5, 0xbf, 0xdd, 0x7f, 0x8d, 0x04, 0x3e, 0xfb, 0x2f, 0x94, 0x54, 0x65, 0xb2, 0x89, 0x95 };
    try testing.expectEqualSlices(u8, &want, &got);
}

test "hashAddressList / hashDigestList match the Python reference implementation, including the empty-list case" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const empty_want = [_]u8{ 0xc5, 0xd2, 0x46, 0x01, 0x86, 0xf7, 0x23, 0x3c, 0x92, 0x7e, 0x7d, 0xb2, 0xdc, 0xc7, 0x03, 0xc0, 0xe5, 0x00, 0xb6, 0x53, 0xca, 0x82, 0x27, 0x3b, 0x7b, 0xfa, 0xd8, 0x04, 0x5d, 0x85, 0xa4, 0x70 };
    const empty_addrs = try api.hashAddressListFn(alloc, &.{});
    try testing.expectEqualSlices(u8, &empty_want, &empty_addrs);
    const empty_hashes = try api.hashDigestListFn(alloc, &.{});
    try testing.expectEqualSlices(u8, &empty_want, &empty_hashes);

    const addr1 = repeat(20, 0x01);
    const addr2 = repeat(20, 0x02);
    const addr_want = [_]u8{ 0xce, 0x6c, 0x5b, 0x9e, 0x2c, 0xfc, 0x57, 0x0f, 0x35, 0x9b, 0x4c, 0xdc, 0x1f, 0x7f, 0x7c, 0x88, 0x07, 0x72, 0x48, 0x38, 0x8f, 0xbb, 0x0c, 0xf6, 0xc8, 0x29, 0xef, 0x8d, 0xa0, 0x29, 0x61, 0x5e };
    const got_addr = try api.hashAddressListFn(alloc, &.{ addr1, addr2 });
    try testing.expectEqualSlices(u8, &addr_want, &got_addr);

    const h1 = repeat(32, 0xaa);
    const h2 = repeat(32, 0xbb);
    const hash_want = [_]u8{ 0x9f, 0x89, 0xfa, 0xaf, 0x14, 0x95, 0x29, 0x83, 0x00, 0xca, 0x41, 0xed, 0xde, 0x79, 0xc5, 0xcc, 0x9c, 0xb9, 0xbf, 0x17, 0xe1, 0xc9, 0xef, 0x97, 0xac, 0xfd, 0xc5, 0x31, 0x94, 0xf9, 0x01, 0xe1 };
    const got_hash = try api.hashDigestListFn(alloc, &.{ h1, h2 });
    try testing.expectEqualSlices(u8, &hash_want, &got_hash);
}

test "mappingSlot matches the Python reference implementation's Solidity mapping-slot formula" {
    const base_slot = api.u64ToSlot32Fn(281);
    const key = api.u64ToSlot32Fn(7);
    const got = api.mappingSlotFn(base_slot, key);
    const want = [_]u8{ 0x41, 0x92, 0x5a, 0x5f, 0x3a, 0xee, 0x45, 0x64, 0x13, 0x08, 0x42, 0xc8, 0xb6, 0x49, 0x61, 0xf2, 0x2a, 0x92, 0xfe, 0x31, 0x47, 0x84, 0x8f, 0xf9, 0xbf, 0xb3, 0xb4, 0x9b, 0x11, 0x55, 0x63, 0x13 };
    try testing.expectEqualSlices(u8, &want, &got);
}

// ─── L2->L1 message extraction ─────────────────────────────────────────────────────────────────────

fn makeLog(alloc: std.mem.Allocator, address: [20]u8, topics: []const [32]u8) !types.Log {
    return .{
        .address = address,
        .topics = try alloc.dupe([32]u8, topics),
        .data = &.{},
        .block_number = 1,
        .tx_hash = repeat(32, 0),
        .tx_index = 0,
        .block_hash = repeat(32, 0),
        .log_index = 0,
    };
}

fn makeReceiptWithLogs(alloc: std.mem.Allocator, logs: []const types.Log) !types.Receipt {
    return .{
        .type = 0,
        .tx_hash = repeat(32, 0),
        .tx_index = 0,
        .block_hash = repeat(32, 0),
        .block_number = 1,
        .from = repeat(20, 0),
        .to = null,
        .cumulative_gas_used = 21000,
        .gas_used = 21000,
        .contract_address = null,
        .logs = try alloc.dupe(types.Log, logs),
        .logs_bloom = @splat(0),
        .status = 1,
        .effective_gas_price = 0,
    };
}

test "extractL2L1Messages collects topics[3] only for matching address+topic0" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const l2_ms_address = repeat(20, 0x11);
    const other_address = repeat(20, 0x22);
    const message_hash = repeat(32, 0x99);
    const other_topic0 = repeat(32, 0x01);
    const bridge_topic0 = [_]u8{ 0xe8, 0x56, 0xc2, 0xb8, 0xbd, 0x4e, 0xb0, 0x02, 0x7c, 0xe3, 0x2e, 0xea, 0xf5, 0x95, 0xc2, 0x1b, 0x0b, 0x6b, 0x46, 0x44, 0xb3, 0x26, 0xe5, 0xb7, 0xbd, 0x80, 0xa1, 0xcf, 0x8d, 0xb7, 0x2e, 0x6c };

    const matching_log = try makeLog(alloc, l2_ms_address, &.{ bridge_topic0, repeat(32, 0), repeat(32, 0), message_hash });
    const wrong_address_log = try makeLog(alloc, other_address, &.{ bridge_topic0, repeat(32, 0), repeat(32, 0), repeat(32, 0xff) });
    const wrong_topic_log = try makeLog(alloc, l2_ms_address, &.{ other_topic0, repeat(32, 0), repeat(32, 0), repeat(32, 0xff) });

    const receipts = [_]types.Receipt{
        try makeReceiptWithLogs(alloc, &.{ wrong_address_log, matching_log, wrong_topic_log }),
    };

    var out = std.ArrayListUnmanaged([32]u8).empty;
    try api.extractL2L1MessagesFn(alloc, &out, &receipts, l2_ms_address);

    try testing.expectEqual(@as(usize, 1), out.items.len);
    try testing.expectEqualSlices(u8, &message_hash, &out.items[0]);
}

// ─── Witness-backed MPT read (hand-built single-account, single-slot trie) ────────────────────────

const KECCAK_EMPTY = primitives.KECCAK_EMPTY;
const EMPTY_TRIE_HASH = mpt.builder.EMPTY_TRIE_HASH;

/// Hex-prefix "leaf, even length" compact path for a full 32-byte key hash: 0x20 prefix nibble byte
/// followed by the 32 hash bytes verbatim (odd flag unset since 64 nibbles is even).
fn leafCompactPath(alloc: std.mem.Allocator, key_hash: [32]u8) ![]u8 {
    const out = try alloc.alloc(u8, 33);
    out[0] = 0x20;
    @memcpy(out[1..], &key_hash);
    return out;
}

fn buildSingleLeafTrie(alloc: std.mem.Allocator, key: []const u8, value_rlp: []const u8) !struct { root: [32]u8, node_rlp: []const u8 } {
    const key_hash = mpt.keccak256(key);
    const compact_path = try leafCompactPath(alloc, key_hash);
    const items = [_][]const u8{
        try rlp.encodeBytes(alloc, compact_path),
        try rlp.encodeBytes(alloc, value_rlp),
    };
    const node_rlp = try rlp.encodeList(alloc, &items);
    return .{ .root = mpt.keccak256(node_rlp), .node_rlp = node_rlp };
}

fn accountLeafValue(alloc: std.mem.Allocator, nonce: u64, balance: u256, storage_root: [32]u8, code_hash: [32]u8) ![]u8 {
    const items = [_][]const u8{
        try rlp.encodeU64(alloc, nonce),
        try rlp.encodeU256(alloc, balance),
        try rlp.encodeBytes(alloc, &storage_root),
        try rlp.encodeBytes(alloc, &code_hash),
    };
    return rlp.encodeList(alloc, &items);
}

test "witness MPT read: account+storage present, absent address, missing node errors" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const address = repeat(20, 0xaa);
    const absent_address = repeat(20, 0xbb);
    const slot = api.u64ToSlot32Fn(5);
    const slot_value: u256 = 0x1234;

    const storage_leaf = try buildSingleLeafTrie(alloc, &slot, try rlp.encodeU256(alloc, slot_value));

    const account_value = try accountLeafValue(alloc, 7, 1_000_000, storage_leaf.root, KECCAK_EMPTY);
    const account_leaf = try buildSingleLeafTrie(alloc, &address, account_value);

    var node_index = try mpt.buildNodeIndex(alloc, &.{ account_leaf.node_rlp, storage_leaf.node_rlp });
    defer node_index.deinit();

    // Present account: nonce/balance/storage_root round-trip.
    const account = (try api.readAccountFn(account_leaf.root, address, &node_index)).?;
    try testing.expectEqual(@as(u64, 7), account.nonce);
    try testing.expectEqual(@as(u256, 1_000_000), account.balance);
    try testing.expectEqualSlices(u8, &storage_leaf.root, &account.storage_root);

    // Present storage slot, reached through the account's storage_root.
    const value = try api.readStorageFn(account_leaf.root, address, slot, &node_index);
    try testing.expectEqual(slot_value, value);

    // Proven absence: a different address is NOT an error, just null / zero.
    const absent = try api.readAccountFn(account_leaf.root, absent_address, &node_index);
    try testing.expect(absent == null);
    const absent_storage = try api.readStorageFn(account_leaf.root, absent_address, slot, &node_index);
    try testing.expectEqual(@as(u256, 0), absent_storage);

    // Missing witness node: the SAME root hash, but the node pool doesn't contain it -> error, not
    // absence (this is the semantic WitnessDatabase deliberately does NOT have — see l2_execution.zig).
    var empty_index = try mpt.buildNodeIndex(alloc, &.{});
    defer empty_index.deinit();
    try testing.expectError(error.InvalidProof, api.readAccountFn(account_leaf.root, address, &empty_index));
}

// ─── Forced-transaction dispatch (§6.5) ────────────────────────────────────────────────────────────
//
// Fixture: a real secp256k1-signed legacy (type-0) transaction, generated with
// `rollup_spec/.venv/bin/python` (coincurve) so `recoverSender` has a genuine signature to recover,
// exactly like a real forced transaction witness.
//   nonce=5, gasPrice=1e9, gas=21000, to=0xbb*20, value=1000, data=b"", chainId=59144.

const CHAIN_ID: u64 = 59144;
const SIGNED_TX_RLP = [_]u8{
    0xf8, 0x68, 0x05, 0x84, 0x3b, 0x9a, 0xca, 0x00, 0x82, 0x52, 0x08, 0x94, 0xbb, 0xbb, 0xbb, 0xbb,
    0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
    0x82, 0x03, 0xe8, 0x80, 0x83, 0x01, 0xce, 0x34, 0xa0, 0xe6, 0x6d, 0x9f, 0xde, 0x9f, 0x41, 0xf9,
    0xf6, 0xf7, 0xa8, 0xf7, 0x3a, 0x31, 0x25, 0x85, 0x6c, 0x5a, 0xac, 0x6d, 0x04, 0x1b, 0x1d, 0x11,
    0xc3, 0x87, 0x98, 0x89, 0x4b, 0xe7, 0x1d, 0xac, 0x36, 0xa0, 0x11, 0x29, 0x3b, 0xe7, 0x12, 0x46,
    0x01, 0x7e, 0x64, 0x6b, 0x7e, 0x98, 0x3d, 0x8d, 0x4a, 0xf2, 0xb3, 0x25, 0x92, 0x79, 0xdf, 0xee,
    0x3d, 0xd1, 0x67, 0x97, 0x8b, 0xee, 0x8e, 0x7c, 0xe2, 0x43,
};
const EXPECTED_SENDER = [_]u8{ 0x87, 0xf6, 0x43, 0x3e, 0xae, 0x75, 0x7d, 0xf1, 0xf4, 0x71, 0xbf, 0x9c, 0xe0, 0x3f, 0xe3, 0x2d, 0x75, 0x1f, 0xf9, 0xa0 };
const TX_TO = [_]u8{ 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb };
// gas(21000) * gasPrice(1e9) + value(1000) = 21_000_000_001_000.
const TX_MAX_GAS_FEE_PLUS_VALUE: u256 = 21_000_000_001_000;

fn decodeFixtureTx(alloc: std.mem.Allocator) !types.TxInput {
    const decoded = try executor.executor_tx_decode.decodeTxs(alloc, &.{&SIGNED_TX_RLP});
    return decoded[0];
}

/// Recovers the sender of an arbitrary raw signed tx, for freshly-built typed-tx fixtures whose
/// sender must be derived from the signed bytes themselves (`decodeFixtureTx`/`EXPECTED_SENDER`
/// serve the one frozen legacy fixture, whose sender is a fixed, pre-computed constant instead).
fn recoverTxSender(alloc: std.mem.Allocator, signed_tx_rlp: []const u8) ![20]u8 {
    const decoded = try executor.executor_tx_decode.decodeTxs(alloc, &.{signed_tx_rlp});
    var tx = decoded[0];
    return (try api.recoverSenderFn(alloc, &tx, CHAIN_ID)).?;
}

test "recoverSender recovers the known signer of the signed-tx fixture" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var tx = try decodeFixtureTx(alloc);
    const sender = (try api.recoverSenderFn(alloc, &tx, CHAIN_ID)).?;
    try testing.expectEqualSlices(u8, &EXPECTED_SENDER, &sender);
}

fn oneAccountNodeIndex(alloc: std.mem.Allocator, address: [20]u8, nonce: u64, balance: u256) !struct { root: [32]u8, index: mpt.NodeIndex } {
    const account_value = try accountLeafValue(alloc, nonce, balance, EMPTY_TRIE_HASH, KECCAK_EMPTY);
    const leaf = try buildSingleLeafTrie(alloc, &address, account_value);
    const index = try mpt.buildNodeIndex(alloc, &.{leaf.node_rlp});
    return .{ .root = leaf.root, .index = index };
}

const zinput = @import("zesu_input");

/// A minimal payload carrying only what `validateForcedTransactions` reads: `block_number` (for the
/// deadline check) and `raw_transactions` (for the INCLUDED/tx-in-block membership check).
fn dummyPayload(block_number: u64, raw_transactions: []const []const u8) zinput.ExecutionPayload {
    return .{
        .parent_hash = repeat(32, 0),
        .fee_recipient = repeat(20, 0),
        .state_root = repeat(32, 0),
        .receipts_root = repeat(32, 0),
        .logs_bloom = @splat(0),
        .prev_randao = repeat(32, 0),
        .block_number = block_number,
        .gas_limit = 30_000_000,
        .gas_used = 0,
        .timestamp = 0,
        .extra_data = &.{},
        .base_fee_per_gas = 0,
        .block_hash = repeat(32, 0),
        .transactions = &.{},
        .raw_transactions = raw_transactions,
        .withdrawals = &.{},
        .blob_gas_used = 0,
        .excess_blob_gas = 0,
    };
}

const DUMMY_PAYLOAD = dummyPayload(10, &.{});

test "validateForcedTransactions: INCLUDED must appear in the payload's transaction list" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.INCLUDED,
        .deadline = 100,
    };

    // tx present in the payload's raw_transactions -> accepted.
    const payload_with_tx = dummyPayload(10, &.{&SIGNED_TX_RLP});
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    const rejected = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, payload_with_tx, fixture.root, &fixture.index, &.{ftx});
    try testing.expectEqual(@as(usize, 0), rejected.len);
    try testing.expectEqual(@as(u64, 1), last_number);
    try testing.expect(!std.mem.eql(u8, &repeat(32, 0), &rolling_hash)); // rolling hash updated regardless

    // Same FTX, but the tx is NOT in the payload -> rejected.
    const payload_without_tx = dummyPayload(10, &.{});
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.IncludedForcedTxNotInBlock,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, payload_without_tx, fixture.root, &fixture.index, &.{ftx}),
    );
}

test "validateForcedTransactions: FILTERED_ADDRESS_FROM bubbles up the sender" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = 100,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    const rejected = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{ftx});
    try testing.expectEqual(@as(usize, 1), rejected.len);
    try testing.expectEqualSlices(u8, &EXPECTED_SENDER, &rejected[0]);
}

test "validateForcedTransactions: FILTERED_ADDRESS_TO bubbles up the recipient" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_TO,
        .deadline = 100,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    const rejected = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{ftx});
    try testing.expectEqual(@as(usize, 1), rejected.len);
    try testing.expectEqualSlices(u8, &TX_TO, &rejected[0]);
}

test "validateForcedTransactions: BAD_NONCE dispatch mirrors the account's nonce at the parent state" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.BAD_NONCE,
        .deadline = 100,
    };

    // Fixture tx.nonce == 5. Account nonce == 0 (mismatch) -> BAD_NONCE correctly declared.
    var mismatch = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer mismatch.index.deinit();
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    _ = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, mismatch.root, &mismatch.index, &.{ftx});

    // Account nonce == 5 (matches tx.nonce) -> BAD_NONCE was declared incorrectly, must error.
    var match = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 5, 0);
    defer match.index.deinit();
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.BadNonceMismatch,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, DUMMY_PAYLOAD, match.root, &match.index, &.{ftx}),
    );
}

test "validateForcedTransactions: BAD_BALANCE dispatch mirrors the gas+value arithmetic" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.BAD_BALANCE,
        .deadline = 100,
    };

    // Balance below gas+value -> BAD_BALANCE correctly declared.
    var insufficient = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, TX_MAX_GAS_FEE_PLUS_VALUE - 1);
    defer insufficient.index.deinit();
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    _ = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, insufficient.root, &insufficient.index, &.{ftx});

    // Balance covers gas+value -> BAD_BALANCE was declared incorrectly, must error.
    var sufficient = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, TX_MAX_GAS_FEE_PLUS_VALUE);
    defer sufficient.index.deinit();
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.BadBalanceMismatch,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, DUMMY_PAYLOAD, sufficient.root, &sufficient.index, &.{ftx}),
    );
}

test "validateForcedTransactions: ascending-number and deadline checks" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    // Out-of-order FTX number (last_processed=0, declared number=2, expected 1).
    const out_of_order = l2_execution_ssz.ForcedTransactionWitness{
        .number = 2,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = 100,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    try testing.expectError(
        error.ForcedTxOutOfOrder,
        api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{out_of_order}),
    );

    // Deadline already passed (payload.block_number=10, deadline=1).
    const expired = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = 1,
    };
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.ForcedTxDeadlineExceeded,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{expired}),
    );
}

test "validateForcedTransactions: an unknown acceptance value is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = 7,
        .deadline = 100,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    try testing.expectError(
        error.UnknownForcedTxAcceptance,
        api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{ftx}),
    );
}

test "validateForcedTransactions: FILTERED_ADDRESS_TO on a contract-creation tx is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A real signed legacy tx with to=null: recoverSender runs unconditionally, before acceptance
    // dispatch, and succeeds here — isolating the rejection below to genuinely
    // FilteredAddressToOnContractCreation.
    const creation_tx_rlp = try tx_fixtures.buildSignedLegacyTx(alloc, "FilteredAddressToCreation", .{
        .nonce = 0,
        .gas_price = 1_000_000_000,
        .gas = 21_000,
        .to = null,
        .value = 0,
        .chain_id = CHAIN_ID,
    });

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = creation_tx_rlp,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_TO,
        .deadline = 100,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    try testing.expectError(
        error.FilteredAddressToOnContractCreation,
        api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{ftx}),
    );
}

test "validateForcedTransactions: BAD_NONCE with a sender proven absent at the parent state is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // An empty node index, checked against the canonical empty-trie root: the fixture tx's sender
    // is proven absent from the very first lookup, with no witness nodes needed to prove it.
    var empty_index = try mpt.buildNodeIndex(alloc, &.{});
    defer empty_index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.BAD_NONCE,
        .deadline = 100,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    try testing.expectError(
        error.ForcedTxSenderAbsent,
        api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, EMPTY_TRIE_HASH, &empty_index, &.{ftx}),
    );
}

test "validateForcedTransactions: BAD_NONCE whose tx is actually in the block is rejected" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.BAD_NONCE,
        .deadline = 100,
    };
    const payload_with_tx = dummyPayload(10, &.{&SIGNED_TX_RLP});
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    try testing.expectError(
        error.InvalidForcedTxFoundInBlock,
        api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, payload_with_tx, fixture.root, &fixture.index, &.{ftx}),
    );
}

test "validateForcedTransactions: a deadline equal to the block number clears the deadline check" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var fixture = try oneAccountNodeIndex(alloc, EXPECTED_SENDER, 0, 0);
    defer fixture.index.deinit();

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = &SIGNED_TX_RLP,
        .acceptance = api.Acceptance.FILTERED_ADDRESS_FROM,
        .deadline = DUMMY_PAYLOAD.block_number,
    };
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    const rejected = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, fixture.root, &fixture.index, &.{ftx});
    try testing.expectEqual(@as(usize, 1), rejected.len);
    try testing.expectEqualSlices(u8, &EXPECTED_SENDER, &rejected[0]);
}

test "validateForcedTransactions: BAD_BALANCE dispatch mirrors a type-2 (EIP-1559) tx's fee+value" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const tx_rlp = try tx_fixtures.buildSignedEip1559Tx(alloc, "BadBalanceEip1559", .{
        .nonce = 0,
        .max_priority_fee = 0,
        .max_fee_per_gas = 1_000_000_000,
        .gas = 21_000,
        .to = repeat(20, 0xcc),
        .value = 1000,
        .chain_id = CHAIN_ID,
    });
    const sender = try recoverTxSender(alloc, tx_rlp);
    // gas(21000) * maxFeePerGas(1e9) + value(1000) = 21_000_000_001_000.
    const max_fee_plus_value: u256 = 21_000_000_001_000;

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = tx_rlp,
        .acceptance = api.Acceptance.BAD_BALANCE,
        .deadline = 100,
    };

    // Balance below fee+value -> BAD_BALANCE correctly declared.
    var insufficient = try oneAccountNodeIndex(alloc, sender, 0, max_fee_plus_value - 1);
    defer insufficient.index.deinit();
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    _ = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, insufficient.root, &insufficient.index, &.{ftx});

    // Balance covers fee+value -> BAD_BALANCE was declared incorrectly, must error.
    var sufficient = try oneAccountNodeIndex(alloc, sender, 0, max_fee_plus_value);
    defer sufficient.index.deinit();
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.BadBalanceMismatch,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, DUMMY_PAYLOAD, sufficient.root, &sufficient.index, &.{ftx}),
    );
}

test "validateForcedTransactions: BAD_BALANCE dispatch mirrors a type-3 (blob) tx's fee+blob-surcharge+value" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // Two versioned hashes, so the blob surcharge term is visibly nonzero.
    const versioned_hashes = [_][32]u8{ repeat(32, 0xd1), repeat(32, 0xd2) };
    const tx_rlp = try tx_fixtures.buildSignedBlobTx(alloc, "BadBalanceBlob", .{
        .nonce = 0,
        .max_priority_fee = 0,
        .max_fee_per_gas = 1_000_000_000,
        .gas = 21_000,
        .to = repeat(20, 0xcc),
        .value = 1000,
        .chain_id = CHAIN_ID,
        .max_fee_per_blob_gas = 1_000,
        .versioned_hashes = &versioned_hashes,
    });
    const sender = try recoverTxSender(alloc, tx_rlp);

    const gas_fee: u256 = @as(u256, 21_000) * @as(u256, 1_000_000_000);
    const blob_surcharge: u256 = @as(u256, versioned_hashes.len) * @as(u256, primitives.GAS_PER_BLOB) * @as(u256, 1_000);
    const max_fee_plus_value: u256 = gas_fee + blob_surcharge + 1000;

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = tx_rlp,
        .acceptance = api.Acceptance.BAD_BALANCE,
        .deadline = 100,
    };

    // Balance below fee+surcharge+value -> BAD_BALANCE correctly declared.
    var insufficient = try oneAccountNodeIndex(alloc, sender, 0, max_fee_plus_value - 1);
    defer insufficient.index.deinit();
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    _ = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, insufficient.root, &insufficient.index, &.{ftx});

    // Balance covers fee+surcharge+value -> BAD_BALANCE was declared incorrectly, must error.
    var sufficient = try oneAccountNodeIndex(alloc, sender, 0, max_fee_plus_value);
    defer sufficient.index.deinit();
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.BadBalanceMismatch,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, DUMMY_PAYLOAD, sufficient.root, &sufficient.index, &.{ftx}),
    );
}

test "validateForcedTransactions: BAD_BALANCE dispatch mirrors a type-4 (EIP-7702) tx's fee+value" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const tx_rlp = try tx_fixtures.buildSignedEip7702Tx(alloc, "BadBalanceEip7702", .{
        .nonce = 0,
        .max_priority_fee = 0,
        .max_fee_per_gas = 1_000_000_000,
        .gas = 21_000,
        .to = repeat(20, 0xcc),
        .value = 1000,
        .chain_id = CHAIN_ID,
    });
    const sender = try recoverTxSender(alloc, tx_rlp);
    // gas(21000) * maxFeePerGas(1e9) + value(1000) = 21_000_000_001_000: maxGasFee dispatches type
    // 4 through the same gas*maxFeePerGas formula as type 2, adding the blob surcharge only for
    // type 3.
    const max_fee_plus_value: u256 = 21_000_000_001_000;

    const ftx = l2_execution_ssz.ForcedTransactionWitness{
        .number = 1,
        .signed_tx_rlp = tx_rlp,
        .acceptance = api.Acceptance.BAD_BALANCE,
        .deadline = 100,
    };

    // Balance below fee+value -> BAD_BALANCE correctly declared.
    var insufficient = try oneAccountNodeIndex(alloc, sender, 0, max_fee_plus_value - 1);
    defer insufficient.index.deinit();
    var rolling_hash: [32]u8 = repeat(32, 0);
    var last_number: u64 = 0;
    _ = try api.validateForcedTransactionsFn(alloc, &rolling_hash, &last_number, CHAIN_ID, DUMMY_PAYLOAD, insufficient.root, &insufficient.index, &.{ftx});

    // Balance covers fee+value -> BAD_BALANCE was declared incorrectly, must error.
    var sufficient = try oneAccountNodeIndex(alloc, sender, 0, max_fee_plus_value);
    defer sufficient.index.deinit();
    var rolling_hash2: [32]u8 = repeat(32, 0);
    var last_number2: u64 = 0;
    try testing.expectError(
        error.BadBalanceMismatch,
        api.validateForcedTransactionsFn(alloc, &rolling_hash2, &last_number2, CHAIN_ID, DUMMY_PAYLOAD, sufficient.root, &sufficient.index, &.{ftx}),
    );
}

// ─── extractL2L1Messages: malformed log ────────────────────────────────────────────────────────────

test "extractL2L1Messages rejects a matching log with fewer than 4 topics" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const l2_ms_address = repeat(20, 0x11);
    const bridge_topic0 = [_]u8{ 0xe8, 0x56, 0xc2, 0xb8, 0xbd, 0x4e, 0xb0, 0x02, 0x7c, 0xe3, 0x2e, 0xea, 0xf5, 0x95, 0xc2, 0x1b, 0x0b, 0x6b, 0x46, 0x44, 0xb3, 0x26, 0xe5, 0xb7, 0xbd, 0x80, 0xa1, 0xcf, 0x8d, 0xb7, 0x2e, 0x6c };

    // Matching address + topic0, but only 3 topics — one short of topics[3], the message hash.
    const short_log = try makeLog(alloc, l2_ms_address, &.{ bridge_topic0, repeat(32, 0), repeat(32, 0) });
    const receipts = [_]types.Receipt{try makeReceiptWithLogs(alloc, &.{short_log})};

    var out = std.ArrayListUnmanaged([32]u8).empty;
    try testing.expectError(
        error.InvalidBridgeMessageLog,
        api.extractL2L1MessagesFn(alloc, &out, &receipts, l2_ms_address),
    );
}

// ─── readL1L2BridgeState: composed reads over a real two-slot storage trie ─────────────────────────

test "readL1L2BridgeState reads the number and rolling hash from real L2MessageService storage" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const l2_ms_address = repeat(20, 0x22);
    const number: u64 = 7;
    const hash_value: [32]u8 = repeat(32, 0x77);

    var index = try mpt.buildNodeIndex(alloc, &.{});
    defer index.deinit();

    // Slot 280: the message number, as a plain u256.
    var storage_root = EMPTY_TRIE_HASH;
    const number_slot = api.u64ToSlot32Fn(280);
    try mpt.updateStorageChainedIndexed(alloc, &storage_root, number_slot, @as(u256, number), &index);

    // mappingSlot(281, number): the rolling hash keyed by that same number.
    const rolling_hash_slot = api.mappingSlotFn(api.u64ToSlot32Fn(281), api.u64ToSlot32Fn(number));
    const hash_as_u256 = std.mem.readInt(u256, &hash_value, .big);
    try mpt.updateStorageChainedIndexed(alloc, &storage_root, rolling_hash_slot, hash_as_u256, &index);

    const account_rlp = try accountLeafValue(alloc, 0, 0, storage_root, KECCAK_EMPTY);
    var state_root = EMPTY_TRIE_HASH;
    try mpt.updateAccountChainedIndexed(alloc, &state_root, mpt.keccak256(&l2_ms_address), account_rlp, &index);

    const bridge_state = try api.readL1L2BridgeStateFn(state_root, l2_ms_address, &index);
    try testing.expectEqualSlices(u8, &hash_value, &bridge_state.hash);
    try testing.expectEqual(number, bridge_state.number);
}

test "readL1L2BridgeState rejects a message number that overflows u64" {
    var arena = std.heap.ArenaAllocator.init(testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const l2_ms_address = repeat(20, 0x22);
    const overflowing_number: u256 = @as(u256, 1) << 64;

    var index = try mpt.buildNodeIndex(alloc, &.{});
    defer index.deinit();

    var storage_root = EMPTY_TRIE_HASH;
    const number_slot = api.u64ToSlot32Fn(280);
    try mpt.updateStorageChainedIndexed(alloc, &storage_root, number_slot, overflowing_number, &index);

    const account_rlp = try accountLeafValue(alloc, 0, 0, storage_root, KECCAK_EMPTY);
    var state_root = EMPTY_TRIE_HASH;
    try mpt.updateAccountChainedIndexed(alloc, &state_root, mpt.keccak256(&l2_ms_address), account_rlp, &index);

    try testing.expectError(
        error.RollingHashNumberOverflow,
        api.readL1L2BridgeStateFn(state_root, l2_ms_address, &index),
    );
}
