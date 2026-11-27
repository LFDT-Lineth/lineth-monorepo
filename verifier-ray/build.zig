const std = @import("std");
const common = @import("build_common");

// Build-option enum: the CLI value (`-Dembedded-input=<value>`) is matched
// against these field names directly, so keep them as the strings users type.
const EmbeddedInputType = enum {
    none,
    valid,
    invalid,
};

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    const r5 = b.option(bool, "r5", "Build for the Lineth R5 zkVM target") orelse false;
    // Allow disabling the Lineth zkVM accelerator wrappers for testing purposes. We only have them for the R5 target, so they are disabled by default unless the r5 option is set.
    const disable_accelerators = (b.option(bool, "disable-accelerators", "Disable Lineth zkVM accelerator wrappers") orelse false) or !r5;
    const verifier_profiling = b.option(
        bool,
        "verifier-profiling",
        "Enable comptime inclusion of profiling counters",
    ) orelse false;
    const r5_marks_arg = (b.option(bool, "r5-marks", "Enable R5 phase markers") orelse false) and r5; // only allow R5 marks when building for R5 target
    // The `embedded-spec` option gives the index of the embedded specification from the fixtures. Later we can make this option more
    // flexible to also allow specifying a path to a spec file, but for now we just embed the spec from the fixtures. This is only used
    // for execution target, not for any test fixtures or library.
    const embedded_spec = b.option(usize, "embedded-spec", "Embedded specification index") orelse 0;
    // The `embedded-input` option is used to embed the input file into the binary to avoid needing to pass it in at runtime as we don't have
    // input serialization yet. This is only used for execution target, not for any test fixtures or library.
    const embedded_input = b.option(EmbeddedInputType, "embedded-input", "Embed the input file into the binary") orelse EmbeddedInputType.none;
    const test_filter = b.option([]const u8, "test-filter", "Skip tests that do not match this filter");
    const test_filters: []const []const u8 = if (test_filter) |f| &.{f} else &.{};
    const input_path = b.option(
        []const u8,
        "input-path",
        "Path to the single proof image for the native smoke runner",
    ) orelse "testdata/devmode_proof_image.bin";
    const target = if (r5)
        common.standardGuestTarget(b)
    else
        b.standardTargetOptions(.{});
    // TODO: consider adding a "release" option that sets optimize to ReleaseFast instead of ReleaseSmall.
    // For R5 the ReleaseFast optimization causes 2x binary size increase but 1/3 reduction in execution time, so it may be worth having if the binary size is not a concern.
    // For native execution we don't really care about the difference between ReleaseSmall and ReleaseFast, so we can just use ReleaseSmall for the optimized native build.
    // The R5 optimize mode is a plain `-Doptimize` (default ReleaseSmall) so that a package that
    // depends on this one can pass it through `b.dependency`.
    const optimize = if (r5)
        b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall
    else
        b.standardOptimizeOption(.{});
    const strip = b.option(bool, "strip", "Omit debug symbols") orelse (r5 or optimize == .ReleaseSmall);

    // Lineth zkVM accelerator - zkvm_exit and precompile accelerators etc.
    const lineth_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
    });
    // conditionally import the Lineth zkVM accelerator module for supported target and when requested
    if (!disable_accelerators) {
        verifier_mod.addImport("lineth_accelerators", lineth_mod);
    }
    // add option for comptime configuration of R5/accelerator-specific code paths in the verifier module
    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", r5);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);
    const profiling_opts = b.addOptions();
    profiling_opts.addOption(bool, "is_enabled", verifier_profiling);
    profiling_opts.addOption(bool, "is_r5_marks", r5_marks_arg);
    verifier_mod.addOptions("profiling_config", profiling_opts);

    // The full-pipeline verify fixtures (PCS-enabled), exposed to tests so the
    // integration tests can drive verifier.verify against real proofs.
    const test_verify_mod = b.addModule("test_verify", .{
        .root_source_file = b.path("testdata/generated/verify.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
        },
    });

    // The generated devmode system (`prover verifier-assets --system devmode`
    // in prover-ray): the system the smoke runner and the rollup guest verify.
    const devmode_system_mod = b.addModule("devmode_system", .{
        .root_source_file = b.path("testdata/generated/devmode_system.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
        },
    });

    const embedded_data_opts = b.addOptions();
    embedded_data_opts.addOption(usize, "spec_index", embedded_spec);
    embedded_data_opts.addOption(bool, "embed_input", embedded_input != EmbeddedInputType.none);
    embedded_data_opts.addOption(bool, "invalid_input", embedded_input == EmbeddedInputType.invalid);

    const main_opts = b.addOptions();
    main_opts.addOption([]const u8, "input_path", input_path);
    const embedded_data_mod = b.addModule("embedded_data", .{
        .root_source_file = b.path("testdata/generated/verify.zig"),
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
            .{ .name = "embedded_data", .module = embedded_data_mod },
            .{ .name = "embedded_data_config", .module = embedded_data_opts.createModule() },
            .{ .name = "main_config", .module = main_opts.createModule() },
            .{ .name = "devmode_system", .module = devmode_system_mod },
        },
    });

    if (r5) {
        // unconditional import for zkvm_exit
        main_mod.addImport("lineth_accelerators", lineth_mod);
        // Link the statically-linked rv64im ELF with the shared entry stub (start.s) + rv64im memory
        // layout + dead-section GC
        common.installGuestElf(b, main_mod, "verifier-ray");
    } else {
        const exe = b.addExecutable(.{ .name = "verifier-ray", .root_module = main_mod });
        exe.root_module.link_libc = true;
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
                // proof_image_test.zig privately maps and rebases a
                // Go-produced image, which needs mmap from libc.
                .link_libc = true,
                .imports = &.{
                    .{ .name = "verifier_ray", .module = verifier_mod },
                    .{ .name = "test_verify", .module = test_verify_mod },
                    .{ .name = "devmode_system", .module = devmode_system_mod },
                },
            }),
            .filters = test_filters,
        });

        const run_unit_tests = b.addRunArtifact(unit_tests);
        const test_step = b.step("test", "Run verifier-ray unit tests");
        test_step.dependOn(&run_unit_tests.step);

        // Profiling counters only do anything when profiling is enabled at build
        // time. This test fails if profiling is not enabled, so it serves as a
        // canary to ensure that the profiling tests are actually testing something
        // and not silently passing due to profiling being disabled.
        const profiling_tests = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/profiling_test.zig"),
                .target = target,
                .optimize = optimize,
                .imports = &.{
                    .{ .name = "verifier_ray", .module = verifier_mod },
                },
            }),
        });

        const run_profiling_tests = b.addRunArtifact(profiling_tests);
        const profiling_test_step = b.step("test-profiling", "Run profiling counter tests");
        profiling_test_step.dependOn(&run_profiling_tests.step);
    }
}
