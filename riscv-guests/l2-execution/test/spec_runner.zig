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
//! and checks it. `extended_vanilla_runner.zig`'s `ExtendedVanillaAdapter` (the sole consumer of
//! this file) wraps the vanilla input into the extended `L2ExecutionProofPrivateInput` shape
//! (dummy-filled rollup fields — see `vanilla_wrap.zig`) and checks validity against the fixture's
//! own expected output.
//!
//! Adapter contract (comptime duck-typed):
//!   pub const label: []const u8
//!   /// Transform the fixture's SSZ StatelessInput into this guest's input bytes; null if
//!   /// adaptation fails. A null is handed to `runAndCheck` as a coarse "this block is invalid"
//!   /// signal, the same way any other guest-pipeline rejection is handled. This matches the
//!   /// granularity a real batch proof (`l2_execution.runL2Execution` over a payload range) reports:
//!   /// one pass/fail verdict for the whole range. Malformed SSZ the EF corpus deliberately feeds a
//!   /// block, expecting rejection, is exactly this case.
//!   pub fn adaptInput(alloc: std.mem.Allocator, ssz_stateless_input: []const u8, ctx: BlockContext) ?[]const u8
//!   /// Run the guest on the adapted input (null if adaptation failed) and compare against the
//!   /// fixture's expected output. Returns true on pass; on failure prints a one-line `FAIL …`
//!   /// diagnostic and returns false.
//!   pub fn runAndCheck(init: std.process.Init, alloc: std.mem.Allocator, guest_input: ?[]const u8, expected_output: []const u8, ctx: BlockContext) !bool
//!   /// True if this block exercises a property belonging to the vanilla reference guest's
//!   /// multi-fork/schedule model rather than this guest's own fixed-fork design. Skipped blocks
//!   /// are excluded entirely from the pass/fail tally.
//!   pub fn shouldSkip(alloc: std.mem.Allocator, ssz_stateless_input: []const u8, ctx: BlockContext) bool

const std = @import("std");
const zkevm_fixture = @import("zkevm_fixture.zig");

pub const Options = struct {
    /// Directory holding the `blockchain_tests/` JSON tree (absolute, supplied by build.zig).
    fixtures_dir: []const u8,
    /// Run a single fixture file instead of walking `fixtures_dir`.
    single_file: ?[]const u8 = null,
    /// Only run test cases whose `network` equals this (case-insensitive), e.g. "Amsterdam".
    fork_filter: ?[]const u8 = null,
    /// Only run fixture files whose relative path matches this `--match` pattern.
    /// A pattern containing `*` or `?` is a glob (against the relative path or its basename);
    /// otherwise it is a substring, e.g. `block_access_lists`.
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
    skipped: u64 = 0,

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
pub fn run(comptime Adapter: type, init: std.process.Init, opts: Options) !Stats {
    const io = init.io;
    const gpa = init.gpa;
    var stats = Stats{};

    if (opts.single_file) |path| {
        try processFile(Adapter, init, path, opts, &stats);
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
        if (opts.path_match) |m| if (!pathMatches(rel_path, m)) continue;
        const full = try std.Io.Dir.path.join(gpa, &.{ opts.fixtures_dir, rel_path });
        defer gpa.free(full);

        const failed_before = stats.failed;
        try processFile(Adapter, init, full, opts, &stats);
        if (opts.stop_on_fail and stats.failed > failed_before) break;
    }

    return stats;
}

/// `--match` filter: glob if the pattern contains `*` or `?`, otherwise substring.
/// Globs are tried against the relative path and its basename so `*.ssz` and
/// `stateless_input.ssz` both work.
pub fn pathMatches(rel_path: []const u8, pattern: []const u8) bool {
    if (isGlobPattern(pattern)) {
        return globMatch(rel_path, pattern) or globMatch(std.fs.path.basename(rel_path), pattern);
    }
    return std.mem.indexOf(u8, rel_path, pattern) != null;
}

fn isGlobPattern(pattern: []const u8) bool {
    return std.mem.indexOfAny(u8, pattern, "*?") != null;
}

fn globMatch(s: []const u8, pattern: []const u8) bool {
    var si: usize = 0;
    var pi: usize = 0;
    var star_p: ?usize = null;
    var star_s: usize = 0;
    while (si < s.len) {
        if (pi < pattern.len and (pattern[pi] == '?' or pattern[pi] == s[si])) {
            si += 1;
            pi += 1;
        } else if (pi < pattern.len and pattern[pi] == '*') {
            star_p = pi;
            star_s = si;
            pi += 1;
        } else if (star_p) |sp| {
            pi = sp + 1;
            star_s += 1;
            si = star_s;
        } else return false;
    }
    while (pi < pattern.len and pattern[pi] == '*') pi += 1;
    return pi == pattern.len;
}

fn processFile(
    comptime Adapter: type,
    init: std.process.Init,
    path: []const u8,
    opts: Options,
    stats: *Stats,
) !void {
    const io = init.io;
    // One arena per file: the parsed JSON, decoded bytes and adapted input live only for this file.
    var arena = std.heap.ArenaAllocator.init(init.gpa);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A fixture we can't read or parse is a failure, not a silent skip: counting it keeps a
    // systemic regression (e.g. parseBlocks breaking across the whole corpus) from passing green.
    const text = std.Io.Dir.cwd().readFileAlloc(io, path, alloc, .limited(1 << 30)) catch |err| {
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

        if (Adapter.shouldSkip(alloc, block.input, ctx)) {
            stats.skipped += 1;
            continue;
        }

        const guest_input = Adapter.adaptInput(alloc, block.input, ctx);
        const ok = Adapter.runAndCheck(init, alloc, guest_input, block.expected_output, ctx) catch |err| blk: {
            std.debug.print("FAIL {s}[{}]  runAndCheck error: {s}\n", .{ ctx.test_name, ctx.block_index, @errorName(err) });
            break :blk false;
        };
        if (ok) stats.passed += 1 else stats.failed += 1;
    }
}
