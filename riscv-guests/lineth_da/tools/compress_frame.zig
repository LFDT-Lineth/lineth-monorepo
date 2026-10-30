//! `compress_frame` — compresses a file into one zstd frame against the package dictionary.
//!
//! usage: compress_frame <payload-in> <frame-out> [level]
//!
//! The frame is raw bytes; a `.ssz`-suffixed output is framed with the 8-byte length prefix
//! `read_input` expects when it is passed to `make exec INPUT=...`.

const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");

const usage = "usage: compress_frame <payload-in> <frame-out> [level]\n";

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len != 3 and args.len != 4) {
        std.debug.print("error: expected <payload-in> <frame-out> [level]\n{s}", .{usage});
        std.process.exit(2);
    }
    const level = if (args.len == 4) try std.fmt.parseInt(i32, args[3], 10) else 3;

    const payload = std.Io.Dir.cwd().readFileAlloc(init.io, args[1], init.gpa, .limited(1 << 30)) catch |err| {
        std.debug.print("error: cannot read '{s}': {s}\n", .{ args[1], @errorName(err) });
        std.process.exit(1);
    };
    defer init.gpa.free(payload);

    const frame = try zstd_compressor.compress(init.gpa, payload, .{
        .level = level,
        .dictionary = lineth_da.dictionary_v2025_04_21,
    });
    defer init.gpa.free(frame);

    std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[2], .data = frame }) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ args[2], @errorName(err) });
        std.process.exit(1);
    };
}
