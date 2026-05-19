const std = @import("std");
const verifier_ray = @import("verifier_ray");

test "vortex verifier reports unsupported until implementation is wired" {
    try std.testing.expectError(
        verifier_ray.vortex.verifier.Error.Unsupported,
        verifier_ray.vortex.verifier.verify(verifier_ray.Proof.empty()),
    );
}
