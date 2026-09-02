//! `zkc-reference-runner` — runs the guest ELF under zkc and checks the result.
//!
//! Two modes:
//!
//! 1. **Corpus reference-test** (`--fixtures`): EF zkevm corpus walk (same ground truth / wrap /
//!    skip semantics as `extended-vanilla-runner`). Validity is the guest exit ecall surfaced by
//!    zkc (exit 0 = valid; nonzero with `EXIT CODE = <n>` = reject). On a valid run, also requires
//!    a parseable `guest_output = 0x…` line in zkc stdout (the write_output public output).
//!
//! 2. **Smoke** (`--input` + `--expect-guest-output`): run one already-extended SSZ input (no wrap)
//!    and assert zkc's `guest_output` hex matches the golden. Used by CI for the committed sample
//!    fixture (`test/testdata/stateless_input.ssz`).

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const vanilla_wrap = @import("vanilla_wrap");

const usage =
    \\zkc-reference-runner — run the guest ELF under zkc (corpus reference-test or smoke golden).
    \\
    \\Corpus mode:
    \\  zkc-reference-runner --fixtures DIR --install-prefix DIR --makefile PATH [options]
    \\Smoke mode:
    \\  zkc-reference-runner --input FILE.ssz --expect-guest-output HEX|FILE --install-prefix DIR --makefile PATH [options]
    \\
    \\  --fixtures DIR             the blockchain_tests/ JSON tree (corpus mode)
    \\  --input FILE               already-extended SSZ input (smoke mode; skips wrap)
    \\  --expect-guest-output X    golden guest_output hex (no 0x) or path to a file containing it
    \\  --install-prefix DIR       zig build install prefix; ELF (+ wrap in corpus mode) under DIR/bin/
    \\  --makefile PATH            arithmetization test Makefile (defines elf-exec / elf-trace)
    \\  --zkc-target NAME          makefile target: elf-exec (default) or elf-trace
    \\  --zkc-flags S              flags for zkc (default: --fast for elf-exec; --stats for elf-trace)
    \\  --file FILE                run a single fixture file instead of walking the tree
    \\  --fork NAME                only fixtures declaring "network": "NAME" (case-insensitive)
    \\  --match SUBSTR             only fixture files whose path contains SUBSTR
    \\  --limit N                  stop after N blocks (dev speed)
    \\  -x                         stop on the first disagreeing block
    \\  --report-only              print the summary but always exit 0
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
var zkc_flags: ?[]const u8 = null;
var zkc_target: []const u8 = "elf-exec";
var tmp_dir: []const u8 = undefined;

const guest_output_prefix = "guest_output = 0x";

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

        const run = try runVanillaBlockUnderZkc(init, alloc, vanilla_ssz);
        const guest_valid = run.outcome == .valid;
        if (guest_valid != expected_valid) {
            std.debug.print(
                "FAIL {s}[{}]  disagree: fixture={s} zkc={s}\n",
                .{ ctx.test_name, ctx.block_index, if (expected_valid) "valid" else "invalid", if (guest_valid) "valid" else "invalid" },
            );
            return false;
        }
        // Valid runs must surface the public output; missing it means write_output / encoding broke.
        if (guest_valid and run.guest_output_hex == null) {
            std.debug.print("FAIL {s}[{}]  valid run missing `{s}…` in zkc stdout\n", .{ ctx.test_name, ctx.block_index, guest_output_prefix });
            return false;
        }
        return true;
    }
};

const Outcome = enum { valid, invalid };

const ZkcRun = struct {
    outcome: Outcome,
    /// Lowercase hex without `0x`, allocated from the caller's allocator when present.
    guest_output_hex: ?[]const u8,
};

/// Wrap one vanilla block to a temp extended SSZ and run it under zkc.
fn runVanillaBlockUnderZkc(
    init: std.process.Init,
    alloc: std.mem.Allocator,
    vanilla_ssz: []const u8,
) !ZkcRun {
    const io = init.io;
    const vanilla_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ssz" });
    const extended_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ext.ssz" });

    try std.Io.Dir.cwd().writeFile(io, .{ .sub_path = vanilla_path, .data = vanilla_ssz });

    // shouldSkip already filtered the wrap's policy-skip cases, so a nonzero wrap exit is a
    // rejection the guest would also produce, surfaced as `invalid`.
    const wrap_res = try std.process.run(alloc, io, .{
        .argv = &.{ wrap, vanilla_path, extended_path },
    });
    switch (wrap_res.term) {
        .exited => |code| if (code != 0) return .{ .outcome = .invalid, .guest_output_hex = null },
        else => return error.WrapCrashed,
    }

    return runExtendedInputUnderZkc(init, alloc, extended_path);
}

/// Run an already-extended SSZ file under zkc via the configured makefile target.
fn runExtendedInputUnderZkc(
    init: std.process.Init,
    alloc: std.mem.Allocator,
    extended_path: []const u8,
) !ZkcRun {
    const json_path = try std.fmt.allocPrint(alloc, "{s}.json", .{extended_path});
    const flags = effectiveZkcFlags();
    const in_arg = try std.fmt.allocPrint(alloc, "IN_BYTES=@{s}", .{extended_path});
    const elf_arg = try std.fmt.allocPrint(alloc, "BIN_EXT={s}", .{elf});
    const json_arg = try std.fmt.allocPrint(alloc, "JSON_EXT={s}", .{json_path});
    const makefile_arg = try std.fmt.allocPrint(alloc, "-f{s}", .{makefile});

    const flags_arg = if (std.mem.eql(u8, zkc_target, "elf-trace"))
        try std.fmt.allocPrint(alloc, "ZKC_TRACE_FLAGS={s}", .{flags})
    else
        try std.fmt.allocPrint(alloc, "ZKC_EXEC_FLAGS={s}", .{flags});

    const res = try std.process.run(alloc, init.io, .{
        .argv = &.{ "make", "-s", makefile_arg, zkc_target, elf_arg, in_arg, json_arg, flags_arg },
    });
    const combined = try std.fmt.allocPrint(alloc, "{s}\n{s}", .{ res.stdout, res.stderr });
    const guest_output_hex = parseGuestOutputHex(alloc, combined);

    switch (res.term) {
        .exited => |code| {
            if (code == 0) return .{ .outcome = .valid, .guest_output_hex = guest_output_hex };
            if (std.mem.indexOf(u8, combined, "EXIT CODE") != null) {
                return .{ .outcome = .invalid, .guest_output_hex = guest_output_hex };
            }
            std.debug.print("--- zkc toolchain failure ---\n{s}\n", .{combined});
            return error.ToolchainFailed;
        },
        else => return error.ZkcCrashed,
    }
}

fn effectiveZkcFlags() []const u8 {
    if (zkc_flags) |f| return f;
    if (std.mem.eql(u8, zkc_target, "elf-trace")) return "--stats";
    return "--fast";
}

/// Find `guest_output = 0x<hex>` in zkc stdout/stderr; return lowercase hex without the `0x`.
fn parseGuestOutputHex(alloc: std.mem.Allocator, text: []const u8) ?[]const u8 {
    const start = std.mem.indexOf(u8, text, guest_output_prefix) orelse return null;
    const hex_start = start + guest_output_prefix.len;
    var hex_end = hex_start;
    while (hex_end < text.len) : (hex_end += 1) {
        const c = text[hex_end];
        const is_hex = (c >= '0' and c <= '9') or (c >= 'a' and c <= 'f') or (c >= 'A' and c <= 'F');
        if (!is_hex) break;
    }
    if (hex_end == hex_start) return null;
    const raw = text[hex_start..hex_end];
    const out = alloc.alloc(u8, raw.len) catch return null;
    for (raw, 0..) |c, i| {
        out[i] = std.ascii.toLower(c);
    }
    return out;
}

fn loadExpectedGuestOutput(init: std.process.Init, alloc: std.mem.Allocator, arg: []const u8) ![]const u8 {
    // Path if the argument names a readable file; otherwise treat as raw hex.
    const raw = std.Io.Dir.cwd().readFileAlloc(init.io, arg, alloc, .limited(4096)) catch {
        return std.mem.trim(u8, arg, " \n\r\t");
    };
    return std.mem.trim(u8, raw, " \n\r\t");
}

fn runSmoke(
    init: std.process.Init,
    alloc: std.mem.Allocator,
    input_path: []const u8,
    expect_arg: []const u8,
) !void {
    const expected = try loadExpectedGuestOutput(init, alloc, expect_arg);
    if (expected.len == 0 or expected.len % 2 != 0) {
        std.debug.print("error: --expect-guest-output must be non-empty even-length hex (got {} chars)\n", .{expected.len});
        std.process.exit(2);
    }

    // make/elf-to-json need a path that resolves from the caller's cwd; keep the user path as-is.
    const run = try runExtendedInputUnderZkc(init, alloc, input_path);
    if (run.outcome != .valid) {
        std.debug.print("FAIL smoke: zkc rejected input '{s}'\n", .{input_path});
        std.process.exit(1);
    }
    const got = run.guest_output_hex orelse {
        std.debug.print("FAIL smoke: missing `{s}…` in zkc stdout\n", .{guest_output_prefix});
        std.process.exit(1);
    };

    // Case-insensitive compare (golden file may use either case).
    if (got.len != expected.len) {
        std.debug.print(
            "FAIL smoke: guest_output length mismatch: got {} hex chars, expected {}\n  got:      {s}\n  expected: {s}\n",
            .{ got.len, expected.len, got, expected },
        );
        std.process.exit(1);
    }
    for (got, expected) |g, e| {
        if (g != std.ascii.toLower(e)) {
            std.debug.print(
                "FAIL smoke: guest_output mismatch\n  got:      {s}\n  expected: {s}\n",
                .{ got, expected },
            );
            std.process.exit(1);
        }
    }
    std.debug.print("OK smoke: guest_output = 0x{s}\n", .{got});
}

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const io = init.io;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    var fixtures_dir: ?[]const u8 = null;
    var smoke_input: ?[]const u8 = null;
    var expect_guest_output: ?[]const u8 = null;
    var install_prefix: ?[]const u8 = null;
    var makefile_arg: ?[]const u8 = null;
    var opts = spec_runner.Options{ .fixtures_dir = "" };
    var report_only = false;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--fixtures")) {
            fixtures_dir = takeValue(args, &i, "--fixtures");
        } else if (std.mem.eql(u8, arg, "--input")) {
            smoke_input = takeValue(args, &i, "--input");
        } else if (std.mem.eql(u8, arg, "--expect-guest-output")) {
            expect_guest_output = takeValue(args, &i, "--expect-guest-output");
        } else if (std.mem.eql(u8, arg, "--install-prefix")) {
            install_prefix = takeValue(args, &i, "--install-prefix");
        } else if (std.mem.eql(u8, arg, "--makefile")) {
            makefile_arg = takeValue(args, &i, "--makefile");
        } else if (std.mem.eql(u8, arg, "--zkc-target")) {
            zkc_target = takeValue(args, &i, "--zkc-target");
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

    if (!std.mem.eql(u8, zkc_target, "elf-exec") and !std.mem.eql(u8, zkc_target, "elf-trace")) {
        fatal("--zkc-target must be elf-exec or elf-trace");
    }

    const prefix = install_prefix orelse fatal("missing --install-prefix");
    elf = try std.fs.path.join(gpa, &.{ prefix, "bin", "evm_execution_guest" });
    wrap = try std.fs.path.join(gpa, &.{ prefix, "bin", "l2-execution-wrap" });
    makefile = makefile_arg orelse fatal("missing --makefile");

    tmp_dir = try makeTempDir(io, gpa);
    defer cleanupTempDir(io, gpa, tmp_dir);

    // Smoke mode: single extended input + golden guest_output.
    if (smoke_input != null or expect_guest_output != null) {
        const input = smoke_input orelse fatal("smoke mode requires --input");
        const expect = expect_guest_output orelse fatal("smoke mode requires --expect-guest-output");
        if (fixtures_dir != null) fatal("pass either --fixtures (corpus) or --input/--expect-guest-output (smoke), not both");
        try runSmoke(init, gpa, input, expect);
        return;
    }

    opts.fixtures_dir = fixtures_dir orelse fatal("missing --fixtures (or use smoke mode: --input + --expect-guest-output)");

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
