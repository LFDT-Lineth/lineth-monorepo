const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");
const zstd_frames = @import("zstd_frames");

const zstd = lineth_da.zstd;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

const zstd_magic = [4]u8{ 0x28, 0xb5, 0x2f, 0xfd };

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

fn randomBytes(len: usize, seed: u64) ![]u8 {
    var prng = std.Random.DefaultPrng.init(seed);
    const bytes = try gpa.alloc(u8, len);
    prng.random().bytes(bytes);
    return bytes;
}

fn expectRejected(result: zstd.Error![]u8) !void {
    if (result) |_| return error.TestUnexpectedSuccess else |_| {}
}

/// `sha256` is the digest of the frame's decompressed content, taken from an independent decoder
/// (python-zstandard 0.23).
const GoldenFrame = struct { frame: []const u8, len: usize, sha256: []const u8 };

const golden_decodable = [_]GoldenFrame{
    .{ .frame = @embedFile("golden_block_128k"), .len = 131_068, .sha256 = "672003418993584239ec5c79232de911699178ad820cd3ca9779abbfe7f7ba7d" },
    .{ .frame = @embedFile("golden_empty_block"), .len = 0, .sha256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
    .{ .frame = @embedFile("golden_rle_first_block"), .len = 1_048_576, .sha256 = "30e14955ebf1352266dc2ff8067e68104607e750abb9d3b36582b8af909fcb58" },
    .{ .frame = @embedFile("golden_zero_seq_2b"), .len = 13, .sha256 = "03ba204e50d126e4674c005e04d82e84c21366780af1f43bd54a37816b6ab340" },
};

const golden_rejected = [_][]const u8{
    @embedFile("golden_error_off0"),
    @embedFile("golden_error_zero_seq_extraneous"),
};

test "linked libzstd is the pinned release" {
    try std.testing.expectEqual(@as(u32, 10506), zstd.version());
}

test "upstream golden frames decompress to their reference digests" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    for (golden_decodable) |golden| {
        const out = try gpa.alloc(u8, golden.len);
        defer gpa.free(out);
        const decoded = try decoder.decompress(golden.frame, out);
        var digest: [32]u8 = undefined;
        std.crypto.hash.sha2.Sha256.hash(decoded, &digest, .{});
        try std.testing.expectEqualStrings(golden.sha256, &std.fmt.bytesToHex(digest, .lower));
    }
}

test "upstream golden corruption frames are rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const out = try gpa.alloc(u8, 1 << 20);
    defer gpa.free(out);
    for (golden_rejected) |frame| {
        try std.testing.expectError(error.MalformedFrame, decoder.decompress(frame, out));
    }
}

test "round trips incompressible payloads" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try randomBytes(500_000, 7);
    defer gpa.free(payload);
    const frame = try packageFrame(payload);
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
}

test "the dictionary is applied to the frame" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const shared = dictionary[dictionary.len - 4096 ..][0..3000];
    const with_dictionary = try packageFrame(shared);
    defer gpa.free(with_dictionary);
    const without_dictionary = try compressWithoutDictionary(shared, .{});
    defer gpa.free(without_dictionary);
    try std.testing.expect(with_dictionary.len * 4 < without_dictionary.len);

    var out: [3000]u8 = undefined;
    try std.testing.expectEqualSlices(u8, shared, try decoder.decompress(with_dictionary, &out));
}

test "hand-assembled raw-block frame with a content checksum" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const content = "hello";
    var frame: [4 + 1 + 1 + 3 + content.len + 4]u8 = undefined;
    @memcpy(frame[0..4], &zstd_magic);
    frame[4] = 0x24; // single segment, 1-byte content size, content checksum
    frame[5] = content.len;
    frame[6..9].* = .{ 0x29, 0x00, 0x00 }; // last block, raw, 5 bytes
    @memcpy(frame[9..][0..content.len], content);
    const checksum: u32 = @truncate(std.hash.XxHash64.hash(0, content));
    std.mem.writeInt(u32, frame[9 + content.len ..][0..4], checksum, .little);

    var out: [16]u8 = undefined;
    try std.testing.expectEqualStrings(content, try decoder.decompress(&frame, &out));

    frame[frame.len - 1] ^= 1;
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&frame, &out));
}

test "hand-assembled run-length-block frame" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    // single segment, 2-byte content size (1000 - 256), one last RLE block of 1000 'a'
    const frame = zstd_magic ++ [_]u8{ 0x60, 0xe8, 0x02, 0x43, 0x1f, 0x00, 'a' };
    var out: [1000]u8 = undefined;
    try std.testing.expectEqualSlices(u8, &([_]u8{'a'} ** 1000), try decoder.decompress(&frame, &out));
    try std.testing.expectError(error.OutputTooLarge, decoder.decompress(&frame, out[0..999]));
}

test "frames declaring a window beyond 128 MiB are rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    // no content size, window descriptor 0x88 (2^27) or 0x90 (2^28), one last raw block holding 'x'
    const largest = zstd_magic ++ [_]u8{ 0x00, 0x88, 0x09, 0x00, 0x00, 'x' };
    const beyond = zstd_magic ++ [_]u8{ 0x00, 0x90, 0x09, 0x00, 0x00, 'x' };
    // the same last raw block under a header that declares the content size 1 and the window 2^28
    const beyond_with_content_size = zstd_magic ++ [_]u8{ 0x80, 0x90, 0x01, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 'x' };
    var out: [8]u8 = undefined;
    try std.testing.expectEqualStrings("x", try decoder.decompress(&largest, &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&beyond, &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&beyond_with_content_size, &out));
}

const BlockKind = enum { raw, rle };

/// A frame of one last block of `len` decoded bytes under a header with no content size and the
/// window descriptor 0x40 (2^18 bytes): `len` bytes of noise for a raw block, one byte for a
/// run-length block of `len` copies of it.
fn lastBlockFrame(kind: BlockKind, len: usize) ![]u8 {
    const size_and_kind: u32 = @as(u32, @intCast(len)) << 3 | @as(u32, @intFromBool(kind == .rle)) << 1 | 1;
    var frame: std.ArrayList(u8) = .empty;
    errdefer frame.deinit(gpa);
    try frame.appendSlice(gpa, &(zstd_magic ++ [_]u8{ 0x00, 0x40 }));
    try frame.appendSlice(gpa, std.mem.asBytes(&std.mem.nativeToLittle(u32, size_and_kind))[0..3]);
    switch (kind) {
        .raw => {
            const noise = try randomBytes(len, len);
            defer gpa.free(noise);
            try frame.appendSlice(gpa, noise);
        },
        .rle => try frame.append(gpa, 'x'),
    }
    return frame.toOwnedSlice(gpa);
}

test "a block beyond 128 KiB is rejected in a frame without content size" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const out = try gpa.alloc(u8, 150_000);
    defer gpa.free(out);
    for ([_]BlockKind{ .raw, .rle }) |kind| {
        const largest = try lastBlockFrame(kind, 131_072);
        defer gpa.free(largest);
        try std.testing.expectEqual(@as(usize, 131_072), (try decoder.decompress(largest, out)).len);

        const beyond = try lastBlockFrame(kind, 150_000);
        defer gpa.free(beyond);
        try std.testing.expectError(error.MalformedFrame, decoder.decompress(beyond, out));
    }
}

test "the largest accepted window is 2^27 + 1 bytes" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    // A single-segment frame declares its content size as its window; one last run-length block
    // of 128 KiB follows, which an 8-byte output buffer cannot hold.
    var out: [8]u8 = undefined;
    var frame = zstd_magic ++ [_]u8{0xe0} ++ ([_]u8{0} ** 8) ++ [_]u8{ 0x03, 0x00, 0x10, 'x' };
    std.mem.writeInt(u64, frame[5..13], (1 << 27) + 1, .little);
    try std.testing.expectError(error.OutputTooLarge, decoder.decompress(&frame, &out));
    std.mem.writeInt(u64, frame[5..13], (1 << 27) + 2, .little);
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&frame, &out));
}

/// A payload whose last bytes repeat its first KiB after 6 KiB of other bytes: a match reaching 7 KiB back.
fn farMatchPayload() ![]u8 {
    const payload = try gpa.alloc(u8, 8192);
    for (payload[0..1024], 0..) |*byte, i| byte.* = @truncate(i);
    var prng = std.Random.DefaultPrng.init(2025);
    prng.random().bytes(payload[1024..7168]);
    @memcpy(payload[7168..], payload[0..1024]);
    return payload;
}

test "a frame without content size declares a window covering its content" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try farMatchPayload();
    defer gpa.free(payload);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    const frame = try compress(payload, .{ .window_log = 17, .block_len = 1024, .content_size = false });
    defer gpa.free(frame);
    try std.testing.expectEqual(@as(u8, 0x38), frame[5]); // the window descriptor: 2^17 bytes
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));

    // The same blocks under a header that declares a window of 1 KiB, which the last match exceeds.
    frame[5] = 0x00;
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(frame, out));
}

test "a frame with content size may declare a window below its content" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try farMatchPayload();
    defer gpa.free(payload);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    const frame = try compress(payload, .{ .window_log = 10 });
    defer gpa.free(frame);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
}

test "an empty payload decompresses to an empty slice" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const frame = try packageFrame("");
    defer gpa.free(frame);
    var out: [8]u8 = undefined;
    try std.testing.expectEqual(@as(usize, 0), (try decoder.decompress(frame, out[0..0])).len);
    try std.testing.expectEqual(@as(usize, 0), (try decoder.decompress(frame, &out)).len);
}

test "input that is not a zstd frame is rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    var out: [4096]u8 = undefined;
    try std.testing.expectError(error.TruncatedFrame, decoder.decompress(&.{}, &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress("not a zstd frame", &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&([_]u8{0} ** 64), &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&(zstd_magic ++ [_]u8{0xff} ** 32), &out));

    var prng = std.Random.DefaultPrng.init(42);
    var noise: [300]u8 = undefined;
    for (0..2000) |_| {
        const len = prng.random().uintLessThan(usize, noise.len + 1);
        prng.random().bytes(noise[0..len]);
        try expectRejected(decoder.decompress(noise[0..len], &out));
    }
}

test "bytes that are not a zstd dictionary are rejected" {
    var raw_content: [4096]u8 = undefined;
    @memset(&raw_content, 'x');
    var corrupt_tables: [4096]u8 = undefined;
    @memset(&corrupt_tables, 0xff);
    @memcpy(corrupt_tables[0..8], dictionary[0..8]);
    const invalid = [_][]const u8{
        &.{},
        dictionary[0..7],
        dictionary[0..8],
        &corrupt_tables,
        &([_]u8{0} ** 65536),
        &raw_content,
    };
    for (invalid) |bytes| {
        try std.testing.expectError(error.InvalidDictionary, zstd.Decoder.init(gpa, bytes));
    }
}

test "every allocation failure while creating a decoder is reported and leaks nothing" {
    var attempts: usize = 0;
    while (attempts < 16) : (attempts += 1) {
        var failing = std.testing.FailingAllocator.init(gpa, .{ .fail_index = attempts });
        if (zstd.Decoder.init(failing.allocator(), dictionary)) |decoder| {
            decoder.deinit();
            try std.testing.expect(attempts >= failing.alloc_index);
        } else |err| {
            try std.testing.expectEqual(error.OutOfMemory, err);
        }
    }
}
