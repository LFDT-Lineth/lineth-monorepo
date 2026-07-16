//! `extended-vanilla-runner` — regression guard: the extended guest, run through the dummy-fill
//! wrap (`vanilla_wrap.wrapVanillaAsExtended`), must agree with the vanilla guest on validity over
//! the real EF zkevm corpus. This is the property `run-execution-specs-ssz-fixtures` depends on,
//! checked here cheaply on the host instead of via ZkC.
//!
//! The vanilla-side check reimplements `evm_execution_guest.runStateless` inline rather than
//! importing it: that module relative-imports `l2_execution.zig`, so combining it with the
//! `l2_execution` module here in one compile unit is a Zig module-graph conflict.
//!
//! The allowed disagreements are `error.ExecutionRequestsNotSupported` (EF fixtures carrying
//! EIP-7685 requests) and `error.WithdrawalsNotSupported` (EF fixtures carrying beacon-chain
//! withdrawals) — both valid to vanilla Ethereum but rejected by Linea policy. Any other
//! disagreement fails the run.
//!
//! Reuses `spec_runner.zig`'s fixture walk; this file supplies only the `Adapter`, CLI, and
//! histogram. Run via `zig build extended-vanilla`.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const vanilla_wrap = @import("vanilla_wrap");
const executor = @import("zesu_executor");
const ssz_decode = @import("zesu_ssz_decode");
const zesu_allocator = @import("zesu_allocator");

/// Error-name -> occurrence count, across every disagreement where the extended pipeline errored.
/// File-scope: the comptime `Adapter` contract has no room for extra per-run state, and this tool
/// runs its whole walk from a single `main()` invocation, so there is only ever one "session".
var error_histogram: std.StringHashMap(u64) = undefined;

fn recordError(name: []const u8) void {
    const entry = error_histogram.getOrPut(name) catch return;
    if (!entry.found_existing) entry.value_ptr.* = 0;
    entry.value_ptr.* += 1;
}

const ExtendedVanillaAdapter = struct {
    pub const label = "extended-vs-vanilla validity parity (dummy-wrapped l2_execution.runL2Execution vs guest.runStateless)";

    pub fn adaptInput(
        alloc: std.mem.Allocator,
        ssz_stateless_input: []const u8,
        ctx: spec_runner.BlockContext,
    ) ![]const u8 {
        _ = ctx;
        return vanilla_wrap.wrapVanillaAsExtended(alloc, ssz_stateless_input);
    }

    pub fn runAndCheck(
        alloc: std.mem.Allocator,
        guest_input: []const u8,
        expected_output: []const u8,
        ctx: spec_runner.BlockContext,
    ) !bool {
        _ = expected_output; // this guard compares extended vs vanilla, not either against the EF expectation

        const extended_in = l2_execution_ssz.decodeInput(alloc, guest_input) catch |err| {
            std.debug.print("FAIL {s}[{}]  extended decodeInput error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            return false;
        };
        // The vanilla bytes are carried verbatim as the single payload's stateless_input_ssz.
        std.debug.assert(extended_in.payloads.len == 1);
        const vanilla_ssz = extended_in.payloads[0].stateless_input_ssz;

        // Vanilla validity check — copy-identical to `evm_execution_guest.runStateless` (see this
        // file's header comment for why it's reimplemented here instead of imported).
        zesu_allocator.set(alloc);
        const vanilla_si = ssz_decode.decode(alloc, vanilla_ssz) catch |err| {
            std.debug.print("FAIL {s}[{}]  vanilla ssz decode error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            return false;
        };
        const vanilla_ep = &vanilla_si.new_payload_request.execution_payload;
        zesu_allocator.set(alloc);
        const vanilla_valid = blk: {
            const proof = executor.executeStatelessInput(alloc, vanilla_si, vanilla_si.chain_config.fork_name) catch break :blk false;
            if (!std.mem.eql(u8, &proof.post_state_root, &vanilla_ep.state_root)) break :blk false;
            if (!std.mem.eql(u8, &proof.receipts_root, &vanilla_ep.receipts_root)) break :blk false;
            break :blk true;
        };

        var extended_err_name: []const u8 = "";
        const extended_valid = blk: {
            _ = l2_execution.runL2Execution(alloc, extended_in) catch |err| {
                extended_err_name = @errorName(err);
                break :blk false;
            };
            break :blk true;
        };

        if (extended_valid == vanilla_valid) return true;

        // The allowed disagreements (see the file header comment): a vanilla-valid block the
        // extended guest rejects for a Linea-policy reason (EIP-7685 requests or withdrawals).
        if (!extended_valid and vanilla_valid and
            (std.mem.eql(u8, extended_err_name, "ExecutionRequestsNotSupported") or
                std.mem.eql(u8, extended_err_name, "WithdrawalsNotSupported")))
        {
            return true;
        }

        if (extended_valid) {
            std.debug.print(
                "FAIL {s}[{}]  disagree: vanilla=invalid extended=valid\n",
                .{ ctx.test_name, ctx.block_index },
            );
        } else {
            recordError(extended_err_name);
            std.debug.print(
                "FAIL {s}[{}]  disagree: vanilla=valid extended=invalid ({s})\n",
                .{ ctx.test_name, ctx.block_index, extended_err_name },
            );
        }
        return false;
    }
};

const usage =
    \\extended-vanilla-runner — regression guard: assert the dummy-wrapped extended l2-execution guest
    \\(l2_execution.runL2Execution) agrees with the vanilla guest (guest.runStateless) on block
    \\validity, over EF zkevm stateless fixtures. The only allowed disagreement is a vanilla-valid
    \\block whose EIP-7685 execution requests the extended guest rejects by Linea policy.
    \\
    \\usage: extended-vanilla-runner [--fixtures DIR] [--file FILE] [--fork NAME] [--match SUBSTR] [--limit N] [-x] [--report-only]
    \\  --fixtures DIR   directory of blockchain_tests JSON fixtures (passed by `zig build extended-vanilla`)
    \\  --file FILE      run a single fixture file instead of the whole directory
    \\  --fork NAME      only run test cases whose network == NAME (case-insensitive), e.g. Amsterdam
    \\  --match SUBSTR   only run fixture files whose path contains SUBSTR, e.g. eip7928_block_level_access_lists
    \\  --limit N        stop after N blocks (dev speed)
    \\  -x               stop on the first disagreeing block
    \\  --report-only    print the summary but always exit 0 (otherwise: exit 1 if any block disagrees)
    \\
;

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    error_histogram = std.StringHashMap(u64).init(gpa);
    defer error_histogram.deinit();

    var opts = spec_runner.Options{ .fixtures_dir = "spec-tests/fixtures/zkevm/blockchain_tests" };
    var report_only = false;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--fixtures") and i + 1 < args.len) {
            i += 1;
            opts.fixtures_dir = args[i];
        } else if (std.mem.eql(u8, arg, "--file") and i + 1 < args.len) {
            i += 1;
            opts.single_file = args[i];
        } else if (std.mem.eql(u8, arg, "--fork") and i + 1 < args.len) {
            i += 1;
            opts.fork_filter = args[i];
        } else if (std.mem.eql(u8, arg, "--match") and i + 1 < args.len) {
            i += 1;
            opts.path_match = args[i];
        } else if (std.mem.eql(u8, arg, "--limit") and i + 1 < args.len) {
            i += 1;
            opts.limit = std.fmt.parseInt(u64, args[i], 10) catch {
                std.debug.print("error: --limit expects an integer, got '{s}'\n", .{args[i]});
                std.process.exit(2);
            };
        } else if (std.mem.eql(u8, arg, "-x")) {
            opts.stop_on_fail = true;
        } else if (std.mem.eql(u8, arg, "--report-only")) {
            report_only = true;
        } else if (std.mem.eql(u8, arg, "-h") or std.mem.eql(u8, arg, "--help")) {
            std.debug.print("{s}", .{usage});
            return;
        } else {
            std.debug.print("error: unexpected argument '{s}'\n{s}", .{ arg, usage });
            std.process.exit(2);
        }
    }

    std.debug.print("running {s}\n  over {s}\n", .{ ExtendedVanillaAdapter.label, opts.single_file orelse opts.fixtures_dir });

    const stats = try spec_runner.run(ExtendedVanillaAdapter, init.io, gpa, opts);

    const total = stats.total();
    const pct: u64 = if (total > 0) 100 * stats.passed / total else 0;
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  {s}\n", .{ExtendedVanillaAdapter.label});
    std.debug.print("  files: {}   blocks: {}   agree: {}   disagree: {}   ({}%)\n", .{
        stats.files, stats.blocks, stats.passed, stats.failed, pct,
    });
    if (error_histogram.count() > 0) {
        std.debug.print("  disagreement error histogram (extended pipeline's error, when it errored):\n", .{});
        var it = error_histogram.iterator();
        while (it.next()) |entry| {
            std.debug.print("    {s}: {}\n", .{ entry.key_ptr.*, entry.value_ptr.* });
        }
    }
    std.debug.print("============================================================\n", .{});

    if (stats.failed > 0 and !report_only) std.process.exit(1);
}
