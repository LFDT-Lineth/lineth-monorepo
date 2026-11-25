const std = @import("std");

const c = @import("lineth_da_h");
const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const host = @import("host");
const host_streams = @import("host_streams");
const lineth_da = @import("lineth_da");

const c_api = host.c_api;
const compressor = host.compressor;
const stream = host.stream;
const Kind = ef_conflations.Kind;
const Status = c_api.Status;

const gpa = std.testing.allocator;
const dictionary = lineth_da.dictionary_v2025_04_21;

// ---- the header and the exports agree ----------------------------------------------------------

fn expectSameLayout(comptime C: type, comptime Z: type) !void {
    try std.testing.expectEqual(@sizeOf(C), @sizeOf(Z));
    try std.testing.expectEqual(@alignOf(C), @alignOf(Z));
    const c_fields = @typeInfo(C).@"struct".fields;
    const z_fields = @typeInfo(Z).@"struct".fields;
    try std.testing.expectEqual(c_fields.len, z_fields.len);
    inline for (c_fields, z_fields) |c_field, z_field| {
        try std.testing.expectEqualStrings(c_field.name, z_field.name);
        try std.testing.expectEqual(@offsetOf(C, c_field.name), @offsetOf(Z, z_field.name));
        try std.testing.expectEqual(@sizeOf(c_field.type), @sizeOf(z_field.type));
    }
}

test "the header and the exports declare their structs with one layout" {
    try expectSameLayout(c.lineth_da_bytes, c_api.Bytes);
    try expectSameLayout(c.lineth_da_compressed, c_api.Compressed);
    try expectSameLayout(c.lineth_da_segment_end, stream.SegmentEnd);
    try expectSameLayout(c.lineth_da_sealed_blob, stream.SealedBlob);
    try expectSameLayout(c.lineth_da_sealed_blobs, c_api.SealedBlobs);
    try expectSameLayout(c.lineth_da_stream_position, stream.Position);
    try std.testing.expectEqual(@sizeOf(c.lineth_da_status), @sizeOf(Status));
}

test "the header and the library agree on the constants and the statuses" {
    try std.testing.expectEqual(@as(u32, c.LINETH_DA_ABI_VERSION), c_api.lineth_da_abi_version());
    try std.testing.expectEqual(stream.blob_len, c.LINETH_DA_BLOB_BYTES);
    try std.testing.expectEqual(48, c.LINETH_DA_KZG_POINT_BYTES);
    const statuses = .{
        .{ c.LINETH_DA_OK, Status.ok },
        .{ c.LINETH_DA_ERR_INVALID_ARGUMENT, Status.invalid_argument },
        .{ c.LINETH_DA_ERR_INVALID_DICTIONARY, Status.invalid_dictionary },
        .{ c.LINETH_DA_ERR_INVALID_BLOCK_RLP, Status.invalid_block_rlp },
        .{ c.LINETH_DA_ERR_SEEDED_BLOB_MISMATCH, Status.seeded_blob_mismatch },
        .{ c.LINETH_DA_ERR_OUT_OF_MEMORY, Status.out_of_memory },
        .{ c.LINETH_DA_ERR_INTERNAL, Status.internal },
    };
    inline for (statuses) |pair| try std.testing.expectEqual(pair[0], @intFromEnum(pair[1]));
}

fn expectSameSignature(comptime name: []const u8) !void {
    const c_fn = @typeInfo(@TypeOf(@field(c, name))).@"fn";
    const z_fn = @typeInfo(@TypeOf(@field(c_api, name))).@"fn";
    try std.testing.expectEqual(c_fn.params.len, z_fn.params.len);
    inline for (c_fn.params, z_fn.params) |c_param, z_param| {
        try std.testing.expectEqual(@sizeOf(c_param.type.?), @sizeOf(z_param.type.?));
    }
    try std.testing.expectEqual(@sizeOf(c_fn.return_type orelse void), @sizeOf(z_fn.return_type orelse void));
}

fn isFunction(comptime T: type, comptime name: []const u8) bool {
    return std.mem.startsWith(u8, name, "lineth_da_") and @typeInfo(@TypeOf(@field(T, name))) == .@"fn";
}

test "the header declares every function the library exports, with its signature" {
    @setEvalBranchQuota(1_000_000);
    var declared: usize = 0;
    inline for (@typeInfo(c).@"struct".decls) |decl| {
        if (comptime isFunction(c, decl.name)) {
            try expectSameSignature(decl.name);
            declared += 1;
        }
    }
    var exported: usize = 0;
    inline for (@typeInfo(c_api).@"struct".decls) |decl| {
        if (comptime isFunction(c_api, decl.name)) {
            try std.testing.expect(@hasDecl(c, decl.name));
            exported += 1;
        }
    }
    try std.testing.expectEqual(10, declared);
    try std.testing.expectEqual(declared, exported);
}

// ---- the calls --------------------------------------------------------------------------------

var message: [256]u8 = undefined;

fn newCompressor(chain_id: u64) !*c_api.Compressor {
    var handle: ?*c_api.Compressor = null;
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_compressor_new(dictionary.ptr, dictionary.len, chain_id, &handle, &message, message.len));
    return handle.?;
}

fn expectMessage(expected: []const u8) !void {
    try std.testing.expectEqualStrings(expected, std.mem.sliceTo(&message, 0));
}

fn selected() !*const ef_conflations.Selection {
    return ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
}

fn compress(handle: *const c_api.Compressor, rlps: []const []const u8, out: *c_api.Compressed) !Status {
    const inputs = try gpa.alloc(c_api.Bytes, rlps.len);
    defer gpa.free(inputs);
    for (rlps, inputs) |rlp, *input| input.* = .{ .ptr = rlp.ptr, .len = rlp.len };
    return c_api.lineth_da_compress(handle, inputs.ptr, inputs.len, out, &message, message.len);
}

fn frameOf(compressed: c_api.Compressed) []const u8 {
    return compressed.frame.?[0..compressed.frame_len];
}

fn openStream(position: *const stream.Position) !*c_api.Stream {
    var handle: ?*c_api.Stream = null;
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_open(position, &handle, &message, message.len));
    return handle.?;
}

fn appendFrame(handle: *c_api.Stream, frame: []const u8, end_block: u64, out: *c_api.SealedBlobs) Status {
    return c_api.lineth_da_stream_append(handle, frame.ptr, frame.len, end_block, out, &message, message.len);
}

fn expectNoBlobs(sealed: c_api.SealedBlobs) !void {
    try std.testing.expectEqual(0, sealed.count);
    try std.testing.expectEqual(null, sealed.blobs);
}

/// The blobs of a stream opened at `position` through the ABI that appends `conflations` and
/// flushes, which `host_streams.release` releases.
fn abiRun(position: *const stream.Position, conflations: []const host_streams.Conflation) !std.ArrayList(stream.SealedBlob) {
    const handle = try openStream(position);
    defer c_api.lineth_da_stream_close(handle);
    var blobs: std.ArrayList(stream.SealedBlob) = .empty;
    errdefer host_streams.release(&blobs);
    for (0..conflations.len + 1) |i| {
        var sealed: c_api.SealedBlobs = undefined;
        const status = if (i < conflations.len)
            appendFrame(handle, conflations[i].frame, conflations[i].end_block, &sealed)
        else
            c_api.lineth_da_stream_flush(handle, &sealed, &message, message.len);
        defer c_api.lineth_da_sealed_blobs_free(&sealed);
        try std.testing.expectEqual(Status.ok, status);
        for ((sealed.blobs orelse continue)[0..sealed.count]) |blob| {
            const ends = try gpa.dupe(stream.SegmentEnd, blob.segmentEnds());
            errdefer gpa.free(ends);
            var copy = blob;
            copy.segment_ends = ends.ptr;
            try blobs.append(gpa, copy);
        }
    }
    return blobs;
}

fn expectSameBlobs(expected: []const stream.SealedBlob, actual: []const stream.SealedBlob) !void {
    try std.testing.expectEqual(expected.len, actual.len);
    for (expected, actual) |*blob, *other| try host_streams.expectSameBlob(blob, other);
}

test "compress makes the frame of the Zig compressor" {
    const selection = try selected();
    const rlps = try selection.conflationRlps(gpa, &.{ .legacy, .fee_market, .access_list });
    defer gpa.free(rlps);
    const handle = try newCompressor(selection.chain_id);
    defer c_api.lineth_da_compressor_free(handle);
    const reference = try compressor.Compressor.init(gpa, dictionary, selection.chain_id);
    defer reference.deinit();

    var out: c_api.Compressed = undefined;
    try std.testing.expectEqual(Status.ok, try compress(handle, rlps, &out));
    defer c_api.lineth_da_compressed_free(&out);

    const expected = try reference.compress(rlps, null);
    defer gpa.free(expected);
    try std.testing.expectEqualSlices(u8, expected, frameOf(out));
}

test "a stream through the ABI seals and resumes like the Zig stream" {
    const conflations = try host_streams.syntheticConflations(gpa, &.{ 60_000, 90_000, 40_000 });
    defer host_streams.freeSynthetic(gpa, conflations);
    const fresh: stream.Position = .fresh(host_streams.genesis);
    var expected = try host_streams.run(&fresh, conflations);
    defer host_streams.release(&expected);
    var actual = try abiRun(&fresh, conflations);
    defer host_streams.release(&actual);
    try std.testing.expectEqual(2, expected.items.len);
    try expectSameBlobs(expected.items, actual.items);

    const cut = host_streams.locate(expected.items, 2);
    const position = host_streams.positionAfter(expected.items, cut);
    var expected_resumed = try host_streams.run(&position, conflations[2..]);
    defer host_streams.release(&expected_resumed);
    var actual_resumed = try abiRun(&position, conflations[2..]);
    defer host_streams.release(&actual_resumed);
    try expectSameBlobs(expected_resumed.items, actual_resumed.items);
}

const Rejection = struct {
    status: Status,
    message: []const u8,
    call: union(enum) {
        compress: struct { chain_id: u64 = 1, blocks: []const []const u8, count: ?usize = null },
        new_compressor: []const u8,
        open: stream.Position,
        append: struct { position: stream.Position, frame: []const u8 },
    },
};

/// The call returns the status and the message of `rejection` and leaves its result empty.
fn expectRejected(rejection: Rejection) !void {
    switch (rejection.call) {
        .compress => |call| {
            const handle = try newCompressor(call.chain_id);
            defer c_api.lineth_da_compressor_free(handle);
            const inputs = try gpa.alloc(c_api.Bytes, call.blocks.len);
            defer gpa.free(inputs);
            for (call.blocks, inputs) |block, *input| input.* = .{ .ptr = block.ptr, .len = block.len };
            var out: c_api.Compressed = .{ .frame = @ptrFromInt(8), .frame_len = 99 };
            try std.testing.expectEqual(rejection.status, c_api.lineth_da_compress(handle, inputs.ptr, call.count orelse inputs.len, &out, &message, message.len));
            try std.testing.expectEqual(null, out.frame);
            try std.testing.expectEqual(0, out.frame_len);
        },
        .new_compressor => |bytes| {
            var handle: ?*c_api.Compressor = @ptrFromInt(8);
            try std.testing.expectEqual(rejection.status, c_api.lineth_da_compressor_new(bytes.ptr, bytes.len, 1, &handle, &message, message.len));
            try std.testing.expectEqual(null, handle);
        },
        .open => |position| {
            var handle: ?*c_api.Stream = @ptrFromInt(8);
            try std.testing.expectEqual(rejection.status, c_api.lineth_da_stream_open(&position, &handle, &message, message.len));
            try std.testing.expectEqual(null, handle);
        },
        .append => |call| {
            const handle = try openStream(&call.position);
            defer c_api.lineth_da_stream_close(handle);
            var out: c_api.SealedBlobs = .{ .blobs = @ptrFromInt(@alignOf(stream.SealedBlob)), .count = 99 };
            try std.testing.expectEqual(rejection.status, appendFrame(handle, call.frame, 2, &out));
            try expectNoBlobs(out);
        },
    }
    try expectMessage(rejection.message);
}

test "a failed call returns its status, writes its message and leaves its result empty" {
    const selection = try selected();
    const legacy = try selection.conflationRlps(gpa, &.{.legacy});
    defer gpa.free(legacy);
    const synthetic = try host_streams.syntheticConflations(gpa, &.{ 2_000, 3_000 });
    defer host_streams.freeSynthetic(gpa, synthetic);
    var blobs = try host_streams.run(&.fresh(host_streams.genesis), synthetic);
    defer host_streams.release(&blobs);
    const garbage = [_]u8{ 0xc0, 0x01 };
    const not_a_dictionary: [4096]u8 = @splat('x');
    const unexplained_blob: [stream.blob_len]u8 = @splat(0xff);

    const rejections = [_]Rejection{
        .{ .status = .invalid_argument, .message = "no blocks", .call = .{ .compress = .{ .blocks = &.{} } } },
        .{ .status = .invalid_argument, .message = "the shared blob is not the canonical packing of a payload", .call = .{ .open = .{
            .data_rolling_hash = @splat(1),
            .parent_blob_data_rolling_hash = @splat(2),
            .shared_blob_carry_over_tail_bytes = 5,
            .shared_blob = &unexplained_blob,
        } } },
        .{ .status = .invalid_dictionary, .message = "not a zstd dictionary that declares an ID", .call = .{ .new_compressor = &not_a_dictionary } },
        .{ .status = .invalid_block_rlp, .message = "block 1 is malformed", .call = .{ .compress = .{
            .chain_id = selection.chain_id,
            .blocks = &.{ legacy[0], &garbage },
        } } },
        .{ .status = .invalid_block_rlp, .message = "a sender of block 0 cannot be recovered", .call = .{ .compress = .{
            .chain_id = 0,
            .blocks = legacy,
        } } },
        .{ .status = .seeded_blob_mismatch, .message = "the appended bytes differ from the shared blob", .call = .{ .append = .{
            .position = host_streams.positionAfter(blobs.items, host_streams.locate(blobs.items, 1)),
            .frame = synthetic[0].frame,
        } } },
        // A count whose entries cannot be held is an allocation that fails before an entry is read.
        .{ .status = .out_of_memory, .message = "out of memory", .call = .{ .compress = .{
            .blocks = &.{&garbage},
            .count = std.math.maxInt(usize) / 16,
        } } },
    };
    for (rejections) |rejection| try expectRejected(rejection);
}

test "a call that lacks a required pointer returns invalid argument" {
    const handle = try newCompressor(1);
    defer c_api.lineth_da_compressor_free(handle);
    var compressed: c_api.Compressed = undefined;
    var sealed: c_api.SealedBlobs = undefined;
    var opened: ?*c_api.Stream = null;
    var unused: ?*c_api.Compressor = null;
    const garbage = [_]u8{ 0xc0, 0x01 };
    const input: c_api.Bytes = .{ .ptr = &garbage, .len = garbage.len };
    const position: stream.Position = .fresh(@splat(1));
    const frame = [_]u8{ 1, 2, 3 };
    const missing_block: [1]c_api.Bytes = .{.{ .ptr = null, .len = 5 }};

    const statuses = [_]Status{
        c_api.lineth_da_compressor_new(null, 5, 1, &unused, &message, message.len),
        c_api.lineth_da_compressor_new(dictionary.ptr, dictionary.len, 1, null, &message, message.len),
        c_api.lineth_da_compress(null, &.{input}, 1, &compressed, &message, message.len),
        c_api.lineth_da_compress(handle, &.{input}, 1, null, &message, message.len),
        c_api.lineth_da_compress(handle, null, 1, &compressed, &message, message.len),
        c_api.lineth_da_compress(handle, &missing_block, 1, &compressed, &message, message.len),
        c_api.lineth_da_stream_open(null, &opened, &message, message.len),
        c_api.lineth_da_stream_open(&position, null, &message, message.len),
        c_api.lineth_da_stream_append(null, &frame, frame.len, 1, &sealed, &message, message.len),
        c_api.lineth_da_stream_flush(null, &sealed, &message, message.len),
    };
    for (statuses) |status| try std.testing.expectEqual(Status.invalid_argument, status);
    try std.testing.expectEqual(Status.invalid_dictionary, c_api.lineth_da_compressor_new(null, 0, 1, &unused, &message, message.len));

    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_open(&position, &opened, &message, message.len));
    defer c_api.lineth_da_stream_close(opened);
    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_stream_append(opened, null, frame.len, 1, &sealed, &message, message.len));
    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_stream_append(opened, null, 0, 1, &sealed, &message, message.len));
    try expectMessage("the frame is empty");
    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_stream_append(opened, &frame, frame.len, 1, null, &message, message.len));
    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_stream_flush(opened, null, &message, message.len));
}

test "a message is cut to the buffer, ends with a NUL, and is emptied by a success" {
    const handle = try newCompressor(1);
    defer c_api.lineth_da_compressor_free(handle);
    var out: c_api.Compressed = undefined;
    const garbage = [_]u8{ 0xc0, 0x01 };
    const inputs = [_]c_api.Bytes{.{ .ptr = &garbage, .len = garbage.len }};

    var small: [8]u8 = @splat(0xaa);
    try std.testing.expectEqual(Status.invalid_block_rlp, c_api.lineth_da_compress(handle, &inputs, 1, &out, &small, small.len));
    try std.testing.expectEqualSlices(u8, "block 0\x00", &small);

    var one: [1]u8 = .{0xaa};
    try std.testing.expectEqual(Status.invalid_block_rlp, c_api.lineth_da_compress(handle, &inputs, 1, &out, &one, one.len));
    try std.testing.expectEqual(0, one[0]);

    var untouched: [4]u8 = @splat(0xaa);
    try std.testing.expectEqual(Status.invalid_block_rlp, c_api.lineth_da_compress(handle, &inputs, 1, &out, &untouched, 0));
    try std.testing.expectEqualSlices(u8, &@as([4]u8, @splat(0xaa)), &untouched);

    try std.testing.expectEqual(Status.invalid_block_rlp, c_api.lineth_da_compress(handle, &inputs, 1, &out, null, 100));

    const opened = try openStream(&.fresh(host_streams.genesis));
    defer c_api.lineth_da_stream_close(opened);
    const frame = [_]u8{ 1, 2, 3 };
    var sealed: c_api.SealedBlobs = undefined;
    var stale: [16]u8 = @splat('x');
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_append(opened, &frame, frame.len, 1, &sealed, &stale, stale.len));
    try std.testing.expectEqual(0, stale[0]);
}

test "a free clears its result, and freeing nothing does nothing" {
    c_api.lineth_da_compressor_free(null);
    c_api.lineth_da_stream_close(null);
    c_api.lineth_da_compressed_free(null);
    c_api.lineth_da_sealed_blobs_free(null);

    var compressed: c_api.Compressed = std.mem.zeroes(c_api.Compressed);
    c_api.lineth_da_compressed_free(&compressed);
    var sealed: c_api.SealedBlobs = .{ .blobs = null, .count = 0 };
    c_api.lineth_da_sealed_blobs_free(&sealed);

    const selection = try selected();
    const legacy = try selection.conflationRlps(gpa, &.{.legacy});
    defer gpa.free(legacy);
    const handle = try newCompressor(selection.chain_id);
    defer c_api.lineth_da_compressor_free(handle);
    try std.testing.expectEqual(Status.ok, try compress(handle, legacy, &compressed));
    c_api.lineth_da_compressed_free(&compressed);
    try std.testing.expectEqual(null, compressed.frame);
    try std.testing.expectEqual(0, compressed.frame_len);
    c_api.lineth_da_compressed_free(&compressed);

    const opened = try openStream(&.fresh(host_streams.genesis));
    defer c_api.lineth_da_stream_close(opened);
    try std.testing.expectEqual(Status.ok, appendFrame(opened, &.{ 1, 2, 3 }, 1, &sealed));
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_flush(opened, &sealed, &message, message.len));
    try std.testing.expectEqual(1, sealed.count);
    c_api.lineth_da_sealed_blobs_free(&sealed);
    try expectNoBlobs(sealed);
    c_api.lineth_da_sealed_blobs_free(&sealed);
}

test "compress makes a frame or returns invalid block RLP for every mutated block, and never aborts" {
    const selection = try selected();
    const handle = try newCompressor(selection.chain_id);
    defer c_api.lineth_da_compressor_free(handle);
    var prng = std.Random.DefaultPrng.init(0x11ea_da);
    const random = prng.random();

    var accepted: usize = 0;
    var rejected: usize = 0;
    const kinds = [_]Kind{ .legacy, .fee_market, .access_list, .set_code };
    for (0..400) |round| {
        const block = try gpa.dupe(u8, selection.entries.get(kinds[round % kinds.len]).rlp);
        defer gpa.free(block);
        var rlp: []const u8 = block;
        switch (random.uintLessThan(u8, 5)) {
            0 => block[random.uintLessThan(usize, block.len)] ^= @as(u8, 1) << random.int(u3),
            1 => for (0..1 + random.uintLessThan(usize, 8)) |_| {
                block[random.uintLessThan(usize, block.len)] = random.int(u8);
            },
            2 => rlp = block[0..random.uintLessThan(usize, block.len)],
            3 => @memset(block[random.uintLessThan(usize, block.len)..][0..1], 0),
            else => block[random.uintLessThan(usize, 16)] = random.int(u8),
        }

        var out: c_api.Compressed = undefined;
        switch (try compress(handle, &.{rlp}, &out)) {
            .ok => {
                accepted += 1;
                c_api.lineth_da_compressed_free(&out);
            },
            .invalid_block_rlp => rejected += 1,
            else => |status| {
                std.debug.print("round {d}: {s}: {s}\n", .{ round, @tagName(status), std.mem.sliceTo(&message, 0) });
                return error.UnexpectedStatus;
            },
        }
    }
    try std.testing.expect(rejected > 0);
    try std.testing.expect(accepted > 0);
}
