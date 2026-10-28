const std = @import("std");
const zkc_machine = @import("zkc_machine");

const min_output_size: usize = 34;
const commitment_size: usize = 32;
const min_log_size: usize = min_output_size - commitment_size;
const output_line = "guest_output = 0x" ++ "00" ** commitment_size;

test "ZkC output protocol joins guest_output and a variable-length guest_log" {
    const alloc = std.testing.allocator;
    const output = try zkc_machine.testing.parseOutput(
        alloc,
        "noise\n  guest_log = 0x" ++ "cd" ** (min_log_size + 3) ++ "  \n" ++
            "  guest_output = 0x" ++ "ab" ** commitment_size ++ "\n",
    );
    defer alloc.free(output);
    try std.testing.expectEqualSlices(
        u8,
        &([_]u8{0xab} ** commitment_size ++ [_]u8{0xcd} ** (min_log_size + 3)),
        output,
    );
}

test "ZkC output protocol rejects missing malformed truncated and duplicate output" {
    const alloc = std.testing.allocator;
    try std.testing.expectError(error.MissingGuestOutput, zkc_machine.testing.parseOutput(alloc, "guest completed"));
    try std.testing.expectError(error.MissingGuestLog, zkc_machine.testing.parseOutput(alloc, output_line));
    try std.testing.expectError(
        error.MissingGuestOutput,
        zkc_machine.testing.parseOutput(alloc, "guest_log = 0x" ++ "00" ** min_output_size),
    );
    try std.testing.expectError(error.InvalidGuestOutput, zkc_machine.testing.parseOutput(alloc, "guest_output = missing-prefix"));
    try std.testing.expectError(error.InvalidGuestLog, zkc_machine.testing.parseOutput(alloc, "guest_log = missing-prefix"));
    try std.testing.expectError(
        error.InvalidGuestOutput,
        zkc_machine.testing.parseOutput(alloc, "guest_output = 0x" ++ "gg" ** commitment_size ++
            "\nguest_log = 0x" ++ "00" ** min_log_size),
    );
    try std.testing.expectError(
        error.InvalidGuestLog,
        zkc_machine.testing.parseOutput(alloc, output_line ++ "\nguest_log = 0x" ++ "gg" ** min_log_size),
    );
    try std.testing.expectError(
        error.InvalidGuestOutputLogSize,
        zkc_machine.testing.parseOutput(alloc, output_line ++ "\nguest_log = 0x" ++ "00" ** (min_log_size - 1)),
    );
    // Even combined length, but neither half is whole bytes: must not be re-paired across the join.
    try std.testing.expectError(
        error.InvalidGuestOutputSize,
        zkc_machine.testing.parseOutput(alloc, output_line ++ "0\nguest_log = 0x0" ++ "00" ** min_log_size),
    );
    try std.testing.expectError(
        error.InvalidGuestLogSize,
        zkc_machine.testing.parseOutput(alloc, output_line ++ "\nguest_log = 0x" ++ "00" ** min_log_size ++ "0"),
    );
    try std.testing.expectError(
        error.DuplicateGuestOutput,
        zkc_machine.testing.parseOutput(alloc, output_line ++ "\n" ++ output_line ++
            "\nguest_log = 0x" ++ "00" ** min_log_size),
    );
    try std.testing.expectError(
        error.DuplicateGuestLog,
        zkc_machine.testing.parseOutput(
            alloc,
            output_line ++ "\nguest_log = 0x" ++ "00" ** min_log_size ++
                "\nguest_log = 0x" ++ "11" ** min_log_size,
        ),
    );
    try std.testing.expectError(
        error.DuplicateGuestOutput,
        zkc_machine.testing.parseOutput(alloc, output_line ++ "\n" ++ "guest_output = missing-prefix"),
    );
}

test "ZkC rejection protocol requires one dedicated nonzero numeric exit line" {
    try std.testing.expect(zkc_machine.testing.hasNonzeroGuestExit("noise\n  EXIT CODE = 7 \r\n"));
    try std.testing.expect(!zkc_machine.testing.hasNonzeroGuestExit("tool mentioned EXIT CODE = 7 inline"));
    try std.testing.expect(!zkc_machine.testing.hasNonzeroGuestExit("EXIT CODE = 0"));
    try std.testing.expect(!zkc_machine.testing.hasNonzeroGuestExit("EXIT CODE = nope"));
    try std.testing.expect(!zkc_machine.testing.hasNonzeroGuestExit("EXIT CODE = 7 trailing"));
    try std.testing.expect(!zkc_machine.testing.hasNonzeroGuestExit("EXIT CODE = 7\nEXIT CODE = 8"));
}
