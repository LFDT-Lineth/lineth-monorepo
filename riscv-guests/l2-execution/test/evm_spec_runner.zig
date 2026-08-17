//! `evm-execution-spec-runner` — runs the **vanilla** l2-execution guest against the EF
//! execution-spec-tests zkevm stateless fixtures.
//!
//! Wired by build.zig's `spec-tests` step, which passes the lazy `execution_spec_tests_zkevm`
//! dependency's `blockchain_tests/` directory as `--fixtures`. All the corpus walking / parsing /
//! reporting lives in the guest-agnostic `spec_runner.zig`; this file only supplies the vanilla
//! `Adapter` and the CLI. A future extended guest adds its own adapter + entry and reuses
//! `spec_runner.zig` unchanged.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const guest = @import("evm_execution_guest");

/// Vanilla adapter: the guest consumes the EF `SszStatelessInput` verbatim (identity), and the
/// expected result is the fixture's 105-byte `SszStatelessValidationResult`. An extended guest
/// would instead decode the StatelessInput, wrap it with its extra fields, and re-encode in
/// `adaptInput` — and may compute its own expectation in `runAndCheck`.
const VanillaAdapter = struct {
    pub const label = "vanilla l2-execution";

    pub fn adaptInput(
        alloc: std.mem.Allocator,
        ssz_stateless_input: []const u8,
        ctx: spec_runner.BlockContext,
    ) ![]const u8 {
        _ = alloc;
        _ = ctx;
        return ssz_stateless_input; // identity — the vanilla guest's input IS the EF StatelessInput
    }

    pub fn runAndCheck(
        alloc: std.mem.Allocator,
        guest_input: []const u8,
        expected_output: []const u8,
        ctx: spec_runner.BlockContext,
    ) !bool {
        const result = guest.runStateless(alloc, guest_input) catch |err| {
            std.debug.print("FAIL {s}[{}]  guest error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            return false;
        };

        if (expected_output.len != result.out.len) {
            std.debug.print(
                "FAIL {s}[{}]  expected {} output bytes, guest produced {}\n",
                .{ ctx.test_name, ctx.block_index, expected_output.len, result.out.len },
            );
            return false;
        }
        if (std.mem.eql(u8, &result.out, expected_output)) return true;

        // Byte 32 of the SSZ result is the successful_validation flag — surface valid/invalid
        // disagreements specially, since those are the most common and most informative.
        const got_valid = result.out[32] == 0x01;
        const exp_valid = expected_output[32] == 0x01;
        if (got_valid != exp_valid) {
            std.debug.print("FAIL {s}[{}]  expected {s}, guest said {s}\n", .{
                ctx.test_name,
                ctx.block_index,
                if (exp_valid) "valid" else "invalid",
                if (got_valid) "valid" else "invalid",
            });
        } else {
            var exp_arr: @TypeOf(result.out) = undefined; // same fixed size as the guest result
            @memcpy(&exp_arr, expected_output);
            const got_hex = std.fmt.bytesToHex(result.out, .lower);
            const exp_hex = std.fmt.bytesToHex(exp_arr, .lower);
            std.debug.print(
                "FAIL {s}[{}]  output mismatch\n  got:      0x{s}\n  expected: 0x{s}\n",
                .{ ctx.test_name, ctx.block_index, &got_hex, &exp_hex },
            );
        }
        return false;
    }
};

const usage =
    \\evm-execution-spec-runner — run the vanilla l2-execution guest against EF zkevm stateless fixtures.
    \\
    \\usage: evm-execution-spec-runner [--fixtures DIR] [--file FILE] [--fork NAME] [--limit N] [-x] [--report-only]
    \\  --fixtures DIR   directory of blockchain_tests JSON fixtures (passed by `zig build spec-tests`)
    \\  --file FILE      run a single fixture file instead of the whole directory
    \\  --fork NAME      only run test cases whose network == NAME (case-insensitive), e.g. Amsterdam
    \\  --match SUBSTR   only run fixture files whose path contains SUBSTR, e.g. block_access_lists
    \\  --limit N        stop after N blocks (dev speed)
    \\  -x               stop on the first failing block
    \\  --report-only    print the summary but always exit 0 (otherwise: exit 1 if any block fails)
    \\
;

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

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

    std.debug.print("running {s} guest over {s}\n", .{ VanillaAdapter.label, opts.single_file orelse opts.fixtures_dir });

    const stats = try spec_runner.run(VanillaAdapter, init.io, gpa, opts);

    const total = stats.total();
    const pct: u64 = if (total > 0) 100 * stats.passed / total else 0;
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  {s}\n", .{VanillaAdapter.label});
    std.debug.print("  files: {}   blocks: {}   passed: {}   failed: {}   ({}%)\n", .{
        stats.files, stats.blocks, stats.passed, stats.failed, pct,
    });
    std.debug.print("============================================================\n", .{});

    if (stats.failed > 0 and !report_only) std.process.exit(1);
}
