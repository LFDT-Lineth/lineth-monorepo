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

test "guest_errors.exitCode pins one representative code per category" {
    const guest_errors = guest.guest_errors;

    // The raw numbers are the point: these codes are load-bearing for operators, so renumbering
    // must break this test. Each arm asserts the enum member's pinned u64 value AND the mapping,
    // so neither a renumber nor a remap slips through. `error.BrokenPipe` stands in for any
    // untriaged error outside both `linea_errors` and the explicitly-categorized `error.OutOfMemory`.
    const E = guest_errors.ExitCode;
    try std.testing.expectEqual(@as(u64, 1), @intFromEnum(guest_errors.exitCode(error.BrokenPipe)));
    try std.testing.expectEqual(@as(u64, 2), @intFromEnum(guest_errors.exitCode(error.InvalidSsz)));
    try std.testing.expectEqual(@as(u64, 3), @intFromEnum(guest_errors.exitCode(error.InvalidStatelessInput)));
    try std.testing.expectEqual(@as(u64, 4), @intFromEnum(guest_errors.exitCode(error.EmptyPayloads)));
    try std.testing.expectEqual(@as(u64, 5), @intFromEnum(guest_errors.exitCode(error.UnsupportedFork)));
    try std.testing.expectEqual(@as(u64, 6), @intFromEnum(guest_errors.exitCode(error.BadNonceMismatch)));
    try std.testing.expectEqual(@as(u64, 7), @intFromEnum(guest_errors.exitCode(error.InvalidProof)));
    try std.testing.expectEqual(@as(u64, 8), @intFromEnum(guest_errors.exitCode(error.OutOfMemory)));
    // The mapping must also land on the named members, not a wrong category.
    try std.testing.expectEqual(E.unknown, guest_errors.exitCode(error.BrokenPipe));
    try std.testing.expectEqual(E.out_of_memory, guest_errors.exitCode(error.OutOfMemory));
    // WitnessDbResolution is the Linea-layer witness-DB case -> witness_resolution (7), distinct
    // from a zesu-internal InvalidWitness. The two engine categories are how a zesu-origin error
    // reaches the operator: decode (9) when the input bytes failed to parse, reject (10) when the
    // engine validated and refused the block.
    try std.testing.expectEqual(E.witness_resolution, guest_errors.exitCode(error.WitnessDbResolution));
    try std.testing.expectEqual(@as(u64, 7), @intFromEnum(guest_errors.exitCode(error.WitnessDbResolution)));
    try std.testing.expectEqual(E.engine_decode, guest_errors.exitCode(error.EngineDecode));
    try std.testing.expectEqual(@as(u64, 9), @intFromEnum(guest_errors.exitCode(error.EngineDecode)));
    try std.testing.expectEqual(E.engine_reject, guest_errors.exitCode(error.EngineReject));
    try std.testing.expectEqual(@as(u64, 10), @intFromEnum(guest_errors.exitCode(error.EngineReject)));
}

test "guest_errors.linea_errors: every listed error maps to a non-unknown category" {
    const guest_errors = guest.guest_errors;
    comptime {
        for (@typeInfo(guest_errors.linea_errors).error_set.?) |e| {
            const err: anyerror = @field(guest_errors.linea_errors, e.name);
            if (guest_errors.exitCode(err) == .unknown) {
                @compileError("guest_errors.linea_errors: '" ++ e.name ++ "' maps to ExitCode.unknown — add an explicit exitCode() category arm");
            }
        }
    }
}

test "guest_errors.zesuErr: zesu-origin errors split into decode/reject, Linea-meaningful ones pass through" {
    const guest_errors = guest.guest_errors;
    const zesuErr = guest_errors.zesuErr;

    // (zesuErr returns the error as a value, not an error union.) Whatever escapes a wrapped zesu
    // call must NOT keep a name that exitCode would file into a Linea category. Decode-flavored
    // zesu errors (RLP/SSZ/hex/container shape) become EngineDecode -> engine_decode (9)...
    for ([_]anyerror{ error.InvalidRlp, error.InvalidSsz, error.MissingField, error.InvalidHex, error.UnexpectedNull, error.InvalidNode }) |err| {
        const wrapped: anyerror = zesuErr(err);
        try std.testing.expectEqual(error.EngineDecode, wrapped);
        try std.testing.expectEqual(guest_errors.ExitCode.engine_decode, guest_errors.exitCode(wrapped));
    }
    // ...including the names that collide with Linea-layer vocabulary: a zesu-origin InvalidSsz is
    // a decode failure here, not the guest's input-envelope category.
    // Every other zesu error is an engine validation reject -> engine_reject (10). This is the
    // default, so a zesu bump adding a new validation error still lands here, and the colliding
    // InvalidProof/InvalidWitness (engine-side) are re-namespaced away from the Linea categories.
    for ([_]anyerror{ error.StateRootMismatch, error.GasLimitExceedsCap, error.InvalidBAL, error.InsufficientBalance, error.InvalidProof, error.InvalidWitness, error.Overflow }) |err| {
        const wrapped: anyerror = zesuErr(err);
        try std.testing.expectEqual(error.EngineReject, wrapped);
        try std.testing.expectEqual(guest_errors.ExitCode.engine_reject, guest_errors.exitCode(wrapped));
    }

    // Linea-meaningful errors pass through unchanged and keep their category: the fork-policy
    // reject, the capacity signal, and the witness-miss-through-WitnessDatabase case.
    try std.testing.expectEqual(error.UnsupportedFork, @as(anyerror, zesuErr(error.UnsupportedFork)));
    try std.testing.expectEqual(error.OutOfMemory, @as(anyerror, zesuErr(error.OutOfMemory)));
    try std.testing.expectEqual(guest_errors.ExitCode.witness_resolution, guest_errors.exitCode(zesuErr(error.WitnessDbResolution)));
}
