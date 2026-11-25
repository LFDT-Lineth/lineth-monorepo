const std = @import("std");
const zkc = @import("zkc");
const execution_machine = @import("execution_machine.zig");

pub const ZkcMachine = struct {
    elf: []const u8,
    makefile: []const u8,
    target: zkc.Target,
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

        const flags = self.flags orelse if (self.target == .@"elf-trace") "--stats" else "";
        const result = try zkc.run(alloc, init.io, .{
            .makefile = self.makefile,
            .target = self.target,
            .elf = self.elf,
            .input = input_path,
            .json = json_path,
            .flags = flags,
        });

        switch (result.term) {
            .exited => |code| {
                if (code == 0) return .{ .accepted = try zkc.guestOutput(alloc, result.report) };
                if ((zkc.guestExitCode(result.report) orelse 0) != 0) return .{ .rejected = null };
                std.debug.print("--- zkc toolchain failure ---\n{s}\n", .{result.report});
                return error.ToolchainFailed;
            },
            else => return error.ZkcCrashed,
        }
    }
};
