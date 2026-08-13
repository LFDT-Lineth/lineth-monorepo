const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    // Same freestanding rv64im guest target + entry stub + linker script as the
    // verifier and the other riscv-guests, via the shared build_common helper.
    const target = common.standardGuestTarget(b);
    const optimize: std.builtin.OptimizeMode = .ReleaseSmall;

    // LZ4 decode touches no crypto, so accelerators are irrelevant here; kept
    // off by default only to match bench_compress's convention of measuring
    // the plain path unless asked otherwise.
    const disable_accelerators = b.option(
        bool,
        "disable-accelerators",
        "Disable Lineth zkVM accelerator wrappers (unused by this benchmark)",
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
    // exactly the same condition or the module fails to resolve. See
    // bench_compress/build.zig for the full explanation (#3455).
    if (!disable_accelerators) {
        verifier_mod.addImport("lineth_accelerators", accel_mod);
    }

    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", true);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);

    const profiling_opts = b.addOptions();
    profiling_opts.addOption(bool, "is_enabled", false);
    profiling_opts.addOption(bool, "is_r5_marks", true);
    verifier_mod.addOptions("profiling_config", profiling_opts);

    // Vendored jedisct1/zig-lz4 (see vendor/lz4/root.zig for provenance and why
    // it is vendored rather than a normal package dependency).
    const lz4_mod = b.addModule("lz4", .{
        .root_source_file = b.path("vendor/lz4/root.zig"),
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
            .{ .name = "lz4", .module = lz4_mod },
        },
    });

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s)
    // + rv64im memory layout + dead-section GC.
    common.installGuestElf(b, main_mod, "bench-lz4");
}
