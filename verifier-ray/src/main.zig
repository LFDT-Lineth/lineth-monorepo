const verifier_ray = @import("verifier_ray");

pub fn main() void {
    _ = verifier_ray.verify(verifier_ray.Proof.empty()) catch {};
}
