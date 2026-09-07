const std = @import("std");
const spec_runner = @import("spec_runner.zig");
const zkevm_fixture = @import("zkevm_fixture.zig");
const vanilla_wrap = @import("vanilla_wrap");

pub const Policy = enum {
    allow_linea_rejections,
    skip_linea_unsupported,
};

pub const CaseContext = struct {
    file_path: []const u8,
    test_name: []const u8,
    block_index: usize,
    network: ?[]const u8,
};

pub fn Suite(comptime Machine: type) type {
    return struct {
        const Self = @This();

        machine: *Machine,
        policy: Policy,
        fork_filter: ?[]const u8 = null,
        record_rejection: ?*const fn (?*anyopaque, []const u8) void = null,
        record_context: ?*anyopaque = null,

        pub fn processFile(
            self: *Self,
            init: std.process.Init,
            path: []const u8,
            case_limit: ?u64,
        ) !spec_runner.FileResult {
            if (!std.mem.endsWith(u8, path, ".json")) return .unrecognized;

            var contribution = spec_runner.Contribution{ .files = 1 };
            var arena = std.heap.ArenaAllocator.init(init.gpa);
            defer arena.deinit();
            const alloc = arena.allocator();
            const text = std.Io.Dir.cwd().readFileAlloc(init.io, path, alloc, .limited(1 << 30)) catch |err| {
                std.debug.print("FAIL cannot read '{s}': {s}\n", .{ path, @errorName(err) });
                contribution.cases = 1;
                contribution.failed = 1;
                return .{ .recognized = contribution };
            };
            const blocks = zkevm_fixture.parseBlocks(alloc, text) catch |err| {
                std.debug.print("FAIL parse failed in '{s}': {s}\n", .{ path, @errorName(err) });
                contribution.cases = 1;
                contribution.failed = 1;
                return .{ .recognized = contribution };
            };

            for (blocks) |block| {
                if (case_limit) |limit| if (contribution.cases >= limit) break;
                if (self.fork_filter) |fork| {
                    const network = block.network orelse continue;
                    if (!std.ascii.eqlIgnoreCase(network, fork)) continue;
                }
                contribution.cases += 1;
                const context = CaseContext{
                    .file_path = path,
                    .test_name = block.test_name,
                    .block_index = block.block_index,
                    .network = block.network,
                };

                if (vanilla_wrap.vanillaHasForkActivationSchedule(alloc, block.input) catch false) {
                    contribution.skipped += 1;
                    continue;
                }
                if (self.policy == .skip_linea_unsupported and try hasUnsupportedPolicyInput(alloc, block.input)) {
                    contribution.skipped += 1;
                    continue;
                }
                if (block.expected_output.len <= 32) {
                    std.debug.print("FAIL {s}[{}] expected output is {} bytes\n", .{ context.test_name, context.block_index, block.expected_output.len });
                    contribution.failed += 1;
                    continue;
                }

                const expected_accepted = block.expected_output[32] == 0x01;
                const wrapped = vanilla_wrap.wrapVanillaAsExtended(alloc, block.input) catch {
                    if (expected_accepted) {
                        std.debug.print("FAIL {s}[{}] fixture=valid machine=rejected (WrapInputFailed)\n", .{ context.test_name, context.block_index });
                        contribution.failed += 1;
                    } else contribution.passed += 1;
                    continue;
                };
                const result = self.machine.run(init, alloc, wrapped) catch |err| {
                    std.debug.print("FAIL {s}[{}] machine error: {s}\n", .{ context.test_name, context.block_index, @errorName(err) });
                    contribution.failed += 1;
                    continue;
                };
                const accepted = result == .accepted;
                const rejection_reason = switch (result) {
                    .accepted => null,
                    .rejected => |reason| reason,
                };
                if (accepted == expected_accepted or
                    (!accepted and expected_accepted and self.policy == .allow_linea_rejections and isAllowedRejection(rejection_reason)))
                {
                    contribution.passed += 1;
                    continue;
                }

                if (rejection_reason) |reason| {
                    if (self.record_rejection) |record| record(self.record_context, @errorName(reason));
                    std.debug.print("FAIL {s}[{}] disagree: fixture={s} machine={s} ({s})\n", .{
                        context.test_name,
                        context.block_index,
                        if (expected_accepted) "valid" else "invalid",
                        if (accepted) "valid" else "invalid",
                        @errorName(reason),
                    });
                } else {
                    std.debug.print("FAIL {s}[{}] disagree: fixture={s} machine={s}\n", .{
                        context.test_name,
                        context.block_index,
                        if (expected_accepted) "valid" else "invalid",
                        if (accepted) "valid" else "invalid",
                    });
                }
                contribution.failed += 1;
            }
            return .{ .recognized = contribution };
        }
    };
}

fn hasUnsupportedPolicyInput(alloc: std.mem.Allocator, input: []const u8) !bool {
    const has_requests = vanilla_wrap.vanillaHasExecutionRequests(alloc, input) catch |err| {
        if (err == error.OutOfMemory) return err;
        return false;
    };
    const has_withdrawals = vanilla_wrap.vanillaHasWithdrawals(alloc, input) catch |err| {
        if (err == error.OutOfMemory) return err;
        return false;
    };
    return has_requests or has_withdrawals;
}

fn isAllowedRejection(reason: ?anyerror) bool {
    const err = reason orelse return false;
    return err == error.ExecutionRequestsNotSupported or err == error.WithdrawalsNotSupported;
}
