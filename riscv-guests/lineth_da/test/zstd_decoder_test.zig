const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");
const zstd_frames = @import("zstd_frames");

const zstd_decoder = lineth_da.zstd_decoder;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

/// The frame the package's encoder writes.
fn encode(payload: []const u8) ![]u8 {
    const encoder = try zstd_encoder.Encoder.init(gpa, dictionary);
    defer encoder.deinit();
    return encoder.compress(gpa, payload);
}

/// A frame against the package dictionary that differs from the encoder's by `options`.
fn variant(payload: []const u8, options: zstd_frames.Options) ![]u8 {
    return zstd_frames.compress(gpa, payload, options);
}

fn randomBytes(len: usize, seed: u64) ![]u8 {
    var prng = std.Random.DefaultPrng.init(seed);
    const bytes = try gpa.alloc(u8, len);
    prng.random().bytes(bytes);
    return bytes;
}

const sample = "a conflation payload, long enough to be framed with a header and a block";

test "a frame decodes to the payload the encoder was given" {
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const incompressible = try randomBytes(500_000, 7);
    defer gpa.free(incompressible);
    for ([_][]const u8{ "", sample, incompressible }) |bytes| {
        const frame = try encode(bytes);
        defer gpa.free(frame);
        const out = try gpa.alloc(u8, bytes.len);
        defer gpa.free(out);
        try decoder.decompress(frame, out);
        try std.testing.expectEqualSlices(u8, bytes, out);
    }
}

test "a frame that breaks a rule of the form is rejected under that rule" {
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();
    var out: [sample.len]u8 = undefined;

    const no_content_size = try variant(sample, .{ .dictionary = dictionary, .content_size = false });
    defer gpa.free(no_content_size);
    try std.testing.expectError(error.MissingContentSize, decoder.decompress(no_content_size, &out));

    const frame = try encode(sample);
    defer gpa.free(frame);
    try std.testing.expectError(error.ContentSizeMismatch, decoder.decompress(frame, out[0 .. out.len - 1]));
    var longer: [sample.len + 1]u8 = undefined;
    try std.testing.expectError(error.ContentSizeMismatch, decoder.decompress(frame, &longer));

    const no_dictionary_id = try variant(sample, .{ .dictionary = dictionary, .dictionary_id = false });
    defer gpa.free(no_dictionary_id);
    try std.testing.expectError(error.MissingDictionaryId, decoder.decompress(no_dictionary_id, &out));

    const other = try zstd_frames.dictionaryWithId(gpa, dictionary, 0x1234_5678);
    defer gpa.free(other);
    const other_dictionary = try variant(sample, .{ .dictionary = other });
    defer gpa.free(other_dictionary);
    try std.testing.expectError(error.DictionaryMismatch, decoder.decompress(other_dictionary, &out));

    const checksummed = try variant(sample, .{ .dictionary = dictionary, .checksum = true });
    defer gpa.free(checksummed);
    try std.testing.expectError(error.ChecksumPresent, decoder.decompress(checksummed, &out));
}

test "a frame declares a window of up to 2^27 bytes" {
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    // A content beyond the 2 MiB window of level 3, so that the header carries a window descriptor.
    const bytes = try randomBytes(3 << 20, 5);
    defer gpa.free(bytes);
    const frame = try encode(bytes);
    defer gpa.free(frame);
    const out = try gpa.alloc(u8, bytes.len);
    defer gpa.free(out);
    try std.testing.expectEqual(@as(u64, 1 << 21), (try zstd_frames.header(frame)).window);

    zstd_frames.patchWindowDescriptor(frame, 0x88);
    try std.testing.expectEqual(@as(u64, 1 << 27), (try zstd_frames.header(frame)).window);
    try decoder.decompress(frame, out);
    try std.testing.expectEqualSlices(u8, bytes, out);

    zstd_frames.patchWindowDescriptor(frame, 0x90);
    try std.testing.expectEqual(@as(u64, 1 << 28), (try zstd_frames.header(frame)).window);
    try std.testing.expectError(error.WindowTooLarge, decoder.decompress(frame, out));
}

test "the input has to be exactly one complete frame" {
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();
    var out: [sample.len]u8 = undefined;

    const frame = try encode(sample);
    defer gpa.free(frame);
    try std.testing.expectError(error.TruncatedFrame, decoder.decompress(&.{}, &out));
    try std.testing.expectError(error.TruncatedFrame, decoder.decompress(frame[0 .. frame.len / 2], &out));

    const skippable = [_]u8{ 0x50, 0x2a, 0x4d, 0x18, 0, 0, 0, 0 };
    for ([_][]const u8{ &.{0}, frame, &skippable }) |suffix| {
        const concatenated = try std.mem.concat(gpa, u8, &.{ frame, suffix });
        defer gpa.free(concatenated);
        try std.testing.expectError(error.TrailingBytes, decoder.decompress(concatenated, &out));
    }

    try std.testing.expectError(error.MalformedFrame, decoder.decompress("not a zstd frame", &out));
    try std.testing.expectError(error.MalformedFrame, decoder.decompress(&skippable, out[0..0]));
}

test "bytes that are not a usable zstd dictionary are rejected" {
    var raw_content: [4096]u8 = undefined;
    @memset(&raw_content, 'x');
    const without_id = try zstd_frames.dictionaryWithId(gpa, dictionary, 0);
    defer gpa.free(without_id);
    const invalid = [_][]const u8{ &raw_content, without_id };
    for (invalid) |bytes| {
        try std.testing.expectError(error.InvalidDictionary, zstd_decoder.Decoder.init(gpa, bytes));
    }
}

test "an allocation refused while creating a decoder is reported as out of memory and leaks nothing" {
    var attempts: usize = 0;
    while (attempts < 16) : (attempts += 1) {
        var failing = std.testing.FailingAllocator.init(gpa, .{ .fail_index = attempts });
        if (zstd_decoder.Decoder.init(failing.allocator(), dictionary)) |decoder| {
            decoder.deinit();
            try std.testing.expect(attempts >= failing.alloc_index);
        } else |err| {
            try std.testing.expectEqual(error.OutOfMemory, err);
        }
    }
}
