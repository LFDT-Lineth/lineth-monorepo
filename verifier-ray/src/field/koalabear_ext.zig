const base = @import("koalabear.zig");

pub const degree = 6;
pub const bytes = degree * base.bytes;

pub const E2 = @import("koalabear_e2.zig").E2;

/// Cubic extension F_{p^6} = F_{p^2}[v]/(v^3 - (u+1)).
///
/// This is an `extern struct` so Fiat-Shamir challenges and other extension
/// values have stable byte-cast layout across native and R5 builds.
pub const Ext = extern struct {
    B0: E2,
    B1: E2,
    B2: E2,

    pub fn zero() Ext {
        return .{ .B0 = E2.zero(), .B1 = E2.zero(), .B2 = E2.zero() };
    }

    pub fn one() Ext {
        return lift(base.Element.one());
    }

    pub fn lift(value: base.Element) Ext {
        return .{
            .B0 = .{ .a0 = value, .a1 = base.Element.zero() },
            .B1 = E2.zero(),
            .B2 = E2.zero(),
        };
    }

    pub fn isZero(self: Ext) bool {
        return self.B0.isZero() and self.B1.isZero() and self.B2.isZero();
    }

    pub fn isBase(self: Ext) bool {
        return self.B0.a1.isZero() and self.B1.isZero() and self.B2.isZero();
    }

    pub fn eql(self: Ext, rhs: Ext) bool {
        return self.B0.eql(rhs.B0) and self.B1.eql(rhs.B1) and self.B2.eql(rhs.B2);
    }

    pub fn add(self: Ext, rhs: Ext) Ext {
        return .{ .B0 = self.B0.add(rhs.B0), .B1 = self.B1.add(rhs.B1), .B2 = self.B2.add(rhs.B2) };
    }

    pub fn sub(self: Ext, rhs: Ext) Ext {
        return .{ .B0 = self.B0.sub(rhs.B0), .B1 = self.B1.sub(rhs.B1), .B2 = self.B2.sub(rhs.B2) };
    }

    pub fn neg(self: Ext) Ext {
        return .{ .B0 = self.B0.neg(), .B1 = self.B1.neg(), .B2 = self.B2.neg() };
    }

    pub fn mulByBase(self: Ext, rhs: base.Element) Ext {
        // 6 independent base multiplies by the same scalar; no combine/reduce-
        // twice pattern exists here (unlike mul/square/inverse), so this is a
        // straight expansion rather than an operation-count reduction.
        const r = @as(u64, rhs.value);
        return .{
            .B0 = .{
                .a0 = base.Element.init(self.B0.a0.value * r),
                .a1 = base.Element.init(self.B0.a1.value * r),
            },
            .B1 = .{
                .a0 = base.Element.init(self.B1.a0.value * r),
                .a1 = base.Element.init(self.B1.a1.value * r),
            },
            .B2 = .{
                .a0 = base.Element.init(self.B2.a0.value * r),
                .a1 = base.Element.init(self.B2.a1.value * r),
            },
        };
    }

    pub fn divByBase(self: Ext, rhs: base.Element) Ext {
        return self.mulByBase(rhs.inverse());
    }

    /// inv2 = (p+1)/2 = 2^{-1} mod p for KoalaBear (p = 2_130_706_433).
    /// Halving is a scalar multiply by this base-field constant.
    pub const inv2: base.Element = .{ .value = (base.modulus + 1) / 2 };

    /// Divide by two. Mirrors prover-ray's field.Ext.Halve: a scalar multiply
    /// by 2^{-1}, used by the FRI fold recurrence.
    pub fn halve(self: Ext) Ext {
        return self.mulByBase(inv2);
    }

    pub fn mul(self: Ext, rhs: Ext) Ext {
        // Karatsuba for cubic extension: 6 E2 muls instead of 9 (schoolbook).
        // F_{p^6} = F_{p^2}[v]/(v^3 - nr), nr = u+1 in F_{p^2}.
        //
        // Fully expanded to raw base-field (u64) scalars: no E2/base.Element
        // helper calls, no intermediate canonical reduction. Every add/sub is
        // a free u64 op; the only `% p` folds are the ones strictly required
        // to keep multiply operands and inputs bounded, plus the 6 final
        // outputs. This removes the E2-call boundary that used to force
        // canonical results (and a remu) after every cross-term E2 mul.
        //
        // Bounds: canonical limbs are < p < 2^31. A raw sum/diff of a small
        // fixed number of such limbs (offset by +2p to avoid unsigned
        // underflow on subtraction) stays far below 2^63, and every value fed
        // into a multiply is folded mod p first, so products are bounded by
        // 3*(p-1)^2 < 2^64.
        const p = @as(u64, base.modulus);

        const x0 = @as(u64, self.B0.a0.value);
        const x1 = @as(u64, self.B0.a1.value);
        const x2 = @as(u64, self.B1.a0.value);
        const x3 = @as(u64, self.B1.a1.value);
        const x4 = @as(u64, self.B2.a0.value);
        const x5 = @as(u64, self.B2.a1.value);
        const y0 = @as(u64, rhs.B0.a0.value);
        const y1 = @as(u64, rhs.B0.a1.value);
        const y2 = @as(u64, rhs.B1.a0.value);
        const y3 = @as(u64, rhs.B1.a1.value);
        const y4 = @as(u64, rhs.B2.a0.value);
        const y5 = @as(u64, rhs.B2.a1.value);

        // t0 = B0*B0, t1 = B1*B1, t2 = B2*B2 — reduced immediately since each
        // is reused in both the cross-term subtractions and the final assembly.
        const rt0_0 = (x0 * y0 + 3 * x1 * y1) % p;
        const rt0_1 = (x0 * y1 + x1 * y0) % p;
        const rt1_0 = (x2 * y2 + 3 * x3 * y3) % p;
        const rt1_1 = (x2 * y3 + x3 * y2) % p;
        const rt2_0 = (x4 * y4 + 3 * x5 * y5) % p;
        const rt2_1 = (x4 * y5 + x5 * y4) % p;

        // Cross terms: (Ai+Aj)*(Bi+Bj), operands folded mod p before multiplying.
        const s12_0 = (x2 + x4) % p;
        const s12_1 = (x3 + x5) % p;
        const r12_0 = (y2 + y4) % p;
        const r12_1 = (y3 + y5) % p;
        const w12_0 = s12_0 * r12_0 + 3 * s12_1 * r12_1;
        const w12_1 = s12_0 * r12_1 + s12_1 * r12_0;

        const s01_0 = (x0 + x2) % p;
        const s01_1 = (x1 + x3) % p;
        const r01_0 = (y0 + y2) % p;
        const r01_1 = (y1 + y3) % p;
        const w01_0 = s01_0 * r01_0 + 3 * s01_1 * r01_1;
        const w01_1 = s01_0 * r01_1 + s01_1 * r01_0;

        const s02_0 = (x0 + x4) % p;
        const s02_1 = (x1 + x5) % p;
        const r02_0 = (y0 + y4) % p;
        const r02_1 = (y1 + y5) % p;
        const w02_0 = s02_0 * r02_0 + 3 * s02_1 * r02_1;
        const w02_1 = s02_0 * r02_1 + s02_1 * r02_0;

        // t12 = w12 - t1 - t2 (offset by +4p so intermediate stays non-negative)
        const t12_0 = (w12_0 + 4 * p - rt1_0 - rt2_0) % p;
        const t12_1 = (w12_1 + 4 * p - rt1_1 - rt2_1) % p;
        const t01_0 = (w01_0 + 4 * p - rt0_0 - rt1_0) % p;
        const t01_1 = (w01_1 + 4 * p - rt0_1 - rt1_1) % p;
        const t02_0 = (w02_0 + 4 * p - rt0_0 - rt2_0) % p;
        const t02_1 = (w02_1 + 4 * p - rt0_1 - rt2_1) % p;

        // D0 = t0 + nr(t12),  D1 = t01 + nr(t2),  D2 = t02 + t1
        // nr(x) = (x0 + 3*x1, x0 + x1)
        const d0_0 = rt0_0 + t12_0 + 3 * t12_1;
        const d0_1 = rt0_1 + t12_0 + t12_1;
        const d1_0 = t01_0 + rt2_0 + 3 * rt2_1;
        const d1_1 = t01_1 + rt2_0 + rt2_1;
        const d2_0 = t02_0 + rt1_0;
        const d2_1 = t02_1 + rt1_1;

        return .{
            .B0 = .{ .a0 = base.Element.init(d0_0), .a1 = base.Element.init(d0_1) },
            .B1 = .{ .a0 = base.Element.init(d1_0), .a1 = base.Element.init(d1_1) },
            .B2 = .{ .a0 = base.Element.init(d2_0), .a1 = base.Element.init(d2_1) },
        };
    }

    pub fn square(self: Ext) Ext {
        // Karatsuba squaring: 6 E2 muls instead of 9 from mul(self).
        // s0 = B0^2, s1 = B1^2, s2 = B2^2
        // c01 = (B0+B1)^2 - s0 - s1 = 2*B0*B1
        // c12 = (B1+B2)^2 - s1 - s2 = 2*B1*B2
        // c02 = (B0+B2)^2 - s0 - s2 = 2*B0*B2
        // D0 = s0 + c12*(u+1), D1 = c01 + s2*(u+1), D2 = c02 + s1
        //
        // Fully expanded to raw base-field (u64) scalars, same technique as
        // mul(): no E2/base.Element helper calls, reduction deferred to the
        // multiply boundaries and the 6 final outputs. See mul()'s comment
        // for the bound analysis; identical here since the operand shapes
        // (canonical limbs, sums of two limbs, Karatsuba-style combines) match.
        const p = @as(u64, base.modulus);

        const x0 = @as(u64, self.B0.a0.value);
        const x1 = @as(u64, self.B0.a1.value);
        const x2 = @as(u64, self.B1.a0.value);
        const x3 = @as(u64, self.B1.a1.value);
        const x4 = @as(u64, self.B2.a0.value);
        const x5 = @as(u64, self.B2.a1.value);

        // s0 = B0^2, s1 = B1^2, s2 = B2^2 — reduced immediately, reused below.
        const rs0_0 = (x0 * x0 + 3 * x1 * x1) % p;
        const rs0_1 = (2 * x0 * x1) % p;
        const rs1_0 = (x2 * x2 + 3 * x3 * x3) % p;
        const rs1_1 = (2 * x2 * x3) % p;
        const rs2_0 = (x4 * x4 + 3 * x5 * x5) % p;
        const rs2_1 = (2 * x4 * x5) % p;

        // c{01,12,02} = (Bi+Bj)^2, operands folded mod p before squaring.
        const a01_0 = (x0 + x2) % p;
        const a01_1 = (x1 + x3) % p;
        const q01_0 = a01_0 * a01_0 + 3 * a01_1 * a01_1;
        const q01_1 = 2 * a01_0 * a01_1;

        const a12_0 = (x2 + x4) % p;
        const a12_1 = (x3 + x5) % p;
        const q12_0 = a12_0 * a12_0 + 3 * a12_1 * a12_1;
        const q12_1 = 2 * a12_0 * a12_1;

        const a02_0 = (x0 + x4) % p;
        const a02_1 = (x1 + x5) % p;
        const q02_0 = a02_0 * a02_0 + 3 * a02_1 * a02_1;
        const q02_1 = 2 * a02_0 * a02_1;

        const c01_0 = (q01_0 + 4 * p - rs0_0 - rs1_0) % p;
        const c01_1 = (q01_1 + 4 * p - rs0_1 - rs1_1) % p;
        const c12_0 = (q12_0 + 4 * p - rs1_0 - rs2_0) % p;
        const c12_1 = (q12_1 + 4 * p - rs1_1 - rs2_1) % p;
        const c02_0 = (q02_0 + 4 * p - rs0_0 - rs2_0) % p;
        const c02_1 = (q02_1 + 4 * p - rs0_1 - rs2_1) % p;

        // D0 = s0 + nr(c12),  D1 = c01 + nr(s2),  D2 = c02 + s1
        // nr(x) = (x0 + 3*x1, x0 + x1)
        const d0_0 = rs0_0 + c12_0 + 3 * c12_1;
        const d0_1 = rs0_1 + c12_0 + c12_1;
        const d1_0 = c01_0 + rs2_0 + 3 * rs2_1;
        const d1_1 = c01_1 + rs2_0 + rs2_1;
        const d2_0 = c02_0 + rs1_0;
        const d2_1 = c02_1 + rs1_1;

        return .{
            .B0 = .{ .a0 = base.Element.init(d0_0), .a1 = base.Element.init(d0_1) },
            .B1 = .{ .a0 = base.Element.init(d1_0), .a1 = base.Element.init(d1_1) },
            .B2 = .{ .a0 = base.Element.init(d2_0), .a1 = base.Element.init(d2_1) },
        };
    }

    pub fn inverse(self: Ext) Ext {
        if (self.isZero()) unreachable;

        // Adjugate elements for the cubic extension inverse:
        //   A = b0^2 - (u+1)*b1*b2
        //   B = (u+1)*b2^2 - b0*b1
        //   C = b1^2 - b0*b2
        // Norm: d = b0*A + (u+1)*(b2*B + b1*C)
        //
        // Fully expanded to raw base-field (u64) scalars, same technique as
        // mul()/square(). Unlike those, cap_a/cap_b/cap_c are each consumed
        // twice (once to build d, once in the final .mul(d_inv)), so they
        // are materialized as canonical base.Element pairs rather than kept
        // as deferred lazy sums — there is no benefit to deferring reduction
        // across a value that gets reduced anyway before its second use.
        const p = @as(u64, base.modulus);

        const x0 = @as(u64, self.B0.a0.value);
        const x1 = @as(u64, self.B0.a1.value);
        const x2 = @as(u64, self.B1.a0.value);
        const x3 = @as(u64, self.B1.a1.value);
        const x4 = @as(u64, self.B2.a0.value);
        const x5 = @as(u64, self.B2.a1.value);

        // E2 products reduced immediately — each used exactly once below.
        const r_b0sq_0 = (x0 * x0 + 3 * x1 * x1) % p;
        const r_b0sq_1 = (2 * x0 * x1) % p;
        const r_b1sq_0 = (x2 * x2 + 3 * x3 * x3) % p;
        const r_b1sq_1 = (2 * x2 * x3) % p;
        const r_b2sq_0 = (x4 * x4 + 3 * x5 * x5) % p;
        const r_b2sq_1 = (2 * x4 * x5) % p;
        const r_b1b2_0 = (x2 * x4 + 3 * x3 * x5) % p;
        const r_b1b2_1 = (x2 * x5 + x3 * x4) % p;
        const r_b0b1_0 = (x0 * x2 + 3 * x1 * x3) % p;
        const r_b0b1_1 = (x0 * x3 + x1 * x2) % p;
        const r_b0b2_0 = (x0 * x4 + 3 * x1 * x5) % p;
        const r_b0b2_1 = (x0 * x5 + x1 * x4) % p;

        // nr(x) = (x0 + 3*x1, x0 + x1) on canonical inputs above.
        const nr_b1b2_0 = r_b1b2_0 + 3 * r_b1b2_1;
        const nr_b1b2_1 = r_b1b2_0 + r_b1b2_1;
        const nr_b2sq_0 = r_b2sq_0 + 3 * r_b2sq_1;
        const nr_b2sq_1 = r_b2sq_0 + r_b2sq_1;

        // cap_a = b0sq - nr(b1b2), cap_b = nr(b2sq) - b0b1, cap_c = b1sq - b0b2
        // (offset by +4p: r_* < p, nr_* < 4p, so a difference of two such terms
        // needs at least +4p to stay non-negative before the final % p).
        const cap_a_0 = (r_b0sq_0 + 4 * p - nr_b1b2_0) % p;
        const cap_a_1 = (r_b0sq_1 + 4 * p - nr_b1b2_1) % p;
        const cap_b_0 = (nr_b2sq_0 + 4 * p - r_b0b1_0) % p;
        const cap_b_1 = (nr_b2sq_1 + 4 * p - r_b0b1_1) % p;
        const cap_c_0 = (r_b1sq_0 + 4 * p - r_b0b2_0) % p;
        const cap_c_1 = (r_b1sq_1 + 4 * p - r_b0b2_1) % p;

        // d = b0*cap_a + nr(b2*cap_b + b1*cap_c), all operands canonical (< p).
        const rb0ca_0 = (x0 * cap_a_0 + 3 * x1 * cap_a_1) % p;
        const rb0ca_1 = (x0 * cap_a_1 + x1 * cap_a_0) % p;
        const rb2cb_0 = (x4 * cap_b_0 + 3 * x5 * cap_b_1) % p;
        const rb2cb_1 = (x4 * cap_b_1 + x5 * cap_b_0) % p;
        const rb1cc_0 = (x2 * cap_c_0 + 3 * x3 * cap_c_1) % p;
        const rb1cc_1 = (x2 * cap_c_1 + x3 * cap_c_0) % p;

        // b2cb + b1cc, folded mod p before mulByNonResidue.
        const sum_0 = (rb2cb_0 + rb1cc_0) % p;
        const sum_1 = (rb2cb_1 + rb1cc_1) % p;
        const nr_sum_0 = sum_0 + 3 * sum_1;
        const nr_sum_1 = sum_0 + sum_1;

        // d, reduced once (needed twice below: once for the norm, once as
        // the a0/a1 operands of the final E2 inversion formula).
        const d0 = (rb0ca_0 + nr_sum_0) % p;
        const d1 = (rb0ca_1 + nr_sum_1) % p;

        // E2 inverse of d, flattened: norm = d0^2 - 3*d1^2 (mod p), then
        // Fermat-invert the norm (base.Element.inverse — a fixed-cost 48
        // squarings + 6 multiplies chain; not reducible by flattening), then
        // d_inv = (d0*norm_inv, -d1*norm_inv). d0,d1 < p so d0^2 < p^2 and
        // 3*d1^2 < 3p^2; offsetting by +3p^2 keeps the subtraction in range.
        const norm = base.Element.init(d0 * d0 + 3 * p * p - 3 * d1 * d1);
        const norm_inv = norm.inverse();
        const di0 = (d0 * norm_inv.value) % p;
        const di1 = ((p - d1) * norm_inv.value) % p;

        // Final scale: cap_{a,b,c} * d_inv, reduced immediately.
        const out_a_0 = (cap_a_0 * di0 + 3 * cap_a_1 * di1) % p;
        const out_a_1 = (cap_a_0 * di1 + cap_a_1 * di0) % p;
        const out_b_0 = (cap_b_0 * di0 + 3 * cap_b_1 * di1) % p;
        const out_b_1 = (cap_b_0 * di1 + cap_b_1 * di0) % p;
        const out_c_0 = (cap_c_0 * di0 + 3 * cap_c_1 * di1) % p;
        const out_c_1 = (cap_c_0 * di1 + cap_c_1 * di0) % p;

        return .{
            .B0 = .{ .a0 = base.Element.init(out_a_0), .a1 = base.Element.init(out_a_1) },
            .B1 = .{ .a0 = base.Element.init(out_b_0), .a1 = base.Element.init(out_b_1) },
            .B2 = .{ .a0 = base.Element.init(out_c_0), .a1 = base.Element.init(out_c_1) },
        };
    }

    pub fn div(self: Ext, rhs: Ext) Ext {
        return self.mul(rhs.inverse());
    }

    // exponent is u64: callers only raise to domain sizes / positions, bounded
    // by the base field (max meaningful exponent p - 1, ~31 bits). x^e depends
    // only on e mod (p^6 - 1), so u64 covers every exponent that occurs.
    pub fn pow(self: Ext, exponent: u64) Ext {
        var result = Ext.one();
        var b = self;
        var exp = exponent;
        while (exp != 0) : (exp >>= 1) {
            if ((exp & 1) == 1) result = result.mul(b);
            b = b.square();
        }
        return result;
    }

    pub fn powComptime(self: Ext, comptime exponent: usize) Ext {
        var result = Ext.one();
        var b = self;
        comptime var exp = exponent;
        inline while (exp != 0) : (exp >>= 1) {
            if ((exp & 1) == 1) result = result.mul(b);
            b = b.square();
        }
        return result;
    }

    pub fn toBytes(self: Ext) [bytes]u8 {
        var out: [bytes]u8 = undefined;
        const limbs = [_]base.Element{ self.B0.a0, self.B0.a1, self.B1.a0, self.B1.a1, self.B2.a0, self.B2.a1 };
        for (limbs, 0..) |limb, i| {
            const encoded = limb.toBytes();
            @memcpy(out[i * base.bytes .. (i + 1) * base.bytes], &encoded);
        }
        return out;
    }

    pub fn fromBytesCanonical(encoded: [bytes]u8) base.Error!Ext {
        return .{
            .B0 = .{
                .a0 = try base.Element.fromBytesCanonical(encoded[0..4].*),
                .a1 = try base.Element.fromBytesCanonical(encoded[4..8].*),
            },
            .B1 = .{
                .a0 = try base.Element.fromBytesCanonical(encoded[8..12].*),
                .a1 = try base.Element.fromBytesCanonical(encoded[12..16].*),
            },
            .B2 = .{
                .a0 = try base.Element.fromBytesCanonical(encoded[16..20].*),
                .a1 = try base.Element.fromBytesCanonical(encoded[20..24].*),
            },
        };
    }

    /// Flattening order: [B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1].
    pub fn fromUints(v: [6]u32) Ext {
        return .{
            .B0 = .{ .a0 = base.Element.init(v[0]), .a1 = base.Element.init(v[1]) },
            .B1 = .{ .a0 = base.Element.init(v[2]), .a1 = base.Element.init(v[3]) },
            .B2 = .{ .a0 = base.Element.init(v[4]), .a1 = base.Element.init(v[5]) },
        };
    }

    /// Flattening order: [B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1].
    pub fn toUints(self: Ext) [6]u32 {
        return .{
            self.B0.a0.value,
            self.B0.a1.value,
            self.B1.a0.value,
            self.B1.a1.value,
            self.B2.a0.value,
            self.B2.a1.value,
        };
    }
};
