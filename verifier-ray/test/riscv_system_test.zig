const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

// Smoke test that the codegen output type-checks as a real verifier.Systems
// value — the first point at which the generate-riscv-system
// pipeline (main.zkc plus the honest, instruction-coverage guest ELF
// AllInOneGuestELF) proves itself correct, independent of decoding a real
// proof or running through R5.
test "riscv_system compiles as verifier.Systems" {
    const systems = riscv_system.system_0_systems;
    try std.testing.expect(systems.pcs.max_entries > 0);
    try std.testing.expect(riscv_system.system_0_spec.total_round_coins > 0);
}

test "riscv_system compact binary decodes to the generated system" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const bundle = try verifier_ray.system_runtime.decodeBundle(&riscv_system.system_0_encoded, arena.allocator());

    try std.testing.expectEqual(riscv_system.system_0_spec.total_round_coins, bundle.spec.total_round_coins);
    try std.testing.expectEqual(riscv_system.system_0_systems.vanishing.modules.len, bundle.systems.vanishing.modules.len);
    try std.testing.expectEqual(riscv_system.system_0_systems.pcs.columns.len, bundle.systems.pcs.columns.len);
    try std.testing.expectEqual(riscv_system.system_0_systems.pcs.witness_map.len, bundle.systems.pcs.witness_map.len);
}

test "riscv_system decoder rejects an unknown schema" {
    try std.testing.expectError(
        error.InvalidMagic,
        verifier_ray.system_runtime.decodeBundle("BAD!", std.testing.allocator),
    );
}
