//! guest-common — small, generic Zig utilities shared by riscv-guests programs, packaged as a
//! reusable Zig module (mirrors lineth-accelerators' own root.zig).
//!
//! Exposes `ssz`: the fixed-width little-endian integer reads/writes and the generic
//! `List[VariableSizeType, N]` SSZ offset-table codec every guest's own container-specific codec
//! builds on. Pure `std`, freestanding-safe, so it compiles into a guest's rv64im ELF as readily as
//! into a native host test.
//!
//! Consumers add a path dependency in their build.zig.zon and import the module, passing the
//! target/optimize they build for (every guest builds the freestanding rv64im ZkC profile), e.g.
//!   const dep = b.dependency("guest_common", .{ .target = target, .optimize = optimize });
//!   some_module.addImport("guest_common", dep.module("guest_common"));

pub const ssz = @import("ssz.zig");
