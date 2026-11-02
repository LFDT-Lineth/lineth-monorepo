//! Tests for the shared rollup proof codec: the byte layout of the public-input tuple and its
//! output frame, round-trips through the public decoders, and rejection of every non-canonical
//! encoding.

const std = @import("std");

const rollup_proof = @import("core_serde").rollup_proof;

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

fn le32(value: u32) [4]u8 {
    var out: [4]u8 = undefined;
    std.mem.writeInt(u32, &out, value, .little);
    return out;
}

fn le64(value: u64) [8]u8 {
    var out: [8]u8 = undefined;
    std.mem.writeInt(u64, &out, value, .little);
    return out;
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
        .l2_l1_messages = &sample_messages,
        .filtered_addresses = &sample_filtered,
        .program_vks = &sample_vks,
        .block_count = 3,
        .l2_messaging_blocks_offsets = &sample_offsets,
    };
}

const head_size = 336;
const messages_offset_pos = 312;
const filtered_offset_pos = 316;
const vks_offset_pos = 320;
const block_count_pos = 324;
const offsets_offset_pos = 332;

const messages_start = head_size;
const filtered_start = messages_start + sample_messages.len * 32;
const vks_start = filtered_start + sample_filtered.len * 20;
const offsets_start = vks_start + sample_vks.len * 32;

// The fields of `samplePublicInputs` in wire order: the fixed-size fields inline, a 4-byte offset
// for each list, then the list tails.
const sample_encoding =
    le64(1000503) ++ le64(1763000123) ++ repeat32(0x03) ++ le64(5) ++
    repeat32(0xc0) ++ repeat32(0x04) ++ le64(16) ++ repeat32(0x05) ++ le64(18) ++
    repeat32(0x47) ++ repeat32(0x1f) ++ repeat32(0x0a) ++ repeat32(0x0b) ++
    le64(4096) ++ le64(33) ++
    le32(messages_start) ++ le32(filtered_start) ++ le32(vks_start) ++
    le64(3) ++ le32(offsets_start) ++
    repeat32(0x08) ++ repeat32(0x18) ++
    repeat20(0x09) ++ repeat20(0x19) ++
    repeat32(0xa1) ++ repeat32(0xa2) ++
    [_]u8{ 1, 0, 3, 0 };

fn expectPublicInputsEqual(want: rollup_proof.RollupPublicInput, got: rollup_proof.RollupPublicInput) !void {
    inline for (std.meta.fields(rollup_proof.RollupPublicInput)) |field| {
        const want_field = @field(want, field.name);
        const got_field = @field(got, field.name);
        switch (@typeInfo(field.type)) {
            .pointer => |pointer| try std.testing.expectEqualSlices(pointer.child, want_field, got_field),
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

// ── Public inputs: layout and round-trips ─────────────────────────────────────

test "the bounds are those of the wire schema" {
    try std.testing.expectEqual(@as(usize, 1 << 26), rollup_proof.MAX_L2_L1_MESSAGES);
    try std.testing.expectEqual(@as(usize, 1 << 16), rollup_proof.MAX_FILTERED_ADDRESSES);
    try std.testing.expectEqual(@as(usize, 1 << 10), rollup_proof.MAX_PROGRAM_VKS);
    try std.testing.expectEqual(@as(usize, 1 << 16), rollup_proof.MAX_MESSAGING_BLOCKS_OFFSETS);
}

test "public inputs: encoding is the fixed head in wire order followed by the list tails" {
    const alloc = std.testing.allocator;
    const encoded = try rollup_proof.encodePublicInputs(alloc, samplePublicInputs());
    defer alloc.free(encoded);
    try std.testing.expectEqualSlices(u8, &sample_encoding, encoded);
}

test "public inputs: decoding returns every field of the encoding" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const decoded = try rollup_proof.decodePublicInputs(arena.allocator(), &sample_encoding);
    try expectPublicInputsEqual(samplePublicInputs(), decoded);
}

test "public inputs: the message, address and verifying-key lists alias the encoded buffer" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    var buffer = sample_encoding;
    const decoded = try rollup_proof.decodePublicInputs(arena.allocator(), &buffer);
    try std.testing.expectEqual(@intFromPtr(&buffer[messages_start]), @intFromPtr(decoded.l2_l1_messages.ptr));
    try std.testing.expectEqual(@intFromPtr(&buffer[filtered_start]), @intFromPtr(decoded.filtered_addresses.ptr));
    try std.testing.expectEqual(@intFromPtr(&buffer[vks_start]), @intFromPtr(decoded.program_vks.ptr));
}

test "public inputs with empty lists are the fixed head and round-trip" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var pi = samplePublicInputs();
    pi.l2_l1_messages = &.{};
    pi.filtered_addresses = &.{};
    pi.program_vks = &.{};
    pi.l2_messaging_blocks_offsets = &.{};
    const encoded = try rollup_proof.encodePublicInputs(alloc, pi);
    try std.testing.expectEqual(@as(usize, head_size), encoded.len);
    try expectPublicInputsEqual(pi, try rollup_proof.decodePublicInputs(alloc, encoded));
}

test "public inputs: each list round-trips on its own" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const empty = rollup_proof.RollupPublicInput{
        .end_block_number = 0,
        .end_block_timestamp = 0,
        .end_l1_l2_bridge_rolling_hash = repeat32(0),
        .end_l1_l2_bridge_rolling_hash_message_number = 0,
        .dynamic_chain_config_hash = repeat32(0),
        .parent_ftx_rolling_hash = repeat32(0),
        .parent_ftx_number = 0,
        .end_ftx_rolling_hash = repeat32(0),
        .end_processed_ftx_number = 0,
        .parent_data_rolling_hash = repeat32(0),
        .end_data_rolling_hash = repeat32(0),
        .parent_block_hash = repeat32(0),
        .end_block_hash = repeat32(0),
        .parent_data_tail_take_bytes = 0,
        .final_data_tail_discard_bytes = 0,
        .l2_l1_messages = &.{},
        .filtered_addresses = &.{},
        .program_vks = &.{},
        .block_count = 0,
        .l2_messaging_blocks_offsets = &.{},
    };
    var only_messages = empty;
    only_messages.l2_l1_messages = &sample_messages;
    var only_filtered = empty;
    only_filtered.filtered_addresses = &sample_filtered;
    var only_vks = empty;
    only_vks.program_vks = &sample_vks;
    var only_offsets = empty;
    only_offsets.l2_messaging_blocks_offsets = &sample_offsets;

    for ([_]rollup_proof.RollupPublicInput{ only_messages, only_filtered, only_vks, only_offsets }) |pi| {
        try expectPublicInputsEqual(pi, try rollup_proof.decodePublicInputs(alloc, try rollup_proof.encodePublicInputs(alloc, pi)));
    }
}

// ── Output frame ──────────────────────────────────────────────────────────────

test "output is the commitment to the 0x1801 frame followed by the frame carrying the SSZ public inputs" {
    const alloc = std.testing.allocator;
    const out = try rollup_proof.encodeOutput(alloc, samplePublicInputs());
    defer alloc.free(out);

    try std.testing.expectEqual(32 + 2 + sample_encoding.len, out.len);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x18, 0x01 }, out[32..34]);
    try std.testing.expectEqualSlices(u8, &sample_encoding, out[34..]);
    try std.testing.expectEqualSlices(u8, &keccak256(out[32..]), out[0..32]);
}

test "output with empty lists is the fixed head plus framing" {
    const alloc = std.testing.allocator;
    var pi = samplePublicInputs();
    pi.l2_l1_messages = &.{};
    pi.filtered_addresses = &.{};
    pi.program_vks = &.{};
    pi.l2_messaging_blocks_offsets = &.{};
    const out = try rollup_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    try std.testing.expectEqual(@as(usize, 32 + 2 + head_size), out.len);
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
            .pointer => @field(changed, field.name) = @field(changed, field.name)[1..],
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
    const l2_execution_output_schema_id = 0x0003;
    const out = try outputOf(alloc, l2_execution_output_schema_id, &sample_encoding);
    defer alloc.free(out);
    try expectDecodeOutputError(error.MalformedFrame, out);
}

test "output: rejects a committed frame whose body is not the canonical encoding" {
    const alloc = std.testing.allocator;
    const out = try outputOf(alloc, rollup_proof.OUTPUT_SCHEMA_ID, sample_encoding[0 .. head_size - 1]);
    defer alloc.free(out);
    try expectDecodeOutputError(error.InvalidSsz, out);
}

// ── Public inputs: malformed encodings ────────────────────────────────────────

fn expectDecodeError(want: anyerror, bytes: []const u8) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(want, rollup_proof.decodePublicInputs(arena.allocator(), bytes));
}

fn sampleWithOffset(pos: usize, value: u32) [sample_encoding.len]u8 {
    var bytes = sample_encoding;
    std.mem.writeInt(u32, bytes[pos..][0..4], value, .little);
    return bytes;
}

test "public inputs: rejects every buffer shorter than the fixed head" {
    for (0..head_size) |len| try expectDecodeError(error.InvalidSsz, sample_encoding[0..len]);
}

test "public inputs: rejects a first list offset that is not the fixed-head size" {
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(messages_offset_pos, head_size + 4));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(messages_offset_pos, head_size - 4));
}

test "public inputs: rejects list offsets that run backwards" {
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(filtered_offset_pos, messages_start - 1));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(vks_offset_pos, filtered_start - 1));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(offsets_offset_pos, vks_start - 1));
}

test "public inputs: rejects a list offset past the end of the buffer" {
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(offsets_offset_pos, sample_encoding.len + 1));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(offsets_offset_pos, 0xFFFFFFFF));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(vks_offset_pos, sample_encoding.len + 1));
}

test "public inputs: rejects list regions that are not whole elements" {
    // One byte short of a whole message, address and verifying key.
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(filtered_offset_pos, messages_start + sample_messages.len * 32 - 1));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(vks_offset_pos, filtered_start + sample_filtered.len * 20 - 1));
    try expectDecodeError(error.InvalidSsz, &sampleWithOffset(offsets_offset_pos, vks_start + sample_vks.len * 32 - 1));

    // A stray byte appended to the u16 list, and one removed from it.
    const trailing = sample_encoding ++ [_]u8{0xEE};
    try expectDecodeError(error.InvalidSsz, &trailing);
    try expectDecodeError(error.InvalidSsz, sample_encoding[0 .. sample_encoding.len - 1]);
}

// Public inputs whose offsets declare tails of the given byte sizes, in a buffer holding the fixed
// head and as many tail bytes as `buffer` has room for: every tail byte is whatever `buffer` holds.
fn publicInputsWithTails(buffer: []u8, messages_len: usize, filtered_len: usize, vks_len: usize, offsets_len: usize) []u8 {
    const len = head_size + messages_len + filtered_len + vks_len + offsets_len;
    @memset(buffer[0..head_size], 0);
    std.mem.writeInt(u32, buffer[messages_offset_pos..][0..4], head_size, .little);
    std.mem.writeInt(u32, buffer[filtered_offset_pos..][0..4], @intCast(head_size + messages_len), .little);
    std.mem.writeInt(u32, buffer[vks_offset_pos..][0..4], @intCast(head_size + messages_len + filtered_len), .little);
    std.mem.writeInt(u32, buffer[offsets_offset_pos..][0..4], @intCast(head_size + messages_len + filtered_len + vks_len), .little);
    return buffer[0..len];
}

test "public inputs: accepts the filtered addresses, verifying keys and offsets at their bound and rejects one element more" {
    const alloc = std.testing.allocator;
    const filtered_max = rollup_proof.MAX_FILTERED_ADDRESSES;
    const vks_max = rollup_proof.MAX_PROGRAM_VKS;
    const offsets_max = rollup_proof.MAX_MESSAGING_BLOCKS_OFFSETS;

    const buffer = try alloc.alloc(u8, head_size + (filtered_max + 1) * 20 + (vks_max + 1) * 32 + (offsets_max + 1) * 2);
    defer alloc.free(buffer);
    @memset(buffer[head_size..], 0);

    var arena = std.heap.ArenaAllocator.init(alloc);
    defer arena.deinit();
    const full = try rollup_proof.decodePublicInputs(arena.allocator(), publicInputsWithTails(buffer, 0, filtered_max * 20, vks_max * 32, offsets_max * 2));
    try std.testing.expectEqual(filtered_max, full.filtered_addresses.len);
    try std.testing.expectEqual(vks_max, full.program_vks.len);
    try std.testing.expectEqual(offsets_max, full.l2_messaging_blocks_offsets.len);

    try expectDecodeError(error.BoundsViolation, publicInputsWithTails(buffer, 0, (filtered_max + 1) * 20, 0, 0));
    try expectDecodeError(error.BoundsViolation, publicInputsWithTails(buffer, 0, 0, (vks_max + 1) * 32, 0));
    try expectDecodeError(error.BoundsViolation, publicInputsWithTails(buffer, 0, 0, 0, (offsets_max + 1) * 2));
}

// Pages the operating system maps and nobody touches: the messages bound is 2^26 messages, 2 GiB of
// tail, and the codec reads only the lengths of such a list. A host that cannot map it skips the test.
fn mapUntouched(len: usize) ?[]u8 {
    const ptr = std.heap.page_allocator.rawAlloc(len, .@"1", @returnAddress()) orelse return null;
    return ptr[0..len];
}

fn unmap(bytes: []u8) void {
    std.heap.page_allocator.rawFree(bytes, .@"1", @returnAddress());
}

test "public inputs: accepts the messages at their bound and rejects one message more" {
    const max_messages = rollup_proof.MAX_L2_L1_MESSAGES;
    const buffer = mapUntouched(head_size + (max_messages + 1) * 32) orelse return error.SkipZigTest;
    defer unmap(buffer);

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const full = try rollup_proof.decodePublicInputs(arena.allocator(), publicInputsWithTails(buffer, max_messages * 32, 0, 0, 0));
    try std.testing.expectEqual(max_messages, full.l2_l1_messages.len);

    try expectDecodeError(error.BoundsViolation, publicInputsWithTails(buffer, (max_messages + 1) * 32, 0, 0, 0));
}

test "public inputs: encoder rejects lists beyond their bound" {
    const alloc = std.testing.allocator;

    const addresses = try alloc.alloc([20]u8, rollup_proof.MAX_FILTERED_ADDRESSES + 1);
    defer alloc.free(addresses);
    @memset(addresses, repeat20(0));
    var too_many_addresses = samplePublicInputs();
    too_many_addresses.filtered_addresses = addresses;
    try std.testing.expectError(error.BoundsViolation, rollup_proof.encodePublicInputs(alloc, too_many_addresses));

    const vks = try alloc.alloc([32]u8, rollup_proof.MAX_PROGRAM_VKS + 1);
    defer alloc.free(vks);
    @memset(vks, repeat32(0));
    var too_many_vks = samplePublicInputs();
    too_many_vks.program_vks = vks;
    try std.testing.expectError(error.BoundsViolation, rollup_proof.encodePublicInputs(alloc, too_many_vks));

    const offsets = try alloc.alloc(u16, rollup_proof.MAX_MESSAGING_BLOCKS_OFFSETS + 1);
    defer alloc.free(offsets);
    @memset(offsets, 0);
    var too_many_offsets = samplePublicInputs();
    too_many_offsets.l2_messaging_blocks_offsets = offsets;
    try std.testing.expectError(error.BoundsViolation, rollup_proof.encodePublicInputs(alloc, too_many_offsets));
}

test "public inputs: encoder rejects more messages than their bound" {
    const messages = mapUntouched((rollup_proof.MAX_L2_L1_MESSAGES + 1) * 32) orelse return error.SkipZigTest;
    defer unmap(messages);
    var too_many_messages = samplePublicInputs();
    too_many_messages.l2_l1_messages = std.mem.bytesAsSlice([32]u8, messages);
    try std.testing.expectError(error.BoundsViolation, rollup_proof.encodePublicInputs(std.testing.allocator, too_many_messages));
}
