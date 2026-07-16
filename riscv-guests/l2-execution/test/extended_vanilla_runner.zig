//! `extended-vanilla-runner` — reference-test guard: the extended guest, run through the dummy-fill
//! wrap (`vanilla_wrap.wrapVanillaAsExtended`), must agree with the EF fixture's OWN expected
//! validity verdict (`successful_validation`, byte 32 of the vanilla `SszStatelessValidationResult`
//! — see zesu's `ssz_output.zig`) over the real EF zkevm corpus. This is the property
//! `run-execution-specs-ssz-fixtures` depends on, checked here cheaply on the host instead of via
//! ZkC.
//!
//! Deliberately NOT a differential check against a second, independently-run implementation (e.g.
//! re-running zesu's own `executor.executeStatelessInput` inline): a reference-test corpus's whole
//! point is to BE the source of truth, so this checks the extended guest against the fixture's own
//! expected result directly — that also catches a bug shared by both the extended and vanilla
//! paths (they delegate to the same zesu executor), which a vanilla-vs-extended differential check
//! never could, since both sides would agree while still being wrong.
//!
//! The allowed disagreements are `error.ExecutionRequestsNotSupported` (EF fixtures carrying
//! EIP-7685 requests) and `error.WithdrawalsNotSupported` (EF fixtures carrying beacon-chain
//! withdrawals) — both valid to vanilla Ethereum but rejected by Lineth policy. Any other
//! disagreement fails the run.
//!
//! Reuses `spec_runner.zig`'s fixture walk; this file supplies only the `Adapter`, CLI, and
//! histogram. Run via `zig build extended-vanilla`.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const vanilla_wrap = @import("vanilla_wrap");

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
    pub const label = "extended guest vs EF fixture ground truth (dummy-wrapped l2_execution.runL2Execution vs the fixture's own successful_validation)";

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
        // Ground truth: the fixture's OWN expected result, not a second, independently-run
        // implementation (see the file header comment for why).
        if (expected_output.len <= 32) {
            std.debug.print("FAIL {s}[{}]  expected_output too short ({} bytes)\n", .{ ctx.test_name, ctx.block_index, expected_output.len });
            return false;
        }
        const expected_valid = expected_output[32] == 0x01;

        const extended_in = l2_execution_ssz.decodeInput(alloc, guest_input) catch |err| {
            std.debug.print("FAIL {s}[{}]  extended decodeInput error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            return false;
        };
        // The vanilla bytes are carried verbatim as the single payload's stateless_input_ssz.
        std.debug.assert(extended_in.payloads.len == 1);

        var extended_err_name: []const u8 = "";
        const extended_valid = blk: {
            _ = l2_execution.runL2Execution(alloc, extended_in) catch |err| {
                extended_err_name = @errorName(err);
                break :blk false;
            };
            break :blk true;
        };

        if (extended_valid == expected_valid) return true;

        // The allowed disagreements (see the file header comment): a fixture-valid block the
        // extended guest rejects for a Lineth-policy reason (EIP-7685 requests or withdrawals).
        if (!extended_valid and expected_valid and
            (std.mem.eql(u8, extended_err_name, "ExecutionRequestsNotSupported") or
                std.mem.eql(u8, extended_err_name, "WithdrawalsNotSupported")))
        {
            return true;
        }

        if (extended_valid) {
            std.debug.print(
                "FAIL {s}[{}]  disagree: fixture=invalid extended=valid\n",
                .{ ctx.test_name, ctx.block_index },
            );
        } else {
            recordError(extended_err_name);
            std.debug.print(
                "FAIL {s}[{}]  disagree: fixture=valid extended=invalid ({s})\n",
                .{ ctx.test_name, ctx.block_index, extended_err_name },
            );
        }
        return false;
    }
};

const usage =
    \\extended-vanilla-runner — reference-test guard: assert the dummy-wrapped extended l2-execution guest
    \\(l2_execution.runL2Execution) agrees with the EF fixture's own expected validity verdict, over
    \\EF zkevm stateless fixtures. The only allowed disagreement is a fixture-valid block whose
    \\EIP-7685 execution requests the extended guest rejects by Lineth policy.
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
