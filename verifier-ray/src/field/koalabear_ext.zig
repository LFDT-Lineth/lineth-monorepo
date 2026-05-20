const base = @import("koalabear.zig");

pub const degree = 6;
pub const bytes = degree * base.bytes;

pub const E2 = @import("koalabear_e2.zig").E2;

/// Cubic extension F_{p^6} = F_{p^2}[v]/(v^3 - (u+1)).
pub const Ext = struct {
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
        return .{ .B0 = self.B0.mulByBase(rhs), .B1 = self.B1.mulByBase(rhs), .B2 = self.B2.mulByBase(rhs) };
    }

    pub fn divByBase(self: Ext, rhs: base.Element) Ext {
        return self.mulByBase(rhs.inverse());
    }

    pub fn mul(self: Ext, rhs: Ext) Ext {
        // F_{p^6} = F_{p^2}[v]/(v^3 - (u+1)), so v^3 = u+1
        // D0 = A0*B0 + (A1*B2 + A2*B1)*(u+1)
        // D1 = A0*B1 + A1*B0 + A2*B2*(u+1)
        // D2 = A0*B2 + A2*B0 + A1*B1
        const d0 = self.B0.mul(rhs.B0).add(self.B1.mul(rhs.B2).add(self.B2.mul(rhs.B1)).mulByNonResidue());
        const d1 = self.B0.mul(rhs.B1).add(self.B1.mul(rhs.B0)).add(self.B2.mul(rhs.B2).mulByNonResidue());
        const d2 = self.B0.mul(rhs.B2).add(self.B2.mul(rhs.B0)).add(self.B1.mul(rhs.B1));
        return .{ .B0 = d0, .B1 = d1, .B2 = d2 };
    }

    pub fn square(self: Ext) Ext {
        return self.mul(self);
    }

    pub fn inverse(self: Ext) Ext {
        if (self.isZero()) unreachable;

        // Adjugate elements for the cubic extension inverse:
        //   A = b0^2 - (u+1)*b1*b2
        //   B = (u+1)*b2^2 - b0*b1
        //   C = b1^2 - b0*b2
        const cap_a = self.B0.mul(self.B0).sub(self.B1.mul(self.B2).mulByNonResidue());
        const cap_b = self.B2.mul(self.B2).mulByNonResidue().sub(self.B0.mul(self.B1));
        const cap_c = self.B1.mul(self.B1).sub(self.B0.mul(self.B2));

        // Norm: d = b0*A + (u+1)*(b2*B + b1*C)
        const d = self.B0.mul(cap_a).add(self.B2.mul(cap_b).add(self.B1.mul(cap_c)).mulByNonResidue());
        const d_inv = d.inverse();

        return .{ .B0 = cap_a.mul(d_inv), .B1 = cap_b.mul(d_inv), .B2 = cap_c.mul(d_inv) };
    }

    pub fn div(self: Ext, rhs: Ext) Ext {
        return self.mul(rhs.inverse());
    }

    pub fn pow(self: Ext, exponent: u256) Ext {
        var result = Ext.one();
        var b = self;
        var exp = exponent;
        while (exp != 0) : (exp >>= 1) {
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

    pub fn fromUints(v1: u32, v2: u32, v3: u32, v4: u32, v5: u32, v6: u32) Ext {
        return .{
            .B0 = .{ .a0 = base.Element.init(v1), .a1 = base.Element.init(v2) },
            .B1 = .{ .a0 = base.Element.init(v3), .a1 = base.Element.init(v4) },
            .B2 = .{ .a0 = base.Element.init(v5), .a1 = base.Element.init(v6) },
        };
    }
};
