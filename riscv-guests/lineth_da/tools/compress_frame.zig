//! `compress_frame` — compresses a payload into one zstd frame against the package dictionary.
//!
//! usage: compress_frame <frame-out> <fixtures-dir> [<payload-in>]
//!
//! The payload is the file `payload-in`, or the canonical payload of the first three valid blocks of
//! the EF zkevm fixtures in `fixtures-dir`, the directory that holds `blockchain_tests/`. The
//! frame is raw bytes; a `.ssz`-suffixed output is framed with the 8-byte length prefix
//! `read_input` expects when it is passed to `make exec INPUT=...`.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");

const usage = "usage: compress_frame <frame-out> <fixtures-dir> [<payload-in>]\n";

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len < 3 or args.len > 4) {
        std.debug.print("error: expected <frame-out> <fixtures-dir> [<payload-in>]\n{s}", .{usage});
        std.process.exit(2);
    }

    const arena = init.arena.allocator();
    const payload = if (args.len == 4)
        std.Io.Dir.cwd().readFileAlloc(init.io, args[3], arena, .limited(1 << 30)) catch |err| {
            std.debug.print("error: cannot read '{s}': {s}\n", .{ args[3], @errorName(err) });
            std.process.exit(1);
        }
    else blk: {
        const corpus_dir = try std.fs.path.join(arena, &.{ args[2], "blockchain_tests", "for_amsterdam" });
        var blocks = try ef_conflations.Blocks.open(init.gpa, init.io, corpus_dir);
        defer blocks.deinit();
        break :blk try blocks.conflationPayload(arena, 3);
    };

    const frame = try zstd_compressor.compress(init.gpa, payload, lineth_da.dictionary_v2025_04_21);
    defer init.gpa.free(frame);

    std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[1], .data = frame }) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ args[1], @errorName(err) });
        std.process.exit(1);
    };
}
