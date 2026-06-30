// Micro-benchmark: measures RISC-V cycle cost of Poseidon2 compression.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start compress,  11 = end compress
//
// The baseline loop matches the measured loop shape with an empty body so the
// runner can subtract loop-counter / branch overhead.

const verifier_ray = @import("verifier_ray");
const poseidon2 = verifier_ray.crypto.poseidon2;
const field = verifier_ray.field.koalabear;

const N: u64 = 10;

fn writeR5(bytes: []const u8) void {
    asm volatile (
        \\li a0, 1
        \\mv a1, %[ptr]
        \\mv a2, %[len]
        \\li a7, 64
        \\ecall
        :
        : [ptr] "r" (@intFromPtr(bytes.ptr)),
          [len] "r" (bytes.len),
        : .{ .a0 = true, .a1 = true, .a2 = true, .a7 = true, .memory = true });
}

fn decimalBuf(buf: []u8, value: u64) []u8 {
    if (value == 0) {
        buf[0] = '0';
        return buf[0..1];
    }
    var tmp: [20]u8 = undefined;
    var n = value;
    var i: usize = tmp.len;
    while (n != 0) {
        i -= 1;
        tmp[i] = '0' + @as(u8, @intCast(n % 10));
        n /= 10;
    }
    const digits = tmp[i..];
    @memcpy(buf[0..digits.len], digits);
    return buf[0..digits.len];
}

fn emitMark(phase: u64, checksum: u32) void {
    const prefix = "COMPRESS-MARK\t";
    var buf: [64]u8 = undefined;
    @memcpy(buf[0..prefix.len], prefix);
    var pos: usize = prefix.len;
    pos += decimalBuf(buf[pos..], phase).len;
    buf[pos] = '\t';
    pos += 1;
    pos += decimalBuf(buf[pos..], checksum).len;
    buf[pos] = '\n';
    pos += 1;
    writeR5(buf[0..pos]);
}

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

    emitMark(0, 0);
    while (i < N) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    emitMark(1, 0);

    emitMark(10, 0);
    i = 0;
    while (i < N) : (i += 1) {
        left = poseidon2.compress(left, right);
    }

    var checksum = left[0];
    inline for (left[1..]) |limb| {
        checksum = checksum.add(limb);
    }
    emitMark(11, checksum.value);

    asm volatile (
        \\li a0, 0
        \\li a7, 93
        \\ecall
    );
    unreachable;
}
