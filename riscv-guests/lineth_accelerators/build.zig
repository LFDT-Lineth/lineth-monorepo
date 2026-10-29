//! Lineth zkVM accelerator wrappers, packaged as a reusable Zig module.
//!
//! Exposes the `lineth_accelerators` module: thin wrappers that issue the custom RISC-V
//! instructions the Lineth prover accelerates plus the standard runtime helpers (zkvm_exit).
//! The matching C interface headers live under include/.
//!
//! Consumers add a path dependency in their build.zig.zon and import the module, passing the
//! target/optimize they build for (every guest builds the freestanding rv64im ZkC profile), e.g.
//!   const dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
//!   some_module.addImport("lineth_zkvm_accel", dep.module("lineth_accelerators"));

const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile as every guest (shared helper); a consumer that passes
    // its own target via b.dependency overrides this default.
    const target = common.standardGuestTarget(b);
    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed — consumers set it through `b.dependency(..., .{ .optimize = ... })` — while still
    // defaulting to ReleaseSmall. (standardOptimizeOption with preferred_optimize_mode would swap
    // `-Doptimize` for `-Drelease`, breaking the dependency pass-through.)
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    _ = b.addModule("lineth_accelerators", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
}
