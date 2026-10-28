//! Zig code shared by riscv-guests programs: generic primitives and cross-guest wire contracts.
//! Pure `std`, freestanding-safe.

pub const ssz = @import("ssz.zig");
pub const l2_execution_proof = @import("l2_execution_proof.zig");
