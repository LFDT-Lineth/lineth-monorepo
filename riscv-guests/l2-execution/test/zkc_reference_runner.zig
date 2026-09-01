//! `zkc-reference-runner` — the ZkC twin of `extended_vanilla_runner.zig`: instead of running
//! `runL2Execution` in-process, it runs each corpus block through the compiled guest ELF under zkc.
//! Ground truth, wrap, and skip semantics are identical to that host runner (see its header); the
//! only difference is the verdict source: the guest's exit ecall surfaces through zkc as exit 0 on
//! valid, or nonzero with an `EXIT CODE = <n>` marker on reject. A nonzero exit without that marker
//! is a toolchain failure, not a verdict.

const std = @import("std");
const vanilla_wrap = @import("vanilla_wrap");
const zkevm_fixture = @import("zkevm_fixture.zig");

const usage =
    \\zkc-reference-runner — run the EF zkevm corpus through the compiled guest ELF under zkc.
    \\
    \\usage: zkc-reference-runner --fixtures DIR --install-prefix DIR --makefile PATH [options]
    \\  --fixtures DIR        the blockchain_tests/ JSON tree (same as extended-vanilla-runner)
    \\  --install-prefix DIR  zig build install prefix; the guest ELF and l2-execution-wrap are
    \\                        resolved as DIR/bin/evm_execution_guest and DIR/bin/l2-execution-wrap
    \\  --makefile PATH       arithmetization test Makefile (defines the elf-exec target)
    \\  --zkc-flags S         flags for `zkc exec` (default: --gogen --fast)
    \\  --file FILE      run a single fixture file instead of walking the tree
    \\  --fork NAME      only fixtures declaring "network": "NAME" (case-insensitive)
    \\  --match SUBSTR   only fixture files whose path contains SUBSTR
    \\  --limit N        stop after N blocks (dev speed)
    \\  -x               stop on the first disagreeing block
    \\  --report-only    print the summary but always exit 0
    \\
;

const Options = struct {
    fixtures_dir: []const u8,
    /// Resolved: <install-prefix>/bin/evm_execution_guest
    elf: []const u8,
    /// Resolved: <install-prefix>/bin/l2-execution-wrap
    wrap: []const u8,
    makefile: []const u8,
    zkc_flags: []const u8 = "--gogen --fast",
    single_file: ?[]const u8 = null,
    fork_filter: ?[]const u8 = null,
    path_match: ?[]const u8 = null,
    limit: ?u64 = null,
    stop_on_fail: bool = false,
    report_only: bool = false,
};

const Stats = struct {
    files: u64 = 0,
    blocks: u64 = 0,
    passed: u64 = 0,
    failed: u64 = 0,
    skipped: u64 = 0,
};

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const io = init.io;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    var fixtures_dir: ?[]const u8 = null;
    var install_prefix: ?[]const u8 = null;
    var makefile: ?[]const u8 = null;
    var o = Options{ .fixtures_dir = "", .elf = "", .wrap = "", .makefile = "" };

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--fixtures")) {
            fixtures_dir = takeValue(args, &i, "--fixtures");
        } else if (std.mem.eql(u8, arg, "--install-prefix")) {
            install_prefix = takeValue(args, &i, "--install-prefix");
        } else if (std.mem.eql(u8, arg, "--makefile")) {
            makefile = takeValue(args, &i, "--makefile");
        } else if (std.mem.eql(u8, arg, "--zkc-flags")) {
            o.zkc_flags = takeValue(args, &i, "--zkc-flags");
        } else if (std.mem.eql(u8, arg, "--file")) {
            o.single_file = takeValue(args, &i, "--file");
        } else if (std.mem.eql(u8, arg, "--fork")) {
            o.fork_filter = takeValue(args, &i, "--fork");
        } else if (std.mem.eql(u8, arg, "--match")) {
            o.path_match = takeValue(args, &i, "--match");
        } else if (std.mem.eql(u8, arg, "--limit")) {
            const v = takeValue(args, &i, "--limit");
            o.limit = std.fmt.parseInt(u64, v, 10) catch {
                std.debug.print("error: --limit expects an integer, got '{s}'\n", .{v});
                std.process.exit(2);
            };
        } else if (std.mem.eql(u8, arg, "-x")) {
            o.stop_on_fail = true;
        } else if (std.mem.eql(u8, arg, "--report-only")) {
            o.report_only = true;
        } else if (std.mem.eql(u8, arg, "-h") or std.mem.eql(u8, arg, "--help")) {
            std.debug.print("{s}", .{usage});
            return;
        } else {
            std.debug.print("error: unexpected argument '{s}'\n{s}", .{ arg, usage });
            std.process.exit(2);
        }
    }
    o.fixtures_dir = fixtures_dir orelse fatal("missing --fixtures");
    const prefix = install_prefix orelse fatal("missing --install-prefix");
    o.elf = try std.fs.path.join(gpa, &.{ prefix, "bin", "evm_execution_guest" });
    o.wrap = try std.fs.path.join(gpa, &.{ prefix, "bin", "l2-execution-wrap" });
    o.makefile = makefile orelse fatal("missing --makefile");

    // Wrapped SSZ + elf_to_json JSON scratch, removed on exit.
    const tmp_dir = try makeTempDir(io, gpa);
    defer cleanupTempDir(io, gpa, tmp_dir);

    var stats = Stats{};
    if (o.single_file) |path| {
        try processFile(init, o, tmp_dir, path, &stats);
    } else {
        var dir = std.Io.Dir.cwd().openDir(io, o.fixtures_dir, .{ .iterate = true }) catch |err| {
            std.debug.print("error: cannot open fixtures dir '{s}': {s}\n", .{ o.fixtures_dir, @errorName(err) });
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
            if (!std.mem.endsWith(u8, entry.path, ".json")) continue;
            try paths.append(gpa, try gpa.dupe(u8, entry.path));
        }
        std.mem.sort([]u8, paths.items, {}, struct {
            fn lessThan(_: void, a: []const u8, b: []const u8) bool {
                return std.mem.lessThan(u8, a, b);
            }
        }.lessThan);

        for (paths.items) |rel_path| {
            if (o.limit) |lim| if (stats.blocks >= lim) break;
            if (o.path_match) |m| if (std.mem.indexOf(u8, rel_path, m) == null) continue;
            const full = try std.fs.path.join(gpa, &.{ o.fixtures_dir, rel_path });
            defer gpa.free(full);

            const failed_before = stats.failed;
            try processFile(init, o, tmp_dir, full, &stats);
            if (o.stop_on_fail and stats.failed > failed_before) break;
        }
    }

    const total = stats.passed + stats.failed;
    const pct: u64 = if (total > 0) 100 * stats.passed / total else 0;
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  zkc reference-test: guest ELF under zkc vs EF fixture ground truth\n", .{});
    std.debug.print("  files: {}   blocks: {}   agree: {}   disagree: {}   skipped: {}   ({}%)\n", .{
        stats.files, stats.blocks, stats.passed, stats.failed, stats.skipped, pct,
    });
    std.debug.print("============================================================\n", .{});

    if (stats.failed > 0 and !o.report_only) std.process.exit(1);
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

fn processFile(
    init: std.process.Init,
    opts: Options,
    tmp_dir: []const u8,
    path: []const u8,
    stats: *Stats,
) !void {
    const io = init.io;
    var arena = std.heap.ArenaAllocator.init(init.gpa);
    defer arena.deinit();
    const alloc = arena.allocator();

    // Unreadable/unparseable fixture = failure, not a silent skip (mirrors spec_runner.zig).
    const text = std.Io.Dir.cwd().readFileAlloc(io, path, alloc, .limited(1 << 30)) catch |err| {
        std.debug.print("FAIL cannot read '{s}': {s}\n", .{ path, @errorName(err) });
        stats.failed += 1;
        return;
    };
    const blocks = zkevm_fixture.parseBlocks(alloc, text) catch |err| {
        std.debug.print("FAIL parse failed in '{s}': {s}\n", .{ path, @errorName(err) });
        stats.failed += 1;
        return;
    };
    if (blocks.len == 0) return;
    stats.files += 1;

    for (blocks) |block| {
        if (opts.limit) |lim| if (stats.blocks >= lim) return;
        if (opts.fork_filter) |want| {
            const got = block.network orelse continue;
            if (!std.ascii.eqlIgnoreCase(got, want)) continue;
        }
        stats.blocks += 1;

        // Same Linea-policy skip allowance as the host reference-test.
        const skip = (vanilla_wrap.vanillaHasForkActivationSchedule(alloc, block.input) catch false) or
            (vanilla_wrap.vanillaHasExecutionRequests(alloc, block.input) catch false) or
            (vanilla_wrap.vanillaHasWithdrawals(alloc, block.input) catch false);
        if (skip) {
            stats.skipped += 1;
            continue;
        }

        if (block.expected_output.len <= 32) {
            std.debug.print("FAIL {s}[{}]  expected_output too short ({} bytes)\n", .{ block.test_name, block.block_index, block.expected_output.len });
            stats.failed += 1;
            if (opts.stop_on_fail) return;
            continue;
        }
        const expected_valid = block.expected_output[32] == 0x01;

        const outcome = runBlockUnderZkc(init, opts, tmp_dir, alloc, block.input) catch |err| {
            std.debug.print("FAIL {s}[{}]  runner error: {s}\n", .{ block.test_name, block.block_index, @errorName(err) });
            stats.failed += 1;
            if (opts.stop_on_fail) return;
            continue;
        };
        if (outcome == .skip) {
            stats.skipped += 1;
            continue;
        }

        const guest_valid = outcome == .valid;
        if (guest_valid == expected_valid) {
            stats.passed += 1;
        } else {
            stats.failed += 1;
            std.debug.print(
                "FAIL {s}[{}]  disagree: fixture={s} zkc={s}\n",
                .{ block.test_name, block.block_index, if (expected_valid) "valid" else "invalid", if (guest_valid) "valid" else "invalid" },
            );
            if (opts.stop_on_fail) return;
        }
    }
}

const Outcome = enum { valid, invalid, skip };

/// Wrap one vanilla block to a temp extended SSZ and run it under zkc via elf-exec; returns the
/// guest's validity verdict (see the file header for the exit-code semantics).
fn runBlockUnderZkc(
    init: std.process.Init,
    opts: Options,
    tmp_dir: []const u8,
    alloc: std.mem.Allocator,
    vanilla_ssz: []const u8,
) !Outcome {
    const io = init.io;
    const vanilla_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ssz" });
    const extended_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ext.ssz" });
    const json_path = try std.fs.path.join(alloc, &.{ tmp_dir, "in.ext.ssz.json" });

    try std.Io.Dir.cwd().writeFile(io, .{ .sub_path = vanilla_path, .data = vanilla_ssz });

    // Wrap exit 3 = policy skip; any other nonzero is a policy rejection, surfaced as `invalid`.
    const wrap_res = try std.process.run(alloc, io, .{
        .argv = &.{ opts.wrap, vanilla_path, extended_path },
    });
    switch (wrap_res.term) {
        .exited => |code| switch (code) {
            0 => {},
            3 => return .skip,
            else => return .invalid,
        },
        else => return error.WrapCrashed,
    }

    const in_arg = try std.fmt.allocPrint(alloc, "IN_BYTES=@{s}", .{extended_path});
    const elf_arg = try std.fmt.allocPrint(alloc, "BIN_EXT={s}", .{opts.elf});
    const json_arg = try std.fmt.allocPrint(alloc, "JSON_EXT={s}", .{json_path});
    const flags_arg = try std.fmt.allocPrint(alloc, "ZKC_EXEC_FLAGS={s}", .{opts.zkc_flags});
    const makefile_arg = try std.fmt.allocPrint(alloc, "-f{s}", .{opts.makefile});

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
