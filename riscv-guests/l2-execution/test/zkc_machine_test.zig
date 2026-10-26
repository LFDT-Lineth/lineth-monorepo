const std = @import("std");
const zkc_machine = @import("zkc_machine");

const min_output_size: usize = 34;

test "ZkC output protocol accepts a variable-length output" {
    const alloc = std.testing.allocator;
    const output = try zkc_machine.testing.parseOutput(
        alloc,
        "noise\n  guest_output = 0x" ++ "ab" ** (min_output_size + 3) ++ "  \n",
    );
    defer alloc.free(output);
    try std.testing.expectEqualSlices(u8, &([_]u8{0xab} ** (min_output_size + 3)), output);
}

test "ZkC output protocol rejects missing malformed truncated and duplicate output" {
    const alloc = std.testing.allocator;
    try std.testing.expectError(error.MissingGuestOutput, zkc_machine.testing.parseOutput(alloc, "guest completed"));
    try std.testing.expectError(error.InvalidGuestOutput, zkc_machine.testing.parseOutput(alloc, "guest_output = missing-prefix"));
    try std.testing.expectError(
        error.InvalidGuestOutput,
        zkc_machine.testing.parseOutput(alloc, "guest_output = 0x" ++ "gg" ** min_output_size),
    );
    try std.testing.expectError(
        error.InvalidGuestOutputSize,
        zkc_machine.testing.parseOutput(alloc, "guest_output = 0x" ++ "00" ** (min_output_size - 1)),
    );
    try std.testing.expectError(
        error.InvalidGuestOutputSize,
        zkc_machine.testing.parseOutput(alloc, "guest_output = 0x" ++ "00" ** min_output_size ++ "0"),
    );
    try std.testing.expectError(
        error.DuplicateGuestOutput,
        zkc_machine.testing.parseOutput(
            alloc,
            "guest_output = 0x" ++ "00" ** min_output_size ++
                "\nguest_output = 0x" ++ "11" ** min_output_size,
        ),
    );
    try std.testing.expectError(
        error.DuplicateGuestOutput,
        zkc_machine.testing.parseOutput(
            alloc,
            "guest_output = 0x" ++ "00" ** min_output_size ++
                "\nguest_output = missing-prefix",
        ),
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
