//! `l2-execution-runner` — native host tool for the extended l2-execution guest.
//!
//! Reads an SSZ-encoded, extended `L2ExecutionProofPrivateInput` file (schema 0x0002 — the same
//! wire format the zkVM guest itself reads via `read_input`/`l2_execution_ssz.decodeInput`), runs
//! `l2_execution.runL2Execution`, and prints the result to stdout.
//!
//! This is deliberately the ONLY place an SSZ/JSON output toggle exists for this guest: the
//! freestanding riscv64 guest (`evm_execution_guest.zig`'s `guestMain`) has no argv — its I/O is
//! pinned to the proving system's `read_input`/`write_output` ABI, so it always emits SSZ. The
//! runtime `--json`/`--ssz` choice belongs on this native runner instead, mirroring zesu's
//! `zevm_stateless --ssz`/`--json` CLI.
//!
//! Output defaults to SSZ (`l2_execution_ssz.encodeOutput`, schema 0x0003 — byte-identical to what
//! the guest would itself emit via `write_output` for the same input); `--json` switches to the
//! `getZkL2ExecutionProofV1.response.json`-shaped output instead (`l2_execution_json`). `--ssz` is
//! accepted explicitly too, as a no-op.

const std = @import("std");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const l2_execution_json = @import("l2_execution_json");

const OutputFormat = enum { ssz, json };

const usage =
    \\l2-execution-runner — run the extended l2-execution guest logic (runL2Execution) against an
    \\SSZ-encoded L2ExecutionProofPrivateInput file (schema 0x0002) and print the result to stdout.
    \\
    \\usage: l2-execution-runner <input.ssz> [--json | --ssz]
    \\  <input.ssz>  path to an SSZ-encoded, schema-0x0002 extended guest input
    \\  --json       print the output as getZkL2ExecutionProofV1.response.json-shaped JSON
    \\  --ssz        print the output as SSZ (schema 0x0003) — the default; accepted explicitly too
    \\
;

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    var input_path: ?[]const u8 = null;
    var format: OutputFormat = .ssz;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--json")) {
            format = .json;
        } else if (std.mem.eql(u8, arg, "--ssz")) {
            format = .ssz;
        } else if (std.mem.eql(u8, arg, "-h") or std.mem.eql(u8, arg, "--help")) {
            std.debug.print("{s}", .{usage});
            return;
        } else if (std.mem.startsWith(u8, arg, "--")) {
            std.debug.print("error: unexpected argument '{s}'\n{s}", .{ arg, usage });
            std.process.exit(2);
        } else if (input_path == null) {
            input_path = arg;
        } else {
            std.debug.print("error: unexpected extra argument '{s}'\n{s}", .{ arg, usage });
            std.process.exit(2);
        }
    }

    const path = input_path orelse {
        std.debug.print("error: missing <input.ssz>\n{s}", .{usage});
        std.process.exit(2);
    };

    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const alloc = arena.allocator();

    const raw_input = std.Io.Dir.cwd().readFileAlloc(init.io, path, alloc, .limited(1 << 30)) catch |err| {
        std.debug.print("error: cannot read '{s}': {s}\n", .{ path, @errorName(err) });
        std.process.exit(1);
    };

    const decoded = l2_execution_ssz.decodeInput(alloc, raw_input) catch |err| {
        std.debug.print("error: failed to decode extended input (expected schema 0x0002): {s}\n", .{@errorName(err)});
        std.process.exit(1);
    };

    // NOTE: the committed `getZkL2ExecutionProofV1.input.ssz` golden vector is a SYNTHETIC,
    // hand-authored fixture with dummy witnesses (stateRoot=0x1111...) — real execution can never
    // reproduce it, so `runL2Execution` erroring here (e.g. `error.InvalidWitness`) against that
    // vector specifically is expected. This runner's job is to fail cleanly (non-zero exit, no
    // panic) on any input it can't process, real or synthetic.
    const result = l2_execution.runL2Execution(alloc, decoded) catch |err| {
        std.debug.print("error: runL2Execution failed: {s}\n", .{@errorName(err)});
        std.process.exit(1);
    };

    const out_bytes = switch (format) {
        .ssz => l2_execution_ssz.encodeOutput(alloc, result) catch |err| {
            std.debug.print("error: failed to SSZ-encode output: {s}\n", .{@errorName(err)});
            std.process.exit(1);
        },
        .json => l2_execution_json.encodeOutputJson(alloc, result) catch |err| {
            std.debug.print("error: failed to JSON-encode output: {s}\n", .{@errorName(err)});
            std.process.exit(1);
        },
    };

    std.Io.File.stdout().writeStreamingAll(init.io, out_bytes) catch |err| {
        std.debug.print("error: failed to write output: {s}\n", .{@errorName(err)});
        std.process.exit(1);
    };
    if (format == .json) {
        std.Io.File.stdout().writeStreamingAll(init.io, "\n") catch {};
    }
}
