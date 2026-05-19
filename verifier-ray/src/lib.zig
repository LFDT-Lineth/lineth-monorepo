pub const proof = @import("proof.zig");
pub const runtime = @import("runtime.zig");
pub const verifier = @import("verifier.zig");

pub const field = struct {
    pub const koalabear = @import("field/koalabear.zig");
    pub const koalabear_ext = @import("field/koalabear_ext.zig");
    pub const vec = @import("field/vec.zig");
};

pub const crypto = struct {
    pub const fiat_shamir = @import("crypto/fiat_shamir.zig");
    pub const poseidon2 = @import("crypto/poseidon2.zig");
};

pub const pcs = struct {
    pub const lagrange = @import("pcs/lagrange.zig");
    pub const polynomial = @import("pcs/polynomial.zig");
};

pub const precompiles = struct {
    pub const interface = @import("precompiles/interface.zig");
    pub const native = @import("precompiles/native.zig");
    pub const riscv = @import("precompiles/riscv.zig");
};

pub const vortex = struct {
    pub const reed_solomon = @import("vortex/reed_solomon.zig");
    pub const ringsis = @import("vortex/ringsis.zig");
    pub const smt = @import("vortex/smt.zig");
    pub const verifier = @import("vortex/verifier.zig");
};

pub const generated = struct {
    pub const stub = @import("generated/stub.zig");
};

pub const Proof = proof.Proof;
pub const VerifyError = verifier.VerifyError;

pub fn verify(p: Proof) VerifyError!void {
    return verifier.verify(p);
}
