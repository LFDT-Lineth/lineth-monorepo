const std = @import("std");

const fixtures = @import("evm_execution_fixtures");
const guest = @import("evm_execution_guest");

// Runs the thin wrapper (vanilla zesu stateless execution) on a real execution-spec-tests zkevm SSZ
// fixture and asserts the serialized validation result matches the fixture's expected output —
// exactly what zesu's own zkevm-blockchain-test-runner checks, end to end, on the native host.
test "guest runs a vanilla zesu stateless block (SSZ) and matches the expected validation result" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const fixture = try fixtures.loadStatelessBlock(allocator, fixtures.embedded.zkevm_stateless_block);
    try std.testing.expectEqualStrings("Amsterdam", fixture.network);
    try std.testing.expectEqual(@as(usize, 105), fixture.expected_output.len);

    const result = try guest.runStateless(allocator, fixture.input);

    try std.testing.expect(result.success);
    try std.testing.expectEqualSlices(u8, fixture.expected_output, &result.out);
}
