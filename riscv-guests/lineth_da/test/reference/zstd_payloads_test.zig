//! The decoder over frames of real payloads: the canonical payloads of consecutive blocks of the EF
//! zkevm fixtures, compressed in every form the guest decodes or must reject.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");
const zstd_frames = @import("zstd_frames");

const zstd = lineth_da.zstd;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

/// The frame the package's compressor produces.
fn packageFrame(payload: []const u8) ![]u8 {
    return zstd_compressor.compress(gpa, payload, dictionary);
}

/// A frame in the form `options` select, against the package dictionary unless they name another.
fn compress(payload: []const u8, options: zstd_frames.Options) ![]u8 {
    var with_dictionary = options;
    if (with_dictionary.dictionary == null) with_dictionary.dictionary = dictionary;
    return zstd_frames.compress(gpa, payload, with_dictionary);
}

fn compressWithoutDictionary(payload: []const u8, options: zstd_frames.Options) ![]u8 {
    var plain = options;
    plain.dictionary = null;
    return zstd_frames.compress(gpa, payload, plain);
}

/// `len` bytes of the real payload stream; payloads of different variants start 6000 bytes apart.
fn real(len: usize, variant: usize) ![]const u8 {
    const offset = variant % 24 * 6_000;
    return (try ef_conflations.stream(std.testing.io, ef_fixtures.amsterdam_blocks, offset + len))[offset..][0..len];
}

/// A copy of the dictionary announcing itself under another dictionary ID.
fn dictionaryWithId(id: u32) ![]u8 {
    const copy = try gpa.dupe(u8, dictionary);
    std.mem.writeInt(u32, copy[4..8], id, .little);
    return copy;
}

test "round trips payloads compressed against the dictionary" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const lens = [_]usize{ 0, 1, 33, 1000, 40_000, 300_000 };
    const levels = [_]i32{ 1, 3, 19 };
    for (lens) |len| {
        const payload = try real(len, len);
        const out = try gpa.alloc(u8, len + 64);
        defer gpa.free(out);
        for (levels) |level| {
            if (level == 19 and len > 40_000) continue;
            const frame = try compress(payload, .{ .level = level });
            defer gpa.free(frame);

            const exact = try decoder.decompress(frame, out[0..len]);
            try std.testing.expectEqualSlices(u8, payload, exact);
            const loose = try decoder.decompress(frame, out);
            try std.testing.expectEqualSlices(u8, payload, loose);
        }
    }
}

test "frames in every header and checksum form decode" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try real(70_000, 11);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    for ([_]bool{ false, true }) |content_size| {
        for ([_]bool{ false, true }) |checksum| {
            for ([_]bool{ false, true }) |dictionary_id| {
                const frame = try compress(payload, .{ .content_size = content_size, .checksum = checksum, .dictionary_id = dictionary_id });
                defer gpa.free(frame);
                try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
            }
        }
    }
}

test "frames compressed without a dictionary decode against the dictionary" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try real(20_000, 5);
    const frame = try compressWithoutDictionary(payload, .{});
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
}

test "every strict prefix of a frame is truncated" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try real(300_000, 3);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    for ([_]bool{ false, true }) |checksum| {
        const frame = try compress(payload, .{ .level = 1, .checksum = checksum });
        defer gpa.free(frame);
        var len: usize = 0;
        while (len < frame.len) : (len += if (len < 64 or frame.len - len < 64) 1 else 997) {
            try std.testing.expectError(error.TruncatedFrame, decoder.decompress(frame[0..len], out));
        }
    }
}

test "bytes after the frame are rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try real(5000, 9);
    const frame = try packageFrame(payload);
    defer gpa.free(frame);
    const second = try packageFrame("another conflation");
    defer gpa.free(second);
    const skippable = [_]u8{ 0x50, 0x2a, 0x4d, 0x18, 0, 0, 0, 0 };
    const out = try gpa.alloc(u8, payload.len + 64);
    defer gpa.free(out);

    for ([_][]const u8{ &.{0}, second, &skippable }) |suffix| {
        const concatenated = try std.mem.concat(gpa, u8, &.{ frame, suffix });
        defer gpa.free(concatenated);
        try std.testing.expectError(error.TrailingBytes, decoder.decompress(concatenated, out));
    }
}

test "output beyond the bound is rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    for ([_]usize{ 1, 100, 5000, 300_000 }) |len| {
        const payload = try real(len, len);
        const out = try gpa.alloc(u8, len);
        defer gpa.free(out);
        for ([_]bool{ false, true }) |content_size| {
            const frame = try compress(payload, .{ .level = 1, .content_size = content_size });
            defer gpa.free(frame);
            try std.testing.expectError(error.OutputTooLarge, decoder.decompress(frame, out[0 .. len - 1]));
            try std.testing.expectError(error.OutputTooLarge, decoder.decompress(frame, out[0..0]));
            try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
        }
    }
}

test "a corrupted checksummed frame decodes to the original bytes or is rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try real(3000, 21);
    const frame = try compress(payload, .{ .checksum = true });
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);

    const corrupted = try gpa.dupe(u8, frame);
    defer gpa.free(corrupted);
    for (0..frame.len) |at| {
        for ([_]u8{ 0x01, 0x10, 0x80, 0xff }) |mask| {
            corrupted[at] = frame[at] ^ mask;
            if (decoder.decompress(corrupted, out)) |decoded| {
                try std.testing.expectEqualSlices(u8, payload, decoded);
            } else |_| {}
            corrupted[at] = frame[at];
        }
    }
}

test "frames announcing another dictionary are rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const other = try dictionaryWithId(0x1234_5678);
    defer gpa.free(other);
    const payload = try real(8000, 13);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);

    const frame = try compress(payload, .{ .dictionary = other });
    defer gpa.free(frame);
    try std.testing.expectError(error.DictionaryMismatch, decoder.decompress(frame, out));

    const other_decoder = try zstd.Decoder.init(gpa, other);
    defer other_decoder.deinit();
    try std.testing.expectEqualSlices(u8, payload, try other_decoder.decompress(frame, out));
    const own_frame = try packageFrame(payload);
    defer gpa.free(own_frame);
    try std.testing.expectError(error.DictionaryMismatch, other_decoder.decompress(own_frame, out));
}

test "a decoder stays usable after rejecting a frame" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const first = try real(9000, 1);
    const second = try real(120_000, 4);
    const first_frame = try packageFrame(first);
    defer gpa.free(first_frame);
    const second_frame = try packageFrame(second);
    defer gpa.free(second_frame);
    const out = try gpa.alloc(u8, second.len);
    defer gpa.free(out);

    try std.testing.expectEqualSlices(u8, first, try decoder.decompress(first_frame, out));
    try std.testing.expectError(error.TruncatedFrame, decoder.decompress(second_frame[0 .. second_frame.len / 2], out));
    try std.testing.expectError(error.OutputTooLarge, decoder.decompress(second_frame, out[0..100]));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress("garbage", out));
    try std.testing.expectEqualSlices(u8, second, try decoder.decompress(second_frame, out));
    try std.testing.expectEqualSlices(u8, first, try decoder.decompress(first_frame, out));
}

test "decompression allocates at most one block buffer" {
    var counting = std.testing.FailingAllocator.init(gpa, .{});
    const decoder = try zstd.Decoder.init(counting.allocator(), dictionary);
    defer decoder.deinit();
    const created = counting.alloc_index;
    try std.testing.expect(counting.allocated_bytes < 1 << 20);

    const payload = try real(300_000, 17);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);

    // A frame with content size decodes in one pass.
    const frame = try packageFrame(payload);
    defer gpa.free(frame);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
    try std.testing.expectError(error.TruncatedFrame, decoder.decompress(frame[0 .. frame.len - 1], out));
    try std.testing.expectEqual(created, counting.alloc_index);

    // A frame without content size streams its blocks through one input buffer of 128 KiB at most, kept for the next frames.
    const streamed = try compress(payload, .{ .content_size = false });
    defer gpa.free(streamed);
    const before = counting.allocated_bytes;
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(streamed, out));
    try std.testing.expectEqual(created + 1, counting.alloc_index);
    try std.testing.expect(counting.allocated_bytes - before <= (128 << 10) + 64);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(streamed, out));
    try std.testing.expectEqual(created + 1, counting.alloc_index);
}

test "an allocation refused while decompressing is reported as out of memory" {
    var probe = std.testing.FailingAllocator.init(gpa, .{});
    const probed = try zstd.Decoder.init(probe.allocator(), dictionary);
    probed.deinit();

    var failing = std.testing.FailingAllocator.init(gpa, .{ .fail_index = probe.alloc_index });
    const decoder = try zstd.Decoder.init(failing.allocator(), dictionary);
    defer decoder.deinit();

    const payload = try real(5000, 23);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    const streamed = try compress(payload, .{ .content_size = false });
    defer gpa.free(streamed);
    try std.testing.expectError(error.OutOfMemory, decoder.decompress(streamed, out));
    const single_pass = try packageFrame(payload);
    defer gpa.free(single_pass);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(single_pass, out));
}

test "a frame of a real payload declares its content size, a window within the bound and blocks within the window" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    for ([_]usize{ 1, 1000, 131_072, 131_073, 400_000 }) |len| {
        const payload = try real(len, 0);
        const frame = try packageFrame(payload);
        defer gpa.free(frame);

        try zstd_frames.expectWithinSpec(gpa, frame, len, zstd.max_window_size);
        const out = try gpa.alloc(u8, len);
        defer gpa.free(out);
        try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
    }
}
