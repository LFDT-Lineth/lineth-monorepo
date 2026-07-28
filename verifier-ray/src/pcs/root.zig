//! FRI + PCS opening-proof verifier (KoalaBear), ported from prover-ray
//! `crypto/koalabear/fri`. Verifier-only: no prover, encoders, or FFT.
//!
//! Barrel re-export mirroring `protocol/root.zig`. The top-level entry point is
//! `verify.verify(system, inputs, proof)`.

pub const params = @import("params.zig");
pub const tree = @import("tree.zig");
pub const paired_leaf = @import("paired_leaf.zig");
pub const layout = @import("layout.zig");
pub const fold = @import("fold.zig");
pub const reconstruct = @import("reconstruct.zig");
pub const verify = @import("verify.zig");

// Common public types, re-exported for convenience.
pub const Params = params.Params;
pub const Octuplet = tree.Octuplet;
pub const System = verify.System;
pub const Proof = verify.Proof;
pub const Inputs = verify.Inputs;
