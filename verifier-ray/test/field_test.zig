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

test "extension pow: a^(p^6-1) == 1 for non-zero element" {
    const p: u256 = field.modulus;
    const field_order = p * p * p * p * p * p - 1;
    const a = ext.Ext.fromUints(1, 2, 3, 4, 5, 6);
    try std.testing.expect(a.pow(field_order).eql(ext.Ext.one()));
}

test "extension lift stores base element in the first limb" {
    const lifted = ext.Ext.lift(field.Element.init(17));
    try std.testing.expect(lifted.B0.a0.eql(field.Element.init(17)));
    try std.testing.expect(lifted.B0.a1.isZero());
    try std.testing.expect(lifted.B1.isZero());
    try std.testing.expect(lifted.B2.isZero());
}
