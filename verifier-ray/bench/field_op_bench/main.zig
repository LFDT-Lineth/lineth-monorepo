// Micro-benchmark: measures RISC-V cycle cost of each KoalaBear field operation.
//
// Each phase chains N operations with the output feeding back as input so the
// compiler cannot constant-fold the loop. Start/end markers bracket only the
// loop body; emitMark overhead is excluded via cycleDelta(start, end).
//
// A baseline phase runs the same loop with an empty (but non-elided) body, so
// the runner can subtract the loop-counter / branch overhead and report the
// pure per-operation cost (delta_op - delta_baseline) / N.
//
// Marker IDs:
//    0 = start baseline,  1 = end baseline
//   10 = start add,      11 = end add
//   20 = start sub,      21 = end sub
//   30 = start neg,      31 = end neg
//   40 = start double,   41 = end double
//   50 = start mul,      51 = end mul
//   60 = start square,   61 = end square
//   70 = start pow,        71 = end pow          (runtime x^n, n = 2^20 domain)
//   72 = start powComptime, 73 = end powComptime  (comptime-unrolled x^n, n = 2^20)
//   80 = start inverse,  81 = end inverse
//   90 = start div,      91 = end div

const kb = @import("koalabear");
const Element = kb.Element;

const N: u64 = 1000;

// ── write syscall ─────────────────────────────────────────────────────────────

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

fn emitMark(phase: u64, acc: u32) void {
    const prefix = "FIELD-MARK\t";
    var buf: [64]u8 = undefined;
    @memcpy(buf[0..prefix.len], prefix);
    var pos: usize = prefix.len;
    pos += decimalBuf(buf[pos..], phase).len;
    buf[pos] = '\t';
    pos += 1;
    pos += decimalBuf(buf[pos..], acc).len;
    buf[pos] = '\n';
    pos += 1;
    writeR5(buf[0..pos]);
}

// ── benchmark loops ───────────────────────────────────────────────────────────

// build_common's start.s entry stub calls `main`, so export under that name.
pub export fn main() noreturn {
    // Volatile reads make initial values opaque to the optimizer, preventing
    // constant folding without polluting the loop body with load/store.
    var v0: u32 = 0x12345678;
    var v1: u32 = 0x9ABCDEF0;
    const a: Element = .{ .value = (@as(*volatile u32, &v0)).* % kb.modulus };
    const b: Element = .{ .value = (@as(*volatile u32, &v1)).* % kb.modulus };

    var acc: Element = undefined;
    var i: u64 = 0;

    // baseline: same loop shape, empty body. The volatile asm barrier keeps the
    // loop from being elided while adding no arithmetic, so its delta captures
    // exactly the loop-counter / branch overhead common to every phase below.
    acc = a;
    emitMark(0, 0);
    i = 0;
    while (i < N) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    emitMark(1, acc.value);

    // add
    acc = a;
    emitMark(10, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.add(b);
    }
    emitMark(11, acc.value);

    // sub
    acc = a;
    emitMark(20, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.sub(b);
    }
    emitMark(21, acc.value);

    // neg
    acc = a;
    emitMark(30, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.neg();
    }
    emitMark(31, acc.value);

    // double
    acc = a;
    emitMark(40, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.double();
    }
    emitMark(41, acc.value);

    // mul
    acc = a;
    emitMark(50, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.mul(b);
    }
    emitMark(51, acc.value);

    // square
    acc = a;
    emitMark(60, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.square();
    }
    emitMark(61, acc.value);

    // pow — exponent is the Lagrange vanishing-polynomial domain size x^n
    // (see polynomial/lagrange.zig). Domains are powers of two up to 2^24
    // (koalabear.max_order_root); we bench a representative n = 2^20. The
    // exponent is read through volatile so it stays a runtime square-and-multiply
    // (~bitlen squarings + popcount muls) rather than being folded/specialized.
    var exp_v: u64 = 1 << 20;
    const exp_n: u64 = (@as(*volatile u64, &exp_v)).*;
    acc = a;
    emitMark(70, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.pow(exp_n);
    }
    emitMark(71, acc.value);

    // powComptime — same exponent n = 2^20, but comptime-unrolled (inline while).
    // This is the path static-domain call sites can use (cf. vanishing.zig
    // powModuleSize). For n = 2^k the unrolled chain is k squarings with no
    // runtime loop/branch/shift, so it should be markedly cheaper than pow above.
    acc = a;
    emitMark(72, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.powComptime(1 << 20);
    }
    emitMark(73, acc.value);

    // inverse — a is nonzero, and inv(inv(x)) == x, so acc alternates between
    // two nonzero values and never hits inverse's zero `unreachable`.
    acc = a;
    emitMark(80, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.inverse();
    }
    emitMark(81, acc.value);

    // div — b is nonzero (so div's internal inverse is well defined) and acc
    // stays nonzero, so repeated division never produces or divides by zero.
    acc = a;
    emitMark(90, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.div(b);
    }
    emitMark(91, acc.value);

    asm volatile (
        \\li a0, 0
        \\li a7, 93
        \\ecall
    );
    unreachable;
}
