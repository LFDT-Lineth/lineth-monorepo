const proof_mod = @import("../proof.zig");

pub const Error = error{Unsupported};

pub fn verify(_: proof_mod.Proof) Error!void {
    return Error.Unsupported;
}
