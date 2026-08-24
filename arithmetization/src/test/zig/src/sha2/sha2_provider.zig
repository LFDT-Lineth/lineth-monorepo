//! Self-checking, freestanding SHA-256 provider test.
//!
//! The expected digests are fixed test vectors produced independently of the
//! provider. In particular, this guest never uses `zkvm_sha256` to derive an
//! expected value.

const lineth_accel = @import("lineth_zkvm_accel");
const provide = @import("zkvm_provide");

// Force zkvm_provide's comptime exports to define zkvm_sha256 in this ELF.
comptime {
    _ = provide;
}

extern fn zkvm_sha256(
    data: [*c]const u8,
    len: usize,
    output: [*c]lineth_accel.zkvm_sha256_hash,
) lineth_accel.zkvm_status;

const Digest = [32]u8;
const HASH_ALIGNMENT: usize = 8;
const MAX_PATTERN_LEN: usize = 129;

const BEFORE_CANARY = [_]u8{ 0xa5, 0x3c, 0x7e, 0x91, 0x42, 0xd8, 0x16, 0xeb };
const AFTER_CANARY = [_]u8{ 0x5a, 0xc3, 0x81, 0x6e, 0xbd, 0x27, 0xe9, 0x14 };

const GuardedHash = extern struct {
    before: [8]u8,
    hash: lineth_accel.zkvm_sha256_hash,
    after: [8]u8,
};

// FIPS 180-4's well-known empty-string and "abc" vectors.
const SHA256_EMPTY: Digest = .{
    0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
    0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
};
const SHA256_ABC: Digest = .{
    0xba, 0x78, 0x16, 0xbf, 0x8f, 0x01, 0xcf, 0xea, 0x41, 0x41, 0x40, 0xde, 0x5d, 0xae, 0x22, 0x23,
    0xb0, 0x03, 0x61, 0xa3, 0x96, 0x17, 0x7a, 0x9c, 0xb4, 0x10, 0xff, 0x61, 0xf2, 0x00, 0x15, 0xad,
};

// SHA-256 of byte[i] = (i * 37 + 11) % 256 at the indicated lengths.
const SHA256_PATTERN_32: Digest = .{
    0x83, 0xb7, 0xa8, 0xed, 0x85, 0x90, 0x53, 0xc8, 0x1d, 0x81, 0x88, 0x70, 0xfa, 0xb1, 0xf8, 0xb1,
    0xae, 0x44, 0xd0, 0x6a, 0x98, 0xa9, 0x66, 0x5d, 0x36, 0x9a, 0x8f, 0xd7, 0xd2, 0x83, 0x8d, 0xed,
};
const SHA256_PATTERN_40: Digest = .{
    0x76, 0xde, 0xf7, 0x58, 0x56, 0xe5, 0xd7, 0x3e, 0xce, 0x01, 0x1b, 0x05, 0x8b, 0x02, 0xd2, 0x05,
    0x99, 0x1a, 0x48, 0xf0, 0xfc, 0xf8, 0xb7, 0xdd, 0xcc, 0x24, 0x00, 0x5d, 0x57, 0x75, 0x9b, 0x23,
};
const SHA256_PATTERN_55: Digest = .{
    0x29, 0x00, 0x46, 0x5f, 0xcb, 0x53, 0x3e, 0x05, 0xa1, 0x58, 0xfd, 0x2b, 0x3b, 0xe0, 0xe5, 0xe3,
    0xb0, 0x37, 0x40, 0xd8, 0x30, 0x60, 0xaa, 0x35, 0x80, 0xe0, 0xd9, 0x8a, 0x96, 0xbf, 0x23, 0x84,
};
const SHA256_PATTERN_56: Digest = .{
    0x31, 0x45, 0x4f, 0xf4, 0x8e, 0xf3, 0x6a, 0xf2, 0xf0, 0x8f, 0xd5, 0x11, 0xbd, 0xc3, 0x7d, 0x9d,
    0x58, 0x55, 0xac, 0x23, 0xe9, 0x92, 0xe5, 0xff, 0x54, 0x45, 0xcb, 0x6b, 0x76, 0x74, 0xa6, 0x74,
};
const SHA256_PATTERN_63: Digest = .{
    0x5f, 0x64, 0x01, 0xb9, 0x65, 0x32, 0xc3, 0x6d, 0xe4, 0xe6, 0x5b, 0xee, 0xc0, 0x40, 0x9b, 0x69,
    0xb1, 0xd1, 0x81, 0x86, 0x4c, 0x80, 0x09, 0xb7, 0xa0, 0x4f, 0x43, 0xe5, 0xd5, 0x63, 0x50, 0xd1,
};
const SHA256_PATTERN_64: Digest = .{
    0x94, 0xeb, 0x5d, 0xe4, 0x94, 0x36, 0x13, 0xfd, 0x04, 0x8d, 0xc9, 0x33, 0x93, 0xab, 0x06, 0x87,
    0x74, 0x05, 0xfa, 0xa3, 0x9c, 0x11, 0xf5, 0x3e, 0x93, 0x86, 0x08, 0x33, 0x39, 0x83, 0x3e, 0x7e,
};
const SHA256_PATTERN_65: Digest = .{
    0xfc, 0x51, 0x86, 0x69, 0xb6, 0xeb, 0x4b, 0x4d, 0xd9, 0x18, 0x27, 0xec, 0xac, 0xef, 0x86, 0x68,
    0x9c, 0x72, 0x5b, 0xd5, 0xba, 0xb8, 0x88, 0xfd, 0x3b, 0x26, 0xdb, 0xb1, 0x96, 0xee, 0xc9, 0x54,
};
const SHA256_PATTERN_119: Digest = .{
    0xb0, 0xdc, 0x41, 0xb1, 0xa3, 0x84, 0xe2, 0xf1, 0x20, 0x3f, 0x03, 0x51, 0xb3, 0x8f, 0xbe, 0xaa,
    0xfc, 0xee, 0xf5, 0x77, 0xce, 0x11, 0x91, 0xd5, 0xbf, 0xc2, 0x5d, 0xa3, 0x9f, 0x72, 0x1e, 0xae,
};
const SHA256_PATTERN_120: Digest = .{
    0x5d, 0xf2, 0x4d, 0xd8, 0x02, 0xac, 0x26, 0x13, 0x2c, 0xe6, 0x08, 0xdc, 0xb5, 0xf0, 0x98, 0x41,
    0xee, 0xf0, 0x39, 0xee, 0x0f, 0x15, 0x2a, 0xcf, 0x98, 0xd2, 0x6d, 0x17, 0xfe, 0x4e, 0x88, 0xe6,
};
const SHA256_PATTERN_127: Digest = .{
    0x0f, 0xe7, 0x29, 0xff, 0x19, 0x25, 0x7b, 0xd6, 0xfe, 0xc8, 0x53, 0xac, 0xc2, 0xea, 0x35, 0x5f,
    0x6b, 0x34, 0xb5, 0x8e, 0x6c, 0x0f, 0x68, 0x4c, 0x3e, 0x18, 0x8f, 0xcd, 0xfc, 0xd9, 0xba, 0xae,
};
const SHA256_PATTERN_128: Digest = .{
    0x0a, 0xed, 0xd4, 0x85, 0x6f, 0x8e, 0xba, 0x09, 0x63, 0x62, 0x73, 0x36, 0xad, 0x51, 0x44, 0xa9,
    0xa7, 0xdb, 0xe1, 0x24, 0x98, 0xe6, 0x06, 0x6f, 0x01, 0x65, 0xfc, 0x97, 0xd8, 0xdd, 0xee, 0x4c,
};
const SHA256_PATTERN_129: Digest = .{
    0x4f, 0x17, 0x57, 0xae, 0x4b, 0xff, 0xba, 0xe8, 0x6d, 0x77, 0x5b, 0x83, 0x17, 0x65, 0xb7, 0x5a,
    0xf1, 0x54, 0xd5, 0x2f, 0x7d, 0xea, 0xa4, 0x6d, 0xd3, 0x78, 0x05, 0x1a, 0x2d, 0x3a, 0xd5, 0x7f,
};

const PatternCase = struct {
    len: usize,
    expected: *const Digest,
};

// Repeat two distinct inputs in strict A/B/A/B order, then alternate short and
// long padding-boundary cases so provider state leakage is observable.
const PATTERN_CASES = [_]PatternCase{
    .{ .len = 55, .expected = &SHA256_PATTERN_55 },
    .{ .len = 129, .expected = &SHA256_PATTERN_129 },
    .{ .len = 55, .expected = &SHA256_PATTERN_55 },
    .{ .len = 129, .expected = &SHA256_PATTERN_129 },
    .{ .len = 56, .expected = &SHA256_PATTERN_56 },
    .{ .len = 128, .expected = &SHA256_PATTERN_128 },
    .{ .len = 63, .expected = &SHA256_PATTERN_63 },
    .{ .len = 127, .expected = &SHA256_PATTERN_127 },
    .{ .len = 64, .expected = &SHA256_PATTERN_64 },
    .{ .len = 120, .expected = &SHA256_PATTERN_120 },
    .{ .len = 65, .expected = &SHA256_PATTERN_65 },
    .{ .len = 119, .expected = &SHA256_PATTERN_119 },
};

export fn main() noreturn {
    if (!runChecks()) {
        lineth_accel.zkvm_exit(1);
    }
    lineth_accel.zkvm_exit(0);
}

fn runChecks() bool {
    const empty_anchor = [_]u8{0x6d};
    const abc = [_]u8{ 'a', 'b', 'c' };

    // A/B/A/B makes accidental retained state fail even for familiar vectors.
    if (!hashMatches(empty_anchor[0..0], &SHA256_EMPTY)) return false;
    if (!hashMatches(abc[0..], &SHA256_ABC)) return false;
    if (!hashMatches(empty_anchor[0..0], &SHA256_EMPTY)) return false;
    if (!hashMatches(abc[0..], &SHA256_ABC)) return false;

    // Offset one byte from an eight-byte-aligned backing buffer on purpose.
    var pattern_storage: [MAX_PATTERN_LEN + 1]u8 align(HASH_ALIGNMENT) = undefined;
    fillPattern(pattern_storage[1..]);
    const pattern: []const u8 = pattern_storage[1..];
    if (@intFromPtr(pattern.ptr) % HASH_ALIGNMENT == 0) return false;

    for (PATTERN_CASES) |test_case| {
        if (!hashMatches(pattern[0..test_case.len], test_case.expected)) return false;
    }

    if (!checkExactOverlap()) return false;
    if (!checkPartialOverlap()) return false;
    if (!checkMultiBlockPartialOverlap()) return false;

    return true;
}

fn hashMatches(data: []const u8, expected: *const Digest) bool {
    var guarded = GuardedHash{
        .before = BEFORE_CANARY,
        .hash = .{ .data = [_]u8{0xcc} ** 32 },
        .after = AFTER_CANARY,
    };

    if (@intFromPtr(&guarded.hash) % HASH_ALIGNMENT != 0) return false;

    const status = zkvm_sha256(data.ptr, data.len, &guarded.hash);
    if (status != .ZKVM_EOK) return false;
    if (!bytesEqual(guarded.before[0..], BEFORE_CANARY[0..])) return false;
    if (!bytesEqual(guarded.hash.data[0..], expected.*[0..])) return false;
    if (!bytesEqual(guarded.after[0..], AFTER_CANARY[0..])) return false;

    return true;
}

fn checkExactOverlap() bool {
    var storage: [48]u8 align(HASH_ALIGNMENT) = [_]u8{0xcc} ** 48;
    copyBytes(storage[0..8], BEFORE_CANARY[0..]);
    fillPattern(storage[8..40]);
    copyBytes(storage[40..48], AFTER_CANARY[0..]);

    const address = @intFromPtr(&storage[8]);
    if (address % HASH_ALIGNMENT != 0) return false;

    const input: [*c]const u8 = @ptrFromInt(address);
    const output: [*c]lineth_accel.zkvm_sha256_hash = @ptrFromInt(address);
    const status = zkvm_sha256(input, 32, output);

    if (status != .ZKVM_EOK) return false;
    if (!bytesEqual(storage[0..8], BEFORE_CANARY[0..])) return false;
    if (!bytesEqual(storage[8..40], SHA256_PATTERN_32[0..])) return false;
    if (!bytesEqual(storage[40..48], AFTER_CANARY[0..])) return false;

    return true;
}

fn checkPartialOverlap() bool {
    // input = [1, 41), output = [8, 40): distinct, overlapping ranges. The
    // output address remains eight-byte aligned while the input is unaligned.
    var storage: [48]u8 align(HASH_ALIGNMENT) = [_]u8{0x69} ** 48;
    storage[0] = 0x96;
    fillPattern(storage[1..41]);

    const input_address = @intFromPtr(&storage[1]);
    const output_address = @intFromPtr(&storage[8]);
    if (input_address % HASH_ALIGNMENT == 0) return false;
    if (output_address % HASH_ALIGNMENT != 0) return false;
    if (input_address == output_address) return false;

    const input: [*c]const u8 = @ptrFromInt(input_address);
    const output: [*c]lineth_accel.zkvm_sha256_hash = @ptrFromInt(output_address);
    const status = zkvm_sha256(input, 40, output);

    // The immediate output guards include the input's first seven bytes and
    // final byte, making an output underrun/overrun visible despite overlap.
    const expected_before = [_]u8{ 0x96, 0x0b, 0x30, 0x55, 0x7a, 0x9f, 0xc4, 0xe9 };
    const expected_after = [_]u8{ 0xae, 0x69, 0x69, 0x69, 0x69, 0x69, 0x69, 0x69 };

    if (status != .ZKVM_EOK) return false;
    if (!bytesEqual(storage[0..8], expected_before[0..])) return false;
    if (!bytesEqual(storage[8..40], SHA256_PATTERN_40[0..])) return false;
    if (!bytesEqual(storage[40..48], expected_after[0..])) return false;

    return true;
}

fn checkMultiBlockPartialOverlap() bool {
    // input = [1, 130), output = [64, 96): the digest overlaps the second
    // message block. An implementation that writes output before absorbing the
    // whole input corrupts bytes it still needs to read and fails this vector.
    var storage: [144]u8 align(HASH_ALIGNMENT) = [_]u8{0x69} ** 144;
    storage[0] = 0x96;
    fillPattern(storage[1..130]);

    const input_address = @intFromPtr(&storage[1]);
    const output_address = @intFromPtr(&storage[64]);
    if (input_address % HASH_ALIGNMENT == 0) return false;
    if (output_address % HASH_ALIGNMENT != 0) return false;

    const input: [*c]const u8 = @ptrFromInt(input_address);
    const output: [*c]lineth_accel.zkvm_sha256_hash = @ptrFromInt(output_address);
    const status = zkvm_sha256(input, 129, output);

    if (status != .ZKVM_EOK) return false;
    if (storage[0] != 0x96) return false;
    if (!patternMatches(storage[56..64], 55)) return false;
    if (!bytesEqual(storage[64..96], SHA256_PATTERN_129[0..])) return false;
    if (!patternMatches(storage[96..104], 95)) return false;
    if (!bytesEqual(storage[130..138], ([_]u8{0x69} ** 8)[0..])) return false;

    return true;
}

fn fillPattern(bytes: []u8) void {
    var i: usize = 0;
    while (i < bytes.len) : (i += 1) {
        bytes[i] = @intCast((i * 37 + 11) % 256);
    }
}

fn patternMatches(bytes: []const u8, start_index: usize) bool {
    var i: usize = 0;
    while (i < bytes.len) : (i += 1) {
        if (bytes[i] != @as(u8, @intCast(((start_index + i) * 37 + 11) % 256))) return false;
    }
    return true;
}

fn copyBytes(destination: []u8, source: []const u8) void {
    var i: usize = 0;
    while (i < destination.len) : (i += 1) {
        destination[i] = source[i];
    }
}

fn bytesEqual(left: []const u8, right: []const u8) bool {
    if (left.len != right.len) return false;

    var i: usize = 0;
    while (i < left.len) : (i += 1) {
        if (left[i] != right[i]) return false;
    }
    return true;
}
