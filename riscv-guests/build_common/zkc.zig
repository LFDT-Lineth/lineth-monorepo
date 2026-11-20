//! Runs a guest ELF under zkc through the arithmetization Makefile and reads zkc's report. zkc
//! prints the output's 32-byte commitment as `guest_output` and the frame after it as `guest_log`.

const std = @import("std");

/// The arithmetization Makefile target that runs the ELF.
pub const Target = enum { @"elf-exec", @"elf-trace" };

pub const Options = struct {
    /// The arithmetization Makefile, which defines the zkc run.
    makefile: []const u8,
    target: Target = .@"elf-exec",
    elf: []const u8,
    /// The file holding the guest's input.
    input: []const u8,
    /// Where the ELF's JSON form goes; the Makefile's default sits beside the ELF.
    json: ?[]const u8 = null,
    /// Flags for `zkc exec` or `zkc trace`, whichever `target` runs.
    flags: []const u8 = "",
};

pub const Run = struct {
    term: std.process.Child.Term,
    /// zkc's stdout followed by its stderr.
    report: []const u8,
};

pub fn run(alloc: std.mem.Allocator, io: std.Io, options: Options) !Run {
    var argv = std.ArrayList([]const u8).empty;
    try argv.append(alloc, "make");
    try argv.append(alloc, "-s");
    try argv.append(alloc, try std.fmt.allocPrint(alloc, "-f{s}", .{options.makefile}));
    try argv.append(alloc, @tagName(options.target));
    try argv.append(alloc, try std.fmt.allocPrint(alloc, "BIN_EXT={s}", .{options.elf}));
    try argv.append(alloc, try std.fmt.allocPrint(alloc, "IN_BYTES=@{s}", .{options.input}));
    if (options.json) |json| try argv.append(alloc, try std.fmt.allocPrint(alloc, "JSON_EXT={s}", .{json}));
    const flags_variable = switch (options.target) {
        .@"elf-exec" => "ZKC_EXEC_FLAGS",
        .@"elf-trace" => "ZKC_TRACE_FLAGS",
    };
    try argv.append(alloc, try std.fmt.allocPrint(alloc, "{s}={s}", .{ flags_variable, options.flags }));

    const process = try std.process.run(alloc, io, .{ .argv = argv.items });
    return .{
        .term = process.term,
        .report = try std.fmt.allocPrint(alloc, "{s}\n{s}", .{ process.stdout, process.stderr }),
    };
}

pub const OutputError = error{
    MissingGuestOutput,
    MissingGuestLog,
    DuplicateGuestOutput,
    DuplicateGuestLog,
    InvalidGuestOutput,
    InvalidGuestLog,
    InvalidGuestOutputSize,
    InvalidGuestLogSize,
    InvalidGuestOutputLogSize,
    OutOfMemory,
};

const guest_output_assignment = "guest_output =";
const guest_output_prefix = "guest_output = 0x";
const guest_log_assignment = "guest_log =";
const guest_log_prefix = "guest_log = 0x";
const min_output_size: usize = 34;

/// The commitment of `guest_output` followed by the frame of `guest_log`.
pub fn guestOutput(alloc: std.mem.Allocator, report: []const u8) OutputError![]u8 {
    var guest_output_hex: ?[]const u8 = null;
    var guest_log_hex: ?[]const u8 = null;
    var lines = std.mem.splitScalar(u8, report, '\n');
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
    const output = try alloc.alloc(u8, (output_hex.len + log_hex.len) / 2);
    errdefer alloc.free(output);
    if (output.len < min_output_size) return error.InvalidGuestOutputLogSize;
    const split = output_hex.len / 2;
    _ = std.fmt.hexToBytes(output[0..split], output_hex) catch return error.InvalidGuestOutput;
    _ = std.fmt.hexToBytes(output[split..], log_hex) catch return error.InvalidGuestLog;
    return output;
}

/// The value of the only `EXIT CODE = N` line; null when there is no such line, more than one, or
/// its value is not a decimal u64.
pub fn guestExitCode(report: []const u8) ?u64 {
    const exit_prefix = "EXIT CODE = ";
    var exit_code: ?u64 = null;
    var lines = std.mem.splitScalar(u8, report, '\n');
    while (lines.next()) |raw_line| {
        const line = std.mem.trim(u8, raw_line, " \t\r");
        if (!std.mem.startsWith(u8, line, exit_prefix)) continue;
        if (exit_code != null) return null;
        exit_code = std.fmt.parseInt(u64, line[exit_prefix.len..], 10) catch return null;
    }
    return exit_code;
}
