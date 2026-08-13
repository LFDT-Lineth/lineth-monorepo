const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const proof_codec = verifier_ray.proof_codec;
const verifier = verifier_ray.verifier;

// The real proof.bin fixture generate-riscv-system produced by proving an
// honest zkc_02.zkc witness and encoding it with codegen.EncodeProof —
// the strongest test the wire format actually round-trips a value the real
// verifier logic accepts: decode it, then run it through verifier.verify.
// Embedded from within riscv_system.zig itself (see that generator) since
// @embedFile cannot reach outside test/'s own package path.
const proof_bytes = riscv_system.proof_bytes;

test "decodeVerifyInput decodes the real riscv_system proof fixture" {
    const systems = riscv_system.system_0_systems;
    // DecodeScratch(systems) is tens of MB for a real system — far larger
    // than the default thread stack, so it must be heap-allocated rather
    // than declared as a local `var` (which segfaults on stack overflow).
    const scratch = try std.testing.allocator.create(proof_codec.DecodeScratch(systems));
    defer std.testing.allocator.destroy(scratch);
    const input = try proof_codec.decodeVerifyInput(systems, proof_bytes, scratch);

    try std.testing.expect(input.proof.rounds.len > 0);
}

test "decoded riscv_system proof verifies end-to-end" {
    const systems = riscv_system.system_0_systems;
    const scratch = try std.testing.allocator.create(proof_codec.DecodeScratch(systems));
    defer std.testing.allocator.destroy(scratch);
    const input = try proof_codec.decodeVerifyInput(systems, proof_bytes, scratch);

    verifier.verify(riscv_system.system_0_spec, systems, input.proof, input.public_inputs) catch |err| {
        std.debug.print("decoded riscv_system proof failed to verify: {s}\n", .{@errorName(err)});
        return err;
    };
}

test "decodeVerifyInput rejects truncated bytes" {
    // Truncating mid-stream can surface as either an outright UnexpectedEof
    // (the cut lands inside a fixed-size read) or a count mismatch against
    // systems' comptime shape (the cut lands between two length-prefixed
    // sections, so the next section's count is read from garbage/zeroed
    // bytes) — either is an acceptable rejection; what matters is that
    // truncated input is never silently accepted.
    const systems = riscv_system.system_0_systems;
    const scratch = try std.testing.allocator.create(proof_codec.DecodeScratch(systems));
    defer std.testing.allocator.destroy(scratch);
    const truncated = proof_bytes[0 .. proof_bytes.len / 2];
    try std.testing.expect(std.meta.isError(proof_codec.decodeVerifyInput(systems, truncated, scratch)));

    const scratch2 = try std.testing.allocator.create(proof_codec.DecodeScratch(systems));
    defer std.testing.allocator.destroy(scratch2);
    const barely_anything = proof_bytes[0..2];
    try std.testing.expectError(
        error.UnexpectedEof,
        proof_codec.decodeVerifyInput(systems, barely_anything, scratch2),
    );
}
