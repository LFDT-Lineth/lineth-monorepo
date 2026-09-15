const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    // Same freestanding rv64im guest target + entry stub + linker script as the
    // verifier and the other riscv-guests, via the shared build_common helper.
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

    // The verifier module requires `r5_config` (see src/crypto/poseidon2.zig).
    // This is always an R5 guest build; `-Ddisable-accelerators=true` selects the
    // software permutation so the benchmark can price the accelerator opcode.
    const disable_accelerators = b.option(
        bool,
        "disable-accelerators",
        "Use the software Poseidon2 permutation instead of the accelerator opcode",
    ) orelse false;
    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", true);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);

    const accel_dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const accel_mod = accel_dep.module("lineth_accelerators");

    if (!disable_accelerators) {
        verifier_mod.addImport("lineth_accelerators", accel_mod);
    }

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
