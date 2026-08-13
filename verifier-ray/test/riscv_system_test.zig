const std = @import("std");
const riscv_system = @import("riscv_system");

// Smoke test that the codegen output type-checks as a real verifier.Systems
// value — the first point at which the generate-riscv-system pipeline
// (CompileToZig against an honest no-memory.zkc proof) proves itself correct,
// independent of decoding a real proof or running through R5.
test "riscv_system compiles as verifier.Systems" {
    const systems = riscv_system.system_0_systems;
    try std.testing.expect(systems.pcs.max_entries > 0);
    try std.testing.expect(riscv_system.system_0_spec.total_round_coins > 0);
}
