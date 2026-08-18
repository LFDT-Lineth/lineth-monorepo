//! Prints, as 0x-prefixed hex on stdout, a framed `SszRollupAggregationProofPrivateInput` this
//! package's own `rollup_aggregation_ssz.encodeInput` produced from a minimal valid sample — the
//! sample input `make exec` runs the guest against. Regenerate
//! `test/testdata/sample.request.ssz.hex` with:
//!   zig build gen-fixture > test/testdata/sample.request.ssz.hex

const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");

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

    const proof: rollup_aggregation_ssz.VerifiableRollupProof = .{
        .program_vk = repeat32(0xbb),
        .proof = .{
            .public_inputs = .{
                .end_block_number = 11,
                .end_block_timestamp = 1763000457,
                .l2_l1_bridge_transaction_tree = repeat32(0x11),
                .parent_l1_l2_bridge_rolling_hash = repeat32(0x22),
                .parent_l1_l2_bridge_rolling_hash_message_number = 0,
                .end_l1_l2_bridge_rolling_hash = repeat32(0x33),
                .end_l1_l2_bridge_rolling_hash_message_number = 7,
                .dynamic_chain_config_hash = repeat32(0xc0),
                .parent_ftx_rolling_hash = repeat32(0x44),
                .parent_ftx_number = 15,
                .end_ftx_rolling_hash = repeat32(0x55),
                .end_processed_ftx_number = 18,
                .filtered_addresses_hash = repeat32(0x66),
                .parent_data_rolling_hash = repeat32(0x47),
                .end_data_rolling_hash = repeat32(0x8d),
                .parent_block_hash = repeat32(0x0a),
                .end_block_hash = repeat32(0x0b),
                .start_offset = 0,
                .end_offset = 131072,
                .program_vks = &[_][32]u8{repeat32(0xaa)},
            },
            .start_block_number = 10,
            .proof = &[_]u8{ 0xab, 0xcd, 0xef },
            .l2_l1_roots = &[_][32]u8{ repeat32(0x77), repeat32(0x88) },
            .filtered_addresses = &[_][20]u8{repeat20(0x01)},
        },
    };

    const input: rollup_aggregation_ssz.RollupAggregationProofPrivateInput = .{
        .rollup_proofs = &[_]rollup_aggregation_ssz.VerifiableRollupProof{proof},
    };

    const encoded = try rollup_aggregation_ssz.encodeInput(alloc, input);

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
