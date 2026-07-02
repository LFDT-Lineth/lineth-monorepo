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
};
