//! The 254-bit packing of a payload into the 4096 field elements of an EIP-4844 blob. The payload,
//! a 0xff terminal and zero padding form a bit stream; the blob's elements carry its consecutive
//! 254-bit groups, each as a 32-byte big-endian integer below 2^254.

const std = @import("std");

pub const field_elements = 4096;
pub const bytes_per_element = 32;
pub const bits_per_element = 254;
/// The physical size of a blob.
pub const blob_len = field_elements * bytes_per_element;
/// The bytes of the bit stream the elements carry.
pub const stream_len = field_elements * bits_per_element / 8;
/// The longest payload a blob holds: the stream without its terminal.
pub const payload_capacity = stream_len - 1;

pub const UnpackError = error{
    /// An element is 2^254 or larger.
    NonCanonicalElement,
    /// The last nonzero byte of the stream is not the 0xff terminal, or the stream is all zero.
    MissingTerminal,
};

pub const PackError = error{PayloadTooLarge};

/// The payload of `blob`, written into `stream` and returned as its prefix. A blob unpacks only
/// when `pack` of the unpacked payload reproduces it.
pub fn unpack(blob: *const [blob_len]u8, stream: *[stream_len]u8) UnpackError![]const u8 {
    var writer: BitWriter = .{ .out = stream };
    for (0..field_elements) |i| {
        const element = blob[i * bytes_per_element ..][0..bytes_per_element];
        const top = std.mem.readInt(u64, element[0..8], .big);
        if (top >> (bits_per_element - 192) != 0) return error.NonCanonicalElement;
        writer.append(top, bits_per_element - 192);
        inline for (1..4) |word| {
            writer.append(std.mem.readInt(u64, element[word * 8 ..][0..8], .big), 64);
        }
    }

    var end: usize = stream_len;
    while (end > 0 and stream[end - 1] == 0) end -= 1;
    if (end == 0 or stream[end - 1] != 0xff) return error.MissingTerminal;
    return stream[0 .. end - 1];
}

/// Packs `payload` into `blob`.
pub fn pack(payload: []const u8, blob: *[blob_len]u8) PackError!void {
    if (payload.len > payload_capacity) return error.PayloadTooLarge;
    var reader: BitReader = .{ .payload = payload };
    for (0..field_elements) |i| {
        const element = blob[i * bytes_per_element ..][0..bytes_per_element];
        std.mem.writeInt(u64, element[0..8], reader.take(bits_per_element - 192), .big);
        inline for (1..4) |word| {
            std.mem.writeInt(u64, element[word * 8 ..][0..8], reader.take(64), .big);
        }
    }
}

/// Appends values most significant bit first into whole bytes.
const BitWriter = struct {
    out: []u8,
    len: usize = 0,
    pending: u128 = 0,
    pending_bits: u8 = 0,

    fn append(self: *BitWriter, value: u64, bits: u8) void {
        self.pending = self.pending << @intCast(bits) | value;
        self.pending_bits += bits;
        while (self.pending_bits >= 8) {
            self.pending_bits -= 8;
            self.out[self.len] = @truncate(self.pending >> @intCast(self.pending_bits));
            self.len += 1;
        }
        self.pending &= (@as(u128, 1) << @intCast(self.pending_bits)) - 1;
    }
};

/// Reads the stream `payload ‖ 0xff ‖ 0x00…` most significant bit first.
const BitReader = struct {
    payload: []const u8,
    next: usize = 0,
    pending: u128 = 0,
    pending_bits: u8 = 0,

    fn take(self: *BitReader, bits: u8) u64 {
        while (self.pending_bits < bits) {
            self.pending = self.pending << 8 | self.nextByte();
            self.pending_bits += 8;
        }
        self.pending_bits -= bits;
        const value: u64 = @truncate((self.pending >> @intCast(self.pending_bits)) & ((@as(u128, 1) << @intCast(bits)) - 1));
        self.pending &= (@as(u128, 1) << @intCast(self.pending_bits)) - 1;
        return value;
    }

    fn nextByte(self: *BitReader) u8 {
        defer self.next += 1;
        if (self.next < self.payload.len) return self.payload[self.next];
        return if (self.next == self.payload.len) 0xff else 0;
    }
};
