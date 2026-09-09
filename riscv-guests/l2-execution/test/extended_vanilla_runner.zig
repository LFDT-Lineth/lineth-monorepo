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
    \\  --limit N        stop after N selected blocks
    \\  -x               stop after the first failing file
    \\  --report-only    report disagreements without a failing exit status
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

    var histogram = std.StringHashMap(u64).init(init.gpa);
    defer histogram.deinit();
    var host = host_machine.HostMachine{};
    var suite = execution_spec_suite.Suite(host_machine.HostMachine){
        .machine = &host,
        .policy = .allow_linea_rejections,
        .fork_filter = fork_filter,
        .record_rejection = recordRejection,
        .record_context = &histogram,
    };

    std.debug.print("running {s}\n", .{label});
    const stats = try spec_runner.run(&suite, init, operands.items, opts);
    printSummary(stats, &histogram);
    if (stats.contribution.failed > 0 and !report_only) return error.DisagreementsDetected;
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

fn fatal(message: []const u8) noreturn {
    std.debug.print("error: {s}\n{s}", .{ message, usage });
    std.process.exit(2);
}
