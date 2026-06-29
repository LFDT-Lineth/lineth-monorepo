// Micro-benchmark: measures RISC-V cycle cost of one Poseidon2 permutation
// (called via compress, which does exactly one permutation(16, &state) call).
//
// Emits a single BENCH-MARK marker after N compress calls so the runner can
// divide the total cycle delta by N to get cycles-per-permutation.

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
    // Use a non-trivial initial state so the compiler cannot fold the calls.
    var left: poseidon2.Digest = undefined;
    var right: poseidon2.Digest = undefined;
    inline for (0..poseidon2.block_size) |i| {
        left[i] = .{ .value = @as(u32, @intCast(i + 1)) };
        right[i] = .{ .value = @as(u32, @intCast(i + poseidon2.block_size + 1)) };
    }

    var i: u64 = 0;
    while (i < N) : (i += 1) {
        // Feed the output back as left input so each call depends on the previous.
        left = poseidon2.compress(left, right);
    }

    // Emit a checksum to prevent dead-code elimination.
    emitMark(1, left[0].value);

    asm volatile (
        \\li a0, 0
        \\li a7, 93
        \\ecall
    );
    unreachable;
}
