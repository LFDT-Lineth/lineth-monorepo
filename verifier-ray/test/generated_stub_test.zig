const std = @import("std");
const verifier_ray = @import("verifier_ray");

test "generated verifier stub advances runtime and reports unsupported" {
    var rt = verifier_ray.runtime.Runtime.init();
    try std.testing.expectError(
        verifier_ray.VerifyError.Unsupported,
        verifier_ray.generated.stub.verifyGenerated(&rt, verifier_ray.Proof.empty()),
    );
    try std.testing.expectEqual(@as(usize, 1), rt.current_round);
}
