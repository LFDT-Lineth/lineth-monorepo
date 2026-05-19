const base = @import("koalabear.zig");

pub const degree = 4;
pub const bytes = degree * base.bytes;

pub const Ext = struct {
    /// Limb order matches prover-ray and gnark-crypto:
    /// B0.A0, B0.A1, B1.A0, B1.A1 for B0 + B1*v with v^2 = u and u^2 = 3.
    limbs: [degree]base.Element,

    pub fn zero() Ext {
        return .{ .limbs = .{
            base.Element.zero(),
            base.Element.zero(),
            base.Element.zero(),
            base.Element.zero(),
        } };
    }

    pub fn one() Ext {
        return lift(base.Element.one());
    }

    pub fn lift(value: base.Element) Ext {
        return .{ .limbs = .{
            value,
            base.Element.zero(),
            base.Element.zero(),
            base.Element.zero(),
        } };
    }

    pub fn isZero(self: Ext) bool {
        for (self.limbs) |limb| {
            if (!limb.isZero()) return false;
        }
        return true;
    }

    pub fn isBase(self: Ext) bool {
        return self.limbs[1].isZero() and self.limbs[2].isZero() and self.limbs[3].isZero();
    }

    pub fn eql(self: Ext, rhs: Ext) bool {
        for (self.limbs, rhs.limbs) |lhs_limb, rhs_limb| {
            if (!lhs_limb.eql(rhs_limb)) return false;
        }
        return true;
    }

    pub fn add(self: Ext, rhs: Ext) Ext {
        var out = Ext.zero();
        for (&out.limbs, self.limbs, rhs.limbs) |*dst, lhs_limb, rhs_limb| {
            dst.* = lhs_limb.add(rhs_limb);
        }
        return out;
    }

    pub fn sub(self: Ext, rhs: Ext) Ext {
        var out = Ext.zero();
        for (&out.limbs, self.limbs, rhs.limbs) |*dst, lhs_limb, rhs_limb| {
            dst.* = lhs_limb.sub(rhs_limb);
        }
        return out;
    }

    pub fn neg(self: Ext) Ext {
        var out = Ext.zero();
        for (&out.limbs, self.limbs) |*dst, limb| {
            dst.* = limb.neg();
        }
        return out;
    }

    pub fn mulByBase(self: Ext, rhs: base.Element) Ext {
        var out = Ext.zero();
        for (&out.limbs, self.limbs) |*dst, limb| {
            dst.* = limb.mul(rhs);
        }
        return out;
    }

    pub fn divByBase(self: Ext, rhs: base.Element) Ext {
        return self.mulByBase(rhs.inverse());
    }

    pub fn mul(self: Ext, rhs: Ext) Ext {
        const a0 = E2{ .a0 = self.limbs[0], .a1 = self.limbs[1] };
        const a1 = E2{ .a0 = self.limbs[2], .a1 = self.limbs[3] };
        const b0 = E2{ .a0 = rhs.limbs[0], .a1 = rhs.limbs[1] };
        const b1 = E2{ .a0 = rhs.limbs[2], .a1 = rhs.limbs[3] };

        const a0b0 = a0.mul(b0);
        const a1b1 = a1.mul(b1);
        const c0 = a0b0.add(a1b1.mulByU());
        const c1 = a0.mul(b1).add(a1.mul(b0));

        return .{ .limbs = .{ c0.a0, c0.a1, c1.a0, c1.a1 } };
    }

    pub fn square(self: Ext) Ext {
        return self.mul(self);
    }

    pub fn pow(self: Ext, exponent: u128) Ext {
        var result = Ext.one();
        var power = self;
        var exp = exponent;
        while (exp != 0) : (exp >>= 1) {
            if ((exp & 1) == 1) {
                result = result.mul(power);
            }
            power = power.square();
        }
        return result;
    }

    pub fn inverse(self: Ext) Ext {
        if (self.isZero()) unreachable;
        return self.pow(field_order_four_minus_two);
    }

    pub fn div(self: Ext, rhs: Ext) Ext {
        return self.mul(rhs.inverse());
    }

    pub fn toBytes(self: Ext) [bytes]u8 {
        var out: [bytes]u8 = undefined;
        for (self.limbs, 0..) |limb, i| {
            const encoded = limb.toBytes();
            @memcpy(out[i * base.bytes .. (i + 1) * base.bytes], &encoded);
        }
        return out;
    }

    pub fn fromBytesCanonical(encoded: [bytes]u8) base.Error!Ext {
        return .{ .limbs = .{
            try base.Element.fromBytesCanonical(encoded[0..4].*),
            try base.Element.fromBytesCanonical(encoded[4..8].*),
            try base.Element.fromBytesCanonical(encoded[8..12].*),
            try base.Element.fromBytesCanonical(encoded[12..16].*),
        } };
    }
};

const field_order = @as(u128, base.modulus);
const field_order_four_minus_two = field_order * field_order * field_order * field_order - 2;

const E2 = struct {
    a0: base.Element,
    a1: base.Element,

    fn add(self: E2, rhs: E2) E2 {
        return .{ .a0 = self.a0.add(rhs.a0), .a1 = self.a1.add(rhs.a1) };
    }

    fn mul(self: E2, rhs: E2) E2 {
        const c0 = self.a0.mul(rhs.a0).add(self.a1.mul(rhs.a1).mul(base.Element.init(3)));
        const c1 = self.a0.mul(rhs.a1).add(self.a1.mul(rhs.a0));
        return .{ .a0 = c0, .a1 = c1 };
    }

    fn mulByU(self: E2) E2 {
        return .{ .a0 = self.a1.mul(base.Element.init(3)), .a1 = self.a0 };
    }
};

pub fn zero() Ext {
    return Ext.zero();
}

pub fn one() Ext {
    return Ext.one();
}
