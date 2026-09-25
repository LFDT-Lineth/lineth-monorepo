const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    const target = common.standardGuestTarget(b);
    const optimize: std.builtin.OptimizeMode = .ReleaseSmall;
    const disable_accelerators = b.option(bool, "disable-accelerators", "Disable Poseidon2 accelerator wrappers") orelse false;
    const accel_dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const accel_mod = accel_dep.module("lineth_accelerators");

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("../../src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
    });
    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", true);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);
    if (!disable_accelerators) verifier_mod.addImport("lineth_accelerators", accel_mod);
    const profiling_options = b.addOptions();
    profiling_options.addOption(bool, "is_enabled", false);
    profiling_options.addOption(bool, "is_r5_marks", true);
    verifier_mod.addOptions("profiling_config", profiling_options);

    const fixture_mod = b.addModule("large_pcs_fixture", .{
        .root_source_file = b.path("../../zig-out/bench/large-pcs.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{.{ .name = "verifier_ray", .module = verifier_mod }},
    });
    const main_mod = b.createModule(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
            .{ .name = "large_pcs_fixture", .module = fixture_mod },
            .{ .name = "lineth_accelerators", .module = accel_mod },
        },
    });

    common.installGuestElf(b, main_mod, "bench-pcs");
}
