const std = @import("std");
const execution_machine = @import("execution_machine");
const l2_execution = @import("l2_execution");
const l2_execution_ssz = @import("l2_execution_ssz");

pub const HostMachine = struct {
    pub fn run(
        _: *HostMachine,
        init: std.process.Init,
        alloc: std.mem.Allocator,
        input: []const u8,
    ) !execution_machine.Result {
        _ = init;
        const decoded = l2_execution_ssz.decodeInput(alloc, input) catch |err| {
            if (err == error.OutOfMemory) return err;
            return .{ .rejected = err };
        };
        const execution = l2_execution.runL2Execution(alloc, decoded) catch |err| {
            if (err == error.OutOfMemory) return err;
            return .{ .rejected = err };
        };
        return .{ .accepted = l2_execution_ssz.encodeOutput(execution.public_inputs) };
    }
};
