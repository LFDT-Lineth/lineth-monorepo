const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    const r5 = b.option(bool, "r5", "Build for the Linea R5 zkVM target") orelse false;
    // Allow disabling the Linea zkVM accelerators wrappers for testing purposes. However, we only have them for the R5 target, so it is disabled by default.
    const disable_accelerators = (b.option(bool, "disable-accelerators", "Disable Linea zkVM accelerator wrappers") orelse false) or !r5;
    // TODO: consider adding a "release" option that sets optimize to ReleaseFast instead of ReleaseSmall.
    // For R5 the ReleaseFast optimization causes 2x binary size increase but 1/3 reduction in execution time, so it may be worth having if the binary size is not a concern.
    // For native execution we don't really care about the difference between ReleaseSmall and ReleaseFast, so we can just use ReleaseSmall for the optimized native build.
    const optimize = if (r5)
        b.standardOptimizeOption(.{ .preferred_optimize_mode = .ReleaseSmall })
    else
        b.standardOptimizeOption(.{});
    const strip = b.option(bool, "strip", "Omit debug symbols") orelse (r5 or optimize == .ReleaseSmall);

    // R5 builds the shared freestanding rv64im ZkC profile (build_common); native builds the host.
    const target = if (r5)
        common.standardGuestTarget(b)
    else
        b.standardTargetOptions(.{});

    // Linea zkVM accelerator - zkvm_exit and precompile accelerators etc.
    const lineth_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
    });
    // conditionally import the Linea zkVM accelerator module for supported target and when requested
    if (!disable_accelerators) {
        verifier_mod.addImport("lineth_accelerators", lineth_mod);
    }
    // add option for comptime configuration of R5/accelerator-specific code paths in the verifier module
    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", r5);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);
    const test_vectors_mod = b.addModule("test_vectors", .{
        .root_source_file = b.path("testdata/generated/vectors.zig"),
        .target = target,
        .optimize = optimize,
    });
    const test_vanishing_mod = b.addModule("test_vanishing", .{
        .root_source_file = b.path("testdata/generated/vanishing.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
        },
    });

    const main_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
            // unconditional import for zkvm_exit
            .{ .name = "lineth_accelerators", .module = lineth_mod },
        },
    });

    if (r5) {
        // Statically-linked rv64im ELF using the shared entry stub (start.s, which calls `main`) +
        // rv64im memory layout + GC from build_common. main.zig exports its R5 entry as `main`.
        common.installGuestElf(b, main_mod, "verifier-ray");
    } else {
        main_mod.link_libc = true;
        const exe = b.addExecutable(.{ .name = "verifier-ray", .root_module = main_mod });
        b.installArtifact(exe);

        const run_exe = b.addRunArtifact(exe);
        if (b.args) |args| run_exe.addArgs(args);

        const run_step = b.step("run", "Run verifier-ray natively");
        run_step.dependOn(&run_exe.step);

        const unit_tests = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/all.zig"),
                .target = target,
                .optimize = optimize,
                .imports = &.{
                    .{ .name = "verifier_ray", .module = verifier_mod },
                    .{ .name = "test_vectors", .module = test_vectors_mod },
                    .{ .name = "test_vanishing", .module = test_vanishing_mod },
                },
            }),
        });

        const run_unit_tests = b.addRunArtifact(unit_tests);
        const test_step = b.step("test", "Run verifier-ray unit tests");
        test_step.dependOn(&run_unit_tests.step);
    }
}
