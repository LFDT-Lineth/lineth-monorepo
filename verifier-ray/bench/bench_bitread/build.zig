const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    const target = common.standardGuestTarget(b);
    const optimize: std.builtin.OptimizeMode = .ReleaseSmall;

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("../../src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
    });
    const profiling_opts = b.addOptions();
    profiling_opts.addOption(bool, "is_enabled", false);
    profiling_opts.addOption(bool, "is_r5_marks", true);
    verifier_mod.addOptions("profiling_config", profiling_opts);

    const accel_dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const accel_mod = accel_dep.module("lineth_accelerators");

    // The reader under test is imported from bench_lzss rather than copied, so
    // these numbers describe the code that actually runs in that benchmark.
    const lzss_mod = b.addModule("lzss", .{
        .root_source_file = b.path("../bench_lzss/lzss.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
    });

    const main_mod = b.createModule(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
            .{ .name = "lineth_accelerators", .module = accel_mod },
            .{ .name = "lzss", .module = lzss_mod },
        },
    });

    common.installGuestElf(b, main_mod, "bench-bitread");
}
