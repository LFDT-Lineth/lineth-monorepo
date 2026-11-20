const std = @import("std");

const c = @import("lineth_da_h");
const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const host = @import("host");
const lineth_da = @import("lineth_da");

const blob_kzg = lineth_da.encoding.blob_kzg;
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

test "a failed call returns its status, writes its message and leaves its result empty" {
    const handle = try newCompressor(1);
    defer c_api.lineth_da_compressor_free(handle);
    var out: c_api.Compressed = undefined;

    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_compress(handle, null, 0, &out, &message, message.len));
    try expectMessage("no blocks");

    const garbage = [_]u8{ 0xc0, 0x01 };
    const inputs = [_]c_api.Bytes{ .{ .ptr = &garbage, .len = garbage.len }, .{ .ptr = &garbage, .len = garbage.len } };
    out.frame_len = 99;
    try std.testing.expectEqual(Status.invalid_block_rlp, c_api.lineth_da_compress(handle, &inputs, inputs.len, &out, &message, message.len));
    try expectMessage("block 0 is malformed");
    try std.testing.expectEqual(null, out.frame);
    try std.testing.expectEqual(0, out.frame_len);

    var not_a_dictionary: [4096]u8 = @splat('x');
    var failed: ?*c_api.Compressor = @ptrFromInt(8);
    try std.testing.expectEqual(Status.invalid_dictionary, c_api.lineth_da_compressor_new(&not_a_dictionary, not_a_dictionary.len, 1, &failed, &message, message.len));
    try std.testing.expectEqual(null, failed);

    // A count whose entries cannot be held is an allocation that fails before an entry is read.
    try std.testing.expectEqual(Status.out_of_memory, c_api.lineth_da_compress(handle, &inputs, std.math.maxInt(usize) / 16, &out, &message, message.len));
    try expectMessage("out of memory");
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

test "a message is cut to the buffer and always ends with a NUL" {
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
}

test "freeing a NULL or an empty result, and closing a NULL handle, do nothing" {
    c_api.lineth_da_compressor_free(null);
    c_api.lineth_da_stream_close(null);
    c_api.lineth_da_compressed_free(null);
    c_api.lineth_da_sealed_blobs_free(null);

    var compressed: c_api.Compressed = std.mem.zeroes(c_api.Compressed);
    c_api.lineth_da_compressed_free(&compressed);
    var sealed: c_api.SealedBlobs = .{ .blobs = null, .count = 0 };
    c_api.lineth_da_sealed_blobs_free(&sealed);
}

test "opening a stream at a position its shared blob does not explain returns invalid argument" {
    var shared: [stream.blob_len]u8 = @splat(0xff);
    const position: stream.Position = .{
        .data_rolling_hash = @splat(1),
        .parent_blob_data_rolling_hash = @splat(2),
        .shared_blob_carry_over_tail_bytes = 5,
        .shared_blob = &shared,
    };
    var handle: ?*c_api.Stream = @ptrFromInt(8);
    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_stream_open(&position, &handle, &message, message.len));
    try std.testing.expectEqual(null, handle);
    try expectMessage("the shared blob is not the canonical packing of a payload");
}

// ---- compress and the streams of the blocks through the ABI ------------------------------------

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
    try expectMessage("");

    const expected = try reference.compress(rlps, null);
    defer gpa.free(expected);
    try std.testing.expectEqualSlices(u8, expected, frameOf(out));
}

test "a compressor copies its dictionary, so the caller may overwrite it" {
    const selection = try selected();
    const rlps = try selection.conflationRlps(gpa, &.{.legacy});
    defer gpa.free(rlps);
    const copy = try gpa.dupe(u8, dictionary);
    var handle: ?*c_api.Compressor = null;
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_compressor_new(copy.ptr, copy.len, selection.chain_id, &handle, &message, message.len));
    defer c_api.lineth_da_compressor_free(handle);
    @memset(copy, 0);
    gpa.free(copy);

    var out: c_api.Compressed = undefined;
    try std.testing.expectEqual(Status.ok, try compress(handle.?, rlps, &out));
    defer c_api.lineth_da_compressed_free(&out);
    try std.testing.expect(out.frame_len > 0);
}

test "compress names a rejected block in its message and leaves the result empty" {
    const selection = try selected();
    const legacy = try selection.conflationRlps(gpa, &.{.legacy});
    defer gpa.free(legacy);
    const handle = try newCompressor(selection.chain_id);
    defer c_api.lineth_da_compressor_free(handle);
    var out: c_api.Compressed = undefined;

    out.frame_len = 99;
    const garbage = [_]u8{ 0xc0, 0x01 };
    try std.testing.expectEqual(Status.invalid_block_rlp, try compress(handle, &.{ legacy[0], &garbage }, &out));
    try expectMessage("block 1 is malformed");
    try std.testing.expectEqual(null, out.frame);
    try std.testing.expectEqual(0, out.frame_len);

    const no_sender = try newCompressor(0);
    defer c_api.lineth_da_compressor_free(no_sender);
    try std.testing.expectEqual(Status.invalid_block_rlp, try compress(no_sender, legacy, &out));
    try expectMessage("a sender of block 0 cannot be recovered");
}

test "a successful call clears the message, and freeing a result twice is harmless" {
    const selection = try selected();
    const rlps = try selection.conflationRlps(gpa, &.{.legacy});
    defer gpa.free(rlps);
    const handle = try newCompressor(selection.chain_id);
    defer c_api.lineth_da_compressor_free(handle);
    var out: c_api.Compressed = undefined;
    const inputs = [_]c_api.Bytes{.{ .ptr = rlps[0].ptr, .len = rlps[0].len }};

    var stale: [16]u8 = @splat('x');
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_compress(handle, &inputs, 1, &out, &stale, stale.len));
    try std.testing.expectEqual(0, stale[0]);

    c_api.lineth_da_compressed_free(&out);
    try std.testing.expectEqual(null, out.frame);
    try std.testing.expectEqual(0, out.frame_len);
    c_api.lineth_da_compressed_free(&out);
}

fn openStream(position: *const stream.Position) !*c_api.Stream {
    var handle: ?*c_api.Stream = null;
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_open(position, &handle, &message, message.len));
    return handle.?;
}

fn expectBlobsEqual(expected: []const stream.SealedBlob, actual: c_api.SealedBlobs) !void {
    try std.testing.expectEqual(expected.len, actual.count);
    for (expected, 0..) |*blob, i| {
        const other = &actual.blobs.?[i];
        try std.testing.expectEqualSlices(u8, &blob.blob, &other.blob);
        try std.testing.expectEqualSlices(u8, &blob.kzg_commitment, &other.kzg_commitment);
        try std.testing.expectEqualSlices(u8, &blob.kzg_proof, &other.kzg_proof);
        try std.testing.expectEqualSlices(u8, &blob.data_rolling_hash, &other.data_rolling_hash);
        try std.testing.expectEqual(blob.payload_bytes, other.payload_bytes);
        try std.testing.expectEqualDeep(blob.segmentEnds(), other.segmentEnds());
    }
}

test "a stream through the ABI seals what the Zig stream seals, and reopens inside a shared blob" {
    const selection = try selected();
    const compressor_handle = try newCompressor(selection.chain_id);
    defer c_api.lineth_da_compressor_free(compressor_handle);
    const legacy = try selection.conflationRlps(gpa, &.{.legacy});
    defer gpa.free(legacy);
    const fee_market = try selection.conflationRlps(gpa, &.{.fee_market});
    defer gpa.free(fee_market);
    var first: c_api.Compressed = undefined;
    var second: c_api.Compressed = undefined;
    try std.testing.expectEqual(Status.ok, try compress(compressor_handle, legacy, &first));
    defer c_api.lineth_da_compressed_free(&first);
    try std.testing.expectEqual(Status.ok, try compress(compressor_handle, fee_market, &second));
    defer c_api.lineth_da_compressed_free(&second);
    const frames = [_][]const u8{ frameOf(first), frameOf(second) };

    // The run through the ABI and the run of the Zig stream seal the same blob.
    const genesis: stream.Hash = @splat(0x42);
    var position: stream.Position = .fresh(genesis);
    const handle = try openStream(&position);
    defer c_api.lineth_da_stream_close(handle);
    var sealed: c_api.SealedBlobs = undefined;
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_append(handle, frames[0].ptr, frames[0].len, 1, &sealed, &message, message.len));
    try std.testing.expectEqual(0, sealed.count);
    try std.testing.expectEqual(null, sealed.blobs);
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_append(handle, frames[1].ptr, frames[1].len, 2, &sealed, &message, message.len));
    try std.testing.expectEqual(0, sealed.count);
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_flush(handle, &sealed, &message, message.len));
    defer c_api.lineth_da_sealed_blobs_free(&sealed);
    try std.testing.expectEqual(1, sealed.count);

    var zig_stream = try stream.Stream.open(gpa, &position);
    defer zig_stream.deinit();
    stream.freeSealed(gpa, try zig_stream.append(frames[0], 1));
    stream.freeSealed(gpa, try zig_stream.append(frames[1], 2));
    const expected = try zig_stream.flush();
    defer stream.freeSealed(gpa, expected);
    try expectBlobsEqual(expected, sealed);

    const blob = &sealed.blobs.?[0];
    try std.testing.expect(blob_kzg.verifyBatch(&.{blob.blob}, &.{blob.kzg_commitment}, &.{blob.kzg_proof}));
    try std.testing.expectEqual(2, blob.segment_end_count);

    // The stream copies the shared blob: the caller's bytes may change after the open.
    const shared = try gpa.create([stream.blob_len]u8);
    defer gpa.destroy(shared);
    shared.* = blob.blob;
    const resume_position: stream.Position = .{
        .data_rolling_hash = blob.data_rolling_hash,
        .parent_blob_data_rolling_hash = blob.prev_data_rolling_hash,
        .shared_blob_carry_over_tail_bytes = blob.segment_ends.?[0].unused_tail_bytes,
        .shared_blob = shared,
    };
    const resumed = try openStream(&resume_position);
    defer c_api.lineth_da_stream_close(resumed);
    @memset(shared, 0);

    var again: c_api.SealedBlobs = undefined;
    try std.testing.expectEqual(Status.invalid_argument, c_api.lineth_da_stream_flush(resumed, &again, &message, message.len));
    try std.testing.expectEqual(Status.seeded_blob_mismatch, c_api.lineth_da_stream_append(resumed, first.frame.?, first.frame_len, 1, &again, &message, message.len));
    try std.testing.expectEqual(0, again.count);
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_append(resumed, frames[1].ptr, frames[1].len, 2, &again, &message, message.len));
    try std.testing.expectEqual(Status.ok, c_api.lineth_da_stream_flush(resumed, &again, &message, message.len));
    defer c_api.lineth_da_sealed_blobs_free(&again);
    try std.testing.expectEqual(1, again.count);
    try std.testing.expectEqualSlices(u8, &blob.blob, &again.blobs.?[0].blob);
    try std.testing.expectEqualSlices(u8, &blob.kzg_proof, &again.blobs.?[0].kzg_proof);
    try std.testing.expectEqual(1, again.blobs.?[0].segment_end_count);
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
