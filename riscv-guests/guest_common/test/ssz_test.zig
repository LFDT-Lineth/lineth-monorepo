//! Tests of guest_common's SSZ codec: the list views, the canonical-encoding rules the decoder
//! enforces, the bounds the encoder enforces, the framing, and the allocations a decode or an
//! encode makes.

const std = @import("std");

const ssz = @import("guest_common").ssz;

// ── Codec over the views ──────────────────────────────────────────────────────

const Allocator = std.mem.Allocator;

const Record = struct {
    id: u64,
    tag: ssz.ByteList(8),
    hashes: ssz.FixedList([32]u8, 4),
    counts: ssz.FixedList(u16, 4),
    blobs: ssz.VariableList(ssz.ByteList(16), 3),
    flag: ssz.StrictBool,
};

// The fixed part of a Record: id, four offsets, flag.
const record_fixed_size = 8 + 4 * 4 + 1;
const tag_offset_pos = 8;
const hashes_offset_pos = 12;
const counts_offset_pos = 16;
const blobs_offset_pos = 20;
const flag_pos = 24;

const Counts = struct {
    id: u64,
    counts: ssz.FixedList(u16, 4),
};

const Flags = struct {
    first: ssz.StrictBool,
    second: ssz.StrictBool,
    number: u32,
};

fn hash(byte: u8) [32]u8 {
    return @splat(byte);
}

const sample_hashes = [_][32]u8{ hash(1), hash(2) };
const sample_counts = [_]u16{ 7, 0x0102, 0xffff };
const sample_blobs = [_]ssz.ByteList(16){
    .{ .bytes = "alpha" },
    .{ .bytes = "" },
    .{ .bytes = "a longer blob!!!" },
};

fn sampleRecord() Record {
    return .{
        .id = 0x1122334455667788,
        .tag = .{ .bytes = "tag" },
        .hashes = .{ .items = &sample_hashes },
        .counts = .{ .items = &sample_counts },
        .blobs = .{ .items = &sample_blobs },
        .flag = .{ .value = true },
    };
}

fn emptyRecord() Record {
    return .{
        .id = 0,
        .tag = .{ .bytes = "" },
        .hashes = .{ .items = &.{} },
        .counts = .{ .items = &.{} },
        .blobs = .{ .items = &.{} },
        .flag = .{ .value = false },
    };
}

fn expectRecordEqual(want: Record, got: Record) !void {
    try std.testing.expectEqual(want.id, got.id);
    try std.testing.expectEqualSlices(u8, want.tag.bytes, got.tag.bytes);
    try std.testing.expectEqual(want.hashes.items.len, got.hashes.items.len);
    for (want.hashes.items, got.hashes.items) |w, g| try std.testing.expectEqualSlices(u8, &w, &g);
    try std.testing.expectEqual(want.counts.items.len, got.counts.items.len);
    for (want.counts.items, got.counts.items) |w, g| try std.testing.expectEqual(w, g);
    try std.testing.expectEqual(want.blobs.items.len, got.blobs.items.len);
    for (want.blobs.items, got.blobs.items) |w, g| try std.testing.expectEqualSlices(u8, w.bytes, g.bytes);
    try std.testing.expectEqual(want.flag.value, got.flag.value);
}

fn expectRoundTrip(value: Record) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const bytes = try ssz.encode(Record, alloc, value);
    const decoded = try ssz.decode(Record, alloc, bytes);
    try expectRecordEqual(value, decoded);
    const again = try ssz.encode(Record, alloc, decoded);
    try std.testing.expectEqualSlices(u8, bytes, again);
}

test "a record with every list filled decodes to what was encoded and re-encodes to the same bytes" {
    try expectRoundTrip(sampleRecord());
}

test "a record with empty lists round-trips" {
    try expectRoundTrip(emptyRecord());
}

test "lists at their bound round-trip" {
    var full = sampleRecord();
    const hashes: [4][32]u8 = .{ hash(1), hash(2), hash(3), hash(4) };
    const counts: [4]u16 = .{ 1, 2, 3, 4 };
    const blobs: [3]ssz.ByteList(16) = .{ .{ .bytes = "0123456789abcdef" }, .{ .bytes = "x" }, .{ .bytes = "" } };
    full.tag = .{ .bytes = "12345678" };
    full.hashes = .{ .items = &hashes };
    full.counts = .{ .items = &counts };
    full.blobs = .{ .items = &blobs };
    try expectRoundTrip(full);
}

test "a boolean round-trips as the byte 1 or 0" {
    const alloc = std.testing.allocator;
    const value: Flags = .{ .first = .{ .value = true }, .second = .{ .value = false }, .number = 0x01020304 };
    const bytes = try ssz.encode(Flags, alloc, value);
    defer alloc.free(bytes);
    try std.testing.expectEqualSlices(u8, &.{ 1, 0, 4, 3, 2, 1 }, bytes);
    const decoded = try ssz.decode(Flags, alloc, bytes);
    try std.testing.expectEqual(value.first.value, decoded.first.value);
    try std.testing.expectEqual(value.second.value, decoded.second.value);
    try std.testing.expectEqual(value.number, decoded.number);
}

fn encodedSample(alloc: Allocator) ![]u8 {
    return ssz.encode(Record, alloc, sampleRecord());
}

fn expectInvalid(bytes: []const u8) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(error.InvalidSsz, ssz.decode(Record, arena.allocator(), bytes));
}

fn setOffset(bytes: []u8, pos: usize, value: usize) void {
    std.mem.writeInt(u32, bytes[pos..][0..4], @intCast(value), .little);
}

test "decoding rejects a first offset that is not the size of the fixed part" {
    const alloc = std.testing.allocator;
    const bytes = try encodedSample(alloc);
    defer alloc.free(bytes);
    setOffset(bytes, tag_offset_pos, record_fixed_size + 1);
    try expectInvalid(bytes);
    setOffset(bytes, tag_offset_pos, record_fixed_size - 1);
    try expectInvalid(bytes);
}

test "decoding rejects bytes between the fixed part and the first list" {
    const alloc = std.testing.allocator;
    const bytes = try encodedSample(alloc);
    defer alloc.free(bytes);

    const gapped = try alloc.alloc(u8, bytes.len + 1);
    defer alloc.free(gapped);
    @memcpy(gapped[0..record_fixed_size], bytes[0..record_fixed_size]);
    gapped[record_fixed_size] = 0;
    @memcpy(gapped[record_fixed_size + 1 ..], bytes[record_fixed_size..]);
    for ([_]usize{ tag_offset_pos, hashes_offset_pos, counts_offset_pos, blobs_offset_pos }) |pos| {
        setOffset(gapped, pos, std.mem.readInt(u32, bytes[pos..][0..4], .little) + 1);
    }
    try expectInvalid(gapped);
}

test "decoding rejects list offsets that decrease" {
    const alloc = std.testing.allocator;
    const bytes = try encodedSample(alloc);
    defer alloc.free(bytes);
    const tag_start = std.mem.readInt(u32, bytes[tag_offset_pos..][0..4], .little);
    setOffset(bytes, hashes_offset_pos, tag_start - 1);
    try expectInvalid(bytes);
}

test "decoding rejects a list offset past the end of the input" {
    const alloc = std.testing.allocator;
    const bytes = try encodedSample(alloc);
    defer alloc.free(bytes);
    setOffset(bytes, blobs_offset_pos, bytes.len + 1);
    try expectInvalid(bytes);
}

test "decoding rejects a list of fixed-size elements that ends inside an element" {
    const alloc = std.testing.allocator;
    const value: Counts = .{ .id = 1, .counts = .{ .items = &sample_counts } };
    const bytes = try ssz.encode(Counts, alloc, value);
    defer alloc.free(bytes);

    const partial = try alloc.alloc(u8, bytes.len + 1);
    defer alloc.free(partial);
    @memcpy(partial[0..bytes.len], bytes);
    partial[bytes.len] = 0;
    try std.testing.expectError(error.InvalidSsz, ssz.decode(Counts, alloc, partial));
    try std.testing.expectError(error.InvalidSsz, ssz.decode(Counts, alloc, bytes[0 .. bytes.len - 1]));
}

test "decoding rejects a boolean byte other than 0 and 1" {
    const alloc = std.testing.allocator;
    const bytes = try encodedSample(alloc);
    defer alloc.free(bytes);
    bytes[flag_pos] = 2;
    try expectInvalid(bytes);
    bytes[flag_pos] = 0xff;
    try expectInvalid(bytes);
}

test "decoding rejects bytes after a container of fixed size" {
    const alloc = std.testing.allocator;
    const bytes = try ssz.encode(Flags, alloc, .{ .first = .{ .value = true }, .second = .{ .value = true }, .number = 1 });
    defer alloc.free(bytes);

    const trailing = try alloc.alloc(u8, bytes.len + 1);
    defer alloc.free(trailing);
    @memcpy(trailing[0..bytes.len], bytes);
    trailing[bytes.len] = 0;
    try std.testing.expectError(error.InvalidSsz, ssz.decode(Flags, alloc, trailing));
    try std.testing.expectError(error.InvalidSsz, ssz.decode(Flags, alloc, bytes[0 .. bytes.len - 1]));
}

test "decoding rejects an input shorter than the fixed part" {
    const alloc = std.testing.allocator;
    const bytes = try encodedSample(alloc);
    defer alloc.free(bytes);
    try expectInvalid(&.{});
    try expectInvalid(bytes[0 .. record_fixed_size - 1]);
}

// The same wire layout as `Record` with every bound one wider, so the encoder accepts what the decoder of `Record` must refuse.
const WideRecord = struct {
    id: u64,
    tag: ssz.ByteList(9),
    hashes: ssz.FixedList([32]u8, 5),
    counts: ssz.FixedList(u16, 5),
    blobs: ssz.VariableList(ssz.ByteList(17), 4),
    flag: ssz.StrictBool,
};

fn emptyWide() WideRecord {
    return .{
        .id = 0,
        .tag = .{ .bytes = "" },
        .hashes = .{ .items = &.{} },
        .counts = .{ .items = &.{} },
        .blobs = .{ .items = &.{} },
        .flag = .{ .value = false },
    };
}

test "decoding rejects each list beyond its bound" {
    const alloc = std.testing.allocator;
    const hashes: [5][32]u8 = .{ hash(1), hash(2), hash(3), hash(4), hash(5) };
    const counts: [5]u16 = .{ 1, 2, 3, 4, 5 };
    const blobs: [4]ssz.ByteList(17) = .{ .{ .bytes = "a" }, .{ .bytes = "b" }, .{ .bytes = "c" }, .{ .bytes = "d" } };
    const long_blobs: [1]ssz.ByteList(17) = .{.{ .bytes = "0123456789abcdefg" }};

    var long_tag = emptyWide();
    long_tag.tag = .{ .bytes = "123456789" };
    var many_hashes = emptyWide();
    many_hashes.hashes = .{ .items = &hashes };
    var many_counts = emptyWide();
    many_counts.counts = .{ .items = &counts };
    var many_blobs = emptyWide();
    many_blobs.blobs = .{ .items = &blobs };
    var long_blob = emptyWide();
    long_blob.blobs = .{ .items = &long_blobs };

    inline for (.{ long_tag, many_hashes, many_counts, many_blobs, long_blob }) |value| {
        const bytes = try ssz.encode(WideRecord, alloc, value);
        defer alloc.free(bytes);
        try expectInvalid(bytes);
    }
}

test "decoding rejects an offset table of a variable list that is not canonical" {
    const alloc = std.testing.allocator;
    const Blobs = struct { blobs: ssz.VariableList(ssz.ByteList(16), 3) };
    const blobs: ssz.VariableList(ssz.ByteList(16), 3) = .{ .items = &sample_blobs };
    const encoded = try ssz.encode(Blobs, alloc, .{ .blobs = blobs });
    defer alloc.free(encoded);
    // Container: one offset, then the list region whose offset table has three entries.
    const region = 4;
    const table = [_]usize{ region, region + 4, region + 8 };
    try std.testing.expectEqual(@as(u32, 12), std.mem.readInt(u32, encoded[region..][0..4], .little));

    const cases = [_]struct { first: u32, second: u32, third: u32 }{
        .{ .first = 13, .second = 17, .third = 17 },
        .{ .first = 0, .second = 12, .third = 12 },
        .{ .first = 12, .second = 4, .third = 16 },
        .{ .first = 12, .second = 13, .third = 12 },
        .{ .first = 12, .second = 13, .third = @intCast(encoded.len) },
        .{ .first = @intCast(encoded.len), .second = 12, .third = 12 },
    };
    for (cases) |case| {
        const corrupted = try alloc.dupe(u8, encoded);
        defer alloc.free(corrupted);
        setOffset(corrupted, table[0], case.first);
        setOffset(corrupted, table[1], case.second);
        setOffset(corrupted, table[2], case.third);
        try std.testing.expectError(error.InvalidSsz, ssz.decode(Blobs, alloc, corrupted));
    }
}

test "encoding rejects each list beyond its bound" {
    const alloc = std.testing.allocator;
    const hashes: [5][32]u8 = .{ hash(1), hash(2), hash(3), hash(4), hash(5) };
    const counts: [5]u16 = .{ 1, 2, 3, 4, 5 };
    const blobs: [4]ssz.ByteList(16) = .{ .{ .bytes = "a" }, .{ .bytes = "b" }, .{ .bytes = "c" }, .{ .bytes = "d" } };
    const long_blobs: [1]ssz.ByteList(16) = .{.{ .bytes = "0123456789abcdefg" }};

    var long_tag = sampleRecord();
    long_tag.tag = .{ .bytes = "123456789" };
    var many_hashes = sampleRecord();
    many_hashes.hashes = .{ .items = &hashes };
    var many_counts = sampleRecord();
    many_counts.counts = .{ .items = &counts };
    var many_blobs = sampleRecord();
    many_blobs.blobs = .{ .items = &blobs };
    var long_blob = sampleRecord();
    long_blob.blobs = .{ .items = &long_blobs };

    inline for (.{ long_tag, many_hashes, many_counts, many_blobs, long_blob }) |value| {
        try std.testing.expectError(error.BoundsViolation, ssz.encode(Record, alloc, value));
    }
}

// ── Allocations ───────────────────────────────────────────────────────────────

/// Counts the calls that reach the allocator behind it.
const CountingAllocator = struct {
    backing: Allocator,
    allocations: usize = 0,
    resizes: usize = 0,

    fn allocator(self: *CountingAllocator) Allocator {
        return .{ .ptr = self, .vtable = &.{ .alloc = alloc, .resize = resize, .remap = remap, .free = free } };
    }

    fn alloc(ctx: *anyopaque, len: usize, alignment: std.mem.Alignment, ret_addr: usize) ?[*]u8 {
        const self: *CountingAllocator = @ptrCast(@alignCast(ctx));
        self.allocations += 1;
        return self.backing.rawAlloc(len, alignment, ret_addr);
    }

    fn resize(ctx: *anyopaque, memory: []u8, alignment: std.mem.Alignment, new_len: usize, ret_addr: usize) bool {
        const self: *CountingAllocator = @ptrCast(@alignCast(ctx));
        self.resizes += 1;
        return self.backing.rawResize(memory, alignment, new_len, ret_addr);
    }

    fn remap(ctx: *anyopaque, memory: []u8, alignment: std.mem.Alignment, new_len: usize, ret_addr: usize) ?[*]u8 {
        const self: *CountingAllocator = @ptrCast(@alignCast(ctx));
        self.resizes += 1;
        return self.backing.rawRemap(memory, alignment, new_len, ret_addr);
    }

    fn free(ctx: *anyopaque, memory: []u8, alignment: std.mem.Alignment, ret_addr: usize) void {
        const self: *CountingAllocator = @ptrCast(@alignCast(ctx));
        self.backing.rawFree(memory, alignment, ret_addr);
    }
};

const Lists = struct {
    tag: ssz.ByteList(8),
    hashes: ssz.FixedList([32]u8, 4),
    counts: ssz.FixedList(u16, 4),
};

test "decoding byte lists and fixed lists allocates nothing" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const value: Lists = .{
        .tag = .{ .bytes = "tag" },
        .hashes = .{ .items = &sample_hashes },
        .counts = .{ .items = &sample_counts },
    };
    const bytes = try ssz.encode(Lists, arena.allocator(), value);

    var counting: CountingAllocator = .{ .backing = arena.allocator() };
    const decoded = try ssz.decode(Lists, counting.allocator(), bytes);
    try std.testing.expectEqual(@as(usize, 0), counting.allocations);
    try std.testing.expectEqualSlices(u8, "tag", decoded.tag.bytes);
    try std.testing.expectEqual(@as(usize, 2), decoded.hashes.items.len);
    try std.testing.expectEqual(@as(usize, 3), decoded.counts.items.len);
}

test "decoding a variable list of byte lists allocates once for its elements" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const bytes = try encodedSample(arena.allocator());

    var counting: CountingAllocator = .{ .backing = arena.allocator() };
    const decoded = try ssz.decode(Record, counting.allocator(), bytes);
    try std.testing.expectEqual(@as(usize, 1), counting.allocations);
    try std.testing.expectEqual(@as(usize, 3), decoded.blobs.items.len);
}

test "decoding an empty variable list allocates nothing" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const bytes = try ssz.encode(Record, arena.allocator(), emptyRecord());

    var counting: CountingAllocator = .{ .backing = arena.allocator() };
    _ = try ssz.decode(Record, counting.allocator(), bytes);
    try std.testing.expectEqual(@as(usize, 0), counting.allocations);
}

test "an integer list reads the same from a misaligned buffer" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const bytes = try ssz.encode(Counts, alloc, .{ .id = 9, .counts = .{ .items = &sample_counts } });

    const shifted = try alloc.alloc(u8, bytes.len + 1);
    @memcpy(shifted[1..], bytes);
    const decoded = try ssz.decode(Counts, alloc, shifted[1..]);
    try std.testing.expectEqual(@as(usize, sample_counts.len), decoded.counts.items.len);
    for (sample_counts, decoded.counts.items) |want, got| try std.testing.expectEqual(want, got);
}

test "encoding allocates its buffer once and never grows it" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    var counting: CountingAllocator = .{ .backing = arena.allocator() };
    const alloc = counting.allocator();

    _ = try ssz.encode(Record, alloc, sampleRecord());
    try std.testing.expectEqual(@as(usize, 1), counting.allocations);
    try std.testing.expectEqual(@as(usize, 0), counting.resizes);

    _ = try ssz.encodeFramed(Record, alloc, 0x0102, sampleRecord());
    try std.testing.expectEqual(@as(usize, 2), counting.allocations);
    try std.testing.expectEqual(@as(usize, 0), counting.resizes);

    _ = try ssz.encodeFramedAfter(Record, alloc, 32, 0x0102, sampleRecord());
    try std.testing.expectEqual(@as(usize, 3), counting.allocations);
    try std.testing.expectEqual(@as(usize, 0), counting.resizes);
}

// ── Framing ───────────────────────────────────────────────────────────────────

test "a framed value is its schema id, big-endian, followed by its encoding" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const framed = try ssz.encodeFramed(Record, alloc, 0x1a2b, sampleRecord());
    try std.testing.expectEqualSlices(u8, &.{ 0x1a, 0x2b }, framed[0..2]);
    try std.testing.expectEqualSlices(u8, try ssz.encode(Record, alloc, sampleRecord()), framed[2..]);
    try expectRecordEqual(sampleRecord(), try ssz.decodeFramed(Record, alloc, framed, 0x1a2b));
}

test "a frame encoded behind a head leaves the head to the caller" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const framed = try ssz.encodeFramed(Record, alloc, 0x0102, sampleRecord());
    const behind = try ssz.encodeFramedAfter(Record, alloc, 5, 0x0102, sampleRecord());
    try std.testing.expectEqualSlices(u8, framed, behind[5..]);
}

test "a frame without its schema id is malformed" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const framed = try ssz.encodeFramed(Record, alloc, 0x1a2b, sampleRecord());
    try std.testing.expectError(error.MalformedFrame, ssz.decodeFramed(Record, alloc, &.{}, 0x1a2b));
    try std.testing.expectError(error.MalformedFrame, ssz.decodeFramed(Record, alloc, framed[0..1], 0x1a2b));
    try std.testing.expectError(error.MalformedFrame, ssz.decodeFramed(Record, alloc, framed, 0x1a2c));
}

test "a frame under its schema id with a body that is not canonical is invalid" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const framed = try ssz.encodeFramed(Record, alloc, 0x1a2b, sampleRecord());
    try std.testing.expectError(error.InvalidSsz, ssz.decodeFramed(Record, alloc, framed[0 .. 2 + record_fixed_size - 1], 0x1a2b));
    framed[2 + flag_pos] = 2;
    try std.testing.expectError(error.InvalidSsz, ssz.decodeFramed(Record, alloc, framed, 0x1a2b));
}

test "encoding a framed value rejects a list beyond its bound" {
    var too_long = sampleRecord();
    too_long.tag = .{ .bytes = "123456789" };
    try std.testing.expectError(error.BoundsViolation, ssz.encodeFramed(Record, std.testing.allocator, 1, too_long));
}
