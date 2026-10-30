//! `truncate_block` — truncates full block RLPs the way the rollup guest does.
//!
//! usage: truncate_block <chain-id> <block-rlp-file>...
//!
//! Prints one line per file: `ok <timestamp> <block-hash> <parent-hash> <prev-randao> <payload>`, with
//! the hashes and the canonical payload of the block's truncation in hex, or `reject <error>`.

const std = @import("std");

const lineth_da = @import("lineth_da");

const truncation = lineth_da.encoding.truncation;

const usage = "usage: truncate_block <chain-id> <block-rlp-file>...\n";

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len < 3) {
        std.debug.print("error: expected <chain-id> and at least one <block-rlp-file>\n{s}", .{usage});
        std.process.exit(2);
    }
    const chain_id = std.fmt.parseInt(u64, args[1], 10) catch {
        std.debug.print("error: chain ID '{s}' is not a 64-bit unsigned integer\n{s}", .{ args[1], usage });
        std.process.exit(2);
    };

    for (args[2..]) |path| {
        var arena = std.heap.ArenaAllocator.init(init.gpa);
        defer arena.deinit();
        var scratch = std.heap.ArenaAllocator.init(init.gpa);
        defer scratch.deinit();

        const block_rlp = std.Io.Dir.cwd().readFileAlloc(init.io, path, arena.allocator(), .limited(1 << 28)) catch |err| {
            std.debug.print("error: cannot read '{s}': {s}\n", .{ path, @errorName(err) });
            std.process.exit(1);
        };
        const line = try describe(arena.allocator(), scratch.allocator(), block_rlp, chain_id);
        try std.Io.File.stdout().writeStreamingAll(init.io, line);
    }
}

fn describe(output: std.mem.Allocator, scratch: std.mem.Allocator, block_rlp: []const u8, chain_id: u64) ![]const u8 {
    const block = truncation.truncateBlockRlp(output, scratch, block_rlp, chain_id) catch |err| {
        return std.fmt.allocPrint(output, "reject {s}\n", .{@errorName(err)});
    };
    const payload = try truncation.encodeBlocks(output, &.{block});
    return std.fmt.allocPrint(output, "ok {d} {x} {x} {x} {x}\n", .{
        block.timestamp,
        &block.block_hash,
        &block.parent_hash,
        &block.prev_randao,
        payload,
    });
}
