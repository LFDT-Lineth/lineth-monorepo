//! `zkc-reference-runner` — runs the guest ELF under zkc and checks the result.
//!
//! Walks `--fixtures` (or `--file`) the same way `extended-vanilla-runner` does. JSON files are
//! EF zkevm fixtures: wrap + validity vs the fixture ground truth (exit 0 = valid; nonzero with
//! `EXIT CODE = <n>` = reject). On a valid run, also requires a parseable `guest_output = 0x…`
//! line in zkc stdout. Already-extended `.ssz` files skip wrap; host `runL2Execution` +
//! `encodeOutput` (schema 0x0003 ‖ keccak256(public_inputs)) is the expected wire output, overlapped
//! with the zkc subprocess. Narrow with `--match` (substring, or glob if the pattern contains
//! `*` / `?`). CI smoke is `--fixtures test/testdata --match stateless_input.ssz`.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const vanilla_wrap = @import("vanilla_wrap");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");

const usage =
    \\zkc-reference-runner — run the guest ELF under zkc.
    \\
    \\  zkc-reference-runner --fixtures DIR --install-prefix DIR --makefile PATH [options]
    \\
    \\  --fixtures DIR             JSON corpus and/or already-extended .ssz files
    \\  --install-prefix DIR       zig build install prefix; ELF (+ wrap for JSON) under DIR/bin/
    \\  --makefile PATH            arithmetization test Makefile (defines elf-exec / elf-trace)
    \\  --zkc-target NAME          makefile target: elf-exec (default) or elf-trace
    \\  --zkc-flags S              flags for zkc (default: --fast for elf-exec; --stats for elf-trace)
    \\  --file FILE                run a single .json fixture or already-extended .ssz
    \\  --fork NAME                only JSON fixtures declaring "network": "NAME" (case-insensitive)
    \\  --match PATTERN            only files whose path matches PATTERN (substring, or glob if it
    \\                            contains * or ?), e.g. stateless_input.ssz or *_empty_block*
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

// TODO : standardize the output to interface with the prover
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

fn decodeHex(alloc: std.mem.Allocator, hex: []const u8) ![]u8 {
    if (hex.len % 2 != 0) return error.InvalidHex;
    const out = try alloc.alloc(u8, hex.len / 2);
    _ = try std.fmt.hexToBytes(out, hex);
    return out;
}

/// Host path: decode extended input → runL2Execution → encodeOutput (schema ‖ keccak256(PI)).
fn nativeExpectedGuestOutput(alloc: std.mem.Allocator, raw_input: []const u8) ![l2_execution_ssz.OUTPUT_SIZE]u8 {
    const decoded = try l2_execution_ssz.decodeInput(alloc, raw_input);
    const result = try l2_execution.runL2Execution(alloc, decoded);
    return l2_execution_ssz.encodeOutput(result.public_inputs);
}

const NativeResult = struct {
    err: ?anyerror = null,
    output: [l2_execution_ssz.OUTPUT_SIZE]u8 = undefined,
};

/// Own arena: do not share `init.gpa` with the main thread (GPA is not thread-safe).
/// `runL2Execution` also installs this allocator on zesu's process-wide singleton
/// (`zesu_allocator.set`). Safe here because the sibling work is a zkc *subprocess*,
/// not another in-process zesu call.
fn nativeExpectedGuestOutputThread(raw_input: []const u8, out: *NativeResult) void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    out.output = nativeExpectedGuestOutput(arena.allocator(), raw_input) catch |err| {
        out.err = err;
        return;
    };
}

/// Already-extended `.ssz` under `--fixtures` (or a single `--file`). No wrap; expected
/// `guest_output` is native encodeOutput. Host work overlaps the zkc subprocess.
fn runExtendedSszFixtures(
    init: std.process.Init,
    opts: spec_runner.Options,
    stats: *spec_runner.Stats,
) !void {
    const io = init.io;
    const gpa = init.gpa;

    if (opts.single_file) |path| {
        if (!std.mem.endsWith(u8, path, ".ssz")) return;
        stats.files += 1;
        stats.blocks += 1;
        if (try runOneExtendedSsz(init, gpa, path)) stats.passed += 1 else stats.failed += 1;
        return;
    }

    var dir = std.Io.Dir.cwd().openDir(io, opts.fixtures_dir, .{ .iterate = true }) catch |err| {
        std.debug.print("error: cannot open fixtures dir '{s}': {}\n", .{ opts.fixtures_dir, err });
        return error.FixturesDirOpenFailed;
    };
    defer dir.close(io);

    var walker = try dir.walk(gpa);
    defer walker.deinit();

    var paths = std.ArrayList([]u8).empty;
    defer {
        for (paths.items) |p| gpa.free(p);
        paths.deinit(gpa);
    }
    while (try walker.next(io)) |entry| {
        if (entry.kind != .file) continue;
        if (!std.mem.endsWith(u8, entry.path, ".ssz")) continue;
        try paths.append(gpa, try gpa.dupe(u8, entry.path));
    }
    std.mem.sort([]u8, paths.items, {}, struct {
        fn lessThan(_: void, a: []u8, b: []u8) bool {
            return std.mem.lessThan(u8, a, b);
        }
    }.lessThan);

    for (paths.items) |rel_path| {
        if (opts.limit) |lim| if (stats.blocks >= lim) break;
        if (opts.path_match) |m| if (!spec_runner.pathMatches(rel_path, m)) continue;
        const full = try std.Io.Dir.path.join(gpa, &.{ opts.fixtures_dir, rel_path });
        defer gpa.free(full);

        stats.files += 1;
        stats.blocks += 1;
        const failed_before = stats.failed;
        if (try runOneExtendedSsz(init, gpa, full)) stats.passed += 1 else stats.failed += 1;
        if (opts.stop_on_fail and stats.failed > failed_before) break;
    }
}

fn runOneExtendedSsz(init: std.process.Init, alloc: std.mem.Allocator, input_path: []const u8) !bool {
    const raw_input = std.Io.Dir.cwd().readFileAlloc(init.io, input_path, alloc, .limited(1 << 30)) catch |err| {
        std.debug.print("FAIL {s}: cannot read: {s}\n", .{ input_path, @errorName(err) });
        return false;
    };

    var native_result = NativeResult{};
    const native_thread = std.Thread.spawn(.{}, nativeExpectedGuestOutputThread, .{ raw_input, &native_result }) catch |err| {
        std.debug.print("FAIL {s}: cannot spawn native runL2Execution thread: {s}\n", .{ input_path, @errorName(err) });
        return false;
    };

    const run = try runExtendedInputUnderZkc(init, alloc, input_path);
    native_thread.join();

    const native_ok = native_result.err == null;
    const zkc_valid = run.outcome == .valid;
    if (!native_ok and !zkc_valid) {
        std.debug.print("OK {s}: native and zkc both reject\n", .{input_path});
        return true;
    }
    if (native_result.err) |err| {
        std.debug.print("FAIL {s}: native runL2Execution failed: {s}\n", .{ input_path, @errorName(err) });
        return false;
    }
    if (!zkc_valid) {
        std.debug.print("FAIL {s}: zkc rejected input\n", .{input_path});
        return false;
    }
    const got_hex = run.guest_output_hex orelse {
        std.debug.print("FAIL {s}: missing `{s}…` in zkc stdout\n", .{ input_path, guest_output_prefix });
        return false;
    };
    const got = decodeHex(alloc, got_hex) catch {
        std.debug.print("FAIL {s}: invalid guest_output hex: {s}\n", .{ input_path, got_hex });
        return false;
    };

    if (!std.mem.eql(u8, got, &native_result.output)) {
        const expected_hex = std.fmt.bytesToHex(&native_result.output, .lower);
        std.debug.print(
            "FAIL {s}: guest_output mismatch (native encodeOutput vs zkc)\n  got:      0x{s}\n  expected: 0x{s}\n",
            .{ input_path, got_hex, &expected_hex },
        );
        return false;
    }
    std.debug.print("OK {s}: guest_output matches native encodeOutput (0x{s})\n", .{ input_path, got_hex });
    return true;
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

    const single_ssz = if (opts.single_file) |p| std.mem.endsWith(u8, p, ".ssz") else false;
    opts.fixtures_dir = fixtures_dir orelse blk: {
        if (opts.single_file == null) fatal("missing --fixtures");
        break :blk "";
    };

    std.debug.print("running {s}\n  over {s}\n", .{ ZkcAdapter.label, opts.single_file orelse opts.fixtures_dir });

    var stats = spec_runner.Stats{};
    if (single_ssz) {
        try runExtendedSszFixtures(init, opts, &stats);
    } else {
        stats = try spec_runner.run(ZkcAdapter, init, opts);
        if (opts.single_file == null) try runExtendedSszFixtures(init, opts, &stats);
    }
    if (stats.files == 0) {
        std.debug.print("error: no matching fixtures under '{s}'\n", .{opts.single_file orelse opts.fixtures_dir});
        std.process.exit(2);
    }

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
