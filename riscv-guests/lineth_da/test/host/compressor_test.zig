const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const host = @import("host");
const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");

const compressor = host.compressor;
const zstd_decoder = lineth_da.zstd_decoder;
const Kind = ef_conflations.Kind;

const gpa = std.testing.allocator;
const dictionary = lineth_da.dictionary_v2025_04_21;

fn selected() !*const ef_conflations.Selection {
    return ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
}

test "compress rejects an empty conflation and bytes that are not a block" {
    const c = try compressor.Compressor.init(gpa, dictionary, 1);
    defer c.deinit();
    var rejection: compressor.Rejection = undefined;

    try std.testing.expectError(error.NoBlocks, c.compress(&.{}, &rejection));

    const garbage = [_]u8{ 0x01, 0x02, 0x03 };
    try std.testing.expectError(error.InvalidBlockRlp, c.compress(&.{ &garbage, &garbage }, &rejection));
    try std.testing.expectEqual(0, rejection.block_index);
    try std.testing.expectEqual(error.MalformedBlock, rejection.reason);
}

test "creating a compressor rejects bytes that are not a zstd dictionary" {
    var not_a_dictionary: [4096]u8 = undefined;
    @memset(&not_a_dictionary, 'x');
    try std.testing.expectError(error.InvalidDictionary, compressor.Compressor.init(gpa, &not_a_dictionary, 1));
    try std.testing.expectError(error.InvalidDictionary, compressor.Compressor.init(gpa, &.{}, 1));
}

test "the frame compress makes equals the package encoder's and decodes to the payload" {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    const a = arena_state.allocator();
    const selection = try selected();
    const c = try compressor.Compressor.init(gpa, dictionary, selection.chain_id);
    defer c.deinit();
    const encoder = try zstd_encoder.Encoder.init(gpa, dictionary);
    defer encoder.deinit();
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const conflations = [_][]const Kind{
        &.{.legacy},
        &.{ .fee_market, .access_list },
        &.{ .set_code, .empty_block },
        &.{ .many_transactions, .empty_block, .legacy },
    };
    for (conflations) |kinds| {
        const frame = try c.compress(try selection.conflationRlps(a, kinds), null);
        defer gpa.free(frame);

        const payload = try selection.conflationPayload(a, kinds);
        const expected = try encoder.compress(a, payload);
        try std.testing.expectEqualSlices(u8, expected, frame);

        const decoded = try a.alloc(u8, payload.len);
        try decoder.decompress(frame, decoded);
        try std.testing.expectEqualSlices(u8, payload, decoded);
    }
}

test "compress rejects a block that cannot be truncated, naming the block and the reason" {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    const a = arena_state.allocator();
    const selection = try selected();
    const legacy = (try selection.conflationRlps(a, &.{.legacy}))[0];
    const c = try compressor.Compressor.init(gpa, dictionary, selection.chain_id);
    defer c.deinit();
    var rejection: compressor.Rejection = undefined;

    const garbage = [_]u8{ 0x01, 0x02, 0x03 };
    try std.testing.expectError(error.InvalidBlockRlp, c.compress(&.{ legacy, &garbage }, &rejection));
    try std.testing.expectEqual(1, rejection.block_index);
    try std.testing.expectEqual(error.MalformedBlock, rejection.reason);

    try std.testing.expectError(error.InvalidBlockRlp, c.compress(&.{legacy[0 .. legacy.len - 1]}, &rejection));
    try std.testing.expectEqual(0, rejection.block_index);

    // A sender is recovered under a positive chain ID.
    const no_chain = try compressor.Compressor.init(gpa, dictionary, 0);
    defer no_chain.deinit();
    try std.testing.expectError(error.InvalidBlockRlp, no_chain.compress(&.{legacy}, &rejection));
    try std.testing.expectEqual(error.SenderRecoveryFailed, rejection.reason);
}

test "a compressor keeps working after the caller frees its dictionary" {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    const a = arena_state.allocator();
    const selection = try selected();
    const rlps = try selection.conflationRlps(a, &.{.legacy});
    const reference = try compressor.Compressor.init(gpa, dictionary, selection.chain_id);
    defer reference.deinit();
    const expected = try reference.compress(rlps, null);
    defer gpa.free(expected);

    const copy = try gpa.dupe(u8, dictionary);
    const c = try compressor.Compressor.init(gpa, copy, selection.chain_id);
    defer c.deinit();
    @memset(copy, 0);
    gpa.free(copy);

    const frame = try c.compress(rlps, null);
    defer gpa.free(frame);
    try std.testing.expectEqualSlices(u8, expected, frame);
}

fn compressUnderFailures(allocator: std.mem.Allocator, rlps: []const []const u8, chain_id: u64) !void {
    const c = try compressor.Compressor.init(allocator, dictionary, chain_id);
    defer c.deinit();
    allocator.free(try c.compress(rlps, null));
}

test "compress reports an allocation that fails and leaks nothing" {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    const selection = try selected();
    const rlps = try selection.conflationRlps(arena_state.allocator(), &.{ .legacy, .fee_market });
    try std.testing.checkAllAllocationFailures(gpa, compressUnderFailures, .{ rlps, selection.chain_id });
}
