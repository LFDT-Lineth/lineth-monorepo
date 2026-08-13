const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    // Same freestanding rv64im guest target + entry stub + linker script as the
    // verifier and the other riscv-guests, via the shared build_common helper.
    const target = common.standardGuestTarget(b);
    const optimize: std.builtin.OptimizeMode = .ReleaseSmall;

    // This benchmark only ever builds for the R5 guest target, so `is_r5_zkvm`
    // is unconditionally true here rather than an option as it is in
    // ../../build.zig. Accelerators follow the same `-Ddisable-accelerators`
    // convention as the main build so the two can be compared directly:
    // measuring the software Poseidon2 path is the point of this benchmark, but
    // the accelerated path is a one-flag change.
    const disable_accelerators = b.option(
        bool,
        "disable-accelerators",
        "Disable Lineth zkVM accelerator wrappers (measure the software path)",
    ) orelse true;

    const accel_dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const accel_mod = accel_dep.module("lineth_accelerators");

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("../../src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
    });
    // src/crypto/poseidon2.zig imports `lineth_accelerators` only when
    // r5_config.disable_accelerators is false, so the import must be supplied on
    // exactly the same condition or the module fails to resolve.
    if (!disable_accelerators) {
        verifier_mod.addImport("lineth_accelerators", accel_mod);
    }

    // r5_config is required by src/crypto/poseidon2.zig since #3455 (Poseidon2
    // acceleration). Without it the guest does not compile at all; this file had
    // not been updated to match, so the benchmark had been broken since then.
    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", true);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);

    const profiling_opts = b.addOptions();
    profiling_opts.addOption(bool, "is_enabled", false);
    profiling_opts.addOption(bool, "is_r5_marks", true);
    verifier_mod.addOptions("profiling_config", profiling_opts);

    const main_mod = b.createModule(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
        .strip = true,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
            .{ .name = "lineth_accelerators", .module = accel_mod },
        },
    });

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s)
    // + rv64im memory layout + dead-section GC.
    common.installGuestElf(b, main_mod, "bench-compress");
}
