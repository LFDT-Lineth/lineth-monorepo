//! `l2-execution-adapt` — native host tool: vanilla BPO2 (or other pre-Amsterdam) stateless-input
//! SSZ → Amsterdam-rewritten extended `L2ExecutionProofPrivateInput` (schema 0x0002).
//!
//! See `amsterdam_adapt.zig` for the rewrite rules (strip withdrawals, relabel fork, recompute
//! state/receipts roots under Amsterdam, synthesise EIP-7928 BAL, dummy-wrap).
//!
//! usage: l2-execution-adapt <vanilla-in.ssz> <extended-out.ssz>

const std = @import("std");
const amsterdam_adapt = @import("amsterdam_adapt");

const usage =
    \\l2-execution-adapt — rewrite a vanilla stateless-input SSZ (e.g. mainnet BPO2, schema 0x1401)
    \\as an Amsterdam L2-compatible extended input (schema 0x0002): strip beacon withdrawals,
    \\re-execute under Amsterdam, patch state/receipts roots + BAL, dummy-wrap.
    \\
    \\usage: l2-execution-adapt <vanilla-in.ssz> <extended-out.ssz>
    \\
;

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    if (args.len == 2 and (std.mem.eql(u8, args[1], "-h") or std.mem.eql(u8, args[1], "--help"))) {
        std.debug.print("{s}", .{usage});
        return;
    }
    if (args.len != 3) {
        std.debug.print("error: expected <vanilla-in.ssz> <extended-out.ssz>\n{s}", .{usage});
        std.process.exit(2);
    }
    const in_path = args[1];
    const out_path = args[2];

    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const alloc = arena.allocator();

    const vanilla = std.Io.Dir.cwd().readFileAlloc(init.io, in_path, alloc, .limited(1 << 30)) catch |err| {
        std.debug.print("error: cannot read '{s}': {s}\n", .{ in_path, @errorName(err) });
        std.process.exit(1);
    };

    const adapted = amsterdam_adapt.adaptVanillaToAmsterdamExtended(alloc, vanilla) catch |err| {
        std.debug.print("error: failed to adapt '{s}': {s}\n", .{ in_path, @errorName(err) });
        std.process.exit(1);
    };

    const file = std.Io.Dir.cwd().createFile(init.io, out_path, .{}) catch |err| {
        std.debug.print("error: cannot create '{s}': {s}\n", .{ out_path, @errorName(err) });
        std.process.exit(1);
    };
    defer file.close(init.io);
    file.writeStreamingAll(init.io, adapted) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ out_path, @errorName(err) });
        std.process.exit(1);
    };
}
