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
    const ssz_mod = b.createModule(.{
        .root_source_file = b.path("src/ssz.zig"),
        .target = native_target,
        .optimize = .Debug,
    });

    const test_step = b.step("test", "Run native Zig unit tests for guest_common's shared primitives");
    const ssz_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/ssz_test.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    ssz_tests.root_module.addImport("guest_common_ssz", ssz_mod);
    test_step.dependOn(&b.addRunArtifact(ssz_tests).step);

    const guest_common_native_mod = b.createModule(.{
        .root_source_file = b.path("src/root.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    const l2_execution_proof_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/l2_execution_proof_test.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    l2_execution_proof_tests.root_module.addImport("guest_common", guest_common_native_mod);
    test_step.dependOn(&b.addRunArtifact(l2_execution_proof_tests).step);
}
