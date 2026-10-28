const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    const keccak_accel = b.option(bool, "keccak-accel", "Enable zkc accelerated keccak precompile (argument passthrough to lineth_accelerators)") orelse false;
    // The shared freestanding rv64im ZkC profile every guest builds for (build_common).
    const target = common.standardGuestTarget(b);

    // Optimize for binary size by default; can be overridden with -Doptimize=
    const optimize = b.standardOptimizeOption(.{
        .preferred_optimize_mode = .ReleaseSmall,
    });

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

    // exposing the zkvm wrappers. Used for accelerated ops, exit codes and exit. The same dependency
    // instance provides the non-accelerated precompiles (through zesu implementation); keccak-accel=false
    // forces the standard zig keccak implementation.
    const lineth_accel_dep = b.dependency("lineth_accelerators", .{
        .target = target,
        .optimize = optimize,
        .@"zkvm-provide" = true,
        .@"keccak-accel" = keccak_accel,
    });
    root_mod.addImport("lineth_zkvm_accel", lineth_accel_dep.module("lineth_accelerators"));
    root_mod.addImport("zkvm_provide", lineth_accel_dep.module("zkvm_provide"));

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s, which calls `main`)
    // + rv64im memory layout + GC from build_common. The test programs export `main`.
    common.installGuestElf(b, root_mod, exe_name);
}
