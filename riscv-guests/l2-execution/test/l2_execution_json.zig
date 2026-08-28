//! JSON encoder for the l2-execution guest's own output (`l2_execution_ssz.L2ExecutionProofOutput`).
//!
//! Field names/shapes mirror the Python reference codec's `proof_io_v1.encode_response` —
//! the same camelCase keys and 0x-hex byte encoding it emits for `getZkL2ExecutionProofV1
//! .response.json` — MINUS `proverVersion` and `proof`: those are prover-layer metadata the
//! guest never produces (see `l2_execution_ssz.L2ExecutionProofOutput`'s doc comment).
//!
//! This is a NATIVE-only convenience: the freestanding zkVM guest always emits SSZ
//! (`l2_execution_ssz.encodeOutput` + the proving system's `write_output`) and has no argv to
//! carry a format flag. The SSZ/JSON runtime toggle lives entirely in the native
//! `l2-execution-runner` host tool, which is the only caller of `encodeOutputJson`.

const std = @import("std");
const l2_execution_ssz = @import("l2_execution_ssz");

const hex_digits = "0123456789abcdef";

/// Appends a `"0x<hex>"` JSON string literal for `bytes`.
fn appendHexField(alloc: std.mem.Allocator, out: *std.ArrayListUnmanaged(u8), bytes: []const u8) !void {
    try out.appendSlice(alloc, "\"0x");
    const start = out.items.len;
    try out.resize(alloc, start + bytes.len * 2);
    for (bytes, 0..) |b, i| {
        out.items[start + i * 2] = hex_digits[b >> 4];
        out.items[start + i * 2 + 1] = hex_digits[b & 0xF];
    }
    try out.appendSlice(alloc, "\"");
}

/// Appends `,"key":"0x<hex>"` (or without the leading comma when `first`).
fn appendKeyHex(alloc: std.mem.Allocator, out: *std.ArrayListUnmanaged(u8), key: []const u8, bytes: []const u8, first: bool) !void {
    if (!first) try out.appendSlice(alloc, ",");
    try out.print(alloc, "\"{s}\":", .{key});
    try appendHexField(alloc, out, bytes);
}

/// Appends `,"key":<decimal>` (or without the leading comma when `first`). All l2-execution PI
/// integers fit comfortably in a JSON number (block numbers/timestamps/message numbers), matching
/// `encode_response`'s plain `int(...)` — no 0x-hex quantities on the output side.
fn appendKeyInt(alloc: std.mem.Allocator, out: *std.ArrayListUnmanaged(u8), key: []const u8, value: u64, first: bool) !void {
    if (!first) try out.appendSlice(alloc, ",");
    try out.print(alloc, "\"{s}\":{d}", .{ key, value });
}

/// Appends a JSON array of `"0x<hex>"` strings for a list of fixed-size byte arrays
/// ([32]u8 hashes or [20]u8 addresses).
fn appendHexList(alloc: std.mem.Allocator, out: *std.ArrayListUnmanaged(u8), comptime T: type, items: []const T) !void {
    for (items, 0..) |item, i| {
        if (i != 0) try out.appendSlice(alloc, ",");
        try appendHexField(alloc, out, &item);
    }
}

fn appendPublicInputs(alloc: std.mem.Allocator, out: *std.ArrayListUnmanaged(u8), pi: l2_execution_ssz.L2ExecutionProofPublicInput) !void {
    try out.appendSlice(alloc, "\"publicInputs\":{");
    try appendKeyHex(alloc, out, "parentBlockHash", &pi.parent_block_hash, true);
    try appendKeyHex(alloc, out, "endBlockHash", &pi.end_block_hash, false);
    try appendKeyInt(alloc, out, "endBlockNumber", pi.end_block_number, false);
    try appendKeyInt(alloc, out, "endBlockTimestamp", pi.end_block_timestamp, false);
    try appendKeyHex(alloc, out, "l2L1MessagesHash", &pi.l2_l1_messages_hash, false);
    try appendKeyHex(alloc, out, "parentL1L2BridgeRollingHash", &pi.parent_l1_l2_bridge_rolling_hash, false);
    try appendKeyInt(alloc, out, "parentL1L2BridgeRollingHashMessageNumber", pi.parent_l1_l2_bridge_rolling_hash_message_number, false);
    try appendKeyHex(alloc, out, "endL1L2BridgeRollingHash", &pi.end_l1_l2_bridge_rolling_hash, false);
    try appendKeyInt(alloc, out, "endL1L2BridgeRollingHashMessageNumber", pi.end_l1_l2_bridge_rolling_hash_message_number, false);
    try appendKeyHex(alloc, out, "dynamicChainConfigHash", &pi.dynamic_chain_config_hash, false);
    try appendKeyHex(alloc, out, "parentFtxRollingHash", &pi.parent_ftx_rolling_hash, false);
    try appendKeyInt(alloc, out, "parentProcessedFtxNumber", pi.parent_processed_ftx_number, false);
    try appendKeyHex(alloc, out, "endFtxRollingHash", &pi.end_ftx_rolling_hash, false);
    try appendKeyInt(alloc, out, "endProcessedFtxNumber", pi.end_processed_ftx_number, false);
    try appendKeyHex(alloc, out, "filteredAddressesHash", &pi.filtered_addresses_hash, false);
    try appendKeyHex(alloc, out, "txFromsHash", &pi.tx_froms_hash, false);
    try out.appendSlice(alloc, "}");
}

/// Encode `v` as a `getZkL2ExecutionProofV1.response.json`-shaped JSON object, minus the
/// prover-attached `proverVersion`/`proof` fields (see the file doc comment). Field order matches
/// `proof_io_v1.encode_response` exactly: `startBlockNumber`, `publicInputs`, `l2L1Messages`,
/// `txFroms`, `filteredAddresses`.
pub fn encodeOutputJson(alloc: std.mem.Allocator, v: l2_execution_ssz.L2ExecutionProofOutput) ![]u8 {
    var out = std.ArrayListUnmanaged(u8).empty;
    errdefer out.deinit(alloc);

    try out.appendSlice(alloc, "{");
    try appendKeyInt(alloc, &out, "startBlockNumber", v.start_block_number, true);
    try out.appendSlice(alloc, ",");
    try appendPublicInputs(alloc, &out, v.public_inputs);

    try out.appendSlice(alloc, ",\"l2L1Messages\":[");
    try appendHexList(alloc, &out, [32]u8, v.l2_l1_messages);
    try out.appendSlice(alloc, "],\"txFroms\":[");
    try appendHexList(alloc, &out, [20]u8, v.tx_froms);
    try out.appendSlice(alloc, "],\"filteredAddresses\":[");
    try appendHexList(alloc, &out, [20]u8, v.filtered_addresses);
    try out.appendSlice(alloc, "]}");

    return out.toOwnedSlice(alloc);
}
