//! Builds the `guest_common` module. `zig build` checks compilation of the package; `zig build
//! test` runs its unit tests.

const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile as every guest (shared helper); a consumer that passes
    // its own target via b.dependency overrides this default.
    const target = common.standardGuestTarget(b);
    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed — consumers set it through `b.dependency(..., .{ .optimize = ... })` — while still
    // defaulting to ReleaseSmall. (standardOptimizeOption with preferred_optimize_mode would swap
    // `-Doptimize` for `-Drelease`, breaking the dependency pass-through.)
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    _ = b.addModule("guest_common", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });

    // ── Native unit tests ────────────────────────────────────────────────────
    // Like every other `b.addTest` artifact in this tree, these run on the native host: a
    // freestanding riscv64 target has no OS to run a `std.testing` binary against.
    const native_target = b.resolveTargetQuery(.{});
    const package_native_mod = common.createPackageModule(b, native_target, .Debug, &.{});

    const test_step = b.step("test", "Run native Zig unit tests for guest_common's shared primitives");
    _ = common.addTests(b, test_step, .{
        .target = native_target,
        .imports = &.{.{ .name = "guest_common", .module = package_native_mod }},
    }, .{});
}
