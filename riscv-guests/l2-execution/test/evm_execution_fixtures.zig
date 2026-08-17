const std = @import("std");

const zkevm_fixture = @import("zkevm_fixture.zig");

/// One zkevm stateless-block fixture (execution-spec-tests `tests-zkevm` format), reduced to the
/// fields the guest smoke test needs.
pub const StatelessBlockFixture = struct {
    network: []const u8,
    /// SSZ-encoded SszStatelessInput — decode with zesu's `ssz_decode`.
    input: []const u8,
    /// Expected 105-byte SSZ SszStatelessValidationResult — compare with zesu's `ssz_output`.
    expected_output: []const u8,
};

pub const embedded = struct {
    /// Embedded straight from the execution_spec_tests_zkevm dependency (see build.zig).
    pub const zkevm_stateless_block = @embedFile("zkevm_stateless_block.json");
};

/// First stateless block of a zkevm fixture — a smoke-test convenience over the shared
/// `zkevm_fixture.parseBlocks` (the full-corpus runner consumes all blocks via the same parser).
pub fn loadStatelessBlock(allocator: std.mem.Allocator, fixture_json: []const u8) !StatelessBlockFixture {
    const blocks = try zkevm_fixture.parseBlocks(allocator, fixture_json);
    if (blocks.len == 0) return error.EmptyFixture;
    const block = blocks[0];
    return .{
        .network = block.network orelse return error.InvalidFixture,
        .input = block.input,
        .expected_output = block.expected_output,
    };
}
