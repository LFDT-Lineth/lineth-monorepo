const std = @import("std");
const execution_machine = @import("execution_machine");

pub const ZkcMachine = struct {
    elf: []const u8,
    makefile: []const u8,
    target: []const u8,
    flags: ?[]const u8,
    temp_dir: []const u8,

    pub fn run(
        self: *ZkcMachine,
        init: std.process.Init,
        alloc: std.mem.Allocator,
        input: []const u8,
    ) !execution_machine.Result {
        const input_path = try std.fs.path.join(alloc, &.{ self.temp_dir, "input.ssz" });
        const json_path = try std.fs.path.join(alloc, &.{ self.temp_dir, "guest.json" });
        try std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = input_path, .data = input });

        const in_arg = try std.fmt.allocPrint(alloc, "IN_BYTES=@{s}", .{input_path});
        const elf_arg = try std.fmt.allocPrint(alloc, "BIN_EXT={s}", .{self.elf});
        const json_arg = try std.fmt.allocPrint(alloc, "JSON_EXT={s}", .{json_path});
        const makefile_arg = try std.fmt.allocPrint(alloc, "-f{s}", .{self.makefile});
        const flags = self.flags orelse if (std.mem.eql(u8, self.target, "elf-trace")) "--stats" else "--fast";
        const flags_arg = if (std.mem.eql(u8, self.target, "elf-trace"))
            try std.fmt.allocPrint(alloc, "ZKC_TRACE_FLAGS={s}", .{flags})
        else
            try std.fmt.allocPrint(alloc, "ZKC_EXEC_FLAGS={s}", .{flags});

        const process = try std.process.run(alloc, init.io, .{
            .argv = &.{ "make", "-s", makefile_arg, self.target, elf_arg, in_arg, json_arg, flags_arg },
        });
        const text = try std.fmt.allocPrint(alloc, "{s}\n{s}", .{ process.stdout, process.stderr });

        switch (process.term) {
            .exited => |code| {
                if (code == 0) return .{ .accepted = try parseProtocolOutput(text) };
                if (hasProtocolNonzeroGuestExit(text)) return .{ .rejected = null };
                std.debug.print("--- zkc toolchain failure ---\n{s}\n", .{text});
                return error.ToolchainFailed;
            },
            else => return error.ZkcCrashed,
        }
    }
};

const output_prefix = "guest_output = 0x";

// TODO: Consume structured ZkC/prover output once the backend exposes it.
fn parseProtocolOutput(text: []const u8) !execution_machine.Output {
    const output_assignment = "guest_output =";
    var output_hex: ?[]const u8 = null;
    var lines = std.mem.splitScalar(u8, text, '\n');
    while (lines.next()) |raw_line| {
        const line = std.mem.trim(u8, raw_line, " \t\r");
        if (!std.mem.startsWith(u8, line, output_assignment)) continue;
        if (output_hex != null) return error.DuplicateGuestOutput;
        if (!std.mem.startsWith(u8, line, output_prefix)) return error.InvalidGuestOutput;
        const remainder = line[output_prefix.len..];
        if (std.mem.indexOfScalar(u8, remainder, '=') != null) return error.DuplicateGuestOutput;
        output_hex = remainder;
    }

    const hex = output_hex orelse return error.MissingGuestOutput;
    if (hex.len != @sizeOf(execution_machine.Output) * 2) return error.InvalidGuestOutputSize;
    var output: execution_machine.Output = undefined;
    _ = std.fmt.hexToBytes(&output, hex) catch return error.InvalidGuestOutput;
    return output;
}

fn hasProtocolNonzeroGuestExit(text: []const u8) bool {
    const exit_prefix = "EXIT CODE = ";
    var exit_code: ?u64 = null;
    var lines = std.mem.splitScalar(u8, text, '\n');
    while (lines.next()) |raw_line| {
        const line = std.mem.trim(u8, raw_line, " \t\r");
        if (!std.mem.startsWith(u8, line, exit_prefix)) continue;
        if (exit_code != null) return false;
        const value = std.fmt.parseInt(u64, line[exit_prefix.len..], 10) catch return false;
        exit_code = value;
    }
    return if (exit_code) |code| code != 0 else false;
}

pub const testing = struct {
    pub fn parseOutput(text: []const u8) !execution_machine.Output {
        return parseProtocolOutput(text);
    }

    pub fn hasNonzeroGuestExit(text: []const u8) bool {
        return hasProtocolNonzeroGuestExit(text);
    }
};
