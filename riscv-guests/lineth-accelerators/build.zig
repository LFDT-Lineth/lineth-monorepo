//! Linea zkVM accelerator wrappers, packaged as a reusable Zig module.
//!
//! Exposes the `lineth_accelerators` module: thin wrappers that issue the custom RISC-V
//! instructions the Linea prover accelerates (keccak, the Poseidon2 permutation, …) plus the
//! standard runtime helpers (zkvm_exit). The matching C interface headers live under include/.
//!
//! Consumers add a path dependency in their build.zig.zon and import the module, e.g.
//!   const dep = b.dependency("lineth_accelerators", .{});
//!   some_module.addImport("linea_zkvm_accel", dep.module("lineth_accelerators"));
//! The module carries no target of its own; it is compiled for whatever target the importing
//! module resolves to (every guest builds the freestanding rv64im ZkC profile).

const std = @import("std");

pub fn build(b: *std.Build) void {
    _ = b.addModule("lineth_accelerators", .{
        .root_source_file = b.path("src/root.zig"),
    });
}
