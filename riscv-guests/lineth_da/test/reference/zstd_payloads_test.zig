//! The encoder and the decoder over the canonical payloads of the selected real blocks of the EF
//! zkevm fixtures.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");
const zstd_frames = @import("zstd_frames");

const zstd_decoder = lineth_da.zstd_decoder;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

/// Compresses `payload`, checks the header of the frame against the DA form and decodes it back.
fn roundTrip(encoder: zstd_encoder.Encoder, decoder: zstd_decoder.Decoder, payload: []const u8) !void {
    const frame = try encoder.compress(gpa, payload);
    defer gpa.free(frame);

    const header = try zstd_frames.header(frame);
    try std.testing.expectEqual(@as(?u64, payload.len), header.content_size);
    try std.testing.expectEqual(std.mem.readInt(u32, dictionary[4..8], .little), header.dictionary_id);
    try std.testing.expect(!header.checksum);
    try std.testing.expect(header.window <= zstd_decoder.max_window_size);

    const out = try gpa.alloc(u8, payload.len);
    defer gpa.free(out);
    try decoder.decompress(frame, out);
    try std.testing.expectEqualSlices(u8, payload, out);
}

test "the payload of every selected block, and of all of them as one conflation, round trips" {
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
    const encoder = try zstd_encoder.Encoder.init(gpa, dictionary);
    defer encoder.deinit();
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    const kinds = std.enums.values(ef_conflations.Kind);
    for (kinds) |kind| {
        const payload = try selection.conflationPayload(gpa, &.{kind});
        defer gpa.free(payload);
        try roundTrip(encoder, decoder, payload);
    }
    const all = try selection.conflationPayload(gpa, kinds);
    defer gpa.free(all);
    try roundTrip(encoder, decoder, all);
}
