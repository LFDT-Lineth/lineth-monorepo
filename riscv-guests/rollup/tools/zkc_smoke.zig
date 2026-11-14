//! Runs a rollup guest ELF under zkc on a request and compares the output it prints with the
//! expected output byte for byte: `zkc_smoke --elf PATH --input PATH --expected PATH --makefile
//! PATH [--zkc-flags TEXT]`. The makefile is arithmetization's, which defines the zkc run. zkc
//! prints the output's 32-byte commitment as `guest_output` and the frame after it as `guest_log`.

const std = @import("std");

const usage = "usage: zkc_smoke --elf PATH --input PATH --expected PATH --makefile PATH [--zkc-flags TEXT]\n";

pub fn main(init: std.process.Init) !void {
    const arena = init.arena.allocator();
    const args = try init.minimal.args.toSlice(arena);

    var elf: ?[]const u8 = null;
    var input: ?[]const u8 = null;
    var expected_path: ?[]const u8 = null;
    var makefile: ?[]const u8 = null;
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
        } else if (std.mem.eql(u8, args[i], "--zkc-flags")) {
            flags = value;
        } else fatal("unknown option");
    }

    const expected = try std.Io.Dir.cwd().readFileAlloc(init.io, expected_path orelse fatal("missing --expected"), arena, .limited(1 << 28));
    const process = try std.process.run(arena, init.io, .{ .argv = &.{
        "make",
        "-s",
        try std.fmt.allocPrint(arena, "-f{s}", .{makefile orelse fatal("missing --makefile")}),
        "elf-exec",
        try std.fmt.allocPrint(arena, "BIN_EXT={s}", .{elf orelse fatal("missing --elf")}),
        try std.fmt.allocPrint(arena, "IN_BYTES=@{s}", .{input orelse fatal("missing --input")}),
        try std.fmt.allocPrint(arena, "ZKC_EXEC_FLAGS={s}", .{flags}),
    } });
    const text = try std.fmt.allocPrint(arena, "{s}\n{s}", .{ process.stdout, process.stderr });

    switch (process.term) {
        .exited => |code| if (code != 0) {
            std.debug.print("--- zkc exited with {d} ---\n{s}\n", .{ code, text });
            return error.ZkcFailed;
        },
        else => return error.ZkcCrashed,
    }

    const commitment = try reportedBytes(arena, text, "guest_output");
    const frame = try reportedBytes(arena, text, "guest_log");
    const actual = try std.mem.concat(arena, u8, &.{ commitment, frame });
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

/// The bytes of the `<name> = 0x...` line of zkc's report; exactly one such line holds an even
/// number of hex digits.
fn reportedBytes(arena: std.mem.Allocator, text: []const u8, comptime name: []const u8) ![]u8 {
    const assignment = name ++ " =";
    const prefix = name ++ " = 0x";
    var hex: ?[]const u8 = null;
    var lines = std.mem.splitScalar(u8, text, '\n');
    while (lines.next()) |raw_line| {
        const line = std.mem.trim(u8, raw_line, " \t\r");
        if (!std.mem.startsWith(u8, line, assignment)) continue;
        if (hex != null) return error.DuplicateReportLine;
        if (!std.mem.startsWith(u8, line, prefix)) return error.InvalidReportLine;
        hex = line[prefix.len..];
    }
    const digits = hex orelse return error.MissingReportLine;
    if (digits.len % 2 != 0) return error.InvalidReportLine;
    const bytes = try arena.alloc(u8, digits.len / 2);
    _ = std.fmt.hexToBytes(bytes, digits) catch return error.InvalidReportLine;
    return bytes;
}

fn fatal(message: []const u8) noreturn {
    std.debug.print("error: {s}\n{s}", .{ message, usage });
    std.process.exit(2);
}
