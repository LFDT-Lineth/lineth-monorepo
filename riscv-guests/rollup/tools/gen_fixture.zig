//! Prints, as 0x-prefixed hex on stdout, a framed `SszRollupProofPrivateInput` this package's own
//! `rollup_ssz.encodeInput` produced from a minimal valid sample — the sample input `make exec`
//! runs the guest against. Regenerate `test/testdata/sample.request.ssz.hex` with:
//!   zig build gen-fixture > test/testdata/sample.request.ssz.hex

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

pub fn main(init: std.process.Init) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const proof: rollup_ssz.VerifiableL2ExecutionProof = .{
        .program_vk = repeat32(0xaa),
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = repeat32(0x0a),
                .end_block_hash = repeat32(0x0b),
                .end_block_number = 11,
                .end_block_timestamp = 1763000200,
                .l2_l1_messages_hash = repeat32(0x06),
                .parent_l1_l2_bridge_rolling_hash = repeat32(0x02),
                .parent_l1_l2_bridge_rolling_hash_message_number = 0,
                .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
                .end_l1_l2_bridge_rolling_hash_message_number = 4,
                .dynamic_chain_config_hash = repeat32(0xc0),
                .parent_ftx_rolling_hash = repeat32(0x04),
                .parent_ftx_number = 15,
                .end_ftx_rolling_hash = repeat32(0x05),
                .end_processed_ftx_number = 18,
                .filtered_addresses_hash = repeat32(0x07),
                .tx_froms_hash = repeat32(0x08),
            },
            .start_block_number = 10,
            .proof = &[_]u8{ 0xab, 0xcd, 0xef },
            .l2_l1_messages = &[_][32]u8{repeat32(0x08)},
            .tx_froms = &[_][20]u8{repeat20(0x01)},
            .filtered_addresses = &[_][20]u8{repeat20(0x03)},
        },
    };

    const input: rollup_ssz.RollupProofPrivateInput = .{
        .parent_data_rolling_hash = repeat32(0x47),
        .start_offset = 4,
        .chain_id = 59144,
        .conflations = &[_]rollup_ssz.ConflationWitness{.{ .block_rlps = &[_][]const u8{&[_]u8{ 0xf9, 0x02, 0x15, 0xa0 }} }},
        .chunks = &[_][32]u8{repeat32(0x1a)},
        .l2_execution_proofs = &[_]rollup_ssz.VerifiableL2ExecutionProof{proof},
        .opaque_prefix_bytes = &[_]u8{ 0xab, 0xab, 0xab, 0xab },
        .opaque_suffix_bytes = &.{},
        .boundary_prev_data_rolling_hash = repeat32(0x39),
    };

    const encoded = try rollup_ssz.encodeInput(alloc, input);

    var out: std.ArrayListUnmanaged(u8) = .empty;
    try out.appendSlice(alloc, "0x");
    const hex_digits = "0123456789abcdef";
    for (encoded) |byte| {
        try out.append(alloc, hex_digits[byte >> 4]);
        try out.append(alloc, hex_digits[byte & 0x0f]);
    }
    try out.append(alloc, '\n');

    try std.Io.File.stdout().writeStreamingAll(init.io, out.items);
}
