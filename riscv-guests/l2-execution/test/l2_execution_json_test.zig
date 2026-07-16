//! Golden-shape test for `l2_execution_json.encodeOutputJson`.
//!
//! Values are lifted straight from the committed `getZkL2ExecutionProofV1.response.json`
//! fixture — the Python reference codec's own `encode_response` output — so this asserts
//! byte-exact agreement with the oracle on field names, field order, and hex format, minus the
//! `proverVersion`/`proof` fields the guest doesn't own (see `l2_execution_json.zig`'s doc
//! comment for why).

const std = @import("std");
const l2_execution_ssz = @import("l2_execution_ssz");
const l2_execution_json = @import("l2_execution_json");

fn repeat(comptime n: usize, byte: u8) [n]u8 {
    var out: [n]u8 = undefined;
    for (&out) |*b| b.* = byte;
    return out;
}

test "encodeOutputJson matches the Python reference response shape (minus proof/proverVersion)" {
    const alloc = std.testing.allocator;

    const output = l2_execution_ssz.L2ExecutionProofOutput{
        .public_inputs = .{
            .parent_block_hash = repeat(32, 0x0a),
            .end_block_hash = repeat(32, 0x0b),
            .end_block_number = 1000503,
            .end_block_timestamp = 1763000123,
            .l2_l1_messages_hash = repeat(32, 0x01),
            .parent_l1_l2_bridge_rolling_hash = repeat(32, 0x02),
            .parent_l1_l2_bridge_rolling_hash_message_number = 0,
            .end_l1_l2_bridge_rolling_hash = repeat(32, 0x03),
            .end_l1_l2_bridge_rolling_hash_message_number = 5,
            .dynamic_chain_config_hash = repeat(32, 0xc0),
            .parent_ftx_rolling_hash = repeat(32, 0x04),
            .parent_processed_ftx_number = 16,
            .end_ftx_rolling_hash = repeat(32, 0x05),
            .end_processed_ftx_number = 18,
            .filtered_addresses_hash = repeat(32, 0x06),
            .tx_froms_hash = repeat(32, 0x07),
        },
        .start_block_number = 1000501,
        .l2_l1_messages = &.{repeat(32, 0x08)},
        .tx_froms = &.{ repeat(20, 0x01), repeat(20, 0x02) },
        .filtered_addresses = &.{repeat(20, 0x09)},
    };

    const got = try l2_execution_json.encodeOutputJson(alloc, output);
    defer alloc.free(got);

    const expected =
        "{\"startBlockNumber\":1000501," ++
        "\"publicInputs\":{" ++
        "\"parentBlockHash\":\"0x0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a\"," ++
        "\"endBlockHash\":\"0x0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b\"," ++
        "\"endBlockNumber\":1000503," ++
        "\"endBlockTimestamp\":1763000123," ++
        "\"l2L1MessagesHash\":\"0x0101010101010101010101010101010101010101010101010101010101010101\"," ++
        "\"parentL1L2BridgeRollingHash\":\"0x0202020202020202020202020202020202020202020202020202020202020202\"," ++
        "\"parentL1L2BridgeRollingHashMessageNumber\":0," ++
        "\"endL1L2BridgeRollingHash\":\"0x0303030303030303030303030303030303030303030303030303030303030303\"," ++
        "\"endL1L2BridgeRollingHashMessageNumber\":5," ++
        "\"dynamicChainConfigHash\":\"0xc0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0\"," ++
        "\"parentFtxRollingHash\":\"0x0404040404040404040404040404040404040404040404040404040404040404\"," ++
        "\"parentProcessedFtxNumber\":16," ++
        "\"endFtxRollingHash\":\"0x0505050505050505050505050505050505050505050505050505050505050505\"," ++
        "\"endProcessedFtxNumber\":18," ++
        "\"filteredAddressesHash\":\"0x0606060606060606060606060606060606060606060606060606060606060606\"," ++
        "\"txFromsHash\":\"0x0707070707070707070707070707070707070707070707070707070707070707\"" ++
        "}," ++
        "\"l2L1Messages\":[\"0x0808080808080808080808080808080808080808080808080808080808080808\"]," ++
        "\"txFroms\":[\"0x0101010101010101010101010101010101010101\",\"0x0202020202020202020202020202020202020202\"]," ++
        "\"filteredAddresses\":[\"0x0909090909090909090909090909090909090909\"]" ++
        "}";

    try std.testing.expectEqualStrings(expected, got);
}
