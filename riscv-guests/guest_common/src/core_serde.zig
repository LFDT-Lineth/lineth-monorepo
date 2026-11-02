//! The cross-guest wire contracts: one namespace per guest boundary, each holding that boundary's
//! types and their canonical SSZ codec.
//!
//! On the freestanding target the output commitments hash through the `zkvm_keccak256` symbol,
//! which the linking program defines; elsewhere through std's Keccak256.

pub const l2_execution_proof = @import("core_serde/l2_execution_proof.zig");
pub const rollup_proof = @import("core_serde/rollup_proof.zig");
