const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile every guest targets (shared helper).
    const target = common.standardGuestTarget(b);

    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed while still defaulting to ReleaseSmall — mirrors l2-execution's own build.zig.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    const gp_name = "rollup_aggregation_guest";

    // ── Guest: statically-linked rv64im ELF ───────────────────────────────────────────────────
    // Two path dependencies: guest_common for the SSZ codec primitives, lineth_accelerators for
    // the zkVM io-interface (read_input/write_output).
    const guest_common_mod = b.dependency("guest_common", .{ .target = target, .optimize = optimize }).module("guest_common");
    const lineth_accel_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const rollup_aggregation_ssz_mod = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation_ssz.zig"),
        .target = target,
        .optimize = optimize,
    });
    rollup_aggregation_ssz_mod.addImport("guest_common", guest_common_mod);

    // `rollup_aggregation_guest.zig` relatively imports its sibling `rollup_aggregation.zig`/
    // `guest_errors.zig` (same module, no separate wiring needed); only the two path
    // dependencies above are named imports.
    const guest_module = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation_guest.zig"),
        .target = target,
        .optimize = optimize,
    });
    guest_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_mod);
    guest_module.addImport("lineth_accelerators", lineth_accel_mod);
    common.clearFreestandingNativeLinkage(b, guest_module);
    common.installGuestElf(b, guest_module, gp_name);

    // ── Native tests ───────────────────────────────────────────────────────────────────────────
    // Like every other `b.addTest` artifact in this tree, these run on the native host: a
    // freestanding riscv64 target has no OS to run a `std.testing` binary against. Only
    // `rollup_aggregation_ssz`/`rollup_aggregation` are needed natively —
    // `rollup_aggregation_guest.zig`'s `guestMain` (the only caller of
    // `lineth_accelerators`) is never referenced by a test, so Zig's lazy analysis never compiles
    // that path for the native target (mirrors l2-execution's own `linea_zkvm_io`/`zkvm_provide`
    // omission from its native `guest_mod`).
    const native_target = b.resolveTargetQuery(.{});
    const guest_common_native_mod = b.dependency("guest_common", .{ .target = native_target, .optimize = .Debug }).module("guest_common");

    const rollup_aggregation_ssz_native_mod = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation_ssz.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    rollup_aggregation_ssz_native_mod.addImport("guest_common", guest_common_native_mod);

    const rollup_aggregation_native_mod = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    rollup_aggregation_native_mod.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);

    const test_step = b.step("test", "Run native Zig unit tests for the rollup-aggregation guest");

    const rollup_aggregation_ssz_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/rollup_aggregation_ssz_test.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    rollup_aggregation_ssz_tests.root_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);
    test_step.dependOn(&b.addRunArtifact(rollup_aggregation_ssz_tests).step);

    const rollup_aggregation_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/rollup_aggregation_test.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    rollup_aggregation_tests.root_module.addImport("rollup_aggregation", rollup_aggregation_native_mod);
    rollup_aggregation_tests.root_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);
    test_step.dependOn(&b.addRunArtifact(rollup_aggregation_tests).step);
}
