//! `l2-execution-wrap` — native host tool that wraps a VANILLA EF stateless input (schema 0x0001)
//! into an EXTENDED `L2ExecutionProofPrivateInput` (schema 0x0002) the extended l2-execution guest
//! reads via `read_input`.
//!
//! This is the ZkC-harness counterpart of the in-process `vanilla_wrap.wrapVanillaAsExtended` used
//! by the host parity regression guard (`test/extended_vanilla_runner.zig`): both share the SAME fill
//! semantics (single payload carrying the vanilla bytes verbatim, empty FTX, zero parent FTX
//! fields, chain_id/coinbase read off the vanilla payload, and — crucially — a ZERO
//! l2_message_service_address, which triggers the guest's bridge-suppression branch so the extended
//! guest runs equivalently to the vanilla stateless engine).
//!
//! usage: l2-execution-wrap <vanilla-in.ssz> <extended-out.ssz>
//!
//! The `run_execution_specs_ssz_fixtures.go` harness invokes this to produce the wrapped input it
//! feeds to the guest, replacing the raw vanilla `.ssz` it used to write when the guest still
//! consumed vanilla input directly.

const std = @import("std");
const vanilla_wrap = @import("vanilla_wrap");

const usage =
    \\l2-execution-wrap — wrap a vanilla stateless-input SSZ (schema 0x0001) into an extended
    \\L2ExecutionProofPrivateInput SSZ (schema 0x0002) with dummy rollup fields (zero
    \\l2MessageServiceAddress -> bridge suppression), so the extended guest runs it like vanilla.
    \\
    \\usage: l2-execution-wrap <vanilla-in.ssz> <extended-out.ssz>
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

    // EIP-7685 execution requests are rejected by Linea policy inside the guest
    // (error.ExecutionRequestsNotSupported), so wrapping + proving such a fixture is pointless.
    // Signal the harness to SKIP via a dedicated exit code (3) rather than producing output.
    const has_requests = vanilla_wrap.vanillaHasExecutionRequests(alloc, vanilla) catch |err| {
        std.debug.print("error: failed to inspect '{s}': {s}\n", .{ in_path, @errorName(err) });
        std.process.exit(1);
    };
    if (has_requests) {
        std.debug.print("skip: '{s}' carries EIP-7685 execution requests (unsupported by Linea policy)\n", .{in_path});
        std.process.exit(3);
    }

    const wrapped = vanilla_wrap.wrapVanillaAsExtended(alloc, vanilla) catch |err| {
        std.debug.print("error: failed to wrap '{s}': {s}\n", .{ in_path, @errorName(err) });
        std.process.exit(1);
    };

    const file = std.Io.Dir.cwd().createFile(init.io, out_path, .{}) catch |err| {
        std.debug.print("error: cannot create '{s}': {s}\n", .{ out_path, @errorName(err) });
        std.process.exit(1);
    };
    defer file.close(init.io);
    file.writeStreamingAll(init.io, wrapped) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ out_path, @errorName(err) });
        std.process.exit(1);
    };
}
