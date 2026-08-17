//! Generic, guest-agnostic runner for EF execution-spec-tests zkevm *stateless* fixtures.
//!
//! Mirrors zesu's `zkevm-blockchain-test-runner` (tools/zkevm_test/main.zig): it walks a
//! `blockchain_tests/` directory and, for every block in every fixture, hands the SSZ
//! `statelessInputBytes` + expected `statelessOutputBytes` to a guest `Adapter`. The fixture JSON
//! shape + hex decoding are NOT re-implemented here — they live in `zkevm_fixture.zig`, shared
//! with the single-fixture smoke test.
//!
//! ── The extension seam ──────────────────────────────────────────────────────────────────
//! Everything here (dir walk, JSON parse, per-block extraction, fork filter, reporting) is
//! guest-agnostic. The *only* guest-specific piece is the comptime `Adapter`, which adapts the
//! fixture's vanilla SSZ `StatelessInput` to whatever shape a given guest consumes and then runs
//! and checks it. The vanilla guest's adapter is identity; a future extended guest (see
//! rollup_spec — extra `forced_transactions`/rollup fields) supplies an adapter whose
//! `adaptInput` wraps + re-encodes the input, and reuses this file unchanged.
//!
//! Adapter contract (comptime duck-typed):
//!   pub const label: []const u8
//!   /// Transform the fixture's SSZ StatelessInput into this guest's input bytes.
//!   pub fn adaptInput(alloc: std.mem.Allocator, ssz_stateless_input: []const u8, ctx: BlockContext) ![]const u8
//!   /// Run the guest on the adapted input and compare against the fixture's expected output.
//!   /// Returns true on pass; on failure prints a one-line `FAIL …` diagnostic and returns false.
//!   pub fn runAndCheck(alloc: std.mem.Allocator, guest_input: []const u8, expected_output: []const u8, ctx: BlockContext) !bool

const std = @import("std");
const zkevm_fixture = @import("zkevm_fixture.zig");

pub const Options = struct {
    /// Directory holding the `blockchain_tests/` JSON tree (absolute, supplied by build.zig).
    fixtures_dir: []const u8,
    /// Run a single fixture file instead of walking `fixtures_dir`.
    single_file: ?[]const u8 = null,
    /// Only run test cases whose `network` equals this (case-insensitive), e.g. "Amsterdam".
    fork_filter: ?[]const u8 = null,
    /// Only run fixture files whose relative path contains this substring, e.g.
    /// "block_access_lists" (the most useful narrowing for this Amsterdam-only corpus).
    path_match: ?[]const u8 = null,
    /// Stop after this many blocks have been attempted (dev speed).
    limit: ?u64 = null,
    /// Stop walking after the first failing block.
    stop_on_fail: bool = false,
};

pub const Stats = struct {
    files: u64 = 0,
    blocks: u64 = 0,
    passed: u64 = 0,
    failed: u64 = 0,

    pub fn total(self: Stats) u64 {
        return self.passed + self.failed;
    }
};

/// Identifies the block currently under test; handed to the adapter for diagnostics and (for an
/// extended guest) any fork/chain-dependent adaptation.
pub const BlockContext = struct {
    file_path: []const u8,
    test_name: []const u8,
    block_index: usize,
    network: ?[]const u8,
};

/// Walk `opts.fixtures_dir` (or run `opts.single_file`) and run every stateless block through
/// `Adapter`. Returns aggregate stats; the caller decides the exit code.
pub fn run(comptime Adapter: type, io: std.Io, gpa: std.mem.Allocator, opts: Options) !Stats {
    var stats = Stats{};

    if (opts.single_file) |path| {
        try processFile(Adapter, io, gpa, path, opts, &stats);
        return stats;
    }

    var dir = std.Io.Dir.cwd().openDir(io, opts.fixtures_dir, .{ .iterate = true }) catch |err| {
        std.debug.print("error: cannot open fixtures dir '{s}': {}\n", .{ opts.fixtures_dir, err });
        return error.FixturesDirOpenFailed;
    };
    defer dir.close(io);

    var walker = try dir.walk(gpa);
    defer walker.deinit();

    // Collect + sort paths so the run order is deterministic across machines.
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
        fn lessThan(_: void, a: []u8, b: []u8) bool {
            return std.mem.lessThan(u8, a, b);
        }
    }.lessThan);

    for (paths.items) |rel_path| {
        if (opts.limit) |lim| if (stats.blocks >= lim) break;
        if (opts.path_match) |m| if (std.mem.indexOf(u8, rel_path, m) == null) continue;
        const full = try std.Io.Dir.path.join(gpa, &.{ opts.fixtures_dir, rel_path });
        defer gpa.free(full);

        const failed_before = stats.failed;
        try processFile(Adapter, io, gpa, full, opts, &stats);
        if (opts.stop_on_fail and stats.failed > failed_before) break;
    }

    return stats;
}

fn processFile(
    comptime Adapter: type,
    io: std.Io,
    gpa: std.mem.Allocator,
    path: []const u8,
    opts: Options,
    stats: *Stats,
) !void {
    // One arena per file: the parsed JSON, decoded bytes and adapted input live only for this file.
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A fixture we can't read or parse is a failure, not a silent skip: counting it keeps a
    // systemic regression (e.g. parseBlocks breaking across the whole corpus) from passing green.
    const text = std.Io.Dir.cwd().readFileAlloc(io, path, alloc, .limited(256 * 1024 * 1024)) catch |err| {
        std.debug.print("FAIL cannot read '{s}': {}\n", .{ path, err });
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

        const ctx = BlockContext{
            .file_path = path,
            .test_name = block.test_name,
            .block_index = block.block_index,
            .network = block.network,
        };
        stats.blocks += 1;

        const guest_input = Adapter.adaptInput(alloc, block.input, ctx) catch |err| {
            std.debug.print("FAIL {s}[{}]  adaptInput error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            stats.failed += 1;
            continue;
        };
        const ok = Adapter.runAndCheck(alloc, guest_input, block.expected_output, ctx) catch |err| blk: {
            std.debug.print("FAIL {s}[{}]  runAndCheck error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            break :blk false;
        };
        if (ok) stats.passed += 1 else stats.failed += 1;
    }
}
