const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
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

    // exposing the zkvm wrappers. Used for accelerated ops, exit codes and exit.
    const lineth_accel_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");
    root_mod.addImport("lineth_zkvm_accel", lineth_accel_mod);

    // non-accelerated precompiles from l2-execution (through zesu implementation). We set keccak-accel=false to force the standard zesu keccak to native implementation of keccak
    const provide_mod = b.dependency("l2_execution", .{ .target = target, .optimize = optimize, .@"keccak-accel" = false }).module("zkvm_provide");
    root_mod.addImport("zkvm_provide", provide_mod);

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s, which calls `main`)
    // + rv64im memory layout + GC from build_common. The test programs export `main`.
    common.installGuestElf(b, root_mod, exe_name);
}
