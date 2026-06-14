//! Parser for EF execution-spec-tests zkevm *stateless* blockchain fixtures.
//!
//! Single source of truth for the fixture JSON shape + hex decoding, shared by the single-fixture
//! smoke test (evm_execution_fixtures.zig) and the full-corpus runner (spec_runner.zig).
//!
//! Fixture shape (zkevm@v0.4.x):
//!   { "<test_name>": { "network": "...", "blocks": [
//!       { "statelessInputBytes": "0x…", "statelessOutputBytes": "0x…", … } ] } }

const std = @import("std");

pub const StatelessBlock = struct {
    test_name: []const u8,
    network: ?[]const u8,
    block_index: usize,
    /// Decoded SSZ SszStatelessInput.
    input: []const u8,
    /// Decoded SSZ SszStatelessValidationResult.
    expected_output: []const u8,
};

/// Parse every stateless block (across all test cases) from one fixture file's JSON.
/// Blocks lacking either statelessInputBytes or statelessOutputBytes are skipped — a few
/// intentionally-invalid blocks omit them. Everything is allocated in `allocator` (intended to be
/// an arena): the returned slice and its strings outlive the internal JSON parse.
pub fn parseBlocks(allocator: std.mem.Allocator, fixture_json: []const u8) ![]StatelessBlock {
    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, fixture_json, .{});
    defer parsed.deinit(); // strings we keep are duped into `allocator` below, so this is safe
    if (parsed.value != .object) return error.InvalidFixture;

    var out = std.ArrayList(StatelessBlock).empty;
    var it = parsed.value.object.iterator();
    while (it.next()) |kv| {
        const test_case = kv.value_ptr.*;
        if (test_case != .object) continue;

        const network = strField(test_case.object, "network");
        const blocks_val = test_case.object.get("blocks") orelse continue;
        if (blocks_val != .array) continue;

        for (blocks_val.array.items, 0..) |block_val, idx| {
            if (block_val != .object) continue;
            const in_hex = strField(block_val.object, "statelessInputBytes") orelse continue;
            const out_hex = strField(block_val.object, "statelessOutputBytes") orelse continue;

            try out.append(allocator, .{
                .test_name = try allocator.dupe(u8, kv.key_ptr.*),
                .network = if (network) |n| try allocator.dupe(u8, n) else null,
                .block_index = idx,
                .input = try hexToOwnedBytes(allocator, in_hex),
                .expected_output = try hexToOwnedBytes(allocator, out_hex),
            });
        }
    }
    return out.items;
}

pub fn strField(obj: std.json.ObjectMap, key: []const u8) ?[]const u8 {
    return switch (obj.get(key) orelse return null) {
        .string => |s| s,
        else => null,
    };
}

pub fn hexToOwnedBytes(allocator: std.mem.Allocator, hex: []const u8) ![]u8 {
    const stripped = if (hex.len >= 2 and hex[0] == '0' and (hex[1] == 'x' or hex[1] == 'X')) hex[2..] else hex;
    if (stripped.len % 2 != 0) return error.OddHexLength;
    const out = try allocator.alloc(u8, stripped.len / 2);
    _ = try std.fmt.hexToBytes(out, stripped);
    return out;
}
