const std = @import("std");

pub fn build(b: *std.Build) void {
    const r5 = b.option(bool, "r5", "Build for the Linea R5 zkVM target") orelse false;
    const vanishing_bench = b.option(bool, "vanishing-bench", "Build the R5 vanishing benchmark guest") orelse false;
    const vanishing_case_index = b.option(usize, "vanishing-case", "Generated vanishing benchmark case index") orelse 0;

    const default_target: std.Target.Query = if (r5) .{
        .cpu_arch = .riscv64,
        .cpu_model = .{ .explicit = &std.Target.riscv.cpu.generic_rv64 },
        .cpu_features_add = std.Target.riscv.featureSet(&.{.m}),
        .cpu_features_sub = std.Target.riscv.featureSet(&.{ .a, .c, .d, .f, .zicsr, .zaamo, .zalrsc }),
        .os_tag = .freestanding,
        .abi = .none,
    } else .{};

    const target = b.standardTargetOptions(.{ .default_target = default_target });
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
    var vanishing_bench_data_mod: *std.Build.Module = undefined;
    if (vanishing_bench) {
        vanishing_bench_data_mod = b.addModule("vanishing_bench_data", .{
            .root_source_file = b.path("bench/generated/vanishing.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "verifier_ray", .module = verifier_mod },
            },
        });
    }
    const vanishing_bench_options = b.addOptions();
    vanishing_bench_options.addOption(usize, "case_index", vanishing_case_index);
    const vanishing_bench_options_mod = vanishing_bench_options.createModule();

    const exe_imports: []const std.Build.Module.Import = if (vanishing_bench) &.{
        .{ .name = "verifier_ray", .module = verifier_mod },
        .{ .name = "vanishing_bench_data", .module = vanishing_bench_data_mod },
        .{ .name = "vanishing_bench_options", .module = vanishing_bench_options_mod },
    } else &.{
        .{ .name = "verifier_ray", .module = verifier_mod },
    };

    const exe = b.addExecutable(.{
        .name = if (vanishing_bench) "verifier-ray-vanishing-bench" else "verifier-ray",
        .root_module = b.createModule(.{
            .root_source_file = b.path(if (vanishing_bench) "bench/vanishing_main.zig" else "src/main.zig"),
            .target = target,
            .optimize = optimize,
            .strip = strip,
            .imports = exe_imports,
        }),
    });

    if (!r5) {
        exe.root_module.link_libc = true;
    }

    if (r5) {
        // Point to assembly overwriting default SP with the one defined in the linker script.
        exe.root_module.addAssemblyFile(b.path("src/start.s"));
        exe.setLinkerScript(b.path("linker_script.ld"));

        // Remove unused code sections for the zkVM binary.
        exe.link_gc_sections = true;
    }

    b.installArtifact(exe);

    if (!r5) {
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
