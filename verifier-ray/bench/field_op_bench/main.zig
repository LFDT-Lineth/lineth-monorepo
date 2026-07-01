// Micro-benchmark: measures RISC-V cycle cost of each KoalaBear field operation.
//
// Each phase chains N operations with the output feeding back as input so the
// compiler cannot constant-fold the loop. Start/end markers bracket only the
// loop body; markR5Value overhead is excluded via cycleDelta(start, end).
//
// A baseline phase runs the same loop with an empty (but non-elided) body, so
// the runner can subtract the loop-counter / branch overhead and report the
// pure per-operation cost (delta_op - delta_baseline) / N.
//
// Marker IDs (base field):
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
//
// Marker IDs (extension field Ext = F_{p^6}):
//  100 = start ext/add,      101 = end ext/add
//  110 = start ext/sub,      111 = end ext/sub
//  120 = start ext/mul,      121 = end ext/mul
//  130 = start ext/square,   131 = end ext/square
//  140 = start ext/inverse,  141 = end ext/inverse
//  150 = start ext/div,      151 = end ext/div
//  160 = start ext/mulByBase,161 = end ext/mulByBase

const vr = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const kb = vr.field.koalabear;
const kbext = vr.field.koalabear_ext;
const Element = kb.Element;
const Ext = kbext.Ext;
const profiling = vr.profiling;

const N: u64 = 1000;

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
    profiling.markR5Value(0, 0);
    i = 0;
    while (i < N) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, acc.value);

    // add
    acc = a;
    profiling.markR5Value(10, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.add(b);
    }
    profiling.markR5Value(11, acc.value);

    // sub
    acc = a;
    profiling.markR5Value(20, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.sub(b);
    }
    profiling.markR5Value(21, acc.value);

    // neg
    acc = a;
    profiling.markR5Value(30, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.neg();
    }
    profiling.markR5Value(31, acc.value);

    // double
    acc = a;
    profiling.markR5Value(40, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.double();
    }
    profiling.markR5Value(41, acc.value);

    // mul
    acc = a;
    profiling.markR5Value(50, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.mul(b);
    }
    profiling.markR5Value(51, acc.value);

    // square
    acc = a;
    profiling.markR5Value(60, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.square();
    }
    profiling.markR5Value(61, acc.value);

    // pow — exponent is the Lagrange vanishing-polynomial domain size x^n
    // (see polynomial/lagrange.zig). Domains are powers of two up to 2^24
    // (koalabear.max_order_root); we bench a representative n = 2^20. The
    // exponent is read through volatile so it stays a runtime square-and-multiply
    // (~bitlen squarings + popcount muls) rather than being folded/specialized.
    var exp_v: u64 = 1 << 20;
    const exp_n: u64 = (@as(*volatile u64, &exp_v)).*;
    acc = a;
    profiling.markR5Value(70, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.pow(exp_n);
    }
    profiling.markR5Value(71, acc.value);

    // powComptime — same exponent n = 2^20, but comptime-unrolled (inline while).
    // This is the path static-domain call sites can use (cf. vanishing.zig
    // powModuleSize). For n = 2^k the unrolled chain is k squarings with no
    // runtime loop/branch/shift, so it should be markedly cheaper than pow above.
    acc = a;
    profiling.markR5Value(72, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.powComptime(1 << 20);
    }
    profiling.markR5Value(73, acc.value);

    // inverse — a is nonzero, and inv(inv(x)) == x, so acc alternates between
    // two nonzero values and never hits inverse's zero `unreachable`.
    acc = a;
    profiling.markR5Value(80, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.inverse();
    }
    profiling.markR5Value(81, acc.value);

    // div — b is nonzero (so div's internal inverse is well defined) and acc
    // stays nonzero, so repeated division never produces or divides by zero.
    acc = a;
    profiling.markR5Value(90, 0);
    i = 0;
    while (i < N) : (i += 1) {
        acc = acc.div(b);
    }
    profiling.markR5Value(91, acc.value);

    // ── Ext (F_{p^6}) benchmarks ─────────────────────────────────────────────

    // Build two nonzero extension elements from volatile seeds so the optimizer
    // cannot constant-fold any of the Ext operations below.
    var ve0: u32 = 0x11111111;
    var ve1: u32 = 0x22222222;
    var ve2: u32 = 0x33333333;
    var ve3: u32 = 0x44444444;
    var ve4: u32 = 0x55555555;
    var ve5: u32 = 0x66666666;
    const ea = Ext.fromUints(.{
        (@as(*volatile u32, &ve0)).* % kb.modulus,
        (@as(*volatile u32, &ve1)).* % kb.modulus,
        (@as(*volatile u32, &ve2)).* % kb.modulus,
        (@as(*volatile u32, &ve3)).* % kb.modulus,
        (@as(*volatile u32, &ve4)).* % kb.modulus,
        (@as(*volatile u32, &ve5)).* % kb.modulus,
    });
    var ve6: u32 = 0x77777777;
    var ve7: u32 = 0x88888888;
    var ve8: u32 = 0x12121212;
    var ve9: u32 = 0x23232323;
    var veA: u32 = 0x34343434;
    var veB: u32 = 0x45454545;
    const eb = Ext.fromUints(.{
        (@as(*volatile u32, &ve6)).* % kb.modulus,
        (@as(*volatile u32, &ve7)).* % kb.modulus,
        (@as(*volatile u32, &ve8)).* % kb.modulus,
        (@as(*volatile u32, &ve9)).* % kb.modulus,
        (@as(*volatile u32, &veA)).* % kb.modulus,
        (@as(*volatile u32, &veB)).* % kb.modulus,
    });

    var eacc: Ext = undefined;

    // ext/add
    eacc = ea;
    profiling.markR5Value(100, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.add(eb);
    }
    profiling.markR5Value(101, eacc.B0.a0.value);

    // ext/sub
    eacc = ea;
    profiling.markR5Value(110, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.sub(eb);
    }
    profiling.markR5Value(111, eacc.B0.a0.value);

    // ext/mul
    eacc = ea;
    profiling.markR5Value(120, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.mul(eb);
    }
    profiling.markR5Value(121, eacc.B0.a0.value);

    // ext/square
    eacc = ea;
    profiling.markR5Value(130, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.square();
    }
    profiling.markR5Value(131, eacc.B0.a0.value);

    // ext/inverse — ea is nonzero, inv(inv(x))==x, so eacc stays nonzero.
    eacc = ea;
    profiling.markR5Value(140, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.inverse();
    }
    profiling.markR5Value(141, eacc.B0.a0.value);

    // ext/div — eb is nonzero.
    eacc = ea;
    profiling.markR5Value(150, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.div(eb);
    }
    profiling.markR5Value(151, eacc.B0.a0.value);

    // ext/mulByBase — scale by base element b.
    eacc = ea;
    profiling.markR5Value(160, 0);
    i = 0;
    while (i < N) : (i += 1) {
        eacc = eacc.mulByBase(b);
    }
    profiling.markR5Value(161, eacc.B0.a0.value);

    accel.zkvm_exit(0);
}
