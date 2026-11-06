const std = @import("std");

const lineth_da = @import("lineth_da");

const blob_payload = lineth_da.encoding.blob_payload;

const gpa = std.testing.allocator;

const blob_len = blob_payload.blob_len;
const stream_len = blob_payload.stream_len;

/// The packing written out bit by bit: the stream `payload ‖ 0xff ‖ 0x00…` fills the low 254 bits
/// of each 32-byte element, most significant bit first.
fn referencePack(payload: []const u8) [blob_len]u8 {
    var blob: [blob_len]u8 = @splat(0);
    for (0..blob_payload.field_elements * blob_payload.bits_per_element) |stream_bit| {
        const byte_index = stream_bit / 8;
        const byte: u8 = if (byte_index < payload.len) payload[byte_index] else if (byte_index == payload.len) 0xff else 0;
        const bit: u8 = (byte >> @intCast(7 - stream_bit % 8)) & 1;
        const element = stream_bit / blob_payload.bits_per_element;
        const position = 2 + stream_bit % blob_payload.bits_per_element;
        blob[element * 32 + position / 8] |= bit << @intCast(7 - position % 8);
    }
    return blob;
}

fn randomPayload(len: usize, seed: u64) ![]u8 {
    var prng = std.Random.DefaultPrng.init(seed);
    const payload = try gpa.alloc(u8, len);
    prng.random().bytes(payload);
    return payload;
}

fn packBlob(payload: []const u8) !*[blob_len]u8 {
    const blob = try gpa.create([blob_len]u8);
    errdefer gpa.destroy(blob);
    try blob_payload.pack(payload, blob);
    return blob;
}

test "the blob geometry is that of EIP-4844 and the 254-bit packing" {
    try std.testing.expectEqual(131072, blob_payload.blob_len);
    try std.testing.expectEqual(130048, blob_payload.stream_len);
    try std.testing.expectEqual(130047, blob_payload.payload_capacity);
}

test "an empty payload packs to the terminal followed by zeros" {
    const blob = try packBlob(&.{});
    defer gpa.destroy(blob);

    // The first element holds the bits 1111_1111 then 246 zero bits below its two zero top bits:
    // 0b00111111, 0b11000000, 0 ...
    try std.testing.expectEqualSlices(u8, &.{ 0x3f, 0xc0 }, blob[0..2]);
    try std.testing.expect(std.mem.allEqual(u8, blob[2..], 0));
}

test "a payload fills the elements 254 bits at a time" {
    const blob = try packBlob(&.{ 0x01, 0x02 });
    defer gpa.destroy(blob);

    // Stream bits 00000001 00000010 11111111 0...: the first element's top two bits are zero, so the
    // payload starts at bit 2 of its first byte: 00|000000, 01|000000, 10|111111, 11|000000 ...
    try std.testing.expectEqualSlices(u8, &.{ 0x00, 0x40, 0xbf, 0xc0 }, blob[0..4]);
    try std.testing.expect(std.mem.allEqual(u8, blob[4..], 0));
}

test "packing matches the bit-by-bit definition" {
    for ([_]usize{ 0, 1, 31, 32, 33, 127, 128, 4000, blob_payload.payload_capacity }, 0..) |len, seed| {
        const payload = try randomPayload(len, seed);
        defer gpa.free(payload);
        const blob = try packBlob(payload);
        defer gpa.destroy(blob);
        const expected = try gpa.create([blob_len]u8);
        defer gpa.destroy(expected);
        expected.* = referencePack(payload);
        try std.testing.expectEqualSlices(u8, expected, blob);
    }
}

test "packed payloads unpack to themselves" {
    const stream = try gpa.create([stream_len]u8);
    defer gpa.destroy(stream);
    for ([_]usize{ 0, 1, 2, 31, 32, 33, 254, 1000, 65536, blob_payload.payload_capacity }, 100..) |len, seed| {
        const payload = try randomPayload(len, seed);
        defer gpa.free(payload);
        const blob = try packBlob(payload);
        defer gpa.destroy(blob);
        try std.testing.expectEqualSlices(u8, payload, try blob_payload.unpack(blob, stream));
    }
}

test "payloads made of terminal-like bytes round-trip" {
    const stream = try gpa.create([stream_len]u8);
    defer gpa.destroy(stream);
    for ([_][]const u8{ &.{0xff}, &.{ 0xff, 0xff, 0xff }, &.{0x00}, &.{ 0x00, 0x00, 0x00 }, &.{ 0xff, 0x00, 0xff, 0x00 } }) |payload| {
        const blob = try packBlob(payload);
        defer gpa.destroy(blob);
        try std.testing.expectEqualSlices(u8, payload, try blob_payload.unpack(blob, stream));
    }
}

test "a payload beyond the capacity does not pack" {
    const payload = try gpa.alloc(u8, blob_payload.payload_capacity + 1);
    defer gpa.free(payload);
    @memset(payload, 1);
    const blob = try gpa.create([blob_len]u8);
    defer gpa.destroy(blob);
    try std.testing.expectError(error.PayloadTooLarge, blob_payload.pack(payload, blob));
}

test "an element of 2^254 or more is rejected" {
    const stream = try gpa.create([stream_len]u8);
    defer gpa.destroy(stream);
    const blob = try packBlob("a payload");
    defer gpa.destroy(blob);

    for ([_]usize{ 0, 1000, blob_payload.field_elements - 1 }) |element| {
        for ([_]u8{ 0x40, 0x80, 0xc0 }) |bits| {
            const forged = try gpa.dupe(u8, blob);
            defer gpa.free(forged);
            forged[element * 32] |= bits;
            try std.testing.expectError(error.NonCanonicalElement, blob_payload.unpack(forged[0..blob_len], stream));
        }
    }
}

test "a blob without a terminal is rejected" {
    const stream = try gpa.create([stream_len]u8);
    defer gpa.destroy(stream);
    const zeros = try gpa.alloc(u8, blob_len);
    defer gpa.free(zeros);
    @memset(zeros, 0);
    try std.testing.expectError(error.MissingTerminal, blob_payload.unpack(zeros[0..blob_len], stream));
}

/// Flips one bit of the stream a blob carries.
fn flipStreamBit(blob: *[blob_len]u8, stream_bit: usize) void {
    const element = stream_bit / blob_payload.bits_per_element;
    const position = 2 + stream_bit % blob_payload.bits_per_element;
    blob[element * 32 + position / 8] ^= @as(u8, 1) << @intCast(7 - position % 8);
}

test "a blob whose last nonzero stream byte is not the terminal is rejected" {
    const stream = try gpa.create([stream_len]u8);
    defer gpa.destroy(stream);
    const payload = "a payload";

    // A terminal byte below 0xff.
    const weakened = try packBlob(payload);
    defer gpa.destroy(weakened);
    flipStreamBit(weakened, payload.len * 8);
    try std.testing.expectError(error.MissingTerminal, blob_payload.unpack(weakened, stream));

    // A nonzero bit in the padding after the terminal.
    const padded = try packBlob(payload);
    defer gpa.destroy(padded);
    flipStreamBit(padded, stream_len * 8 - 1);
    try std.testing.expectError(error.MissingTerminal, blob_payload.unpack(padded, stream));
}
