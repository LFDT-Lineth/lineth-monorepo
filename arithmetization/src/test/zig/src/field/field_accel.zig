// Field accelerator (lib/field/field.zkc) against plain u512/u768 references, for every 256-bit
// and 384-bit field.
//   make exec TEST=field/field_accel.zig
// Exits with a per-check code on the first mismatch.

const lineth_accel = @import("lineth_zkvm_accel");
const field_op_256 = lineth_accel.lineth_zkvm_field_op_256;
const field_op_384 = lineth_accel.lineth_zkvm_field_op_384;

const Elem256 = [4]u64;
const Elem384 = [6]u64;

// LINETH_FIELD_OP_* (include/lineth_accelerators.h)
const op_mul = 0;
const op_div = 1;
const op_sqrt = 2;

// LINETH_FIELD_* 256-bit ids (1..7)
const fields_256 = [_]struct { id: u32, p: u256 }{
    .{ .id = 1, .p = 0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f }, // secp256k1 Fp
    .{ .id = 2, .p = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141 }, // secp256k1 Fr
    .{ .id = 3, .p = 0xffffffff00000001000000000000000000000000ffffffffffffffffffffffff }, // secp256r1 Fp
    .{ .id = 4, .p = 0xffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551 }, // secp256r1 Fr
    .{ .id = 5, .p = 0x30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47 }, // bn254 Fp
    .{ .id = 6, .p = 0x30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001 }, // bn254 Fr
    .{ .id = 7, .p = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001 }, // bls12-381 Fr
};

// LINETH_FIELD_BLS12_381_FP (8)
const bls12_381_fp_id: u32 = 8;
const bls12_381_fp_p: u384 = 0x1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaaab;

const samples_256 = [_]u256{
    0x0123456789abcdeffedcba98765432100f1e2d3c4b5a69788796a5b4c3d2e1f0,
    0xdeadbeefcafebabe0011223344556677deadbeefcafebabe0011223344556677,
    0x5555555555555555555555555555555555555555555555555555555555555555,
    2,
};

const samples_384 = [_]u384{
    0x150123456789abcdeffedcba98765432100f1e2d3c4b5a69788796a5b4c3d2e1f0112233445566778899aabbccddeeff,
    0x0fdeadbeefcafebabe0011223344556677deadbeefcafebabe0011223344556677deadbeefcafebabe00112233445566,
    0x195555555555555555555555555555555555555555555555555555555555555555555555555555555555555555555555,
    3,
};

var code: u32 = 0;

fn check(ok: bool) void {
    code += 1;
    if (!ok) lineth_accel.zkvm_exit(code);
}

fn toElem256(v: u256) Elem256 {
    return @bitCast(v);
}

fn fromElem256(e: Elem256) u256 {
    return @bitCast(e);
}

fn toElem384(v: u384) Elem384 {
    return @bitCast(v);
}

fn fromElem384(e: Elem384) u384 {
    return @bitCast(e);
}

fn mulmod256(a: u256, b: u256, p: u256) u256 {
    return @intCast((@as(u512, a) * b) % p);
}

fn mulmod384(a: u384, b: u384, p: u384) u384 {
    return @intCast((@as(u768, a) * b) % p);
}

fn mul256(f: u32, a: u256, b: u256) u256 {
    var out: Elem256 = undefined;
    const ea = toElem256(a);
    const eb = toElem256(b);
    _ = field_op_256(op_mul, f, &out, &ea, &eb);
    return fromElem256(out);
}

fn div256(f: u32, a: u256, b: u256, status: *u32) u256 {
    var out: Elem256 = undefined;
    const ea = toElem256(a);
    const eb = toElem256(b);
    status.* = field_op_256(op_div, f, &out, &ea, &eb);
    return fromElem256(out);
}

fn sqrt256(f: u32, a: u256, status: *u32) u256 {
    var out: Elem256 = undefined;
    const ea = toElem256(a);
    status.* = field_op_256(op_sqrt, f, &out, &ea, &ea);
    return fromElem256(out);
}

fn mul384(f: u32, a: u384, b: u384) u384 {
    var out: Elem384 = undefined;
    const ea = toElem384(a);
    const eb = toElem384(b);
    _ = field_op_384(op_mul, f, &out, &ea, &eb);
    return fromElem384(out);
}

fn div384(f: u32, a: u384, b: u384, status: *u32) u384 {
    var out: Elem384 = undefined;
    const ea = toElem384(a);
    const eb = toElem384(b);
    status.* = field_op_384(op_div, f, &out, &ea, &eb);
    return fromElem384(out);
}

fn sqrt384(f: u32, a: u384, status: *u32) u384 {
    var out: Elem384 = undefined;
    const ea = toElem384(a);
    status.* = field_op_384(op_sqrt, f, &out, &ea, &ea);
    return fromElem384(out);
}

export fn main() noreturn {
    var status: u32 = undefined;
    for (fields_256) |field| {
        const p = field.p;
        const f = field.id;
        const big = p > (1 << 255);
        for (samples_256) |s| {
            const a = s % p;
            const b = (s *% 0x9e3779b97f4a7c15 +% 12345) % p;

            check(mul256(f, a, b) == mulmod256(a, b, p));
            check(mul256(f, p - 1, b) == mulmod256(p - 1, b, p));
            check(mul256(f, a, 0) == 0);

            const q = div256(f, a, b, &status);
            check(status == 0 and q < p and mulmod256(q, b, p) == a);

            const i = div256(f, 1, a, &status);
            check(status == 0 and i < p and mulmod256(i, a, p) == 1);

            check(div256(f, a, 0, &status) == 0 and status == 1);
            check(div256(f, 0, 0, &status) == 0 and status == 1);
            check(div256(f, 0, b, &status) == 0 and status == 0);
            if (big) {
                if (a <= ~p) {
                    check(mul256(f, a + p, b) == mulmod256(a, b, p));
                    check(mul256(f, b, a + p) == mulmod256(a, b, p));
                }
                check(div256(f, a, p, &status) == 0 and status == 1);
            }
            if (p & 3 == 3) {
                const sq = mulmod256(a, a, p);
                const r = sqrt256(f, sq, &status);
                check(status == 0 and r < p and mulmod256(r, r, p) == sq);
                const r2 = sqrt256(f, p - sq, &status);
                check(status == 1 and r2 < p and mulmod256(r2, r2, p) == sq);
            }
        }
        if (p & 3 == 3) {
            check(sqrt256(f, 0, &status) == 0 and status == 0);
        }
        // Aliasing (out == a) AND 32-byte boundary check (limbs 4..5 untouched, ignored on input)
        var x_guarded: Elem384 = undefined;
        var y_guarded: Elem384 = undefined;
        const x0 = toElem256(samples_256[0] % p);
        const y0 = toElem256(samples_256[1] % p);
        @memcpy(x_guarded[0..4], &x0);
        @memcpy(y_guarded[0..4], &y0);
        x_guarded[4] = 0xdeadbeefcafebabe;
        x_guarded[5] = 0x0123456789abcdef;
        y_guarded[4] = 0xffffffffffffffff;
        y_guarded[5] = 0xffffffffffffffff;
        const want = mulmod256(samples_256[0] % p, samples_256[1] % p, p);
        _ = field_op_256(op_mul, f, x_guarded[0..4], x_guarded[0..4], y_guarded[0..4]);
        check(fromElem256(x_guarded[0..4].*) == want);
        check(x_guarded[4] == 0xdeadbeefcafebabe and x_guarded[5] == 0x0123456789abcdef);
    }

    // 384-bit field: BLS12-381 Fp (id = 8, p = 3 mod 4)
    {
        const p = bls12_381_fp_p;
        const f = bls12_381_fp_id;
        for (samples_384) |s| {
            const a = s % p;
            const b = (s *% 0x9e3779b97f4a7c15 +% 12345) % p;

            check(mul384(f, a, b) == mulmod384(a, b, p));
            check(mul384(f, p - 1, b) == mulmod384(p - 1, b, p));
            check(mul384(f, a, 0) == 0);
            // Unreduced input in [p, 2p)
            check(mul384(f, a + p, b) == mulmod384(a, b, p));
            check(mul384(f, b, a + p) == mulmod384(a, b, p));

            const q = div384(f, a, b, &status);
            check(status == 0 and q < p and mulmod384(q, b, p) == a);

            const i = div384(f, 1, a, &status);
            check(status == 0 and i < p and mulmod384(i, a, p) == 1);

            check(div384(f, a, 0, &status) == 0 and status == 1);
            check(div384(f, a, p, &status) == 0 and status == 1);
            check(div384(f, 0, b, &status) == 0 and status == 0);

            const sq = mulmod384(a, a, p);
            const r = sqrt384(f, sq, &status);
            check(status == 0 and r < p and mulmod384(r, r, p) == sq);
            const r2 = sqrt384(f, p - sq, &status);
            check(status == 1 and r2 < p and mulmod384(r2, r2, p) == sq);
        }
        check(sqrt384(f, 0, &status) == 0 and status == 0);

        // Aliasing on 48-byte buffer
        var x: Elem384 = toElem384(samples_384[0] % p);
        const y: Elem384 = toElem384(samples_384[1] % p);
        const want = mulmod384(samples_384[0] % p, samples_384[1] % p, p);
        _ = field_op_384(op_mul, f, &x, &x, &y);
        check(fromElem384(x) == want);
    }

    lineth_accel.zkvm_exit(0);
}
