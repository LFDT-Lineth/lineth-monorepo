//! Builds the `guest_common` (generic primitives) and `core_serde` (cross-guest wire contracts)
//! modules. `zig build` checks compilation of the package; `zig build test` runs its unit tests.

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

    const shared = Shared.init(b, target, optimize);
    const guest_common = b.addModule("guest_common", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{ shared.ssz, shared.accelerators },
    });
    _ = b.addModule("core_serde", .{
        .root_source_file = b.path("src/core_serde.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{ .{ .name = "guest_common", .module = guest_common }, shared.accelerators },
    });

    // The host-only checks of the contract tests: consumers import it for their own wire structs.
    const contract = b.addModule("ssz_contract", .{
        .root_source_file = b.path("test/support/ssz_contract.zig"),
        .target = b.resolveTargetQuery(.{}),
        .optimize = .Debug,
    });

    // ── Native unit tests ────────────────────────────────────────────────────
    // Like every other `b.addTest` artifact in this tree, these run on the native host: a
    // freestanding riscv64 target has no OS to run a `std.testing` binary against.
    const native_target = b.resolveTargetQuery(.{});
    const native_shared = Shared.init(b, native_target, .Debug);
    const guest_common_native_mod = common.createPackageModule(b, native_target, .Debug, &.{ native_shared.ssz, native_shared.accelerators });
    const core_serde_native_mod = b.createModule(.{
        .root_source_file = b.path("src/core_serde.zig"),
        .target = native_target,
        .optimize = .Debug,
        .imports = &.{ .{ .name = "guest_common", .module = guest_common_native_mod }, native_shared.accelerators },
    });

    const test_step = b.step("test", "Run native Zig unit tests for guest_common's shared primitives");
    const tests = common.addTests(b, test_step, .{
        .target = native_target,
        .imports = &.{
            .{ .name = "guest_common", .module = guest_common_native_mod },
            .{ .name = "core_serde", .module = core_serde_native_mod },
            .{ .name = "ssz_contract", .module = contract },
            .{ .name = "spec_testdata", .module = common.specTestdataModule(b) },
        },
    }, .{});
    const crypto = common.HostCrypto.forTarget(b, native_target, .{}) orelse return;
    crypto.linkAll(tests);
}

/// The imports the sources name, for one target: the SSZ library and the accelerator calls. Every
/// consumer's instance of the dependencies has the same options, so one compilation holds each
/// module once.
const Shared = struct {
    ssz: std.Build.Module.Import,
    accelerators: std.Build.Module.Import,

    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) Shared {
        const ssz = b.dependency("ssz", .{ .target = target, .optimize = optimize });
        const accelerators = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
        return .{
            .ssz = .{ .name = "ssz", .module = ssz.module("ssz.zig") },
            .accelerators = .{ .name = "zkvm_accelerators", .module = accelerators.module("zkvm_accelerators") },
        };
    }
};
