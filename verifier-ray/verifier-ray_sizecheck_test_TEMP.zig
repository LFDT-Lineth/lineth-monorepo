const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");
const proof_codec = verifier_ray.proof_codec;

test "print DecodeScratch size" {
    const systems = riscv_system.system_0_systems;
    const T = proof_codec.DecodeScratch(systems);
    std.debug.print("\nsizeOf(DecodeScratch) = {} bytes ({d:.2} MB)\n", .{ @sizeOf(T), @as(f64, @floatFromInt(@sizeOf(T))) / (1024.0 * 1024.0) });
}
