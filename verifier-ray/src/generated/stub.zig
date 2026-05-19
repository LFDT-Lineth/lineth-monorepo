const proof_mod = @import("../proof.zig");
const runtime_mod = @import("../runtime.zig");
const verifier = @import("../verifier.zig");

pub fn verifyGenerated(rt: *runtime_mod.Runtime, p: proof_mod.Proof) verifier.VerifyError!void {
    _ = p;
    rt.advanceRound();
    return verifier.VerifyError.Unsupported;
}
