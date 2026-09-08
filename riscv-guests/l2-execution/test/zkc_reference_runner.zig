//! Composes execution-spec validity and extended-SSZ differential suites for ZkC.

const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const host_machine = @import("host_machine");
const zkc_machine = @import("zkc_machine");
const execution_spec_suite = @import("execution_spec_suite.zig");
const smoke_suite = @import("smoke_suite.zig");

const label = "ZkC guest vs host and execution-spec fixture ground truth";
const usage =
    \\usage: zkc-reference-runner [options] PATH...
    \\
    \\PATH may be a file or a recursively traversed directory. Operands run in order; files within
    \\directories run lexically. Patterns containing `/` match paths relative to the operand root;
    \\patterns without `/` match basenames at any depth.
    \\  --install-prefix DIR  Zig install prefix containing the guest ELF
    \\  --makefile PATH       arithmetization Makefile
    \\  --zkc-target NAME     elf-exec (default) or elf-trace
    \\  --zkc-flags TEXT      flags forwarded to ZkC
    \\  --fork NAME           filter execution-spec network, case-insensitively
    \\  --match GLOB          select with *, ?, character classes, or recursive **; quote the glob
    \\  --limit N             stop after N selected cases
    \\  -x                    stop after the first failing file
    \\  --report-only         report disagreements without a failing exit status
    \\
;

fn CompositeSuite(comptime ExecutionSpec: type, comptime Smoke: type) type {
    return struct {
        execution_spec: *ExecutionSpec,
        smoke: *Smoke,

        pub fn processFile(self: *@This(), init: std.process.Init, path: []const u8, limit: ?u64) !spec_runner.FileResult {
            const execution_spec_result = try self.execution_spec.processFile(init, path, limit);
            if (execution_spec_result != .unrecognized) return execution_spec_result;
            return self.smoke.processFile(init, path, limit);
        }
    };
}

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    var operands = std.ArrayList([]const u8).empty;
    defer operands.deinit(init.gpa);
    var opts = spec_runner.Options{};
    var prefix: ?[]const u8 = null;
    var makefile: ?[]const u8 = null;
    var target: []const u8 = "elf-exec";
    var flags: ?[]const u8 = null;
    var fork_filter: ?[]const u8 = null;
    var report_only = false;
    var options_enabled = true;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        const arg = args[i];
        if (options_enabled and std.mem.eql(u8, arg, "--")) {
            options_enabled = false;
        } else if (options_enabled and std.mem.eql(u8, arg, "--install-prefix")) {
            prefix = takeValue(args, &i, arg);
        } else if (options_enabled and std.mem.eql(u8, arg, "--makefile")) {
            makefile = takeValue(args, &i, arg);
        } else if (options_enabled and std.mem.eql(u8, arg, "--zkc-target")) {
            target = takeValue(args, &i, arg);
        } else if (options_enabled and std.mem.eql(u8, arg, "--zkc-flags")) {
            flags = takeValue(args, &i, arg);
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
    if (!std.mem.eql(u8, target, "elf-exec") and !std.mem.eql(u8, target, "elf-trace")) {
        fatal("--zkc-target must be elf-exec or elf-trace");
    }

    const install_prefix = prefix orelse fatal("missing --install-prefix");
    const elf = try std.fs.path.join(init.gpa, &.{ install_prefix, "bin", "evm_execution_guest" });
    defer init.gpa.free(elf);
    const temp_dir = try makeTempDir(init);
    defer cleanupTempDir(init, temp_dir);

    var zkc = zkc_machine.ZkcMachine{
        .elf = elf,
        .makefile = makefile orelse fatal("missing --makefile"),
        .target = target,
        .flags = flags,
        .temp_dir = temp_dir,
    };
    var host = host_machine.HostMachine{};
    var execution_spec = execution_spec_suite.Suite(zkc_machine.ZkcMachine){
        .machine = &zkc,
        .policy = .skip_linea_unsupported,
        .fork_filter = fork_filter,
    };
    var smoke = smoke_suite.Suite(host_machine.HostMachine, zkc_machine.ZkcMachine){ .host = &host, .candidate = &zkc };
    var suite = CompositeSuite(@TypeOf(execution_spec), @TypeOf(smoke)){
        .execution_spec = &execution_spec,
        .smoke = &smoke,
    };

    std.debug.print("running {s}\n", .{label});
    const stats = spec_runner.run(&suite, init, operands.items, opts) catch |err| fatal(@errorName(err));
    printSummary(stats);
    if (stats.contribution.failed > 0 and !report_only) std.process.exit(1);
}

fn printSummary(stats: spec_runner.Stats) void {
    const values = stats.contribution;
    const pct: u64 = if (stats.total() == 0) 0 else 100 * values.passed / stats.total();
    std.debug.print("\n============================================================\n", .{});
    std.debug.print("  {s}\n", .{label});
    std.debug.print("  files: {}   cases: {}   agree: {}   disagree: {}   skipped: {}   ({}%)\n", .{
        values.files, values.cases, values.passed, values.failed, values.skipped, pct,
    });
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

fn makeTempDir(init: std.process.Init) ![]u8 {
    const result = try std.process.run(init.gpa, init.io, .{ .argv = &.{ "mktemp", "-d" } });
    defer init.gpa.free(result.stderr);
    switch (result.term) {
        .exited => |code| if (code != 0) return error.MktempFailed,
        else => return error.MktempFailed,
    }
    const owned = try init.gpa.dupe(u8, std.mem.trim(u8, result.stdout, " \n\r\t"));
    init.gpa.free(result.stdout);
    return owned;
}

fn cleanupTempDir(init: std.process.Init, path: []u8) void {
    std.Io.Dir.cwd().deleteTree(init.io, path) catch {};
    init.gpa.free(path);
}
