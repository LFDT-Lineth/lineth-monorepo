const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile as every guest (shared helper).
    const target = common.standardGuestTarget(b);
    // Use b.option directly (not standardOptimizeOption) to keep the `-Doptimize` enum option
    // exposed, mirroring l2-execution/build.zig's own reasoning.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    const lineth_accel_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const guest_module = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });
    guest_module.addImport("lineth_zkvm_accel", lineth_accel_mod);
    common.clearFreestandingNativeLinkage(b, guest_module);
    common.installGuestElf(b, guest_module, "minimal_keccak_guest");
}
