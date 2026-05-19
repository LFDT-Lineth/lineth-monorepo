const proof_mod = @import("../proof.zig");

pub const Proof = struct {
    siblings: []const proof_mod.Commitment,
};

pub const Error = error{Unsupported};

pub fn verify(_: Proof, _: proof_mod.Commitment, _: proof_mod.Commitment) Error!void {
    return Error.Unsupported;
}
