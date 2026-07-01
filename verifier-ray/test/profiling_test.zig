const std = @import("std");
const verifier_ray = @import("verifier_ray");

const profiling = verifier_ray.profiling;
const poseidon2 = verifier_ray.crypto.poseidon2;

// Profiling counters are inert unless profiling is enabled at build time, so
// these tests are meaningless without it. `profiling.enabled` is comptime-known,
// so fail the build with an actionable message when the flag is missing.
comptime {
    if (!profiling.enabled) {
        @compileError(
            "test-profiling requires profiling to be enabled at build time; " ++
                "run `zig build test-profiling -Dverifier-profiling`",
        );
    }
}

test "reset zeroes the counters" {
    const zero = poseidon2.zeroDigest();
    _ = poseidon2.compress(zero, zero);

    profiling.reset();
    try std.testing.expectEqual(@as(u64, 0), profiling.snapshot().poseidon2_compress);
}

test "Poseidon2 compress increments the counter once per call" {
    profiling.reset();

    const zero = poseidon2.zeroDigest();
    _ = poseidon2.compress(zero, zero);
    _ = poseidon2.compress(zero, zero);
    _ = poseidon2.compress(zero, zero);

    try std.testing.expectEqual(@as(u64, 3), profiling.snapshot().poseidon2_compress);
}
