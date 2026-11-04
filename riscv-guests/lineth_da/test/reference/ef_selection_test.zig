//! The selection of real blocks that the reference suites read: every block is one of the corpus,
//! and together they cover what they are picked for.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");

const gpa = std.testing.allocator;

const type_bits = struct {
    const legacy = 1 << 0;
    const access_list = 1 << 1;
    const fee_market = 1 << 2;
    const blob = 1 << 3;
    const set_code = 1 << 4;
};

test "the selected blocks cover the transaction types, calldata, lists, sizes and counts the suites need" {
    var selection = try ef_conflations.Selection.load(gpa, std.testing.io, ef_fixtures.amsterdam_blocks);
    defer selection.deinit();
    const entry = selection.entries;

    // Each accepted transaction type, and no other.
    try std.testing.expectEqual(@as(u8, type_bits.legacy), entry.get(.legacy).profile.types);
    try std.testing.expectEqual(@as(u8, type_bits.access_list), entry.get(.access_list).profile.types);
    try std.testing.expectEqual(@as(u8, type_bits.fee_market), entry.get(.fee_market).profile.types);
    try std.testing.expectEqual(@as(u8, type_bits.set_code), entry.get(.set_code).profile.types);

    // Lists: an access list with several entries, and an authorization list.
    try std.testing.expect(entry.get(.access_list).profile.access_list);
    try std.testing.expect(entry.get(.set_code).profile.authorization_list);
    try std.testing.expect(entry.get(.large_lists).profile.access_list);
    try std.testing.expect(entry.get(.large_lists).profile.authorization_list);

    // The block that truncation rejects: one blob transaction.
    const blob_transaction = selection.blob_transaction.fact;
    try std.testing.expectEqual(@as(u8, type_bits.blob), blob_transaction.profile.types);
    try std.testing.expectEqual(@as(usize, 1), blob_transaction.profile.transactions);

    // Calldata.
    try std.testing.expect(entry.get(.calldata_heavy).profile.max_data_len >= 32 * 1024);

    // Block shapes: no transaction, many transactions, and blocks of hundreds of KB.
    try std.testing.expectEqual(@as(usize, 0), entry.get(.empty_block).profile.transactions);
    try std.testing.expect(entry.get(.many_transactions).profile.transactions >= 20);
    try std.testing.expect(entry.get(.large_lists).rlp.len >= 100 * 1000);
    try std.testing.expect(entry.get(.large_block).rlp.len >= 250 * 1000);
}
