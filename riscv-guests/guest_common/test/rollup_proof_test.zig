//! Tests for the shared rollup proof codec: the output frame, round-trips of the public-input
//! tuple and the encoder's bounds. The SSZ rules themselves are tested once in `ssz_test.zig`.

const std = @import("std");

const rollup_proof = @import("core_serde").rollup_proof;

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

const sample_messages = [_][32]u8{ repeat32(0x08), repeat32(0x18) };
const sample_filtered = [_][20]u8{ repeat20(0x09), repeat20(0x19) };
const sample_vks = [_][32]u8{ repeat32(0xa1), repeat32(0xa2) };
const sample_offsets = [_]u16{ 1, 3 };

fn samplePublicInputs() rollup_proof.RollupPublicInput {
    return .{
        .end_block_number = 1000503,
        .end_block_timestamp = 1763000123,
        .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
        .end_l1_l2_bridge_rolling_hash_message_number = 5,
        .dynamic_chain_config_hash = repeat32(0xc0),
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_ftx_number = 16,
        .end_ftx_rolling_hash = repeat32(0x05),
        .end_processed_ftx_number = 18,
        .parent_data_rolling_hash = repeat32(0x47),
        .end_data_rolling_hash = repeat32(0x1f),
        .parent_block_hash = repeat32(0x0a),
        .end_block_hash = repeat32(0x0b),
        .parent_data_tail_take_bytes = 4096,
        .final_data_tail_discard_bytes = 33,
        .l2_l1_messages = .{ .items = &sample_messages },
        .filtered_addresses = .{ .items = &sample_filtered },
        .program_vks = .{ .items = &sample_vks },
        .block_count = 3,
        .l2_messaging_blocks_offsets = .{ .items = &sample_offsets },
    };
}

fn withEmptyLists(pi: rollup_proof.RollupPublicInput) rollup_proof.RollupPublicInput {
    var empty = pi;
    empty.l2_l1_messages = .{ .items = &.{} };
    empty.filtered_addresses = .{ .items = &.{} };
    empty.program_vks = .{ .items = &.{} };
    empty.l2_messaging_blocks_offsets = .{ .items = &.{} };
    return empty;
}

fn expectPublicInputsEqual(want: rollup_proof.RollupPublicInput, got: rollup_proof.RollupPublicInput) !void {
    inline for (std.meta.fields(rollup_proof.RollupPublicInput)) |field| {
        const want_field = @field(want, field.name);
        const got_field = @field(got, field.name);
        switch (@typeInfo(field.type)) {
            .@"struct" => {
                try std.testing.expectEqual(want_field.items.len, got_field.items.len);
                for (want_field.items, got_field.items) |w, g| {
                    if (@typeInfo(@TypeOf(w)) == .array) {
                        try std.testing.expectEqualSlices(u8, &w, &g);
                    } else {
                        try std.testing.expectEqual(w, g);
                    }
                }
            },
            .array => try std.testing.expectEqualSlices(u8, &want_field, &got_field),
            else => try std.testing.expectEqual(want_field, got_field),
        }
    }
}

fn keccak256(bytes: []const u8) [32]u8 {
    var digest: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(bytes, &digest, .{});
    return digest;
}

test "public inputs decode to what was encoded, including empty lists" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    for ([_]rollup_proof.RollupPublicInput{ samplePublicInputs(), withEmptyLists(samplePublicInputs()) }) |pi| {
        const bytes = try rollup_proof.encodePublicInputs(alloc, pi);
        try expectPublicInputsEqual(pi, try rollup_proof.decodePublicInputs(alloc, bytes));
    }
}

test "output is the commitment to the 0x1801 frame followed by the frame carrying the public inputs" {
    const alloc = std.testing.allocator;
    const pi = samplePublicInputs();
    const out = try rollup_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    const body = try rollup_proof.encodePublicInputs(alloc, pi);
    defer alloc.free(body);

    try std.testing.expectEqual(32 + 2 + body.len, out.len);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x18, 0x01 }, out[32..34]);
    try std.testing.expectEqualSlices(u8, body, out[34..]);
    try std.testing.expectEqualSlices(u8, &keccak256(out[32..]), out[0..32]);
}

test "output decodes back to the public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const out = try rollup_proof.encodeOutput(alloc, samplePublicInputs());
    try expectPublicInputsEqual(samplePublicInputs(), try rollup_proof.decodeOutput(alloc, out));
}

test "hashPublicInputs is the commitment at the head of the output" {
    const alloc = std.testing.allocator;
    const pi = samplePublicInputs();
    const out = try rollup_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    const hash = try rollup_proof.hashPublicInputs(alloc, pi);
    try std.testing.expectEqualSlices(u8, out[0..32], &hash);
}

test "hashPublicInputs commits to every field of the public inputs" {
    const alloc = std.testing.allocator;
    const hash = try rollup_proof.hashPublicInputs(alloc, samplePublicInputs());

    inline for (std.meta.fields(rollup_proof.RollupPublicInput)) |field| {
        var changed = samplePublicInputs();
        switch (@typeInfo(field.type)) {
            .int => @field(changed, field.name) += 1,
            .array => @field(changed, field.name)[0] ^= 1,
            .@"struct" => @field(changed, field.name).items = @field(changed, field.name).items[1..],
            else => @compileError("unhandled public-input field type"),
        }
        const changed_hash = try rollup_proof.hashPublicInputs(alloc, changed);
        try std.testing.expect(!std.mem.eql(u8, &hash, &changed_hash));
    }
}

fn expectDecodeOutputError(want: anyerror, bytes: []const u8) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(want, rollup_proof.decodeOutput(arena.allocator(), bytes));
}

// The output of `body` under the schema id, with the commitment of that frame.
fn outputOf(alloc: std.mem.Allocator, schema_id: u16, body: []const u8) ![]u8 {
    const out = try alloc.alloc(u8, 32 + 2 + body.len);
    std.mem.writeInt(u16, out[32..34], schema_id, .big);
    @memcpy(out[34..], body);
    out[0..32].* = keccak256(out[32..]);
    return out;
}

test "output: rejects an output shorter than the commitment and the schema id" {
    const alloc = std.testing.allocator;
    const out = try rollup_proof.encodeOutput(alloc, samplePublicInputs());
    defer alloc.free(out);
    try expectDecodeOutputError(error.MalformedFrame, &.{});
    try expectDecodeOutputError(error.MalformedFrame, out[0..33]);
}

test "output: rejects a commitment that is not the keccak256 of the frame" {
    const alloc = std.testing.allocator;
    const out = try rollup_proof.encodeOutput(alloc, samplePublicInputs());
    defer alloc.free(out);

    out[0] ^= 1;
    try expectDecodeOutputError(error.MalformedFrame, out);
    out[0] ^= 1;

    out[out.len - 1] ^= 1;
    try expectDecodeOutputError(error.MalformedFrame, out);
}

test "output: rejects a committed frame under another schema id" {
    const alloc = std.testing.allocator;
    const body = try rollup_proof.encodePublicInputs(alloc, samplePublicInputs());
    defer alloc.free(body);
    const l2_execution_output_schema_id = 0x0003;
    const out = try outputOf(alloc, l2_execution_output_schema_id, body);
    defer alloc.free(out);
    try expectDecodeOutputError(error.MalformedFrame, out);
}

test "output: rejects a committed frame whose body is not the canonical encoding" {
    const alloc = std.testing.allocator;
    const body = try rollup_proof.encodePublicInputs(alloc, samplePublicInputs());
    defer alloc.free(body);
    const out = try outputOf(alloc, rollup_proof.OUTPUT_SCHEMA_ID, body[0 .. body.len - 1]);
    defer alloc.free(out);
    try expectDecodeOutputError(error.InvalidSsz, out);
}

test "public inputs: encoder rejects a list beyond its bound" {
    const alloc = std.testing.allocator;
    const addresses = try alloc.alloc([20]u8, rollup_proof.MAX_FILTERED_ADDRESSES + 1);
    defer alloc.free(addresses);
    @memset(addresses, repeat20(0));
    var too_many = samplePublicInputs();
    too_many.filtered_addresses = .{ .items = addresses };
    try std.testing.expectError(error.BoundsViolation, rollup_proof.encodePublicInputs(alloc, too_many));
}
