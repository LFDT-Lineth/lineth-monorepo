//! `mainnet-runner` — Linea mainnet zkevm fixture runner.
//!
//! Walks a directory of vanilla SSZ files produced by `zesu-convert` (one per mainnet block),
//! wraps each into an extended `L2ExecutionProofPrivateInput` with dummy Linea rollup fields
//! (zero l2_message_service_address, suppressing the bridge reads), and runs it through
//! `l2_execution.runL2Execution`, expecting success.
//!
//! The vanilla→extended wrap is identical to `extended_vanilla_runner.zig`'s approach so the
//! same EVM execution seam is exercised, but with real Linea mainnet blocks (larger, more
//! gas-intensive) rather than the EF corpus. The two allowed-disagreement errors from the EF
//! runner (`WithdrawalsNotSupported`, `ExecutionRequestsNotSupported`) apply here too.
//!
//! Run via `zig build mainnet-test -- --fixtures DIR`.

const std = @import("std");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");
const vanilla_wrap = @import("vanilla_wrap");

const usage =
    \\mainnet-runner — run Linea mainnet zkevm SSZ fixtures through the extended l2-execution guest.
    \\
    \\usage: mainnet-runner --fixtures DIR [--file FILE] [--limit N] [-x] [--report-only]
    \\  --fixtures DIR   directory of .ssz vanilla SSZ files (produced by zesu-convert)
    \\  --file FILE      run a single .ssz file instead of the whole directory
    \\  --limit N        stop after N files (dev speed)
    \\  -x               stop on the first failing file
    \\  --report-only    print summary but always exit 0
    \\
;

pub fn main(init: std.process.Init) !void {
    const gpa = init.gpa;
    const args = try init.minimal.args.toSlice(init.arena.allocator());

    var fixtures_dir: ?[]const u8 = null;
    var single_file: ?[]const u8 = null;
    var limit: ?u64 = null;
    var stop_on_fail = false;
    var report_only = false;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (std.mem.eql(u8, arg, "--fixtures") and i + 1 < args.len) {
            i += 1;
            fixtures_dir = args[i];
        } else if (std.mem.eql(u8, arg, "--file") and i + 1 < args.len) {
            i += 1;
            single_file = args[i];
        } else if (std.mem.eql(u8, arg, "--limit") and i + 1 < args.len) {
            i += 1;
            limit = std.fmt.parseInt(u64, args[i], 10) catch {
                std.debug.print("error: --limit expects an integer, got '{s}'\n", .{args[i]});
                std.process.exit(2);
            };
        } else if (std.mem.eql(u8, arg, "-x")) {
            stop_on_fail = true;
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

    var passed: u64 = 0;
    var failed: u64 = 0;

    if (single_file) |path| {
        const result = runFile(init.io, gpa, path);
        if (result == .pass) passed += 1 else failed += 1;
    } else if (fixtures_dir) |dir_path| {
        try runDir(init.io, gpa, dir_path, limit, stop_on_fail, &passed, &failed);
    } else {
        std.debug.print("error: --fixtures or --file is required\n{s}", .{usage});
        std.process.exit(2);
    }

    const total = passed + failed;
    const pct: u64 = if (total > 0) 100 * passed / total else 0;
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  Linea mainnet zkevm fixtures\n", .{});
    std.debug.print("  files: {}   passed: {}   failed: {}   ({}%)\n", .{ total, passed, failed, pct });
    std.debug.print("============================================================\n", .{});

    if (failed > 0 and !report_only) std.process.exit(1);
}

const FileResult = enum { pass, fail };

fn runDir(
    io: std.Io,
    gpa: std.mem.Allocator,
    dir_path: []const u8,
    limit: ?u64,
    stop_on_fail: bool,
    passed: *u64,
    failed: *u64,
) !void {
    var dir = std.Io.Dir.cwd().openDir(io, dir_path, .{ .iterate = true }) catch |err| {
        std.debug.print("error: cannot open fixtures dir '{s}': {}\n", .{ dir_path, err });
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
        if (limit) |lim| if (passed.* + failed.* >= lim) break;

        const full = try std.Io.Dir.path.join(gpa, &.{ dir_path, rel_path });
        defer gpa.free(full);

        const result = runFile(io, gpa, full);
        if (result == .pass) passed.* += 1 else {
            failed.* += 1;
            if (stop_on_fail) break;
        }
    }
}

fn runFile(io: std.Io, gpa: std.mem.Allocator, path: []const u8) FileResult {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const alloc = arena.allocator();

    const vanilla_ssz = std.Io.Dir.cwd().readFileAlloc(io, path, alloc, .limited(1 << 30)) catch |err| {
        std.debug.print("FAIL {s}: read error: {s}\n", .{ path, @errorName(err) });
        return .fail;
    };

    const extended_ssz = vanilla_wrap.wrapVanillaAsExtended(alloc, vanilla_ssz) catch |err| {
        std.debug.print("FAIL {s}: wrap error: {s}\n", .{ path, @errorName(err) });
        return .fail;
    };

    const decoded = l2_execution_ssz.decodeInput(alloc, extended_ssz) catch |err| {
        std.debug.print("FAIL {s}: decode error: {s}\n", .{ path, @errorName(err) });
        return .fail;
    };

    _ = l2_execution.runL2Execution(alloc, decoded) catch |err| {
        // Lineth policy: withdrawals and EIP-7685 execution requests are unsupported.
        // Mainnet blocks may carry these; treating them as allowed disagreements mirrors
        // the EF reference-test runner's handling (see extended_vanilla_runner.zig).
        if (err == error.WithdrawalsNotSupported or err == error.ExecutionRequestsNotSupported) {
            return .pass;
        }
        std.debug.print("FAIL {s}: execution error: {s}\n", .{ path, @errorName(err) });
        return .fail;
    };

    return .pass;
}
