//! Runs a guest ELF under zkc on a request and compares the output it prints with the
//! expected output byte for byte: `zkc_smoke --elf PATH --input PATH --expected PATH --makefile
//! PATH [--zkc-target NAME] [--zkc-flags TEXT]`. The makefile is arithmetization's, which defines
//! the zkc run; the target is `elf-exec` unless `elf-trace` is named. The guest's output is the
//! commitment and frame zkc reports.

const std = @import("std");
const zkc = @import("zkc");

const usage = "usage: zkc_smoke --elf PATH --input PATH --expected PATH --makefile PATH [--zkc-target elf-exec|elf-trace] [--zkc-flags TEXT]\n";

pub fn main(init: std.process.Init) !void {
    const arena = init.arena.allocator();
    const args = try init.minimal.args.toSlice(arena);

    var elf: ?[]const u8 = null;
    var input: ?[]const u8 = null;
    var expected_path: ?[]const u8 = null;
    var makefile: ?[]const u8 = null;
    var target: zkc.Target = .@"elf-exec";
    var flags: []const u8 = "";
    var i: usize = 1;
    while (i < args.len) : (i += 2) {
        if (i + 1 >= args.len) fatal("missing value");
        const value = args[i + 1];
        if (std.mem.eql(u8, args[i], "--elf")) {
            elf = value;
        } else if (std.mem.eql(u8, args[i], "--input")) {
            input = value;
        } else if (std.mem.eql(u8, args[i], "--expected")) {
            expected_path = value;
        } else if (std.mem.eql(u8, args[i], "--makefile")) {
            makefile = value;
        } else if (std.mem.eql(u8, args[i], "--zkc-target")) {
            target = std.meta.stringToEnum(zkc.Target, value) orelse fatal("unknown zkc target");
        } else if (std.mem.eql(u8, args[i], "--zkc-flags")) {
            flags = value;
        } else fatal("unknown option");
    }

    const expected = try std.Io.Dir.cwd().readFileAlloc(init.io, expected_path orelse fatal("missing --expected"), arena, .limited(1 << 28));
    const run = try zkc.run(arena, init.io, .{
        .makefile = makefile orelse fatal("missing --makefile"),
        .target = target,
        .elf = elf orelse fatal("missing --elf"),
        .input = input orelse fatal("missing --input"),
        .flags = flags,
    });

    switch (run.term) {
        .exited => |code| if (code != 0) {
            std.debug.print("--- zkc exited with {d} ---\n{s}\n", .{ code, run.report });
            return error.ZkcFailed;
        },
        else => return error.ZkcCrashed,
    }

    const actual = try zkc.guestOutput(arena, run.report);
    if (!std.mem.eql(u8, expected, actual)) {
        const common = std.mem.indexOfDiff(u8, expected, actual) orelse @min(expected.len, actual.len);
        std.debug.print(
            "the guest output differs from the native one at byte {d} (expected {d} bytes, guest {d} bytes)\nexpected: {x}\nguest:    {x}\n",
            .{ common, expected.len, actual.len, expected, actual },
        );
        return error.OutputMismatch;
    }
    std.debug.print("the guest output equals the native one ({d} bytes)\n", .{actual.len});
}

fn fatal(message: []const u8) noreturn {
    std.debug.print("error: {s}\n{s}", .{ message, usage });
    std.process.exit(2);
}
