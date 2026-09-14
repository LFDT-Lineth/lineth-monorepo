const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

test "koalabear element reduces and adds modulo the field" {
    const a = field.Element.init(field.modulus - 1);
    const b = field.Element.init(2);
    try std.testing.expect(a.add(b).eql(field.Element.one()));
}

test "koalabear element multiplication uses the field modulus" {
    const a = field.Element.init(field.modulus - 1);
    try std.testing.expect(a.mul(a).eql(field.Element.one()));
}

test "extension pow: lift(b)^(p-1) == 1 for non-zero base element" {
    // Ext.pow takes a u64 exponent (max meaningful exponent is p - 1). A base
    // element lifted into the extension lies in the prime subfield, so by
    // Fermat's little theorem it satisfies b^(p-1) == 1 there. p - 1 fits in u64.
    const exponent: u64 = field.modulus - 1;
    const b = ext.Ext.lift(field.Element.init(7));
    try std.testing.expect(b.pow(exponent).eql(ext.Ext.one()));
}

test "extension lift stores base element in the first limb" {
    const lifted = ext.Ext.lift(field.Element.init(17));
    try std.testing.expect(lifted.B0.a0.eql(field.Element.init(17)));
    try std.testing.expect(lifted.B0.a1.isZero());
    try std.testing.expect(lifted.B1.isZero());
    try std.testing.expect(lifted.B2.isZero());
}

test "koalabear element halve matches multiplying by the modular inverse of two" {
    const two_inv = field.Element.init(2).inverse();
    const even = field.Element.init(4);
    const odd = field.Element.init(5);
    try std.testing.expect(even.halve().eql(even.mul(two_inv)));
    try std.testing.expect(odd.halve().eql(odd.mul(two_inv)));
}

test "koalabear element halve is the inverse of double" {
    const values = [_]field.Element{
        field.Element.zero(),
        field.Element.one(),
        field.Element.init(field.modulus - 1),
        field.Element.init(field.modulus - 2),
    };
    for (values) |v| {
        try std.testing.expect(v.halve().double().eql(v));
    }
}

test "extension halve matches dividing by the base field's two" {
    const e = ext.Ext.fromUints(.{ 1, 2, 3, 4, 5, 6 });
    try std.testing.expect(e.halve().eql(e.divByBase(field.Element.init(2))));
}
