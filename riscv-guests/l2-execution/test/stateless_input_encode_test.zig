//! Tests for the test-only vanilla `StatelessInput` SSZ encoder.
//!
//! Two independent guarantees:
//!   - round-trip: a hand-built `StatelessInput`, touching every variable-length branch the wire
//!     format has, survives encode-then-decode with zesu's real decoder.
//!   - golden re-encode: decoding a real EF fixture's vanilla bytes and re-encoding the result
//!     reproduces those bytes exactly, byte-for-byte.

const std = @import("std");
const input = @import("zesu_input");
const ssz_decode = @import("zesu_ssz_decode");
const fixtures = @import("evm_execution_fixtures");
const stateless_input_encode = @import("stateless_input_encode");

fn repeat(comptime n: usize, byte: u8) [n]u8 {
    var out: [n]u8 = undefined;
    for (&out) |*b| b.* = byte;
    return out;
}

fn expectByteListListEqual(want: []const []const u8, got: []const []const u8) !void {
    try std.testing.expectEqual(want.len, got.len);
    for (want, got) |w, g| try std.testing.expectEqualSlices(u8, w, g);
}

// ─── Hand-crafted legacy transactions ──────────────────────────────────────────────────────────────
// Two standalone legacy-RLP transactions (`raw[0] >= 0xc0`, the decoder's legacy-tx branch), built and
// checked field-by-field against the decoder's own parsing formulas before being frozen here as
// literals. They differ in every field that varies a legacy transaction's RLP shape: presence of a
// `to` address (tx A) vs. contract creation (tx B), an EIP-155 `chainId`-carrying `v` (tx A) vs. a
// pre-EIP-155 `v` with no chain id (tx B), and an empty vs. non-empty `data`.
//
// Tx A: nonce=7, gasPrice=1e9, gas=21000, to=0xaa*20, value=1000, data=b"", chainId=59144, yParity=1.
const TX_A_RLP = [_]u8{
    0xf8, 0x67, 0x07, 0x84, 0x3b, 0x9a, 0xca, 0x00, 0x82, 0x52, 0x08, 0x94, 0xaa, 0xaa, 0xaa, 0xaa,
    0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
    0x82, 0x03, 0xe8, 0x80, 0x83, 0x01, 0xce, 0x34, 0xa0, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd,
    0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd,
    0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x9f, 0xfe, 0xdc, 0xba, 0x09, 0x87, 0x65,
    0x43, 0x21, 0xfe, 0xdc, 0xba, 0x09, 0x87, 0x65, 0x43, 0x21, 0xfe, 0xdc, 0xba, 0x09, 0x87, 0x65,
    0x43, 0x21, 0xfe, 0xdc, 0xba, 0x09, 0x87, 0x65, 0x43,
};
const TX_A_TO = repeat(20, 0xaa);
const TX_A_R: u256 = 0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef;
const TX_A_S: u256 = 0xfedcba0987654321fedcba0987654321fedcba0987654321fedcba09876543;

// Tx B: nonce=0, gasPrice=2e9, gas=100000, to=None (creation), value=0, data=6 bytes, v=28 (pre-155).
const TX_B_RLP = [_]u8{
    0xf8, 0x55, 0x80, 0x84, 0x77, 0x35, 0x94, 0x00, 0x83, 0x01, 0x86, 0xa0, 0x80, 0x80, 0x86, 0x60,
    0x80, 0x60, 0x40, 0x52, 0x00, 0x1c, 0xa0, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22,
    0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22,
    0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x9f, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
    0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
    0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33,
};
const TX_B_DATA = [_]u8{ 0x60, 0x80, 0x60, 0x40, 0x52, 0x00 };
const TX_B_R: u256 = 0x2222222222222222222222222222222222222222222222222222222222222222;
// One canonical-encoding byte narrower than TX_B_R: the RLP bytes below strip a leading zero byte
// off this value, so it decodes to a 31-byte (not 32-byte) big-endian integer.
const TX_B_S: u256 = 90462569716653277674664832038037428010367175520031690655826237506182132531;

// ─── Other variable-length fixture data ────────────────────────────────────────────────────────────

const NODE_0 = repeat(10, 0xa1);
const NODE_2 = repeat(4, 0xa3);
// A zero-length middle entry exercises the offset-table's degenerate adjacent-equal-offsets case.
const NODES = [_][]const u8{ &NODE_0, &[_]u8{}, &NODE_2 };

const CODE_0 = repeat(50, 0xb1);
const CODE_1 = repeat(1, 0xb2);
const CODES = [_][]const u8{ &CODE_0, &CODE_1 };

const HEADER_0 = repeat(90, 0xc1);
const HEADER_1 = repeat(32, 0xc2);
const HEADERS = [_][]const u8{ &HEADER_0, &HEADER_1 };

const PUBKEY_0 = repeat(65, 0xd1);
const PUBKEY_1 = repeat(65, 0xd2);
const PUBKEYS = [_][]const u8{ &PUBKEY_0, &PUBKEY_1 };

const WITHDRAWALS = [_]input.Withdrawal{
    .{ .index = 1, .validator_index = 2, .address = repeat(20, 0x08), .amount = 32_000_000_000 },
    .{ .index = 2, .validator_index = 3, .address = repeat(20, 0x09), .amount = 1 },
};

const VERSIONED_HASHES = [_][32]u8{ repeat(32, 0x0a), repeat(32, 0x0b) };

const DEPOSITS_BYTES = repeat(192, 0xde); // one packed SszDepositRequest-sized item, opaque here
const BUILDER_EXITS_BYTES = repeat(68, 0xef); // one packed SszBuilderExitRequest-sized item

const EXTRA_DATA = [_]u8{ 0xde, 0xad, 0xbe, 0xef };
const BLOCK_ACCESS_LIST_BYTES = repeat(16, 0x99);

/// A hand-built `StatelessInput` touching every variable-length branch the wire format has: multiple
/// transactions, a zero-length witness entry alongside non-empty ones, present public keys, non-empty
/// withdrawals/versioned-hashes/execution-requests, and the fork-activation optionals in opposite
/// states from each other (`activation_block` set, `activation_timestamp` unset).
fn sampleInput() input.StatelessInput {
    return .{
        .new_payload_request = .{
            .execution_payload = .{
                .parent_hash = repeat(32, 0x01),
                .fee_recipient = repeat(20, 0x02),
                .state_root = repeat(32, 0x03),
                .receipts_root = repeat(32, 0x04),
                .logs_bloom = @splat(0),
                .prev_randao = repeat(32, 0x05),
                .block_number = 1_000_501,
                .gas_limit = 30_000_000,
                .gas_used = 42_000,
                .timestamp = 1_763_000_000,
                .extra_data = &EXTRA_DATA,
                .base_fee_per_gas = 7_000_000_000,
                .block_hash = repeat(32, 0x06),
                // The encoder reads `raw_transactions`, the wire-format field; this decoded
                // convenience view stays empty here and is repopulated by decode.
                .transactions = &.{},
                .raw_transactions = &.{ &TX_A_RLP, &TX_B_RLP },
                .withdrawals = &WITHDRAWALS,
                .blob_gas_used = 131_072,
                .excess_blob_gas = 0,
                .slot_number = 424_242,
                .block_access_list = &BLOCK_ACCESS_LIST_BYTES,
            },
            .parent_beacon_block_root = repeat(32, 0x07),
            .versioned_hashes = &VERSIONED_HASHES,
            .execution_requests = .{
                .deposits = &DEPOSITS_BYTES,
                .withdrawals = &.{},
                .consolidations = &.{},
                .builder_deposits = &.{},
                .builder_exits = &BUILDER_EXITS_BYTES,
            },
        },
        .witness = .{
            .nodes = &NODES,
            .codes = &CODES,
            .headers = &HEADERS,
        },
        .chain_config = .{
            .chain_id = 59144,
            .active_fork_idx = 0x15, // Amsterdam
            .activation_block = 12_345,
            .activation_timestamp = null,
        },
        .public_keys = &PUBKEYS,
    };
}

test "encode then decode round-trips every field, covering every variable-length branch" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = sampleInput();
    const encoded = try stateless_input_encode.encode(alloc, value);
    const decoded = try ssz_decode.decode(alloc, encoded);

    const want_ep = value.new_payload_request.execution_payload;
    const ep = decoded.new_payload_request.execution_payload;
    try std.testing.expectEqualSlices(u8, &want_ep.parent_hash, &ep.parent_hash);
    try std.testing.expectEqualSlices(u8, &want_ep.fee_recipient, &ep.fee_recipient);
    try std.testing.expectEqualSlices(u8, &want_ep.state_root, &ep.state_root);
    try std.testing.expectEqualSlices(u8, &want_ep.receipts_root, &ep.receipts_root);
    try std.testing.expectEqualSlices(u8, &want_ep.logs_bloom, &ep.logs_bloom);
    try std.testing.expectEqualSlices(u8, &want_ep.prev_randao, &ep.prev_randao);
    try std.testing.expectEqual(want_ep.block_number, ep.block_number);
    try std.testing.expectEqual(want_ep.gas_limit, ep.gas_limit);
    try std.testing.expectEqual(want_ep.gas_used, ep.gas_used);
    try std.testing.expectEqual(want_ep.timestamp, ep.timestamp);
    try std.testing.expectEqualSlices(u8, want_ep.extra_data, ep.extra_data);
    try std.testing.expectEqual(want_ep.base_fee_per_gas, ep.base_fee_per_gas);
    try std.testing.expectEqualSlices(u8, &want_ep.block_hash, &ep.block_hash);
    try std.testing.expectEqual(want_ep.blob_gas_used, ep.blob_gas_used);
    try std.testing.expectEqual(want_ep.excess_blob_gas, ep.excess_blob_gas);
    try std.testing.expectEqual(want_ep.slot_number, ep.slot_number);
    try std.testing.expectEqualSlices(u8, want_ep.block_access_list, ep.block_access_list);

    // Transactions: the raw RLP bytes round-trip byte-exact, and independently decode to exactly the
    // transactions this fixture encodes — proof the raw bytes this encoder wrote are the real thing,
    // not opaque filler.
    try std.testing.expectEqual(@as(usize, 2), ep.raw_transactions.len);
    try std.testing.expectEqualSlices(u8, &TX_A_RLP, ep.raw_transactions[0]);
    try std.testing.expectEqualSlices(u8, &TX_B_RLP, ep.raw_transactions[1]);

    try std.testing.expectEqual(@as(usize, 2), ep.transactions.len);
    const tx_a = ep.transactions[0];
    try std.testing.expectEqual(@as(u64, 7), tx_a.nonce);
    try std.testing.expectEqual(@as(u128, 1_000_000_000), tx_a.gas_price);
    try std.testing.expectEqual(@as(u64, 21_000), tx_a.gas_limit);
    try std.testing.expectEqualSlices(u8, &TX_A_TO, &tx_a.to.?);
    try std.testing.expectEqual(@as(u256, 1000), tx_a.value);
    try std.testing.expectEqual(@as(usize, 0), tx_a.data.len);
    try std.testing.expectEqual(@as(?u64, 59144), tx_a.chain_id);
    try std.testing.expectEqual(@as(u64, 1), tx_a.v);
    try std.testing.expectEqual(TX_A_R, tx_a.r);
    try std.testing.expectEqual(TX_A_S, tx_a.s);

    const tx_b = ep.transactions[1];
    try std.testing.expectEqual(@as(u64, 0), tx_b.nonce);
    try std.testing.expectEqual(@as(u128, 2_000_000_000), tx_b.gas_price);
    try std.testing.expectEqual(@as(u64, 100_000), tx_b.gas_limit);
    try std.testing.expectEqual(@as(?[20]u8, null), tx_b.to);
    try std.testing.expectEqual(@as(u256, 0), tx_b.value);
    try std.testing.expectEqualSlices(u8, &TX_B_DATA, tx_b.data);
    try std.testing.expectEqual(@as(?u64, null), tx_b.chain_id);
    try std.testing.expectEqual(@as(u64, 1), tx_b.v);
    try std.testing.expectEqual(TX_B_R, tx_b.r);
    try std.testing.expectEqual(TX_B_S, tx_b.s);

    try std.testing.expectEqual(WITHDRAWALS.len, ep.withdrawals.len);
    for (WITHDRAWALS, ep.withdrawals) |want, got| {
        try std.testing.expectEqual(want.index, got.index);
        try std.testing.expectEqual(want.validator_index, got.validator_index);
        try std.testing.expectEqualSlices(u8, &want.address, &got.address);
        try std.testing.expectEqual(want.amount, got.amount);
    }

    const npr = value.new_payload_request;
    const got_npr = decoded.new_payload_request;
    try std.testing.expectEqualSlices(u8, &npr.parent_beacon_block_root, &got_npr.parent_beacon_block_root);
    try std.testing.expectEqual(VERSIONED_HASHES.len, got_npr.versioned_hashes.len);
    for (VERSIONED_HASHES, got_npr.versioned_hashes) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);

    try std.testing.expectEqualSlices(u8, npr.execution_requests.deposits, got_npr.execution_requests.deposits);
    try std.testing.expectEqualSlices(u8, npr.execution_requests.withdrawals, got_npr.execution_requests.withdrawals);
    try std.testing.expectEqualSlices(u8, npr.execution_requests.consolidations, got_npr.execution_requests.consolidations);
    try std.testing.expectEqualSlices(u8, npr.execution_requests.builder_deposits, got_npr.execution_requests.builder_deposits);
    try std.testing.expectEqualSlices(u8, npr.execution_requests.builder_exits, got_npr.execution_requests.builder_exits);

    try expectByteListListEqual(&NODES, decoded.witness.nodes);
    try expectByteListListEqual(&CODES, decoded.witness.codes);
    try expectByteListListEqual(&HEADERS, decoded.witness.headers);

    try std.testing.expectEqual(value.chain_config.chain_id, decoded.chain_config.chain_id);
    try std.testing.expectEqualStrings("Amsterdam", decoded.chain_config.fork_name.?);
    try std.testing.expectEqual(value.chain_config.active_fork_idx, decoded.chain_config.active_fork_idx);
    try std.testing.expectEqual(value.chain_config.activation_block, decoded.chain_config.activation_block);
    try std.testing.expectEqual(value.chain_config.activation_timestamp, decoded.chain_config.activation_timestamp);

    try expectByteListListEqual(&PUBKEYS, decoded.public_keys);
}

test "encode reproduces the EF fixture's real vanilla stateless-input bytes byte-for-byte" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const fixture = try fixtures.loadStatelessBlock(alloc, fixtures.embedded.zkevm_stateless_block);
    const decoded = try ssz_decode.decode(alloc, fixture.input);
    const re_encoded = try stateless_input_encode.encode(alloc, decoded);

    try std.testing.expectEqualSlices(u8, fixture.input, re_encoded);
}
