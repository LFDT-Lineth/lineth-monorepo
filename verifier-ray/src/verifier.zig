const proof_mod = @import("proof.zig");
const runtime_mod = @import("runtime.zig");
const generated = @import("generated/stub.zig");

pub const VerifyError = error{
    EmptyProof,
    Unsupported,
    InvalidProof,
};

pub fn verify(p: proof_mod.Proof) VerifyError!void {
    if (p.proof_bytes.len == 0 and p.commitments.len == 0 and p.public_inputs.len == 0) {
        return VerifyError.EmptyProof;
    }

    var rt = runtime_mod.Runtime.init();
    return generated.verifyGenerated(&rt, p);
}
