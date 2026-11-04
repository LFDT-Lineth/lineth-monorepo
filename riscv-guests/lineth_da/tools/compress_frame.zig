//! `compress_frame` — builds the input of the `zstd` smoke guest: a payload compressed into one zstd
//! frame against the package dictionary, behind the payload's length as four big-endian bytes.
//!
//! usage: compress_frame <input-out> <fixtures-dir> [<payload-in>]
//!
//! The payload is the file `payload-in`, or the canonical payload of the conflation `sample` of the
//! block selection of the EF zkevm fixtures in `fixtures-dir`, the directory that holds
//! `blockchain_tests/`. The output is raw bytes; a `.ssz`-suffixed output is framed with the 8-byte
//! length prefix `read_input` expects when it is passed to `make exec INPUT=...`.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");

const usage = "usage: compress_frame <input-out> <fixtures-dir> [<payload-in>]\n";

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len < 3 or args.len > 4) {
        std.debug.print("error: expected <input-out> <fixtures-dir> [<payload-in>]\n{s}", .{usage});
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
        var selection = try ef_conflations.Selection.load(init.gpa, init.io, corpus_dir);
        defer selection.deinit();
        break :blk try selection.conflationPayload(arena, &ef_conflations.sample);
    };

    const encoder = try zstd_encoder.Encoder.init(init.gpa, lineth_da.dictionary_v2025_04_21);
    defer encoder.deinit();
    const frame = try encoder.compress(init.gpa, payload);
    defer init.gpa.free(frame);

    const input = try init.gpa.alloc(u8, 4 + frame.len);
    defer init.gpa.free(input);
    std.mem.writeInt(u32, input[0..4], @intCast(payload.len), .big);
    @memcpy(input[4..], frame);

    std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[1], .data = input }) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ args[1], @errorName(err) });
        std.process.exit(1);
    };
}
