const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    const keccak_accel = b.option(bool, "keccak-accel", "Enable zkc accelerated keccak precompile (argument passthrough to lineth_accelerators)");
    const zkvm_provide = b.option(bool, "zkvm-provide", "Link the zkvm_* precompile providers (argument passthrough to lineth_accelerators; fetches zesu, builds Constantine)") orelse false;
    // The shared freestanding rv64im ZkC profile every guest builds for (build_common).
    const target = common.standardGuestTarget(b);

    // Use b.option directly (not standardOptimizeOption) so `-Doptimize=` stays exposed while
    // defaulting to ReleaseSmall (standardOptimizeOption with preferred_optimize_mode defaults to
    // Debug unless `-Drelease` is passed).
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    const path = b.option([]const u8, "path", "Source path under src/, without .zig") orelse @panic("'-Dpath=<path>' is required");

    // e.g. path = "src_optional_subfolder/your_test"
    const source = b.fmt("src/{s}.zig", .{path});

    // binary name = "your_test", not "src_optional_subfolder/your_test"
    const exe_name = std.fs.path.stem(std.fs.path.basename(path));

    const root_mod = b.createModule(.{
        .root_source_file = b.path(source),
        .target = target,
        .optimize = optimize,
    });

    // The zkvm wrappers: accelerated ops, exit codes and exit. With -Dzkvm-provide the same dependency
    // instance also provides the zkvm_* precompile providers; keccak-accel=false selects the standard
    // zig keccak among them.
    const lineth_accel_dep = if (keccak_accel) |accel| b.dependency("lineth_accelerators", .{
        .target = target,
        .optimize = optimize,
        .@"zkvm-provide" = zkvm_provide,
        .@"keccak-accel" = accel,
    }) else b.dependency("lineth_accelerators", .{
        .target = target,
        .optimize = optimize,
        .@"zkvm-provide" = zkvm_provide,
    });
    root_mod.addImport("lineth_zkvm_accel", lineth_accel_dep.module("lineth_accelerators"));
    if (zkvm_provide) root_mod.addImport("zkvm_provide", lineth_accel_dep.module("zkvm_provide"));

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s, which calls `main`)
    // + rv64im memory layout + GC from build_common. The test programs export `main`.
    common.installGuestElf(b, root_mod, exe_name);
}
