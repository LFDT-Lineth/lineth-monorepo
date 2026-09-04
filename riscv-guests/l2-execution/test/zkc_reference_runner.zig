//! `zkc-reference-runner` — runs the EF zkevm corpus through the compiled guest ELF under zkc,
//! instead of in-process. Ground truth, wrap, and skip semantics are the host reference-test's
//! (spec_runner.zig supplies the corpus walk and reporting); the only difference is the verdict
//! source: the guest's exit ecall surfaces through zkc as exit 0 on valid, or nonzero with an
//! `EXIT CODE = <n>` marker on reject. A nonzero exit without that marker is a toolchain failure,
//! not a verdict.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const vanilla_wrap = @import("vanilla_wrap");

const usage =
    \\zkc-reference-runner — run the EF zkevm corpus through the compiled guest ELF under zkc.
    \\
    \\usage: zkc-reference-runner --fixtures DIR --install-prefix DIR --makefile PATH [options]
    \\  --fixtures DIR        the blockchain_tests/ JSON tree (same as extended-vanilla-runner)
    \\  --install-prefix DIR  zig build install prefix; the guest ELF and l2-execution-wrap are
    \\                        resolved as DIR/bin/evm_execution_guest and DIR/bin/l2-execution-wrap
    \\  --makefile PATH       arithmetization test Makefile (defines the elf-exec target)
    \\  --zkc-flags S         flags for `zkc exec` (default: --fast)
    \\  --file FILE      run a single fixture file instead of walking the tree
    \\  --fork NAME      only fixtures declaring "network": "NAME" (case-insensitive)
    \\  --match SUBSTR   only fixture files whose path contains SUBSTR
    \\  --limit N        stop after N blocks (dev speed)
    \\  -x               stop on the first disagreeing block
    \\  --report-only    print the summary but always exit 0
    \\
;

// Session state for the zkc subprocess, set once in main before the walk. The comptime Adapter
// contract is stateless, so these live at file scope (same pattern as extended_vanilla_runner.zig's
// error_histogram). Wrapped SSZ + elf_to_json JSON scratch go under tmp_dir, removed on exit.
var elf: []const u8 = undefined;
var wrap: []const u8 = undefined;
var makefile: []const u8 = undefined;
/// `zkc exec` runs the guest on the tail-call interpreter backend, which reuses one frame for the
/// interpreter's per-instruction tail call. The gogen backend lowers that tail call to a native Go
/// call, spending one Go stack frame per emulated instruction, so a corpus-scale block exhausts
/// Go's 1 GB goroutine stack.
var zkc_flags: []const u8 = "--fast";
var tmp_dir: []const u8 = undefined;

const ZkcAdapter = struct {
    pub const label = "zkc reference-test: guest ELF under zkc vs EF fixture ground truth";

    /// The Linea-policy skips, all presence-detectable on the vanilla input (fork-activation
    /// schedule, EIP-7685 execution requests, beacon-chain withdrawals). Keeping them here means a
    /// wrapped block never policy-skips at run time: the wrap's exit 3 case is pre-empted.
    pub fn shouldSkip(
        alloc: std.mem.Allocator,
        ssz_stateless_input: []const u8,
        ctx: spec_runner.BlockContext,
    ) bool {
        _ = ctx;
        return (vanilla_wrap.vanillaHasForkActivationSchedule(alloc, ssz_stateless_input) catch false) or
            (vanilla_wrap.vanillaHasExecutionRequests(alloc, ssz_stateless_input) catch false) or
            (vanilla_wrap.vanillaHasWithdrawals(alloc, ssz_stateless_input) catch false);
    }

    /// The guest consumes the vanilla SSZ verbatim (the wrap happens as a subprocess at run time,
    /// producing the extended bytes the ELF actually reads), so adaptation is the identity.
    pub fn adaptInput(
        alloc: std.mem.Allocator,
        ssz_stateless_input: []const u8,
        ctx: spec_runner.BlockContext,
    ) ?[]const u8 {
        _ = alloc;
        _ = ctx;
        return ssz_stateless_input;
    }

    pub fn runAndCheck(
        init: std.process.Init,
        alloc: std.mem.Allocator,
        guest_input: ?[]const u8,
        expected_output: []const u8,
        ctx: spec_runner.BlockContext,
    ) !bool {
        if (expected_output.len <= 32) {
            std.debug.print("FAIL {s}[{}]  expected_output too short ({} bytes)\n", .{ ctx.test_name, ctx.block_index, expected_output.len });
            return false;
        }
        const expected_valid = expected_output[32] == 0x01;
        const vanilla_ssz = guest_input orelse return false; // adaptInput never fails

        const outcome = try runBlockUnderZkc(init, alloc, vanilla_ssz);
        const guest_valid = outcome == .valid;
        if (guest_valid == expected_valid) return true;

        std.debug.print(
            "FAIL {s}[{}]  disagree: fixture={s} zkc={s}\n",
            .{ ctx.test_name, ctx.block_index, if (expected_valid) "valid" else "invalid", if (guest_valid) "valid" else "invalid" },
        );
        return false;
    }
};

const Outcome = enum { valid, invalid };

/// Wrap one vanilla block to a temp extended SSZ and run it under zkc via elf-exec; returns the
/// guest's validity verdict (see the file header for the exit-code semantics).
fn runBlockUnderZkc(
    init: std.process.Init,
    alloc: std.mem.Allocator,
    vanilla_ssz: []const u8,
) !Outcome {
    const io = init.io;
    const vanilla_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ssz" });
    const extended_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ext.ssz" });
    const json_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ext.ssz.json" });

    try std.Io.Dir.cwd().writeFile(io, .{ .sub_path = vanilla_path, .data = vanilla_ssz });

    // shouldSkip already filtered the wrap's policy-skip cases, so a nonzero wrap exit is a
    // rejection the guest would also produce, surfaced as `invalid`.
    const wrap_res = try std.process.run(alloc, io, .{
        .argv = &.{ wrap, vanilla_path, extended_path },
    });
    switch (wrap_res.term) {
        .exited => |code| if (code != 0) return .invalid,
        else => return error.WrapCrashed,
    }

    const in_arg = try std.fmt.allocPrint(alloc, "IN_BYTES=@{s}", .{extended_path});
    const elf_arg = try std.fmt.allocPrint(alloc, "BIN_EXT={s}", .{elf});
    const json_arg = try std.fmt.allocPrint(alloc, "JSON_EXT={s}", .{json_path});
    const flags_arg = try std.fmt.allocPrint(alloc, "ZKC_EXEC_FLAGS={s}", .{zkc_flags});
    const makefile_arg = try std.fmt.allocPrint(alloc, "-f{s}", .{makefile});

    const res = try std.process.run(alloc, io, .{
        .argv = &.{ "make", "-s", makefile_arg, "elf-exec", elf_arg, in_arg, json_arg, flags_arg },
    });
    switch (res.term) {
        .exited => |code| {
            if (code == 0) return .valid;
            const combined = try std.fmt.allocPrint(alloc, "{s}\n{s}", .{ res.stdout, res.stderr });
            if (std.mem.indexOf(u8, combined, "EXIT CODE") != null) return .invalid;
            std.debug.print("--- zkc toolchain failure ---\n{s}\n", .{combined});
            return error.ToolchainFailed;
        },
        else => return error.ZkcCrashed,
    }
}

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const io = init.io;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    var fixtures_dir: ?[]const u8 = null;
    var install_prefix: ?[]const u8 = null;
    var makefile_arg: ?[]const u8 = null;
    var opts = spec_runner.Options{ .fixtures_dir = "" };
    var report_only = false;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--fixtures")) {
            fixtures_dir = takeValue(args, &i, "--fixtures");
        } else if (std.mem.eql(u8, arg, "--install-prefix")) {
            install_prefix = takeValue(args, &i, "--install-prefix");
        } else if (std.mem.eql(u8, arg, "--makefile")) {
            makefile_arg = takeValue(args, &i, "--makefile");
        } else if (std.mem.eql(u8, arg, "--zkc-flags")) {
            zkc_flags = takeValue(args, &i, "--zkc-flags");
        } else if (std.mem.eql(u8, arg, "--file")) {
            opts.single_file = takeValue(args, &i, "--file");
        } else if (std.mem.eql(u8, arg, "--fork")) {
            opts.fork_filter = takeValue(args, &i, "--fork");
        } else if (std.mem.eql(u8, arg, "--match")) {
            opts.path_match = takeValue(args, &i, "--match");
        } else if (std.mem.eql(u8, arg, "--limit")) {
            const v = takeValue(args, &i, "--limit");
            opts.limit = std.fmt.parseInt(u64, v, 10) catch {
                std.debug.print("error: --limit expects an integer, got '{s}'\n", .{v});
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
    opts.fixtures_dir = fixtures_dir orelse fatal("missing --fixtures");
    const prefix = install_prefix orelse fatal("missing --install-prefix");
    elf = try std.fs.path.join(gpa, &.{ prefix, "bin", "evm_execution_guest" });
    wrap = try std.fs.path.join(gpa, &.{ prefix, "bin", "l2-execution-wrap" });
    makefile = makefile_arg orelse fatal("missing --makefile");

    tmp_dir = try makeTempDir(io, gpa);
    defer cleanupTempDir(io, gpa, tmp_dir);

    std.debug.print("running {s}\n  over {s}\n", .{ ZkcAdapter.label, opts.single_file orelse opts.fixtures_dir });

    const stats = try spec_runner.run(ZkcAdapter, init, opts);

    const total = stats.total();
    const pct: u64 = if (total > 0) 100 * stats.passed / total else 0;
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  {s}\n", .{ZkcAdapter.label});
    std.debug.print("  files: {}   blocks: {}   agree: {}   disagree: {}   skipped: {}   ({}%)\n", .{
        stats.files, stats.blocks, stats.passed, stats.failed, stats.skipped, pct,
    });
    std.debug.print("============================================================\n", .{});

    if (stats.failed > 0 and !report_only) std.process.exit(1);
}

fn takeValue(args: []const []const u8, i: *usize, name: []const u8) []const u8 {
    if (i.* + 1 < args.len) {
        i.* += 1;
        return args[i.*];
    }
    std.debug.print("error: {s} expects a value\n{s}", .{ name, usage });
    std.process.exit(2);
}

fn fatal(msg: []const u8) noreturn {
    std.debug.print("error: {s}\n{s}", .{ msg, usage });
    std.process.exit(2);
}

fn makeTempDir(io: std.Io, gpa: std.mem.Allocator) ![]const u8 {
    const res = try std.process.run(gpa, io, .{ .argv = &.{ "mktemp", "-d" } });
    switch (res.term) {
        .exited => |code| if (code != 0) return error.MktempFailed,
        else => return error.MktempFailed,
    }
    return std.mem.trim(u8, res.stdout, " \n\r\t");
}

fn cleanupTempDir(io: std.Io, gpa: std.mem.Allocator, path: []const u8) void {
    _ = std.process.run(gpa, io, .{ .argv = &.{ "rm", "-rf", path } }) catch {};
}
