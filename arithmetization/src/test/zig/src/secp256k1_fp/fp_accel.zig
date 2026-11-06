//! Checks the secp256k1 field accelerator against a software reference.
//! Run with the opt-in entry point (ZKC_MAIN=.../riscv/main_fieldaccel.zkc).
const lineth_accel = @import("lineth_zkvm_accel");

const Fp = lineth_accel.secp256k1_fp_t;

// p = 2^256 - 2^32 - 977
const P: Fp = .{ .limbs = .{ 0xFFFFFFFEFFFFFC2F, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF } };

const Vec = struct { a: Fp, b: Fp, want: Fp };

fn fp(l0: u64, l1: u64, l2: u64, l3: u64) Fp {
    return .{ .limbs = .{ l0, l1, l2, l3 } };
}

// want = a*b mod p, precomputed with python.
const vectors = [_]Vec{
    .{ .a = fp(2, 0, 0, 0), .b = fp(3, 0, 0, 0), .want = fp(6, 0, 0, 0) },
    // (p-1)^2 = 1
    .{ .a = fp(0xFFFFFFFEFFFFFC2E, ~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0)), .b = fp(0xFFFFFFFEFFFFFC2E, ~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0)), .want = fp(1, 0, 0, 0) },
    // lazily reduced inputs: p * x = 0 and (p+1) * 5 = 5
    .{ .a = P, .b = fp(7, 0, 0, 0), .want = fp(0, 0, 0, 0) },
    .{ .a = fp(0xFFFFFFFEFFFFFC30, ~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0)), .b = fp(5, 0, 0, 0), .want = fp(5, 0, 0, 0) },
    // 2^256 - 1 is p + 0x1000003D0, so its square is 0x1000003D0^2
    .{ .a = fp(~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0)), .b = fp(~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0), ~@as(u64, 0)), .want = fp(0x7a0000e8900, 1, 0, 0) },
};

fn eq(a: Fp, b: Fp) bool {
    return a.limbs[0] == b.limbs[0] and a.limbs[1] == b.limbs[1] and a.limbs[2] == b.limbs[2] and a.limbs[3] == b.limbs[3];
}

export fn main() noreturn {
    for (vectors) |v| {
        var out: Fp = undefined;
        lineth_accel.secp256k1_fp_mul(&out, &v.a, &v.b);
        if (!eq(out, v.want)) lineth_accel.zkvm_exit(1);

        // aliasing: destination = first source
        var al = v.a;
        lineth_accel.secp256k1_fp_mul(&al, &al, &v.b);
        if (!eq(al, v.want)) lineth_accel.zkvm_exit(2);

        if (eq(v.a, v.b)) {
            var sq: Fp = undefined;
            lineth_accel.secp256k1_fp_sq(&sq, &v.a);
            if (!eq(sq, v.want)) lineth_accel.zkvm_exit(3);
            var sa = v.a;
            lineth_accel.secp256k1_fp_sq(&sa, &sa);
            if (!eq(sa, v.want)) lineth_accel.zkvm_exit(4);
        }
    }
    // inv: 2^-1 = (p+1)/2, 0 -> 0, p -> 0 (lazy), a * a^-1 = 1
    const two = fp(2, 0, 0, 0);
    var inv2: Fp = undefined;
    lineth_accel.secp256k1_fp_inv(&inv2, &two);
    if (!eq(inv2, fp(0xffffffff7ffffe18, ~@as(u64, 0), ~@as(u64, 0), 0x7fffffffffffffff))) lineth_accel.zkvm_exit(5);
    var z: Fp = fp(1, 1, 1, 1);
    lineth_accel.secp256k1_fp_inv(&z, &fp(0, 0, 0, 0));
    if (!eq(z, fp(0, 0, 0, 0))) lineth_accel.zkvm_exit(6);
    lineth_accel.secp256k1_fp_inv(&z, &P);
    if (!eq(z, fp(0, 0, 0, 0))) lineth_accel.zkvm_exit(7);
    var ia = vectors[1].a;
    var prod: Fp = undefined;
    lineth_accel.secp256k1_fp_inv(&ia, &ia); // aliased: (p-1)^-1 = p-1
    lineth_accel.secp256k1_fp_mul(&prod, &ia, &vectors[1].a);
    if (!eq(prod, fp(1, 0, 0, 0))) lineth_accel.zkvm_exit(8);

    // sqrt: 4 is a square; 3 and 7 are not (root is then a root of -a); 0 is a square
    var sr: lineth_accel.secp256k1_fp_sqrt_t = undefined;
    lineth_accel.secp256k1_fp_sqrt(&sr, &fp(4, 0, 0, 0));
    if (sr.is_square != 1 or !eq(sr.root, fp(2, 0, 0, 0))) lineth_accel.zkvm_exit(9);
    lineth_accel.secp256k1_fp_sqrt(&sr, &fp(3, 0, 0, 0));
    if (sr.is_square != 0 or !eq(sr.root, fp(0x7d8d27ae1cd5f852, 0xc61f6d15da14ecd4, 0x233770c2a797962c, 0x0a2d2ba93507f1df))) lineth_accel.zkvm_exit(10);
    lineth_accel.secp256k1_fp_sqrt(&sr, &fp(7, 0, 0, 0));
    if (sr.is_square != 0 or !eq(sr.root, fp(0xeecb4b6fba9360dc, 0x38ed44e3932a7179, 0x0727cd69b4a7333d, 0x8f537eefdfc1606a))) lineth_accel.zkvm_exit(11);
    lineth_accel.secp256k1_fp_sqrt(&sr, &fp(0, 0, 0, 0));
    if (sr.is_square != 1 or !eq(sr.root, fp(0, 0, 0, 0))) lineth_accel.zkvm_exit(12);
    lineth_accel.secp256k1_fp_sqrt(&sr, &P); // lazy input p = 0
    if (sr.is_square != 1 or !eq(sr.root, fp(0, 0, 0, 0))) lineth_accel.zkvm_exit(13);

    lineth_accel.zkvm_exit(0);
}
