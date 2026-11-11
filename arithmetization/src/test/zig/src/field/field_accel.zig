// Field accelerator (lib/field/field.zkc) against a plain u512 reference, for every field.
//   make exec TEST=field/field_accel.zig
// Exits with a per-check code on the first mismatch.

const lineth_accel = @import("lineth_zkvm_accel");
const field_op = lineth_accel.lineth_zkvm_field_op;

const Element = [4]u64;

// LINETH_FIELD_OP_* (include/lineth_accelerators.h)
const op_mul = 0;
const op_div = 1;
const op_sqrt = 2;

// LINETH_FIELD_* ids
const fields = [_]struct { id: u32, p: u256 }{
    .{ .id = 1, .p = 0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f }, // secp256k1 Fp
    .{ .id = 2, .p = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141 }, // secp256k1 Fr
    .{ .id = 3, .p = 0xffffffff00000001000000000000000000000000ffffffffffffffffffffffff }, // secp256r1 Fp
    .{ .id = 4, .p = 0xffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551 }, // secp256r1 Fr
    .{ .id = 5, .p = 0x30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47 }, // bn254 Fp
    .{ .id = 6, .p = 0x30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001 }, // bn254 Fr
    .{ .id = 7, .p = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001 }, // bls12-381 Fr
};

const samples = [_]u256{
    0x0123456789abcdeffedcba98765432100f1e2d3c4b5a69788796a5b4c3d2e1f0,
    0xdeadbeefcafebabe0011223344556677deadbeefcafebabe0011223344556677,
    0x5555555555555555555555555555555555555555555555555555555555555555,
    2,
};

var code: u32 = 0;

fn check(ok: bool) void {
    code += 1;
    if (!ok) lineth_accel.zkvm_exit(code);
}

fn toElem(v: u256) Element {
    return @bitCast(v);
}

fn fromElem(e: Element) u256 {
    return @bitCast(e);
}

fn mulmod(a: u256, b: u256, p: u256) u256 {
    return @intCast((@as(u512, a) * b) % p);
}

fn mul(f: u32, a: u256, b: u256) u256 {
    var out: Element = undefined;
    const ea = toElem(a);
    const eb = toElem(b);
    _ = field_op(op_mul, f, &out, &ea, &eb);
    return fromElem(out);
}

fn div(f: u32, a: u256, b: u256, status: *u32) u256 {
    var out: Element = undefined;
    const ea = toElem(a);
    const eb = toElem(b);
    status.* = field_op(op_div, f, &out, &ea, &eb);
    return fromElem(out);
}

fn sqrt(f: u32, a: u256, status: *u32) u256 {
    var out: Element = undefined;
    const ea = toElem(a);
    status.* = field_op(op_sqrt, f, &out, &ea, &ea);
    return fromElem(out);
}

export fn main() noreturn {
    var status: u32 = undefined;
    for (fields) |field| {
        const p = field.p;
        const f = field.id;
        const big = p > (1 << 255);
        for (samples) |s| {
            const a = s % p;
            const b = (s *% 0x9e3779b97f4a7c15 +% 12345) % p;

            // MUL, reduced output
            check(mul(f, a, b) == mulmod(a, b, p));
            check(mul(f, p - 1, b) == mulmod(p - 1, b, p));
            check(mul(f, a, 0) == 0);
            // DIV: out * b = a
            const q = div(f, a, b, &status);
            check(status == 0 and q < p and mulmod(q, b, p) == a);
            // INV = DIV(1, x)
            const i = div(f, 1, a, &status);
            check(status == 0 and i < p and mulmod(i, a, p) == 1);
            // x/0 = 0, status 1
            check(div(f, a, 0, &status) == 0 and status == 1);
            check(div(f, 0, 0, &status) == 0 and status == 1);
            check(div(f, 0, b, &status) == 0 and status == 0);
            if (big) {
                // inputs in [p, 2^256) are reduced once
                if (a <= ~p) {
                    check(mul(f, a + p, b) == mulmod(a, b, p));
                    check(mul(f, b, a + p) == mulmod(a, b, p));
                }
                check(div(f, a, p, &status) == 0 and status == 1);
            }
            // SQRT where p = 3 mod 4
            if (p & 3 == 3) {
                const sq = mulmod(a, a, p);
                const r = sqrt(f, sq, &status);
                check(status == 0 and r < p and mulmod(r, r, p) == sq);
                // -sq is a non-square: out^2 = sq, status 1
                const r2 = sqrt(f, p - sq, &status);
                check(status == 1 and r2 < p and mulmod(r2, r2, p) == sq);
            }
        }
        if (p & 3 == 3) {
            check(sqrt(f, 0, &status) == 0 and status == 0);
        }
        // aliasing: out = a
        var x: Element = toElem(samples[0] % p);
        const y: Element = toElem(samples[1] % p);
        const want = mulmod(samples[0] % p, samples[1] % p, p);
        _ = field_op(op_mul, f, &x, &x, &y);
        check(fromElem(x) == want);
    }
    lineth_accel.zkvm_exit(0);
}
