const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    const r5 = b.option(bool, "r5", "Build for the Linea R5 zkVM target") orelse false;

    // R5 builds the shared freestanding rv64im ZkC profile (build_common); native builds the host.
    const target = if (r5)
        common.standardGuestTarget(b)
    else
        b.standardTargetOptions(.{});
    // TODO: consider adding a "release" option that sets optimize to ReleaseFast instead of ReleaseSmall.
    // For R5 the ReleaseFast optimization causes 2x binary size increase but 1/3 reduction in execution time, so it may be worth having if the binary size is not a concern.
    // For native execution we don't really care about the difference between ReleaseSmall and ReleaseFast, so we can just use ReleaseSmall for the optimized native build.
    const optimize = if (r5)
        b.standardOptimizeOption(.{ .preferred_optimize_mode = .ReleaseSmall })
    else
        b.standardOptimizeOption(.{});
    const strip = b.option(bool, "strip", "Omit debug symbols") orelse (r5 or optimize == .ReleaseSmall);

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
    });
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
    // Linea zkVM accelerator wrappers — main.zig's R5 entry point uses zkvm_exit.
    const lineth_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const main_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
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
