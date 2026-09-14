//! Deterministic traversal and aggregation for file-oriented test suites.
//!
//! Suite contract (comptime duck-typed):
//!   pub fn processFile(init: std.process.Init, path: []const u8, case_limit: ?u64) !FileResult

const std = @import("std");
const zlob = @import("zlob");

const match_flags = zlob.ZlobFlags{ .doublestar_recursive = true };

const Matcher = struct {
    compiled: zlob.CompiledPattern,
    matches_basename: bool,

    fn init(alloc: std.mem.Allocator, source: []const u8) !Matcher {
        return .{
            .compiled = try zlob.compilePattern(alloc, source, match_flags),
            .matches_basename = std.mem.indexOfScalar(u8, source, '/') == null,
        };
    }

    fn deinit(self: *Matcher) void {
        self.compiled.deinit();
    }

    fn matches(self: Matcher, path: []const u8) bool {
        const subject = if (self.matches_basename) std.fs.path.basename(path) else path;
        return self.compiled.matches(subject, match_flags);
    }
};

pub const Options = struct {
    /// Glob matched against an operand-relative directory entry or a direct file's basename.
    match_pattern: ?[]const u8 = null,
    /// Stop after this many selected cases.
    limit: ?u64 = null,
    /// Stop after the first file contribution containing a failure.
    stop_on_fail: bool = false,
};

pub const Contribution = struct {
    files: u64 = 0,
    cases: u64 = 0,
    passed: u64 = 0,
    failed: u64 = 0,
    skipped: u64 = 0,

    pub fn add(self: *Contribution, other: Contribution) void {
        self.files += other.files;
        self.cases += other.cases;
        self.passed += other.passed;
        self.failed += other.failed;
        self.skipped += other.skipped;
    }

    pub fn total(self: Contribution) u64 {
        return self.passed + self.failed;
    }
};

pub const FileResult = union(enum) {
    unrecognized,
    recognized: Contribution,
};

pub const Stats = struct {
    recognized_files: u64 = 0,
    contribution: Contribution = .{},

    pub fn total(self: Stats) u64 {
        return self.contribution.total();
    }
};

/// Process operands in order. Files within each directory operand are visited recursively in
/// lexical order. Repeated operands are intentionally processed repeatedly.
pub fn run(suite: anytype, init: std.process.Init, operands: []const []const u8, opts: Options) !Stats {
    var matcher = if (opts.match_pattern) |source| try Matcher.init(init.gpa, source) else null;
    defer if (matcher) |*value| value.deinit();

    var stats = Stats{};
    for (operands) |operand| {
        if (std.mem.eql(u8, operand, "-")) return error.StdinNotSupported;
        const stat = std.Io.Dir.cwd().statFile(init.io, operand, .{}) catch |err| {
            std.debug.print("error: cannot inspect path '{s}': {s}\n", .{ operand, @errorName(err) });
            return error.PathInspectionFailed;
        };
        switch (stat.kind) {
            .directory => try processDirectory(suite, init, operand, if (matcher) |*value| value else null, opts, &stats),
            .file => {
                if (matches(std.fs.path.basename(operand), if (matcher) |*value| value else null)) {
                    try processOne(suite, init, operand, opts, &stats);
                }
            },
            else => return error.UnsupportedPathType,
        }
        if (shouldStop(stats, opts)) break;
    }
    if (stats.recognized_files == 0) return error.NoRecognizedFiles;
    if (stats.contribution.cases == 0) return error.NoSelectedCases;
    if (stats.total() == 0) return error.NoExecutedCases;
    return stats;
}

fn processDirectory(
    suite: anytype,
    init: std.process.Init,
    root: []const u8,
    matcher: ?*const Matcher,
    opts: Options,
    stats: *Stats,
) !void {
    var dir = std.Io.Dir.cwd().openDir(init.io, root, .{ .iterate = true }) catch |err| {
        std.debug.print("error: cannot open directory '{s}': {s}\n", .{ root, @errorName(err) });
        return error.DirectoryOpenFailed;
    };
    defer dir.close(init.io);

    var walker = try dir.walk(init.gpa);
    defer walker.deinit();
    var paths = std.ArrayList([]u8).empty;
    defer {
        for (paths.items) |path| init.gpa.free(path);
        paths.deinit(init.gpa);
    }
    while (try walker.next(init.io)) |entry| {
        if (entry.kind != .file or !matches(entry.path, matcher)) continue;
        try paths.append(init.gpa, try init.gpa.dupe(u8, entry.path));
    }
    std.mem.sort([]u8, paths.items, {}, struct {
        fn lessThan(_: void, a: []u8, b: []u8) bool {
            return std.mem.lessThan(u8, a, b);
        }
    }.lessThan);

    for (paths.items) |relative_path| {
        const full_path = try std.Io.Dir.path.join(init.gpa, &.{ root, relative_path });
        defer init.gpa.free(full_path);
        try processOne(suite, init, full_path, opts, stats);
        if (shouldStop(stats.*, opts)) return;
    }
}

fn processOne(suite: anytype, init: std.process.Init, path: []const u8, opts: Options, stats: *Stats) !void {
    const remaining = if (opts.limit) |limit| limit -| stats.contribution.cases else null;
    if (remaining == 0) return;
    switch (try suite.processFile(init, path, remaining)) {
        .unrecognized => {},
        .recognized => |contribution| {
            if (remaining) |limit| if (contribution.cases > limit) return error.SuiteExceededCaseLimit;
            if (contribution.cases != contribution.passed + contribution.failed + contribution.skipped) {
                return error.InvalidSuiteContribution;
            }
            stats.recognized_files += 1;
            stats.contribution.add(contribution);
        },
    }
}

fn matches(path: []const u8, matcher: ?*const Matcher) bool {
    const value = matcher orelse return true;
    return value.matches(path);
}

fn shouldStop(stats: Stats, opts: Options) bool {
    if (opts.limit) |limit| if (stats.contribution.cases >= limit) return true;
    return opts.stop_on_fail and stats.contribution.failed > 0;
}

const RecordingSuite = struct {
    paths: std.ArrayList([]u8) = .empty,
    root: ?[]const u8 = null,

    fn deinit(self: *RecordingSuite) void {
        for (self.paths.items) |path| std.testing.allocator.free(path);
        self.paths.deinit(std.testing.allocator);
    }

    pub fn processFile(self: *RecordingSuite, _: std.process.Init, path: []const u8, limit: ?u64) !FileResult {
        if (!std.mem.endsWith(u8, path, ".case")) return .unrecognized;
        const recorded_path = if (self.root) |root| std.mem.trimStart(u8, path[root.len..], std.fs.path.sep_str) else std.fs.path.basename(path);
        try self.paths.append(std.testing.allocator, try std.testing.allocator.dupe(u8, recorded_path));
        if (limit == 0) return .{ .recognized = .{ .files = 1 } };
        return .{ .recognized = .{
            .files = 1,
            .cases = 1,
            .passed = if (std.mem.indexOf(u8, path, "fail") == null) 1 else 0,
            .failed = if (std.mem.indexOf(u8, path, "fail") == null) 0 else 1,
        } };
    }
};

fn testInit() std.process.Init {
    return .{
        .minimal = undefined,
        .arena = undefined,
        .gpa = std.testing.allocator,
        .io = std.testing.io,
        .environ_map = undefined,
        .preopens = undefined,
    };
}

fn tempPath(alloc: std.mem.Allocator, temp: *std.testing.TmpDir, suffix: []const u8) ![]u8 {
    return std.fmt.allocPrint(alloc, ".zig-cache/tmp/{s}/{s}", .{ temp.sub_path, suffix });
}

test "file and directory operands preserve operand order and lexical directory order" {
    var temp = std.testing.tmpDir(.{ .iterate = true });
    defer temp.cleanup();
    try temp.dir.createDir(std.testing.io, "nested", .default_dir);
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "z.case", .data = "" });
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "a.case", .data = "" });
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "nested/m.case", .data = "" });

    const directory = try tempPath(std.testing.allocator, &temp, "");
    defer std.testing.allocator.free(directory);
    const file = try tempPath(std.testing.allocator, &temp, "z.case");
    defer std.testing.allocator.free(file);
    var suite = RecordingSuite{};
    defer suite.deinit();

    const stats = try run(&suite, testInit(), &.{ file, directory, file }, .{});
    try std.testing.expectEqual(@as(u64, 5), stats.recognized_files);
    try std.testing.expectEqual(@as(u64, 5), stats.contribution.cases);
    const expected = [_][]const u8{ "z.case", "a.case", "m.case", "z.case", "z.case" };
    try std.testing.expectEqual(expected.len, suite.paths.items.len);
    for (expected, suite.paths.items) |want, actual| try std.testing.expectEqualStrings(want, actual);
}

test "glob patterns select files by operand-relative path" {
    var temp = std.testing.tmpDir(.{ .iterate = true });
    defer temp.cleanup();
    try temp.dir.createDir(std.testing.io, "nested", .default_dir);
    try temp.dir.createDir(std.testing.io, "nested/deeper", .default_dir);
    inline for (&.{
        "a.case",
        "b.case",
        "c.txt",
        "file1.case",
        "file2.case",
        "file10.case",
        "target.case",
        "nested/other.case",
        "nested/deeper/target.case",
    }) |path| try temp.dir.writeFile(std.testing.io, .{ .sub_path = path, .data = "" });

    const directory = try tempPath(std.testing.allocator, &temp, "");
    defer std.testing.allocator.free(directory);

    const Case = struct {
        pattern: []const u8,
        expected: []const []const u8,
    };
    const cases = [_]Case{
        .{ .pattern = "*.case", .expected = &.{ "a.case", "b.case", "file1.case", "file10.case", "file2.case", "nested/deeper/target.case", "nested/other.case", "target.case" } },
        .{ .pattern = "file?.case", .expected = &.{ "file1.case", "file2.case" } },
        .{ .pattern = "[ab].case", .expected = &.{ "a.case", "b.case" } },
        .{ .pattern = "**/target.case", .expected = &.{ "nested/deeper/target.case", "target.case" } },
    };

    for (cases) |case| {
        var suite = RecordingSuite{ .root = directory };
        defer suite.deinit();
        const stats = try run(&suite, testInit(), &.{directory}, .{ .match_pattern = case.pattern });
        try std.testing.expectEqual(@as(u64, @intCast(case.expected.len)), stats.recognized_files);
        try std.testing.expectEqual(case.expected.len, suite.paths.items.len);
        for (case.expected, suite.paths.items) |expected, actual| try std.testing.expectEqualStrings(expected, actual);
    }
}

test "glob patterns with separators match the operand-relative path" {
    var temp = std.testing.tmpDir(.{ .iterate = true });
    defer temp.cleanup();
    try temp.dir.createDir(std.testing.io, "nested", .default_dir);
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "target.case", .data = "" });
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "nested/target.case", .data = "" });
    const directory = try tempPath(std.testing.allocator, &temp, "");
    defer std.testing.allocator.free(directory);

    var suite = RecordingSuite{ .root = directory };
    defer suite.deinit();
    const stats = try run(&suite, testInit(), &.{directory}, .{ .match_pattern = "nested/*.case" });
    try std.testing.expectEqual(@as(u64, 1), stats.contribution.cases);
    try std.testing.expectEqualStrings("nested/target.case", suite.paths.items[0]);
}

test "direct file glob matching uses the basename" {
    var temp = std.testing.tmpDir(.{ .iterate = true });
    defer temp.cleanup();
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "selected.case", .data = "" });
    const file = try tempPath(std.testing.allocator, &temp, "selected.case");
    defer std.testing.allocator.free(file);

    var suite = RecordingSuite{};
    defer suite.deinit();
    const stats = try run(&suite, testInit(), &.{file}, .{ .match_pattern = "selected.case" });
    try std.testing.expectEqual(@as(u64, 1), stats.recognized_files);
}

test "glob selection limit and stop-on-failure affect observable traversal" {
    var temp = std.testing.tmpDir(.{ .iterate = true });
    defer temp.cleanup();
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "01-selected.case", .data = "" });
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "02-fail-selected.case", .data = "" });
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "03-selected.case", .data = "" });
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "ignored.case", .data = "" });
    const directory = try tempPath(std.testing.allocator, &temp, "");
    defer std.testing.allocator.free(directory);

    var stopped = RecordingSuite{};
    defer stopped.deinit();
    const stopped_stats = try run(&stopped, testInit(), &.{directory}, .{ .match_pattern = "*selected.case", .stop_on_fail = true });
    try std.testing.expectEqual(@as(u64, 2), stopped_stats.contribution.cases);
    try std.testing.expectEqual(@as(u64, 1), stopped_stats.contribution.failed);

    var limited = RecordingSuite{};
    defer limited.deinit();
    const limited_stats = try run(&limited, testInit(), &.{directory}, .{ .match_pattern = "*selected.case", .limit = 1 });
    try std.testing.expectEqual(@as(u64, 1), limited_stats.contribution.cases);
    try std.testing.expectEqual(@as(usize, 1), limited.paths.items.len);
}

test "runner rejects runs with no recognized files or no selected cases" {
    var temp = std.testing.tmpDir(.{ .iterate = true });
    defer temp.cleanup();
    try temp.dir.writeFile(std.testing.io, .{ .sub_path = "ignored.txt", .data = "" });
    const directory = try tempPath(std.testing.allocator, &temp, "");
    defer std.testing.allocator.free(directory);
    var suite = RecordingSuite{};
    defer suite.deinit();
    try std.testing.expectError(error.NoRecognizedFiles, run(&suite, testInit(), &.{directory}, .{}));
    try std.testing.expectError(error.NoRecognizedFiles, run(&suite, testInit(), &.{directory}, .{ .match_pattern = "absent*" }));

    const EmptySuite = struct {
        pub fn processFile(_: *@This(), _: std.process.Init, _: []const u8, _: ?u64) !FileResult {
            return .{ .recognized = .{ .files = 1 } };
        }
    };
    var empty_suite = EmptySuite{};
    const ignored = try tempPath(std.testing.allocator, &temp, "ignored.txt");
    defer std.testing.allocator.free(ignored);
    try std.testing.expectError(error.NoSelectedCases, run(&empty_suite, testInit(), &.{ignored}, .{}));

    const InconsistentSuite = struct {
        pub fn processFile(_: *@This(), _: std.process.Init, _: []const u8, _: ?u64) !FileResult {
            return .{ .recognized = .{ .files = 1, .cases = 1 } };
        }
    };
    var inconsistent_suite = InconsistentSuite{};
    try std.testing.expectError(error.InvalidSuiteContribution, run(&inconsistent_suite, testInit(), &.{ignored}, .{}));

    const SkippingSuite = struct {
        pub fn processFile(_: *@This(), _: std.process.Init, _: []const u8, _: ?u64) !FileResult {
            return .{ .recognized = .{ .files = 1, .cases = 1, .skipped = 1 } };
        }
    };
    var skipping_suite = SkippingSuite{};
    try std.testing.expectError(error.NoExecutedCases, run(&skipping_suite, testInit(), &.{ignored}, .{}));
}
