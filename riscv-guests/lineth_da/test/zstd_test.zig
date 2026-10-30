const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");

const zstd = lineth_da.zstd;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

const zstd_magic = [4]u8{ 0x28, 0xb5, 0x2f, 0xfd };

fn compress(payload: []const u8, options: zstd_compressor.Options) ![]u8 {
    var with_dictionary = options;
    if (with_dictionary.dictionary == null) with_dictionary.dictionary = dictionary;
    return zstd_compressor.compress(gpa, payload, with_dictionary);
}

fn compressWithoutDictionary(payload: []const u8, options: zstd_compressor.Options) ![]u8 {
    var plain = options;
    plain.dictionary = null;
    return zstd_compressor.compress(gpa, payload, plain);
}

/// A copy of the dictionary announcing itself under another dictionary ID.
fn dictionaryWithId(id: u32) ![]u8 {
    const copy = try gpa.dupe(u8, dictionary);
    std.mem.writeInt(u32, copy[4..8], id, .little);
    return copy;
}

/// Bytes shaped like a transaction list: addresses and hashes drawn from small pools, short
/// integers and unpredictable signatures.
fn transactionLikePayload(len: usize, seed: u64) ![]u8 {
    var prng = std.Random.DefaultPrng.init(seed);
    const random = prng.random();
    var addresses: [8][20]u8 = undefined;
    var hashes: [8][32]u8 = undefined;
    for (&addresses) |*address| random.bytes(address);
    for (&hashes) |*hash| random.bytes(hash);

    const payload = try gpa.alloc(u8, len);
    var written: usize = 0;
    while (written < len) {
        var record: [96]u8 = undefined;
        const kind = random.uintLessThan(u8, 4);
        const used: usize = switch (kind) {
            0 => blk: {
                record[0] = 0x94;
                @memcpy(record[1..21], &addresses[random.uintLessThan(usize, addresses.len)]);
                break :blk 21;
            },
            1 => blk: {
                record[0] = 0xa0;
                @memcpy(record[1..33], &hashes[random.uintLessThan(usize, hashes.len)]);
                break :blk 33;
            },
            2 => blk: {
                record[0] = 0x82;
                random.bytes(record[1..3]);
                break :blk 3;
            },
            else => blk: {
                record[0] = 0xa0;
                random.bytes(record[1..33]);
                break :blk 33;
            },
        };
        const take = @min(used, len - written);
        @memcpy(payload[written..][0..take], record[0..take]);
        written += take;
    }
    return payload;
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

test "round trips payloads compressed against the dictionary" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const lens = [_]usize{ 0, 1, 33, 1000, 40_000, 300_000 };
    const levels = [_]i32{ 1, 3, 19 };
    for (lens) |len| {
        const payload = try transactionLikePayload(len, len);
        defer gpa.free(payload);
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

test "round trips incompressible payloads" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try randomBytes(500_000, 7);
    defer gpa.free(payload);
    const frame = try compress(payload, .{});
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
}

test "frames in every header and checksum form decode" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try transactionLikePayload(70_000, 11);
    defer gpa.free(payload);
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

test "the dictionary is applied to the frame" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const shared = dictionary[dictionary.len - 4096 ..][0..3000];
    const with_dictionary = try compress(shared, .{});
    defer gpa.free(with_dictionary);
    const without_dictionary = try compressWithoutDictionary(shared, .{});
    defer gpa.free(without_dictionary);
    try std.testing.expect(with_dictionary.len * 4 < without_dictionary.len);

    var out: [3000]u8 = undefined;
    try std.testing.expectEqualSlices(u8, shared, try decoder.decompress(with_dictionary, &out));
}

test "frames compressed without a dictionary decode against the dictionary" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try transactionLikePayload(20_000, 5);
    defer gpa.free(payload);
    const frame = try compressWithoutDictionary(payload, .{});
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
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
    var out: [8]u8 = undefined;
    try std.testing.expectEqualStrings("x", try decoder.decompress(&largest, &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&beyond, &out));
}

test "an empty payload decompresses to an empty slice" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const frame = try compress("", .{});
    defer gpa.free(frame);
    var out: [8]u8 = undefined;
    try std.testing.expectEqual(@as(usize, 0), (try decoder.decompress(frame, out[0..0])).len);
    try std.testing.expectEqual(@as(usize, 0), (try decoder.decompress(frame, &out)).len);
}

test "every strict prefix of a frame is truncated" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try transactionLikePayload(300_000, 3);
    defer gpa.free(payload);
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

    const payload = try transactionLikePayload(5000, 9);
    defer gpa.free(payload);
    const frame = try compress(payload, .{});
    defer gpa.free(frame);
    const second = try compress("another conflation", .{});
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
        const payload = try transactionLikePayload(len, len);
        defer gpa.free(payload);
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

test "a corrupted checksummed frame decodes to the original bytes or is rejected" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const payload = try transactionLikePayload(3000, 21);
    defer gpa.free(payload);
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
    const payload = try transactionLikePayload(8000, 13);
    defer gpa.free(payload);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);

    const frame = try compress(payload, .{ .dictionary = other });
    defer gpa.free(frame);
    try std.testing.expectError(error.DictionaryMismatch, decoder.decompress(frame, out));

    const other_decoder = try zstd.Decoder.init(gpa, other);
    defer other_decoder.deinit();
    try std.testing.expectEqualSlices(u8, payload, try other_decoder.decompress(frame, out));
    const own_frame = try compress(payload, .{});
    defer gpa.free(own_frame);
    try std.testing.expectError(error.DictionaryMismatch, other_decoder.decompress(own_frame, out));
}

test "a decoder stays usable after rejecting a frame" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const first = try transactionLikePayload(9000, 1);
    defer gpa.free(first);
    const second = try transactionLikePayload(120_000, 2);
    defer gpa.free(second);
    const first_frame = try compress(first, .{});
    defer gpa.free(first_frame);
    const second_frame = try compress(second, .{});
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

test "decompression allocates only while the decoder is created" {
    var counting = std.testing.FailingAllocator.init(gpa, .{});
    const decoder = try zstd.Decoder.init(counting.allocator(), dictionary);
    defer decoder.deinit();
    const allocations = counting.alloc_index;
    try std.testing.expect(counting.allocated_bytes < 1 << 20);

    const payload = try transactionLikePayload(300_000, 17);
    defer gpa.free(payload);
    const frame = try compress(payload, .{});
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
    try std.testing.expectError(error.TruncatedFrame, decoder.decompress(frame[0 .. frame.len - 1], out));
    try std.testing.expectEqual(allocations, counting.alloc_index);
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
