const std = @import("std");
const execution_machine = @import("execution_machine");
const zkc_machine = @import("zkc_machine");

test "ZkC output protocol accepts one exact-sized output" {
    const output = try zkc_machine.testing.parseOutput(
        "noise\n  guest_output = 0x" ++ "00" ** @sizeOf(execution_machine.Output) ++ "  \n",
    );
    try std.testing.expectEqualSlices(u8, &([_]u8{0} ** @sizeOf(execution_machine.Output)), &output);
}

test "ZkC output protocol rejects missing malformed wrong-sized and duplicate output" {
    const output_size = @sizeOf(execution_machine.Output);
    try std.testing.expectError(error.MissingGuestOutput, zkc_machine.testing.parseOutput("guest completed"));
    try std.testing.expectError(error.InvalidGuestOutput, zkc_machine.testing.parseOutput("guest_output = missing-prefix"));
    try std.testing.expectError(
        error.InvalidGuestOutput,
        zkc_machine.testing.parseOutput("guest_output = 0x" ++ "gg" ** output_size),
    );
    try std.testing.expectError(
        error.InvalidGuestOutputSize,
        zkc_machine.testing.parseOutput("guest_output = 0x" ++ "00" ** (output_size - 1)),
    );
    try std.testing.expectError(
        error.InvalidGuestOutputSize,
        zkc_machine.testing.parseOutput("guest_output = 0x" ++ "00" ** (output_size + 1)),
    );
    try std.testing.expectError(
        error.DuplicateGuestOutput,
        zkc_machine.testing.parseOutput(
            "guest_output = 0x" ++ "00" ** output_size ++
                "\nguest_output = 0x" ++ "11" ** output_size,
        ),
    );
    try std.testing.expectError(
        error.DuplicateGuestOutput,
        zkc_machine.testing.parseOutput(
            "guest_output = 0x" ++ "00" ** output_size ++
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
