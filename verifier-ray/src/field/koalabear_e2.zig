const base = @import("koalabear.zig");

/// Quadratic extension F_{p^2} = F_p[u]/(u^2 - 3).
///
/// This is an `extern struct` so nested extension-field values keep declaration
/// order when they are part of byte-cast verifier inputs.
pub const E2 = extern struct {
    a0: base.Element,
    a1: base.Element,

    pub fn zero() E2 {
        return .{ .a0 = base.Element.zero(), .a1 = base.Element.zero() };
    }

    pub fn isZero(self: E2) bool {
        return self.a0.isZero() and self.a1.isZero();
    }

    pub fn eql(self: E2, rhs: E2) bool {
        return self.a0.eql(rhs.a0) and self.a1.eql(rhs.a1);
    }

    pub fn neg(self: E2) E2 {
        return .{ .a0 = self.a0.neg(), .a1 = self.a1.neg() };
    }

    pub fn add(self: E2, rhs: E2) E2 {
        return .{ .a0 = self.a0.add(rhs.a0), .a1 = self.a1.add(rhs.a1) };
    }

    pub fn sub(self: E2, rhs: E2) E2 {
        return .{ .a0 = self.a0.sub(rhs.a0), .a1 = self.a1.sub(rhs.a1) };
    }

    pub fn mul(self: E2, rhs: E2) E2 {
        const c0 = self.a0.mul(rhs.a0).add(self.a1.mul(rhs.a1).mul(base.Element.init(3)));
        const c1 = self.a0.mul(rhs.a1).add(self.a1.mul(rhs.a0));
        return .{ .a0 = c0, .a1 = c1 };
    }

    pub fn mulByBase(self: E2, rhs: base.Element) E2 {
        return .{ .a0 = self.a0.mul(rhs), .a1 = self.a1.mul(rhs) };
    }

    /// Multiply by the non-residue (u+1) of the cubic extension: result = (a0 + 3*a1) + (a0 + a1)*u
    pub fn mulByNonResidue(self: E2) E2 {
        const three = base.Element.init(3);
        return .{
            .a0 = self.a0.add(self.a1.mul(three)),
            .a1 = self.a0.add(self.a1),
        };
    }

    /// Invert in F_p[u]/(u^2 - 3): norm = a0^2 - 3*a1^2
    pub fn inverse(self: E2) E2 {
        const three = base.Element.init(3);
        const norm = self.a0.mul(self.a0).sub(self.a1.mul(self.a1).mul(three));
        const norm_inv = norm.inverse();
        return .{
            .a0 = self.a0.mul(norm_inv),
            .a1 = self.a1.neg().mul(norm_inv),
        };
    }
};
