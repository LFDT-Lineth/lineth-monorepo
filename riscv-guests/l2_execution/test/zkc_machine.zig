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
        const flags = self.flags orelse if (std.mem.eql(u8, self.target, "elf-trace")) "--stats" else "";
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
                if (code == 0) return .{ .accepted = try parseProtocolOutput(alloc, text) };
                if (hasProtocolNonzeroGuestExit(text)) return .{ .rejected = null };
                std.debug.print("--- zkc toolchain failure ---\n{s}\n", .{text});
                return error.ToolchainFailed;
            },
            else => return error.ZkcCrashed,
        }
    }
};

const guest_output_prefix = "guest_output = 0x";
const guest_log_prefix = "guest_log = 0x";
const min_output_size: usize = 34;

// TODO: Consume structured ZkC/prover output once the backend exposes it.
fn parseProtocolOutput(alloc: std.mem.Allocator, text: []const u8) !execution_machine.Output {
    const guest_output_assignment = "guest_output =";
    const guest_log_assignment = "guest_log =";
    var guest_output_hex: ?[]const u8 = null;
    var guest_log_hex: ?[]const u8 = null;
    var lines = std.mem.splitScalar(u8, text, '\n');
    while (lines.next()) |raw_line| {
        const line = std.mem.trim(u8, raw_line, " \t\r");
        if (std.mem.startsWith(u8, line, guest_output_assignment)) {
            if (guest_output_hex != null) return error.DuplicateGuestOutput;
            if (!std.mem.startsWith(u8, line, guest_output_prefix)) return error.InvalidGuestOutput;
            const remainder = line[guest_output_prefix.len..];
            if (std.mem.indexOfScalar(u8, remainder, '=') != null) return error.DuplicateGuestOutput;
            guest_output_hex = remainder;
        } else if (std.mem.startsWith(u8, line, guest_log_assignment)) {
            if (guest_log_hex != null) return error.DuplicateGuestLog;
            if (!std.mem.startsWith(u8, line, guest_log_prefix)) return error.InvalidGuestLog;
            const remainder = line[guest_log_prefix.len..];
            if (std.mem.indexOfScalar(u8, remainder, '=') != null) return error.DuplicateGuestLog;
            guest_log_hex = remainder;
        }
    }
    const output_hex = guest_output_hex orelse return error.MissingGuestOutput;
    const log_hex = guest_log_hex orelse return error.MissingGuestLog;
    // Each half must be whole bytes on its own; an odd split would otherwise misalign the join.
    if (output_hex.len % 2 != 0) return error.InvalidGuestOutputSize;
    if (log_hex.len % 2 != 0) return error.InvalidGuestLogSize;
    // `write_output` puts the 32-byte commitment in guest_output and the rest (2-byte schema id,
    // then the SSZ body) in guest_log; rejoin them to match the host machine's encodeOutput bytes.
    const output = try alloc.alloc(u8, (output_hex.len + log_hex.len) / 2);
    errdefer alloc.free(output);
    if (output.len < min_output_size) return error.InvalidGuestOutputLogSize;
    const split = output_hex.len / 2;
    _ = std.fmt.hexToBytes(output[0..split], output_hex) catch return error.InvalidGuestOutput;
    _ = std.fmt.hexToBytes(output[split..], log_hex) catch return error.InvalidGuestLog;
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
    pub fn parseOutput(alloc: std.mem.Allocator, text: []const u8) !execution_machine.Output {
        return parseProtocolOutput(alloc, text);
    }

    pub fn hasNonzeroGuestExit(text: []const u8) bool {
        return hasProtocolNonzeroGuestExit(text);
    }
};
