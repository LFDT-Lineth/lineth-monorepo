// Micro-benchmark: measures RISC-V cycle cost of Poseidon2 compression.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start compress,  11 = end compress
//
// The baseline loop matches the measured loop shape with an empty body so the
// runner can subtract loop-counter / branch overhead.

const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const poseidon2 = verifier_ray.crypto.poseidon2;
const field = verifier_ray.field.koalabear;
const profiling = verifier_ray.profiling;

const N: u64 = 10;

// build_common's start.s entry stub calls `main`, so export under that name.
pub export fn main() noreturn {
    // Volatile reads make the input digests opaque to the optimizer, preventing
    // the compression chain from being constant-folded or deleted.
    var seed0: u32 = 0x12345678;
    var seed1: u32 = 0x9ABCDEF0;
    var left: poseidon2.Digest = undefined;
    var right: poseidon2.Digest = undefined;
    inline for (0..poseidon2.block_size) |i| {
        const left_seed = (@as(*volatile u32, &seed0)).*;
        const right_seed = (@as(*volatile u32, &seed1)).*;
        left[i] = .{ .value = @as(u32, @intCast((@as(u64, left_seed) + i + 1) % field.modulus)) };
        right[i] = .{ .value = @as(u32, @intCast((@as(u64, right_seed) + i + poseidon2.block_size + 1) % field.modulus)) };
    }

    var i: u64 = 0;

    profiling.markR5Value(0, 0);
    while (i < N) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    profiling.markR5Value(10, 0);
    i = 0;
    while (i < N) : (i += 1) {
        left = poseidon2.compress(left, right);
    }

    var checksum = left[0];
    inline for (left[1..]) |limb| {
        checksum = checksum.add(limb);
    }
    profiling.markR5Value(11, checksum.value);

    accel.zkvm_exit(0);
}
