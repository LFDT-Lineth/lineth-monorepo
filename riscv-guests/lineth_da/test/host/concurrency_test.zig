const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const host = @import("host");
const host_streams = @import("host_streams");
const lineth_da = @import("lineth_da");

const compressor = host.compressor;
const stream = host.stream;
const Kind = ef_conflations.Kind;
const SealedBlob = stream.SealedBlob;

const allocator = std.testing.allocator;
const dictionary = lineth_da.dictionary_v2025_04_21;

/// Opens a stream at `position`, appends the conflations and flushes, recording the failure if any.
fn play(position: *const stream.Position, conflations: []const host_streams.Conflation, outcome: *Outcome) void {
    if (host_streams.run(position, conflations)) |blobs| {
        outcome.blobs = blobs;
    } else |err| {
        outcome.failure = err;
    }
}

const Outcome = struct {
    blobs: std.ArrayList(SealedBlob) = .empty,
    failure: ?anyerror = null,

    fn deinit(self: *Outcome) void {
        host_streams.release(&self.blobs);
    }

    fn expectSame(expected: *const Outcome, actual: *const Outcome) !void {
        try std.testing.expectEqual(null, actual.failure);
        try std.testing.expectEqual(expected.blobs.items.len, actual.blobs.items.len);
        for (expected.blobs.items, actual.blobs.items) |*left, *right| try host_streams.expectSameBlob(left, right);
    }
};

test "two streams on two threads seal what each seals alone, one of them resumed inside a shared blob" {
    const conflations = try host_streams.syntheticConflations(allocator, &.{ 60_000, 90_000, 40_000, 150_000 });
    defer host_streams.freeSynthetic(allocator, conflations);

    const fresh: stream.Position = .fresh(host_streams.genesis);
    var full: Outcome = .{};
    defer full.deinit();
    play(&fresh, conflations, &full);
    try std.testing.expectEqual(null, full.failure);

    // A rollup ended with the third conflation, inside the second blob.
    const resumed = host_streams.positionAfter(full.blobs.items, host_streams.locate(full.blobs.items, 3));
    var resumed_alone: Outcome = .{};
    defer resumed_alone.deinit();
    play(&resumed, conflations[3..], &resumed_alone);
    try std.testing.expectEqual(null, resumed_alone.failure);

    var fresh_together: Outcome = .{};
    defer fresh_together.deinit();
    var resumed_together: Outcome = .{};
    defer resumed_together.deinit();
    const first = try std.Thread.spawn(.{}, play, .{ &fresh, conflations, &fresh_together });
    const second = try std.Thread.spawn(.{}, play, .{ &resumed, conflations[3..], &resumed_together });
    first.join();
    second.join();

    try full.expectSame(&fresh_together);
    try resumed_alone.expectSame(&resumed_together);
}

/// A conflation of selected blocks with the frame a lone compress gives it.
const BlockConflation = struct {
    rlps: []const []const u8,
    frame: []u8,
};

fn conflationsOf(arena: std.mem.Allocator, c: *const compressor.Compressor, selection_groups: []const []const Kind) ![]BlockConflation {
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
    const list = try arena.alloc(BlockConflation, selection_groups.len);
    for (list, selection_groups) |*conflation, kinds| {
        const rlps = try selection.conflationRlps(arena, kinds);
        conflation.* = .{ .rlps = rlps, .frame = try c.compress(rlps, null) };
    }
    return list;
}

fn freeFrames(list: []const BlockConflation) void {
    for (list) |conflation| allocator.free(conflation.frame);
}

/// Compresses `repeats` conflations of `list` on `c`, from the `first`th on, and records the first
/// failure or difference from the lone compress.
fn compressRepeatedly(c: *const compressor.Compressor, list: []const BlockConflation, first: usize, repeats: usize, failure: *?anyerror) void {
    failure.* = null;
    for (0..repeats) |round| {
        const conflation = list[(first + round) % list.len];
        const frame = c.compress(conflation.rlps, null) catch |err| {
            failure.* = err;
            return;
        };
        defer allocator.free(frame);
        if (!std.mem.eql(u8, conflation.frame, frame)) {
            failure.* = error.DifferentFromLoneCompress;
            return;
        }
    }
}

const groups = [_][]const Kind{
    &.{.legacy},
    &.{.fee_market},
    &.{ .legacy, .fee_market },
    &.{ .set_code, .empty_block },
};

test "threads compressing on one compressor make the frames a lone thread makes" {
    var arena_state = std.heap.ArenaAllocator.init(allocator);
    defer arena_state.deinit();
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
    const c = try compressor.Compressor.init(allocator, dictionary, selection.chain_id);
    defer c.deinit();
    const list = try conflationsOf(arena_state.allocator(), &c, &groups);
    defer freeFrames(list);

    var failures: [6]?anyerror = undefined;
    var threads: [6]std.Thread = undefined;
    for (&threads, &failures, 0..) |*thread, *failure, first| {
        thread.* = try std.Thread.spawn(.{}, compressRepeatedly, .{ &c, list, first, 25, failure });
    }
    for (threads) |thread| thread.join();

    for (failures) |failure| try std.testing.expectEqual(null, failure);
}

test "two compressors on two threads make the frames a lone thread makes" {
    var arena_state = std.heap.ArenaAllocator.init(allocator);
    defer arena_state.deinit();
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
    const first = try compressor.Compressor.init(allocator, dictionary, selection.chain_id);
    defer first.deinit();
    const second = try compressor.Compressor.init(allocator, dictionary, selection.chain_id);
    defer second.deinit();
    const first_list = try conflationsOf(arena_state.allocator(), &first, groups[0..2]);
    defer freeFrames(first_list);
    const second_list = try conflationsOf(arena_state.allocator(), &second, groups[2..]);
    defer freeFrames(second_list);

    var failures: [2]?anyerror = undefined;
    const on_first = try std.Thread.spawn(.{}, compressRepeatedly, .{ &first, first_list, 0, 40, &failures[0] });
    const on_second = try std.Thread.spawn(.{}, compressRepeatedly, .{ &second, second_list, 0, 40, &failures[1] });
    on_first.join();
    on_second.join();

    for (failures) |failure| try std.testing.expectEqual(null, failure);
}
