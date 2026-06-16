const std = @import("std");

pub fn build(b: *std.Build) void {
    // Default to a strict RV64IM target (base integer + multiply/divide only):
    //   - baseline_rv64: RV64I base ISA
    //   - adds M extension: integer multiply/divide
    //   - strips A (atomics), C (compressed), D (double float), F (single float),
    //     Zicsr (CSR instructions), Zaamo/Zalrsc (atomic sub-extensions)
    const target = b.standardTargetOptions(.{
        .default_target = .{
            .cpu_arch = .riscv64,
            .cpu_model = .{ .explicit = &std.Target.riscv.cpu.generic_rv64 },
            .cpu_features_add = std.Target.riscv.featureSet(&.{.m}), // Zicclsm extension does not affect the generated ELF so it is omitted
            .cpu_features_sub = std.Target.riscv.featureSet(&.{ .a, .c, .d, .f, .zicsr, .zaamo, .zalrsc }),
            .os_tag = .freestanding,
            .abi = .none, // LP64 (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted
        },
    });

    // Optimize for binary size by default; can be overridden with -Doptimize=
    const optimize = b.standardOptimizeOption(.{
        .preferred_optimize_mode = .ReleaseSmall,
    });

    const path = b.option([]const u8, "path", "Source path under src/, without .zig") orelse @panic("'-Dpath=<path>' is required");

    // e.g. path = "src_optional_subfolder/your_test"
    const source = b.fmt("src/{s}.zig", .{path});

    // binary name = "your_test", not "src_optional_subfolder/your_test"
    const exe_name = std.fs.path.stem(std.fs.path.basename(path));

    const exe = b.addExecutable(.{
        .name = exe_name,
        .root_module = b.createModule(.{
            .root_source_file = b.path(source),
            .target = target,
            .optimize = optimize,
        }),
    });

    // exposing the zkvm wrappers
    const wrappers = b.createModule(.{
        .root_source_file = b.path("../../wrappers/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    exe.root_module.addImport("wrappers", wrappers);

    // Point to assembly overwriting default SP with the one defined in the linker script
    exe.root_module.addAssemblyFile(b.path("src/start.s"));
    exe.setLinkerScript(b.path("../linker_script.ld"));

    // Remove unused code sections
    exe.link_gc_sections = true;

    b.installArtifact(exe);
}
