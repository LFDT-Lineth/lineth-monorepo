//! A stream reopened at a cut of a random stream and fed the remaining conflations seals the blobs
//! that the uninterrupted stream sealed after the cut.

const std = @import("std");

const host_streams = @import("host_streams");

const gpa = std.testing.allocator;

fn checkSeed(seed: u64) !void {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    var scenario = try host_streams.Scenario.init(arena_state.allocator(), seed);
    defer scenario.deinit();
    const blobs = scenario.blobs.items;

    for (scenario.cuts[0 .. scenario.cuts.len - 1]) |cut| {
        const end = scenario.locateCut(cut);
        var resumed = try host_streams.run(&host_streams.positionAfter(blobs, end), scenario.conflations[cut + 1 ..]);
        defer host_streams.release(&resumed);

        const first = if (end.tail > 0) end.blob else end.blob + 1;
        try std.testing.expectEqual(blobs.len - first, resumed.items.len);
        for (blobs[first..], resumed.items, 0..) |*expected, *actual, index| {
            try host_streams.expectSameBlobBytes(expected, actual);
            // The shared blob reports only the cuts of the segments appended after the resume.
            const reported_from = if (index == 0 and end.tail > 0) end.end + 1 else 0;
            try std.testing.expectEqualDeep(expected.segmentEnds()[reported_from..], actual.segmentEnds());
        }
    }
}

test "a stream reopened at each cut seals the same blobs as the uninterrupted stream" {
    for (1..9) |seed| try checkSeed(seed);
}
