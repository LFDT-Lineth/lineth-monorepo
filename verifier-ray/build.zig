const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});
    const strip = b.option(bool, "strip", "Omit debug symbols") orelse (optimize == .ReleaseSmall);

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

    const exe = b.addExecutable(.{
        .name = "verifier-ray",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
            .strip = strip,
            .imports = &.{
                .{ .name = "verifier_ray", .module = verifier_mod },
            },
        }),
    });
    b.installArtifact(exe);

    const unit_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/all.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "verifier_ray", .module = verifier_mod },
                .{ .name = "test_vectors", .module = test_vectors_mod },
            },
        }),
    });

    const run_unit_tests = b.addRunArtifact(unit_tests);
    const test_step = b.step("test", "Run verifier-ray unit tests");
    test_step.dependOn(&run_unit_tests.step);
}
