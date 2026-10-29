const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // src/dummy.zig is both the verifier and the (embedded) circuit, so it is
    // exported under the two module names a guest imports from verifier-ray:
    // `verifier_ray` and `riscv_system`.
    // verifier-ray's library: its field, proof types and image_relocation.
    const verifier_ray_lib = b.dependency("verifier_ray", .{ .target = target, .optimize = optimize }).module("verifier_ray");
    const dummy_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("src/dummy.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{.{ .name = "verifier_ray_lib", .module = verifier_ray_lib }},
    });
    b.modules.put(b.allocator, b.dupe("riscv_system"), dummy_mod) catch @panic("OOM");

    // Unit tests, including the Go/Zig agreement on the proof images written by
    // prover-ray/dummy (TestZigFixturesAreUpToDate).
    const test_mod = b.createModule(.{
        .root_source_file = b.path("test/dummy_test.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "verifier_ray", .module = dummy_mod },
            .{ .name = "riscv_system", .module = dummy_mod },
        },
    });
    for ([_][]const u8{ "dummy_image", "dummy_image_tampered", "dummy_pair_image", "dummy_pair_image_mismatch" }) |name| {
        test_mod.addAnonymousImport(name, .{ .root_source_file = b.path(b.fmt("testdata/{s}.bin", .{name})) });
    }
    // A real verifier-ray proof image, to check that the dummy rejects it. Test
    // data only; the package does not depend on verifier-ray.
    test_mod.addAnonymousImport("real_image", .{ .root_source_file = b.path("../../../verifier-ray/testdata/proof_image.bin") });

    const tests = b.addTest(.{ .root_module = test_mod });
    const test_step = b.step("test", "Run the dummy verifier unit tests");
    test_step.dependOn(&b.addRunArtifact(tests).step);

    // Compiles the verifier for the R5 guest target (riscv-guests/build_common's
    // standardGuestTarget).
    const r5_target = b.resolveTargetQuery(.{
        .cpu_arch = .riscv64,
        .cpu_model = .{ .explicit = &std.Target.riscv.cpu.generic_rv64 },
        .cpu_features_add = std.Target.riscv.featureSet(&.{.m}),
        .cpu_features_sub = std.Target.riscv.featureSet(&.{ .a, .c, .d, .f, .zicsr, .zaamo, .zalrsc }),
        .os_tag = .freestanding,
        .abi = .none,
    });
    // For R5, verifier-ray picks the guest target and its release mode itself.
    const r5_verifier_ray_lib = b.dependency("verifier_ray", .{ .r5 = true, .release = true }).module("verifier_ray");
    const r5_dummy_mod = b.createModule(.{
        .root_source_file = b.path("src/dummy.zig"),
        .target = r5_target,
        .optimize = .ReleaseSmall,
        .imports = &.{.{ .name = "verifier_ray_lib", .module = r5_verifier_ray_lib }},
    });
    const r5_lib = b.addLibrary(.{
        .name = "dummy-verifier-r5",
        .linkage = .static,
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/r5_check.zig"),
            .target = r5_target,
            .optimize = .ReleaseSmall,
            .imports = &.{
                .{ .name = "verifier_ray", .module = r5_dummy_mod },
                .{ .name = "riscv_system", .module = r5_dummy_mod },
            },
        }),
    });
    const r5_step = b.step("check-r5", "Compile the dummy verifier for the R5 guest target");
    r5_step.dependOn(&r5_lib.step);
}
