//! The cross-guest wire contracts: one namespace per guest boundary, each holding that boundary's
//! types and their canonical SSZ codec.

pub const l2_execution_proof = @import("core_serde/l2_execution_proof.zig");
