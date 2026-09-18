//! Runs the extended host guest against execution-spec fixture validity.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const host_machine = @import("host_machine");
const execution_spec_suite = @import("execution_spec_suite.zig");

const label = "extended guest vs execution-spec fixture ground truth";
const usage =
    \\usage: extended-vanilla-runner [options] PATH...
    \\
    \\PATH may be a file or a recursively traversed directory. Operands run in order; files within
    \\directories run lexically. Patterns containing `/` match paths relative to the operand root;
    \\patterns without `/` match basenames at any depth.
    \\  --fork NAME      require an exact network name, case-insensitively
    \\  --match GLOB     select files with *, ?, character classes, or recursive **; quote the glob
    \\  --jobs N         run selected fixture files with up to N workers (default: 1)
    \\  --limit N        stop after N selected blocks
    \\  -x               stop after the first failing file
    \\  --report-only    report disagreements without a failing exit status
    \\
    \\With --jobs N (N > 1), each worker may read one fixture up to the runner's 1 GiB input cap.
    \\Actual memory varies with fixture content and guest execution, so plan capacity for up to N such workers.
    \\
;

fn recordRejection(context: ?*anyopaque, name: []const u8) void {
    const histogram: *std.StringHashMap(u64) = @ptrCast(@alignCast(context.?));
    const entry = histogram.getOrPut(name) catch return;
    if (!entry.found_existing) entry.value_ptr.* = 0;
    entry.value_ptr.* += 1;
}

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    var operands = std.ArrayList([]const u8).empty;
    defer operands.deinit(init.gpa);
    var opts = spec_runner.Options{};
    var fork_filter: ?[]const u8 = null;
    var report_only = false;
    var jobs: usize = 1;
    var options_enabled = true;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (options_enabled and std.mem.eql(u8, arg, "--")) {
            options_enabled = false;
        } else if (options_enabled and std.mem.eql(u8, arg, "--fork")) {
            fork_filter = takeValue(args, &i, arg);
        } else if (options_enabled and std.mem.eql(u8, arg, "--match")) {
            opts.match_pattern = takeValue(args, &i, arg);
        } else if (options_enabled and std.mem.eql(u8, arg, "--jobs")) {
            jobs = parseJobs(takeValue(args, &i, arg));
        } else if (options_enabled and std.mem.eql(u8, arg, "--limit")) {
            opts.limit = parseLimit(takeValue(args, &i, arg));
        } else if (options_enabled and std.mem.eql(u8, arg, "-x")) {
            opts.stop_on_fail = true;
        } else if (options_enabled and std.mem.eql(u8, arg, "--report-only")) {
            report_only = true;
        } else if (options_enabled and (std.mem.eql(u8, arg, "-h") or std.mem.eql(u8, arg, "--help"))) {
            std.debug.print("{s}", .{usage});
            return;
        } else if (options_enabled and std.mem.startsWith(u8, arg, "-")) {
            fatal(if (std.mem.eql(u8, arg, "-")) "stdin is not supported" else "unknown option");
        } else try operands.append(init.gpa, arg);
    }
    if (operands.items.len == 0) fatal("missing PATH operand");
    if (jobs > 1 and opts.limit != null) fatal("--limit cannot be combined with --jobs greater than one");
    if (jobs > 1 and opts.stop_on_fail) fatal("-x cannot be combined with --jobs greater than one");

    var histogram = std.StringHashMap(u64).init(init.gpa);
    defer histogram.deinit();

    std.debug.print("running {s}\n", .{label});
    const stats = if (jobs == 1) try runSequential(init, operands.items, opts, fork_filter, &histogram) else try runParallel(init, operands.items, opts, fork_filter, jobs, &histogram);
    printSummary(stats, &histogram);
    if (stats.contribution.failed > 0 and !report_only) return error.DisagreementsDetected;
}

fn runSequential(
    init: std.process.Init,
    operands: []const []const u8,
    opts: spec_runner.Options,
    fork_filter: ?[]const u8,
    histogram: *std.StringHashMap(u64),
) !spec_runner.Stats {
    var host = host_machine.HostMachine{};
    var suite = execution_spec_suite.Suite(host_machine.HostMachine){
        .machine = &host,
        .policy = .skip_linea_unsupported,
        .fork_filter = fork_filter,
        .record_rejection = recordRejection,
        .record_context = histogram,
    };
    return spec_runner.run(&suite, init, operands, opts);
}

const WorkerResult = struct {
    stats: spec_runner.Stats = .{},
    err: ?anyerror = null,
    histogram: std.StringHashMap(u64),

    fn init(alloc: std.mem.Allocator) WorkerResult {
        return .{ .histogram = std.StringHashMap(u64).init(alloc) };
    }

    fn deinit(self: *WorkerResult) void {
        self.histogram.deinit();
    }
};

const Worker = struct {
    init: std.process.Init,
    paths: []const spec_runner.Path,
    opts: spec_runner.Options,
    fork_filter: ?[]const u8,
    result: *WorkerResult,

    fn run(self: *Worker) void {
        var host = host_machine.HostMachine{};
        var suite = execution_spec_suite.Suite(host_machine.HostMachine){
            .machine = &host,
            .policy = .skip_linea_unsupported,
            .fork_filter = self.fork_filter,
            .record_rejection = recordRejection,
            .record_context = &self.result.histogram,
        };
        const stats = spec_runner.processPaths(&suite, self.init, self.paths, self.opts) catch |err| {
            self.result.err = err;
            return;
        };
        self.result.stats = stats;
    }
};

fn runParallel(
    init: std.process.Init,
    operands: []const []const u8,
    opts: spec_runner.Options,
    fork_filter: ?[]const u8,
    jobs: usize,
    histogram: *std.StringHashMap(u64),
) !spec_runner.Stats {
    var paths = try spec_runner.collectPaths(init, operands, opts.match_pattern);
    defer spec_runner.deinitPaths(init.gpa, &paths);
    if (paths.items.len == 0) return error.NoRecognizedFiles;

    const worker_count = @min(jobs, paths.items.len);
    var initialized: usize = 0;
    const results = try init.gpa.alloc(WorkerResult, worker_count);
    defer {
        for (results[0..initialized]) |*result| result.deinit();
        init.gpa.free(results);
    }
    const workers = try init.gpa.alloc(Worker, worker_count);
    defer init.gpa.free(workers);
    const threads = try init.gpa.alloc(std.Thread, worker_count);
    defer init.gpa.free(threads);

    var spawned: usize = 0;
    var spawn_error: ?anyerror = null;
    for (0..worker_count) |index| {
        var worker_init = init;
        worker_init.gpa = std.heap.page_allocator;
        results[index] = WorkerResult.init(worker_init.gpa);
        initialized += 1;
        workers[index] = .{
            .init = worker_init,
            .paths = spec_runner.shardPaths(paths.items, index, worker_count),
            .opts = .{ .progress_job = .{ .index = index + 1, .total = worker_count } },
            .fork_filter = fork_filter,
            .result = &results[index],
        };
        threads[index] = std.Thread.spawn(.{}, Worker.run, .{&workers[index]}) catch |err| {
            spawn_error = err;
            break;
        };
        spawned += 1;
    }
    for (threads[0..spawned]) |thread| thread.join();
    if (spawn_error) |err| return err;

    var total = spec_runner.Stats{};
    var first_error: ?anyerror = null;
    for (results) |*result| {
        if (result.err) |err| {
            if (first_error == null) first_error = err;
            continue;
        }
        total.recognized_files += result.stats.recognized_files;
        total.contribution.add(result.stats.contribution);
        var iterator = result.histogram.iterator();
        while (iterator.next()) |entry| {
            const destination = try histogram.getOrPut(entry.key_ptr.*);
            if (!destination.found_existing) destination.value_ptr.* = 0;
            destination.value_ptr.* += entry.value_ptr.*;
        }
    }
    if (first_error) |err| return err;
    try spec_runner.validateStats(total);
    return total;
}

fn printSummary(stats: spec_runner.Stats, histogram: *std.StringHashMap(u64)) void {
    const values = stats.contribution;
    const pct: u64 = if (stats.total() == 0) 0 else 100 * values.passed / stats.total();
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  {s}\n", .{label});
    std.debug.print("  files: {}   blocks: {}   agree: {}   disagree: {}   skipped: {}   ({}%)\n", .{
        values.files, values.cases, values.passed, values.failed, values.skipped, pct,
    });
    if (histogram.count() > 0) {
        std.debug.print("  disagreement rejection histogram:\n", .{});
        var iterator = histogram.iterator();
        while (iterator.next()) |entry| std.debug.print("    {s}: {}\n", .{ entry.key_ptr.*, entry.value_ptr.* });
    }
    std.debug.print("============================================================\n", .{});
}

fn takeValue(args: []const []const u8, index: *usize, name: []const u8) []const u8 {
    if (index.* + 1 >= args.len) fatal(name);
    index.* += 1;
    return args[index.*];
}

fn parseLimit(value: []const u8) u64 {
    const limit = std.fmt.parseInt(u64, value, 10) catch fatal("--limit expects an integer");
    if (limit == 0) fatal("--limit must be greater than zero");
    return limit;
}

fn parseJobs(value: []const u8) usize {
    const jobs = std.fmt.parseInt(usize, value, 10) catch fatal("--jobs expects a positive integer");
    if (jobs == 0) fatal("--jobs must be greater than zero");
    return jobs;
}

fn fatal(message: []const u8) noreturn {
    std.debug.print("error: {s}\n{s}", .{ message, usage });
    std.process.exit(2);
}
