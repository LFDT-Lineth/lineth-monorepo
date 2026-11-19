//! The cross-guest wire contracts: one namespace per guest boundary, each holding that boundary's
//! types and their canonical SSZ codec.
//!
//! The output commitments hash through the `zkvm_keccak256` accelerator symbol on every target;
//! the linking program provides it.

pub const l2_execution_proof = @import("core_serde/l2_execution_proof.zig");
pub const rollup_proof = @import("core_serde/rollup_proof.zig");
