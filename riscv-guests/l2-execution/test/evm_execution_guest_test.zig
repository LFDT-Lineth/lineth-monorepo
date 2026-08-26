const std = @import("std");

const fixtures = @import("evm_execution_fixtures");
const guest = @import("evm_execution_guest");
const executor = @import("zesu_executor");
const ssz_decode = @import("zesu_ssz_decode");
const zesu_allocator = @import("zesu_allocator");
const mpt = @import("zesu_mpt");

// Proves the log-preserving seam (guest.execution.executeStatelessInputWithLogs, src/execution.zig)
// computes the SAME pre/post/receipts roots as zesu's vanilla executor.executeStatelessInput on the
// same fixture — i.e. adding the log-preserving path does not change validation outcomes. The
// committed fixture is an empty block, so there are no logs to preserve; this only proves parity of
// the roots and that the logs slice is (trivially) empty.
test "executeStatelessInputWithLogs matches vanilla executeStatelessInput's roots" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const fixture = try fixtures.loadStatelessBlock(allocator, fixtures.embedded.zkevm_stateless_block);
    const si = try ssz_decode.decode(allocator, fixture.input);

    // executeStatelessInput (unlike executeStatelessInputWithLogs) relies on the zesu_allocator
    // singleton being set by the caller.
    zesu_allocator.set(allocator);
    const vanilla = try executor.executeStatelessInput(allocator, si, si.chain_config.fork_name);

    var node_index = try mpt.buildNodeIndex(allocator, si.witness.nodes);
    defer node_index.deinit();
    const with_logs = try guest.execution.executeStatelessInputWithLogs(allocator, si, si.chain_config.fork_name.?, &node_index);

    try std.testing.expectEqualSlices(u8, &vanilla.pre_state_root, &with_logs.pre_state_root);
    try std.testing.expectEqualSlices(u8, &vanilla.post_state_root, &with_logs.post_state_root);
    try std.testing.expectEqualSlices(u8, &vanilla.receipts_root, &with_logs.receipts_root);
    try std.testing.expectEqual(vanilla.receipts.len, with_logs.receipts.len);
    for (with_logs.receipts) |receipt| {
        try std.testing.expectEqual(@as(usize, 0), receipt.logs.len);
    }
}
