const std = @import("std");
const execution_machine = @import("execution_machine");
const spec_runner = @import("spec_runner.zig");

pub fn Suite(comptime Host: type, comptime Candidate: type) type {
    return struct {
        const Self = @This();

        host: *Host,
        candidate: *Candidate,

        pub fn processFile(
            self: *Self,
            init: std.process.Init,
            path: []const u8,
            case_limit: ?u64,
        ) !spec_runner.FileResult {
            if (!std.mem.endsWith(u8, path, ".ssz")) return .unrecognized;
            if (case_limit == 0) return .{ .recognized = .{ .files = 1 } };

            var arena = std.heap.ArenaAllocator.init(init.gpa);
            defer arena.deinit();
            const input = std.Io.Dir.cwd().readFileAlloc(init.io, path, arena.allocator(), .limited(1 << 30)) catch |err| {
                std.debug.print("FAIL {s}: cannot read: {s}\n", .{ path, @errorName(err) });
                return .{ .recognized = .{ .files = 1, .cases = 1, .failed = 1 } };
            };

            var host_run = ThreadResult{};
            const thread = std.Thread.spawn(.{}, runHost, .{ self.host, init, input, &host_run }) catch |err| {
                std.debug.print("FAIL {s}: cannot spawn host machine: {s}\n", .{ path, @errorName(err) });
                return .{ .recognized = .{ .files = 1, .cases = 1, .failed = 1 } };
            };
            var joined = false;
            defer if (!joined) thread.join();

            const candidate_result = self.candidate.run(init, arena.allocator(), input) catch |err| {
                std.debug.print("FAIL {s}: candidate adapter error: {s}\n", .{ path, @errorName(err) });
                return .{ .recognized = .{ .files = 1, .cases = 1, .failed = 1 } };
            };
            thread.join();
            joined = true;
            if (host_run.err) |err| {
                std.debug.print("FAIL {s}: host adapter error: {s}\n", .{ path, @errorName(err) });
                return .{ .recognized = .{ .files = 1, .cases = 1, .failed = 1 } };
            }
            if (!sameResult(path, host_run.result, candidate_result)) {
                return .{ .recognized = .{ .files = 1, .cases = 1, .failed = 1 } };
            }
            std.debug.print("OK {s}: host and candidate {s}\n", .{ path, @tagName(std.meta.activeTag(candidate_result)) });
            return .{ .recognized = .{ .files = 1, .cases = 1, .passed = 1 } };
        }

        const ThreadResult = struct {
            result: execution_machine.Result = .{ .rejected = null },
            err: ?anyerror = null,
        };

        fn runHost(host: *Host, init: std.process.Init, input: []const u8, output: *ThreadResult) void {
            var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
            defer arena.deinit();
            output.result = host.run(init, arena.allocator(), input) catch |err| {
                output.err = err;
                return;
            };
        }
    };
}

fn sameResult(path: []const u8, host: execution_machine.Result, candidate: execution_machine.Result) bool {
    const host_tag = std.meta.activeTag(host);
    const candidate_tag = std.meta.activeTag(candidate);
    if (host_tag != candidate_tag) {
        std.debug.print("FAIL {s}: disposition mismatch (host={s}, candidate={s})\n", .{
            path, @tagName(host_tag), @tagName(candidate_tag),
        });
        return false;
    }
    switch (host) {
        .accepted => |expected| switch (candidate) {
            .accepted => |actual| {
                if (!std.mem.eql(u8, &expected, &actual)) {
                    const actual_hex = std.fmt.bytesToHex(actual, .lower);
                    const expected_hex = std.fmt.bytesToHex(expected, .lower);
                    std.debug.print("FAIL {s}: accepted output mismatch\n  got:      0x{s}\n  expected: 0x{s}\n", .{
                        path, &actual_hex, &expected_hex,
                    });
                    return false;
                }
            },
            .rejected => unreachable,
        },
        .rejected => {},
    }
    return true;
}
